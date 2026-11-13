package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/YangTaeyoung/folio/internal/dns"
)

type dnsAPIRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Comment string `json:"comment"`
}
type cloudflareFixture struct {
	mu           sync.Mutex
	records      map[string]dnsAPIRecord
	createdZones []string
}

func (f *cloudflareFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	result := func(v any) {
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": v, "result_info": map[string]int{"total_pages": 1}})
	}
	if r.Header.Get("Authorization") != "Bearer fixture-token" {
		w.WriteHeader(403)
		return
	}
	if r.URL.Path == "/zones" {
		result([]dns.Zone{{ID: "parent", Name: "example.test"}, {ID: "nested", Name: "sub.example.test"}, {ID: "foreign", Name: "other.test"}})
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "zones" || parts[2] != "dns_records" {
		http.NotFound(w, r)
		return
	}
	if r.Method == "GET" && len(parts) == 3 {
		list := []dnsAPIRecord{}
		for _, rec := range f.records {
			if strings.EqualFold(rec.Name, r.URL.Query().Get("name")) {
				list = append(list, rec)
			}
		}
		result(list)
		return
	}
	if r.Method == "POST" {
		var rec dnsAPIRecord
		if e := json.NewDecoder(r.Body).Decode(&rec); e != nil {
			w.WriteHeader(400)
			return
		}
		rec.ID = "record-" + random()[:12]
		f.records[rec.ID] = rec
		f.createdZones = append(f.createdZones, parts[1])
		result(rec)
		return
	}
	if len(parts) == 4 && r.Method == "GET" {
		result(f.records[parts[3]])
		return
	}
	if len(parts) == 4 && r.Method == "DELETE" {
		delete(f.records, parts[3])
		result(map[string]string{"id": parts[3]})
		return
	}
	http.NotFound(w, r)
}
func connectedCloudflare(t *testing.T) (*client, *cloudflareFixture) {
	t.Helper()
	c := fresh(t)
	c.setup()
	fixture := &cloudflareFixture{records: map[string]dnsAPIRecord{}}
	api := httptest.NewServer(fixture)
	t.Cleanup(api.Close)
	c.s.cfg.CloudflareAPIBase = api.URL
	c.s.cfg.HTTPClient = api.Client()
	code, out, _ := c.call("POST", "/api/admin/dns/cloudflare/connect", map[string]any{"apiToken": "fixture-token"})
	if code != 200 {
		t.Fatal(code, out)
	}
	return c, fixture
}
func domainUIShape(c *client, hostname, zone string) string {
	c.t.Helper()
	_, out := c.upload("", "", "index.html", "site")
	artifactID := out["artifact"].(map[string]any)["id"].(string)
	payload := map[string]any{"hostname": hostname, "artifactId": artifactID, "provider": "cloudflare", "target": "origin.example.test"}
	if zone != "" {
		payload["cloudflareZoneId"] = zone
	}
	code, out, _ := c.call("POST", "/api/domains", payload)
	if code != 201 {
		c.t.Fatal(code, out)
	}
	return out["domain"].(map[string]any)["id"].(string)
}
func TestDomainUIShapeSelectsLongestCloudflareZone(t *testing.T) {
	c, f := connectedCloudflare(t)
	id := domainUIShape(c, "preview.sub.example.test", "")
	code, out, _ := c.call("POST", "/api/domains/"+id+"/provision", map[string]any{})
	if code != 200 {
		t.Fatalf("UI shape failed: %d %v", code, out)
	}
	if c.s.st.Domains[id].ZoneID != "nested" {
		t.Fatal("wrong zone", c.s.st.Domains[id].ZoneID)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.createdZones) != 2 || f.createdZones[0] != "nested" || f.createdZones[1] != "nested" {
		t.Fatal(f.createdZones)
	}
	if c.s.st.Domains[id].TXTRecordID == "" {
		t.Fatal("success without ownership record")
	}
}
func TestDomainRejectsWrongExplicitZoneAndLabelSuffix(t *testing.T) {
	for _, tc := range []struct{ host, zone string }{{"preview.sub.example.test", "foreign"}, {"preview.notexample.test", ""}} {
		t.Run(tc.host+tc.zone, func(t *testing.T) {
			c, f := connectedCloudflare(t)
			id := domainUIShape(c, tc.host, tc.zone)
			code, _, _ := c.call("POST", "/api/domains/"+id+"/provision", map[string]any{})
			if code != 400 {
				t.Fatal("wrong-zone provisioning", code)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.createdZones) != 0 {
				t.Fatal("provider DNS write before zone validation")
			}
		})
	}
}

type browserDNSFixture struct {
	zone                                    dns.Zone
	ownershipError                          error
	routeMatches                            bool
	ownershipCalls, routeCalls, deleteCalls int
	status                                  dns.TunnelStatus
}

func (f *browserDNSFixture) Close() error { return nil }
func (f *browserDNSFixture) StartLogin(context.Context) (dns.LoginStatus, error) {
	return dns.LoginStatus{}, nil
}
func (f *browserDNSFixture) Login(string) (dns.LoginStatus, error) { return dns.LoginStatus{}, nil }
func (f *browserDNSFixture) Tunnel() (dns.TunnelStatus, error)     { return f.status, nil }
func (f *browserDNSFixture) ConfigureTunnel(context.Context, string, []string) (dns.TunnelStatus, error) {
	return f.status, nil
}
func (f *browserDNSFixture) StartTunnel(context.Context) (dns.TunnelStatus, error) {
	return f.status, nil
}
func (f *browserDNSFixture) StopTunnel(context.Context) (dns.TunnelStatus, error) {
	return f.status, nil
}
func (f *browserDNSFixture) RouteDNS(context.Context, string) error         { f.routeCalls++; return nil }
func (f *browserDNSFixture) SelectedZone(context.Context) (dns.Zone, error) { return f.zone, nil }
func (f *browserDNSFixture) ProvisionOwnership(context.Context, string, string) (string, error) {
	f.ownershipCalls++
	if f.ownershipError != nil {
		return "", f.ownershipError
	}
	return "browser-txt-record", nil
}
func (f *browserDNSFixture) VerifyRoute(context.Context, string, string) (bool, error) {
	return f.routeMatches, nil
}
func (f *browserDNSFixture) DeleteOwned(context.Context, string, string, string) error {
	f.deleteCalls++
	return nil
}
func TestBrowserDNSProvisionRequiresActualTXTPermission(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "permission denied"}[denied], func(t *testing.T) {
			c := fresh(t)
			c.setup()
			f := &browserDNSFixture{zone: dns.Zone{ID: "selected", Name: "example.test"}, routeMatches: true, status: dns.TunnelStatus{Configured: true, ID: "dedicated", Hostnames: []string{"preview.example.test"}}}
			if denied {
				f.ownershipError = dns.ErrBrowserDNSUnavailable
			}
			c.s.dns = f
			id := domainUIShape(c, "preview.example.test", "")
			code, out, _ := c.call("POST", "/api/domains/"+id+"/provision", map[string]any{})
			if denied {
				if code != 409 || out["error"].(map[string]any)["code"] != "browser_dns_unavailable" {
					t.Fatal("false DNS success", code, out)
				}
				if f.routeCalls != 0 || c.s.st.Domains[id].TXTRecordID != "" {
					t.Fatal("route/write success despite missing TXT permission")
				}
			} else {
				if code != 200 || c.s.st.Domains[id].TXTRecordID != "browser-txt-record" || f.routeCalls != 1 {
					t.Fatal(code, out, f)
				}
			}
			if f.ownershipCalls != 1 {
				t.Fatal(f.ownershipCalls)
			}
		})
	}
}
func TestBrowserDNSZoneMismatchWritesNothing(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := &browserDNSFixture{zone: dns.Zone{ID: "foreign", Name: "outside.test"}, status: dns.TunnelStatus{Configured: true, ID: "dedicated", Hostnames: []string{"preview.example.test"}}}
	c.s.dns = f
	id := domainUIShape(c, "preview.example.test", "")
	code, _, _ := c.call("POST", "/api/domains/"+id+"/provision", map[string]any{})
	if code != 400 || f.routeCalls != 0 || f.ownershipCalls != 0 {
		t.Fatal(code, f)
	}
}

// Keep the sentinel contract explicit: a network failure is not permission denial.
func TestDNSFailureDistinguishesBrowserPermission(t *testing.T) {
	permission := dnsFailure(dns.ErrBrowserDNSUnavailable)
	var api *apiError
	if !errors.As(permission, &api) || api.status != 409 {
		t.Fatal(permission)
	}
	network := dnsFailure(context.DeadlineExceeded)
	if !errors.As(network, &api) || api.status != 502 {
		t.Fatal(network)
	}
}

func TestBrowserDNSVerificationAndExplicitCleanupStayTruthful(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := &browserDNSFixture{zone: dns.Zone{ID: "selected", Name: "example.test"}, status: dns.TunnelStatus{Configured: true, ID: "dedicated", Hostnames: []string{"preview.example.test"}}}
	c.s.dns = f
	id := domainUIShape(c, "preview.example.test", "")
	code, out, _ := c.call("POST", "/api/domains/"+id+"/provision", map[string]any{})
	if code != 200 {
		t.Fatal(code, out)
	}
	d := c.s.st.Domains[id]
	c.s.cfg.DNSResolver = &fakeResolver{txt: map[string][]string{"_folio.preview.example.test": {d.Challenge}}, cname: map[string]string{}}
	c.s.cfg.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		if f.routeMatches {
			c.s.ContentHandler().ServeHTTP(w, r)
		} else {
			w.WriteString("wrong origin")
		}
		return w.Result(), nil
	})}
	code, out, _ = c.call("POST", "/api/domains/"+id+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] == "verified" {
		t.Fatal("provider mismatch ignored", code, out)
	}
	f.routeMatches = true
	code, out, _ = c.call("POST", "/api/domains/"+id+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] != "verified" {
		t.Fatal("confirmed route not verified", code, out)
	}
	code, out, _ = c.call("DELETE", "/api/domains/"+id+"?deleteDns=true", nil)
	if code != 409 || out["error"].(map[string]any)["code"] != "manual_dns_cleanup_required" || f.deleteCalls != 1 || c.s.st.Domains[id].TXTRecordID != "" {
		t.Fatal("claimed unmarked CNAME deletion", code, out, f)
	}
	f.routeMatches = false
	code, out, _ = c.call("DELETE", "/api/domains/"+id+"?deleteDns=true", nil)
	if code != 204 {
		t.Fatal("could not detach after manual route removal", code, out)
	}
}

func TestManualProxiedDomainNeedsPublicTXTAndExactHTTPSProof(t *testing.T) {
	c := fresh(t)
	c.setup()
	_, out := c.upload("", "", "index.html", "private")
	artifactID := out["artifact"].(map[string]any)["id"].(string)
	code, out, _ := c.call("POST", "/api/domains", map[string]any{"hostname": "manual.example.test", "artifactId": artifactID, "provider": "manual", "target": "owned.cfargotunnel.com"})
	if code != 201 {
		t.Fatal(code, out)
	}
	id := out["domain"].(map[string]any)["id"].(string)
	d := c.s.st.Domains[id]
	resolver := &fakeResolver{txt: map[string][]string{}, cname: map[string]string{"manual.example.test": "manual.example.test"}}
	c.s.cfg.DNSResolver = resolver
	correctBody := true
	proofCalls := 0
	c.s.cfg.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		proofCalls++
		if r.URL.Scheme != "https" || r.URL.Host != "manual.example.test" || r.URL.Path != "/.well-known/folio-challenge/"+d.Challenge {
			t.Fatal("untrusted proof request", r.URL)
		}
		w := httptest.NewRecorder()
		if correctBody {
			c.s.ContentHandler().ServeHTTP(w, r)
		} else {
			w.WriteString("some other origin")
		}
		return w.Result(), nil
	})}
	code, out, _ = c.call("POST", "/api/domains/"+id+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] == "verified" || proofCalls != 0 {
		t.Fatal("accepted HTTPS without ownership TXT", code, out, proofCalls)
	}
	resolver.txt["_folio.manual.example.test"] = []string{d.Challenge}
	correctBody = false
	code, out, _ = c.call("POST", "/api/domains/"+id+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] == "verified" {
		t.Fatal("accepted wrong HTTPS origin body", code, out)
	}
	correctBody = true
	code, out, _ = c.call("POST", "/api/domains/"+id+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] != "verified" {
		t.Fatal("valid manual proxied origin failed", code, out)
	}
}
