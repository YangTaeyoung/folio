package server

import (
	"context"
	"errors"
	"github.com/YangTaeyoung/folio/internal/dns"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Domain struct {
	ID         string       `json:"id"`
	Hostname   string       `json:"hostname"`
	ArtifactID string       `json:"artifactId"`
	Provider   string       `json:"provider"`
	Status     string       `json:"status"`
	Records    []dns.Record `json:"records"`
	LastError  string       `json:"lastError"`
	VerifiedAt *time.Time   `json:"verifiedAt"`
}
type domainRecord struct {
	Domain
	Challenge, Target, ZoneID, RecordID, TXTRecordID string
	TunnelRoute                                      bool
	BrowserDNS                                       bool
}
type dnsManager interface {
	Close() error
	StartLogin(context.Context) (dns.LoginStatus, error)
	Login(string) (dns.LoginStatus, error)
	Tunnel() (dns.TunnelStatus, error)
	ConfigureTunnel(context.Context, string, []string) (dns.TunnelStatus, error)
	StartTunnel(context.Context) (dns.TunnelStatus, error)
	StopTunnel(context.Context) (dns.TunnelStatus, error)
	RouteDNS(context.Context, string) error
}

func (s *Server) manager() (dnsManager, error) {
	m, ok := s.dns.(dnsManager)
	if !ok {
		return nil, fail(503, "dns_unavailable", "DNS login integration is unavailable")
	}
	return m, nil
}
func (s *Server) cloudflare() (*dns.Cloudflare, error) {
	var token string
	if e := s.decrypt(s.st.DNSSecret, &token); e != nil {
		return nil, e
	}
	if token == "" {
		return nil, fail(409, "dns_not_connected", "Connect a Cloudflare API token first")
	}
	return &dns.Cloudflare{APIToken: token, APIBase: s.cfg.CloudflareAPIBase, HTTPClient: s.cfg.HTTPClient}, nil
}
func dnsFailure(e error) error {
	if errors.Is(e, dns.ErrBrowserDNSUnavailable) {
		return fail(409, "browser_dns_unavailable", "Cloudflare browser login did not authorize ownership DNS changes; configure the displayed TXT manually or connect a Cloudflare API token")
	}
	if errors.Is(e, dns.ErrCollision) {
		return fail(409, "dns_collision", "DNS record already exists; Folio will not overwrite it")
	}
	if errors.Is(e, dns.ErrUnsupported) {
		return fail(409, "provider_unsupported", "Configure DNS manually; this provider/template is unsupported")
	}
	return fail(502, "dns_failed", "DNS operation failed; verify provider credentials, DNS records and ingress configuration")
}
func (s *Server) verifyDomain(ctx context.Context, p principal, d domainRecord) (domainRecord, error) {
	if _, exists := s.st.Artifacts[d.ArtifactID]; !exists {
		return d, fail(409, "artifact_deleted", "The bound artifact no longer exists")
	}
	cf, _ := s.cloudflare()
	certificate, _ := s.dns.(browserDNS)
	s.mu.Unlock()
	ok, e := dns.VerifyWithResolver(ctx, s.cfg.DNSResolver, d.Hostname, d.Challenge, d.Target)

	// Proxied DNS can intentionally hide its CNAME. Public TXT ownership plus
	// this installation's exact-host HTTPS challenge proves the actual ingress
	// for manual DNS as well as API/browser-managed records. Never follow redirects.
	if !ok || d.TunnelRoute {
		owned, ownershipErr := dns.VerifyOwnershipWithResolver(ctx, s.cfg.DNSResolver, d.Hostname, d.Challenge)
		ok = false
		e = ownershipErr
		if e == nil && owned {
			if s.originProof(ctx, d) {
				ok = true
			} else if d.TunnelRoute {
				if d.BrowserDNS && certificate != nil {
					_, routeErr := certificate.VerifyRoute(ctx, d.Hostname, d.Target)
					if routeErr != nil && !errors.Is(routeErr, dns.ErrBrowserDNSUnavailable) {
						e = routeErr
					}
				}
			} else if d.Provider == "cloudflare" && cf != nil && d.ZoneID != "" {
				ok, e = cf.VerifyRoute(ctx, d.ZoneID, d.Hostname, d.Target)
			}
		}
	}

	s.mu.Lock()
	current, exists := s.st.Domains[d.ID]
	if !exists || current.Hostname != d.Hostname || current.Challenge != d.Challenge || current.Target != d.Target || current.TunnelRoute != d.TunnelRoute || current.BrowserDNS != d.BrowserDNS {
		return d, fail(409, "domain_changed", "Domain changed during verification; retry")
	}
	if p.user != nil {
		u, ok := s.st.Users[p.user.ID]
		se, sessionOK := s.st.Sessions[p.credential]
		if !ok || u.Disabled || u.Role != "admin" || !sessionOK || se.UserID != u.ID || !se.ExpiresAt.After(now()) {
			return d, fail(401, "unauthorized", "Administrator session expired during verification")
		}
		p.user = &u.User
	}
	d.Status = "pending"
	d.VerifiedAt = nil
	d.LastError = ""
	if e != nil || !ok {
		d.LastError = "Ownership TXT and routing records have not both been verified"
		if e != nil {
			d.Status = "error"
		}
	} else {
		n := now()
		d.Status = "verified"
		d.VerifiedAt = &n
	}
	err := s.mutate(func() error { s.st.Domains[d.ID] = d; s.audit(p, "domain.verified", d.ID, d.Status); return nil })
	return d, err
}
func (s *Server) domains(w http.ResponseWriter, r *http.Request, p principal) error {
	if e := s.require(r, p, true, ""); e != nil {
		return e
	}
	path := r.URL.Path
	if path == "/api/admin/dns/providers" && r.Method == "GET" {
		_, tokenErr := s.cloudflare()
		writeJSON(w, 200, map[string]any{"providers": []map[string]any{{"id": "manual", "name": "Manual DNS", "supported": true}, {"id": "cloudflare", "name": "Cloudflare", "supported": true, "connected": tokenErr == nil, "browserLogin": true}, {"id": "domainconnect", "name": "Domain Connect", "supported": s.cfg.DomainConnect != nil && s.cfg.DomainConnect.ProviderID != "" && len(s.cfg.DomainConnect.AllowedProviderHosts) > 0, "instructions": "A registered provider template and allowed provider host must be configured; otherwise use manual DNS"}}})
		return nil
	}
	if path == "/api/admin/dns/cloudflare/connect" && r.Method == "POST" {
		var in struct{ APIToken string }
		if e := decode(r, &in); e != nil {
			return e
		}
		var cf *dns.Cloudflare
		var e error
		if in.APIToken == "" {
			cf, e = s.cloudflare()
			if e != nil {
				return e
			}
		} else {
			cf = &dns.Cloudflare{APIToken: in.APIToken, APIBase: s.cfg.CloudflareAPIBase, HTTPClient: s.cfg.HTTPClient}
		}
		zones, e := cf.Zones(r.Context())
		if e != nil {
			return dnsFailure(e)
		}
		if in.APIToken != "" {
			secret, e := s.encrypt(in.APIToken)
			if e != nil {
				return e
			}
			if e = s.mutate(func() error { s.st.DNSSecret = secret; s.audit(p, "dns.cloudflare_connected", "", ""); return nil }); e != nil {
				return e
			}
		}
		writeJSON(w, 200, map[string]any{"connected": true, "zones": zones})
		return nil
	}
	if strings.HasPrefix(path, "/api/admin/dns/cloudflare/login") {
		m, e := s.manager()
		if e != nil {
			return e
		}
		if path == "/api/admin/dns/cloudflare/login" && r.Method == "POST" {
			out, e := m.StartLogin(context.WithoutCancel(r.Context()))
			if e != nil {
				return dnsFailure(e)
			}
			s.audit(p, "dns.login_started", out.ID, "")
			if e = s.save(); e != nil {
				return e
			}
			writeJSON(w, 200, out)
			return nil
		}
		if r.Method == "GET" {
			out, e := m.Login(strings.TrimPrefix(path, "/api/admin/dns/cloudflare/login/"))
			if e != nil {
				return dnsFailure(e)
			}
			writeJSON(w, 200, out)
			return nil
		}
	}
	if strings.HasPrefix(path, "/api/admin/dns/cloudflare/tunnel") {
		m, e := s.manager()
		if e != nil {
			return e
		}
		var out dns.TunnelStatus
		if path == "/api/admin/dns/cloudflare/tunnel" && r.Method == "GET" {
			out, e = m.Tunnel()
		} else if path == "/api/admin/dns/cloudflare/tunnel" && r.Method == "POST" {
			var in struct {
				Name      string
				Hostnames []string
				Create    bool
			}
			if e = decode(r, &in); e != nil {
				return e
			}
			if !in.Create {
				return fail(400, "explicit_create_required", "Explicit create:true is required for ingress setup")
			}
			for _, host := range in.Hostnames {
				allowed := strings.EqualFold(host, strings.Split(s.appHost, ":")[0])
				if !allowed {
					_, allowed = s.artifactForHost(host)
					if !allowed {
						for _, d := range s.st.Domains {
							if d.Hostname == host {
								allowed = true
								break
							}
						}
					}
				}
				if !allowed {
					return fail(400, "unbound_hostname", "Tunnel hosts must be the app, an artifact canonical host or a verified custom domain")
				}
			}
			out, e = m.ConfigureTunnel(r.Context(), in.Name, in.Hostnames)
		} else if path == "/api/admin/dns/cloudflare/tunnel/start" && r.Method == "POST" {
			out, e = m.StartTunnel(r.Context())
		} else if path == "/api/admin/dns/cloudflare/tunnel/stop" && r.Method == "POST" {
			out, e = m.StopTunnel(r.Context())
		} else {
			return fail(404, "not_found", "Not found")
		}
		if e != nil {
			return dnsFailure(e)
		}
		if r.Method == "POST" {
			s.audit(p, "dns.tunnel_changed", "", path)
			if e = s.save(); e != nil {
				return e
			}
		}
		writeJSON(w, 200, out)
		return nil
	}
	dc := s.cfg.DomainConnect
	if dc == nil {
		dc = &dns.DomainConnect{StateKey: s.key, CallbackURL: s.cfg.AppURL + "/api/dns/domainconnect/callback"}
	}
	if path == "/api/admin/dns/domainconnect/discover" && r.Method == "POST" {
		var in struct{ Domain string }
		if e := decode(r, &in); e != nil {
			return e
		}
		out, e := dc.Discover(r.Context(), in.Domain)
		if e != nil {
			return dnsFailure(e)
		}
		writeJSON(w, 200, out)
		return nil
	}
	if path == "/api/dns/domainconnect/callback" && r.Method == "GET" {
		claims, e := dc.ConsumeState(r.URL.Query().Get("state"), p.user.ID)
		if e != nil {
			return fail(400, "invalid_state", "Invalid or expired Domain Connect state")
		}
		d, ok := s.st.Domains[claims.DomainID]
		if !ok || d.Hostname != claims.Hostname {
			return fail(400, "invalid_state", "Domain Connect state does not match domain")
		}
		d, e = s.verifyDomain(r.Context(), p, d)
		if e != nil {
			return e
		}
		http.Redirect(w, r, "/?domain="+url.QueryEscape(d.ID)+"#domains", 303)
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/domains"), "/")
	if len(parts) == 1 && r.Method == "GET" {
		list := []Domain{}
		for _, d := range s.st.Domains {
			list = append(list, d.Domain)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Hostname < list[j].Hostname })
		writeJSON(w, 200, map[string]any{"domains": list})
		return nil
	}
	if len(parts) == 1 && r.Method == "POST" {
		var in struct{ Hostname, ArtifactID, Provider, Target, CloudflareZoneID string }
		if e := decode(r, &in); e != nil {
			return e
		}
		in.Hostname = strings.ToLower(strings.TrimSuffix(in.Hostname, "."))
		if e := dns.ValidateHostname(in.Hostname); e != nil {
			return fail(400, "invalid_hostname", "Enter a fully qualified DNS hostname")
		}
		a, ok := s.st.Artifacts[in.ArtifactID]
		if !ok {
			return fail(404, "not_found", "Artifact not found")
		}
		if in.Provider != "manual" && in.Provider != "cloudflare" && in.Provider != "domainconnect" {
			return fail(400, "invalid_provider", "Unknown DNS provider")
		}
		appURL, _ := url.Parse(s.cfg.AppURL)
		if strings.EqualFold(in.Hostname, appURL.Hostname()) {
			return fail(409, "hostname_reserved", "Management hostname cannot execute artifact content")
		}
		if _, ok := s.artifactForHost(in.Hostname); ok {
			return fail(409, "hostname_exists", "Hostname is already bound")
		}
		for _, d := range s.st.Domains {
			if d.Hostname == in.Hostname {
				return fail(409, "hostname_exists", "Hostname is already bound")
			}
		}
		target := in.Target
		if target == "" {
			target = s.canonical(a).Hostname()
		}
		if net.ParseIP(target) == nil {
			if e := dns.ValidateHostname(target); e != nil {
				return fail(400, "invalid_target", "Target must be an IP address or hostname")
			}
		}
		id := random()
		challenge := "folio-verification=" + random()
		d := domainRecord{Domain: Domain{ID: id, Hostname: in.Hostname, ArtifactID: in.ArtifactID, Provider: in.Provider, Status: "pending", Records: dns.DesiredRecords(in.Hostname, challenge, target)}, Challenge: challenge, Target: target, ZoneID: in.CloudflareZoneID}
		if e := s.mutate(func() error { s.st.Domains[id] = d; s.audit(p, "domain.created", id, ""); return nil }); e != nil {
			return e
		}
		writeJSON(w, 201, map[string]any{"domain": d.Domain, "ingressRequired": true})
		return nil
	}
	if len(parts) < 2 {
		return fail(404, "not_found", "Not found")
	}
	id := parts[1]
	d, ok := s.st.Domains[id]
	if !ok {
		return fail(404, "not_found", "Domain not found")
	}
	if len(parts) == 3 && parts[2] == "verify" && r.Method == "POST" {
		d, e := s.verifyDomain(r.Context(), p, d)
		if e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"domain": d.Domain})
		return nil
	}
	if len(parts) == 3 && parts[2] == "authorize" && r.Method == "POST" {
		if d.Provider != "domainconnect" {
			return fail(400, "invalid_provider", "This domain does not use Domain Connect")
		}
		out, e := dc.Authorize(r.Context(), d.Hostname, p.user.ID, d.ID, map[string]string{"target": d.Target, "challenge": d.Challenge})
		if e != nil {
			return dnsFailure(e)
		}
		s.audit(p, "domain.authorized", id, "")
		if e = s.save(); e != nil {
			return e
		}
		writeJSON(w, 200, out)
		return nil
	}
	if len(parts) == 3 && parts[2] == "provision" && r.Method == "POST" {
		if d.Provider != "cloudflare" {
			return fail(409, "manual_dns_required", "Use the displayed DNS records for this provider")
		}
		cf, e := s.cloudflare()

		if e != nil {
			return s.provisionBrowserDomain(w, r, p, d)
		}
		zone, err := selectCloudflareZone(r.Context(), cf, d.ZoneID, d.Hostname)
		if err != nil {
			return err
		}
		d.ZoneID = zone.ID

		if d.RecordID == "" {
			rid, err := cf.Provision(r.Context(), d.ZoneID, d.Hostname, d.Target)
			if err != nil {
				return dnsFailure(err)
			}
			d.RecordID = rid
			if err = s.mutate(func() error { s.st.Domains[id] = d; s.audit(p, "domain.routing_provisioned", id, ""); return nil }); err != nil {
				cf.DeleteOwned(context.WithoutCancel(r.Context()), d.ZoneID, rid, d.Hostname, d.Target)
				return err
			}
		}
		if d.TXTRecordID == "" {
			txtid, err := cf.ProvisionOwnership(r.Context(), d.ZoneID, d.Hostname, d.Challenge)
			if err != nil {
				d.LastError = "Ownership TXT provisioning failed; the routing record is retained for retry or explicit deletion"
				s.mutate(func() error { s.st.Domains[id] = d; s.audit(p, "domain.provision_failed", id, ""); return nil })
				return dnsFailure(err)
			}
			d.TXTRecordID = txtid
		}
		d.LastError = ""
		if e = s.mutate(func() error { s.st.Domains[id] = d; s.audit(p, "domain.provisioned", id, ""); return nil }); e != nil {
			return e
		}

		writeJSON(w, 200, map[string]any{"domain": d.Domain})
		return nil
	}
	if len(parts) == 2 && r.Method == "DELETE" {

		if r.URL.Query().Get("deleteDns") == "true" {
			if e := s.deleteDomainDNS(r.Context(), p, &d); e != nil {
				return e
			}
		}

		if e := s.mutate(func() error { delete(s.st.Domains, id); s.audit(p, "domain.deleted", id, ""); return nil }); e != nil {
			return e
		}
		w.WriteHeader(204)
		return nil
	}
	return fail(404, "not_found", "Not found")
}

// originProof verifies actual HTTPS ingress for proxied tunnel DNS, where CNAME
// lookup intentionally does not expose the provider target. No redirect is trusted.
func (s *Server) originProof(ctx context.Context, d domainRecord) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client := s.cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	safe := *client
	safe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, e := http.NewRequestWithContext(ctx, "GET", "https://"+d.Hostname+"/.well-known/folio-challenge/"+url.PathEscape(d.Challenge), nil)
	if e != nil {
		return false
	}
	res, e := safe.Do(req)
	if e != nil {
		return false
	}
	defer res.Body.Close()
	body, e := io.ReadAll(io.LimitReader(res.Body, 1024))
	return e == nil && res.StatusCode == 200 && string(body) == "folio-ingress:"+d.ID+":"+d.Challenge
}
