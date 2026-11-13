package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/YangTaeyoung/folio/internal/dns"
)

// Certificate DNS capabilities are optional so older DNS integrations and test
// managers can still implement login/tunnel lifecycle without exposing secrets.
type browserDNS interface {
	SelectedZone(context.Context) (dns.Zone, error)
	ProvisionOwnership(context.Context, string, string) (string, error)
	VerifyRoute(context.Context, string, string) (bool, error)
	DeleteOwned(context.Context, string, string, string) error
}

func inDNSZone(hostname, zone string) bool {
	hostname = strings.ToLower(strings.TrimSuffix(hostname, "."))
	zone = strings.ToLower(strings.TrimSuffix(zone, "."))
	return zone != "" && (hostname == zone || strings.HasSuffix(hostname, "."+zone))
}
func selectCloudflareZone(ctx context.Context, cf *dns.Cloudflare, explicit, hostname string) (dns.Zone, error) {
	zones, e := cf.Zones(ctx)
	if e != nil {
		return dns.Zone{}, dnsFailure(e)
	}
	var selected dns.Zone
	for _, zone := range zones {
		if explicit != "" {
			if zone.ID != explicit {
				continue
			}
			if !inDNSZone(hostname, zone.Name) {
				return dns.Zone{}, fail(400, "zone_mismatch", "Hostname is outside the selected Cloudflare zone")
			}
			return zone, nil
		}
		if inDNSZone(hostname, zone.Name) && (len(zone.Name) > len(selected.Name) || (len(zone.Name) == len(selected.Name) && zone.ID < selected.ID)) {
			selected = zone
		}
	}
	if selected.ID == "" {
		return dns.Zone{}, fail(400, "zone_not_found", "No accessible Cloudflare zone contains this hostname; connect a token for the correct zone or configure DNS manually")
	}
	return selected, nil
}
func (s *Server) provisionBrowserDomain(w http.ResponseWriter, r *http.Request, p principal, d domainRecord) error {
	m, e := s.manager()
	if e != nil {
		return e
	}
	cert, ok := s.dns.(browserDNS)
	if !ok {
		return fail(409, "browser_dns_unavailable", "This login integration cannot create ownership TXT records; connect a Cloudflare API token or configure DNS manually")
	}
	status, e := m.Tunnel()
	if e != nil || !status.Configured || !contains(status.Hostnames, d.Hostname) {
		return fail(409, "tunnel_not_configured", "Configure a dedicated tunnel with this exact hostname, or connect an API token")
	}
	zone, e := cert.SelectedZone(r.Context())
	if e != nil {
		return dnsFailure(e)
	}
	if !inDNSZone(d.Hostname, zone.Name) || (d.ZoneID != "" && d.ZoneID != zone.ID) {
		return fail(400, "zone_mismatch", "Hostname must belong to the browser-login selected Cloudflare zone")
	}
	d.ZoneID = zone.ID
	d.Target = status.ID + ".cfargotunnel.com"
	d.TunnelRoute = true
	d.BrowserDNS = true
	d.Status = "pending"
	d.VerifiedAt = nil
	d.Records = dns.DesiredRecords(d.Hostname, d.Challenge, d.Target)
	// Persist the exact desired route before provider work, and require real TXT
	// permission before attempting a route. A denied capability is never success.
	if e = s.mutate(func() error { s.st.Domains[d.ID] = d; s.audit(p, "domain.browser_dns_prepared", d.ID, ""); return nil }); e != nil {
		return e
	}
	if d.TXTRecordID == "" {
		recordID, err := cert.ProvisionOwnership(r.Context(), d.Hostname, d.Challenge)
		if err != nil {
			d.Status = "error"
			d.LastError = "Ownership TXT could not be created. Configure the displayed TXT manually or connect a Cloudflare API token."
			if e = s.mutate(func() error {
				s.st.Domains[d.ID] = d
				s.audit(p, "domain.provision_failed", d.ID, "ownership")
				return nil
			}); e != nil {
				return e
			}
			return dnsFailure(err)
		}
		if recordID == "" {
			return fail(502, "dns_failed", "Provider did not confirm ownership TXT creation")
		}
		d.TXTRecordID = recordID
		if e = s.mutate(func() error { s.st.Domains[d.ID] = d; s.audit(p, "domain.ownership_provisioned", d.ID, ""); return nil }); e != nil {
			return e
		}
	}
	if e = m.RouteDNS(r.Context(), d.Hostname); e != nil {
		d.Status = "error"
		d.LastError = "Routing record could not be provisioned; the Folio-owned TXT is retained for retry or explicit deletion"
		if saveErr := s.mutate(func() error {
			s.st.Domains[d.ID] = d
			s.audit(p, "domain.provision_failed", d.ID, "routing")
			return nil
		}); saveErr != nil {
			return saveErr
		}
		return dnsFailure(e)
	}
	d.LastError = ""
	if e = s.mutate(func() error {
		s.st.Domains[d.ID] = d
		s.audit(p, "domain.provisioned", d.ID, "browser certificate")
		return nil
	}); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]any{"domain": d.Domain, "instructions": "Ownership TXT and routing were created. Start the tunnel, then verify DNS and HTTPS ingress."})
	return nil
}

func (s *Server) deleteDomainDNS(ctx context.Context, p principal, d *domainRecord) error {
	cf, tokenErr := s.cloudflare()
	cert, hasCert := s.dns.(browserDNS)
	if d.RecordID != "" || d.TXTRecordID != "" {
		if tokenErr != nil && (!d.BrowserDNS || !hasCert) {
			return fail(409, "dns_not_connected", "Connect the Cloudflare credential that owns these DNS records before deleting them")
		}
		remove := func(id, name, value string) error {
			if tokenErr == nil {
				return cf.DeleteOwned(ctx, d.ZoneID, id, name, value)
			}
			return cert.DeleteOwned(ctx, id, name, value)
		}
		if d.RecordID != "" {
			if e := remove(d.RecordID, d.Hostname, d.Target); e != nil {
				return dnsFailure(e)
			}
			d.RecordID = ""
			if e := s.mutate(func() error { s.st.Domains[d.ID] = *d; s.audit(p, "domain.routing_deleted", d.ID, ""); return nil }); e != nil {
				return e
			}
		}
		if d.TXTRecordID != "" {
			if e := remove(d.TXTRecordID, "_folio."+d.Hostname, d.Challenge); e != nil {
				return dnsFailure(e)
			}
			d.TXTRecordID = ""
			if e := s.mutate(func() error { s.st.Domains[d.ID] = *d; s.audit(p, "domain.ownership_deleted", d.ID, ""); return nil }); e != nil {
				return e
			}
		}
	}
	// cloudflared's CNAME has no Folio ownership comment. Do not delete an
	// unmarked provider record or claim that explicit DNS deletion succeeded.
	if d.TunnelRoute {
		routeExists := true
		var e error
		if tokenErr == nil {
			routeExists, e = cf.VerifyRoute(ctx, d.ZoneID, d.Hostname, d.Target)
		} else if hasCert {
			routeExists, e = cert.VerifyRoute(ctx, d.Hostname, d.Target)
		}
		if e != nil || routeExists {
			d.Status = "error"
			d.VerifiedAt = nil
			d.LastError = "The cloudflared tunnel CNAME is not Folio-commented and was retained. Remove that routing record manually, then retry; detaching without DNS deletion remains available."
			if saveErr := s.mutate(func() error {
				s.st.Domains[d.ID] = *d
				s.audit(p, "domain.manual_dns_cleanup_required", d.ID, "")
				return nil
			}); saveErr != nil {
				return saveErr
			}
			return fail(409, "manual_dns_cleanup_required", d.LastError)
		}
	}
	return nil
}
