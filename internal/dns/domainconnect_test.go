package dns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func domainConnectFixture(t *testing.T) (*DomainConnect, *httptest.Server, *bool) {
	t.Helper()
	unsupported := false
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/example.com/settings":
			json.NewEncoder(w).Encode(map[string]string{"providerId": "dns.example", "providerName": "Test DNS", "urlSyncUX": srv.URL, "urlAPI": srv.URL})
		case "/v2/domainTemplates/providers/folio.example/services/publish":
			if unsupported {
				w.WriteHeader(404)
			} else {
				w.WriteHeader(200)
			}
		default:
			t.Error("unexpected path", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	u, _ := url.Parse(srv.URL)
	d := &DomainConnect{Resolver: fakeResolver{txt: map[string][]string{"_domainconnect.example.com": {u.Host}}}, HTTPClient: srv.Client(), ProviderID: "folio.example", ServiceID: "publish", CallbackURL: "https://folio.example/api/dns/domainconnect/callback", AllowedProviderHosts: []string{u.Host}, StateKey: []byte(strings.Repeat("x", 32))}
	return d, srv, &unsupported
}
func TestDomainConnectDiscoveryApplyAndSingleUseState(t *testing.T) {
	d, srv, _ := domainConnectFixture(t)
	defer srv.Close()
	ctx := context.Background()
	discovered, err := d.Discover(ctx, "demo.example.com")
	if err != nil || !discovered.Supported || discovered.Domain != "example.com" || discovered.Host != "demo" {
		t.Fatal(discovered, err)
	}
	auth, err := d.Authorize(ctx, "demo.example.com", "admin", "domain-1", map[string]string{"target": "origin.example.com", "challenge": "ownership"})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(auth.AuthorizationURL)
	q := u.Query()
	if u.Path != "/v2/domainTemplates/providers/folio.example/services/publish/apply" || q.Get("host") != "demo" || q.Get("domain") != "example.com" || q.Get("redirect_uri") != d.CallbackURL {
		t.Fatal(u)
	}
	state := q.Get("state")
	if _, err = d.ConsumeState(state+"x", "admin"); err == nil {
		t.Fatal("tampering accepted")
	}
	if _, err = d.ConsumeState(state, "other-admin"); err == nil {
		t.Fatal("wrong admin accepted")
	}
	claims, err := d.ConsumeState(state, "admin")
	if err != nil || claims.DomainID != "domain-1" || claims.Hostname != "demo.example.com" {
		t.Fatal(claims, err)
	}
	if _, err = d.ConsumeState(state, "admin"); err == nil {
		t.Fatal("replay accepted")
	}
	if _, err = d.Authorize(ctx, "demo.example.com", "admin", "domain-1", map[string]string{"redirect_uri": "https://evil.example"}); err == nil {
		t.Fatal("callback overwritten")
	}
	d.StateTTL = time.Millisecond
	auth, err = d.Authorize(ctx, "demo.example.com", "admin", "domain-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ = url.Parse(auth.AuthorizationURL)
	time.Sleep(5 * time.Millisecond)
	if _, err = d.ConsumeState(u.Query().Get("state"), "admin"); err == nil {
		t.Fatal("expired state accepted")
	}
}
func TestDomainConnectUnsupportedAndUntrustedDestinations(t *testing.T) {
	d, srv, unsupported := domainConnectFixture(t)
	defer srv.Close()
	*unsupported = true
	result, err := d.Discover(context.Background(), "demo.example.com")
	if err != nil || result.Supported || result.Instructions == "" {
		t.Fatal(result, err)
	}
	if _, err = d.Authorize(context.Background(), "demo.example.com", "admin", "domain", nil); err != ErrUnsupported {
		t.Fatal(err)
	}
	d.AllowedProviderHosts = nil
	result, err = d.Discover(context.Background(), "demo.example.com")
	if err != nil || result.Supported {
		t.Fatal(result, err)
	}
	for _, raw := range []string{"http://dns.example", "https://dns.example@evil.example", "https://evil.example", "https://dns.example?redirect=evil", "https://dns.example/#evil"} {
		d.AllowedProviderHosts = []string{"dns.example"}
		if _, err = d.allowed(raw); err == nil {
			t.Fatal("untrusted destination accepted", raw)
		}
	}
	d.AllowedProviderHosts = []string{"dns.example"}
	d.Resolver = fakeResolver{txt: map[string][]string{"_domainconnect.example.com": {"127.0.0.1"}}}
	result, err = d.Discover(context.Background(), "example.com")
	if err != nil || result.Supported {
		t.Fatal("SSRF destination accepted", result, err)
	}
}
func TestDomainConnectDoesNotFollowRedirects(t *testing.T) {
	reached := false
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer target.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	u, _ := url.Parse(source.URL)
	d := &DomainConnect{HTTPClient: source.Client(), AllowedProviderHosts: []string{u.Host}}
	status, err := d.get(context.Background(), source.URL, nil)
	if err != nil || status != 302 || reached {
		t.Fatal(status, err, reached)
	}
}
