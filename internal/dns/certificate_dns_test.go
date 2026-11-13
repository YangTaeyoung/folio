package dns

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserCertificateSelectedZoneOwnershipRouteAndDelete(t *testing.T) {
	m := fakeManager(t)
	os.WriteFile(m.defaultCert, certificate(), 0600)
	if err := m.ImportCertificate(m.defaultCert); err != nil {
		t.Fatal(err)
	}
	var record *cfRecord
	posts, deletes := 0, 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-certificate" {
			t.Error("certificate token not used privately")
		}
		var result any
		switch {
		case r.URL.Path == "/zones/selected-zone":
			result = Zone{ID: "selected-zone", Name: "example.com"}
		case r.Method == http.MethodPost:
			posts++
			var body struct{ Type, Name, Content, Comment string }
			json.NewDecoder(r.Body).Decode(&body)
			record = &cfRecord{ID: "browser-txt", Type: body.Type, Name: body.Name, Content: body.Content, Comment: body.Comment}
			result = record
		case r.Method == http.MethodDelete:
			deletes++
			result = map[string]string{"id": "browser-txt"}
			record = nil
		case strings.HasSuffix(r.URL.Path, "/browser-txt"):
			result = record
		case r.URL.Query().Get("name") == "demo.example.com":
			result = []cfRecord{{ID: "route", Type: "CNAME", Name: "demo.example.com", Content: fakeTunnelID + ".cfargotunnel.com"}}
		default:
			if record == nil {
				result = []cfRecord{}
			} else {
				result = []cfRecord{*record}
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
	}))
	defer provider.Close()
	m.DNSAPIBase = provider.URL
	ctx := context.Background()
	zone, err := m.SelectedZone(ctx)
	if err != nil || zone.ID != "selected-zone" || zone.Name != "example.com" {
		t.Fatal(zone, err)
	}
	id, err := m.ProvisionOwnership(ctx, "demo.example.com", "challenge")
	if err != nil || id != "browser-txt" || posts != 1 || record.Name != "_folio.demo.example.com" {
		t.Fatal(id, err, posts, record)
	}
	if _, err = m.ProvisionOwnership(ctx, "demo.example.com", "challenge"); !errors.Is(err, ErrCollision) || posts != 1 {
		t.Fatal("existing TXT overwritten", err, posts)
	}
	for _, host := range []string{"anotherexample.com", "demo.example.net"} {
		if _, err = m.ProvisionOwnership(ctx, host, "challenge"); err == nil {
			t.Fatal("foreign zone accepted", host)
		}
		if _, err = m.ConfigureTunnel(ctx, "test", []string{host}); err == nil {
			t.Fatal("foreign tunnel route accepted", host)
		}
	}
	if _, err = os.Stat(filepath.Join(m.DataDir, "dns", "tunnel.json")); !os.IsNotExist(err) {
		t.Fatal("foreign hostname created tunnel", err)
	}
	if ok, err := m.VerifyRoute(ctx, "demo.example.com", fakeTunnelID+".cfargotunnel.com"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := m.VerifyRoute(ctx, "demo.example.com", "unrelated.cfargotunnel.com"); err != nil || ok {
		t.Fatal("wrong route accepted", ok, err)
	}
	if err = m.DeleteOwned(ctx, id, "_folio.demo.example.com", "wrong-challenge"); !errors.Is(err, ErrNotOwned) || deletes != 0 {
		t.Fatal(err, deletes)
	}
	if err = m.DeleteOwned(ctx, id, "_folio.demo.example.com", "challenge"); err != nil || deletes != 1 {
		t.Fatal(err, deletes)
	}
}
func TestBrowserCertificatePermissionDenialRequiresManualFallback(t *testing.T) {
	for _, denyZone := range []bool{false, true} {
		t.Run(map[bool]string{false: "TXTDenied", true: "ZoneDenied"}[denyZone], func(t *testing.T) {
			m := fakeManager(t)
			os.WriteFile(m.defaultCert, certificate(), 0600)
			if err := m.ImportCertificate(m.defaultCert); err != nil {
				t.Fatal(err)
			}
			posts := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/zones/selected-zone" && !denyZone {
					json.NewEncoder(w).Encode(map[string]any{"success": true, "result": Zone{ID: "selected-zone", Name: "example.com"}})
					return
				}
				if r.Method == http.MethodGet && !denyZone {
					json.NewEncoder(w).Encode(map[string]any{"success": true, "result": []cfRecord{}})
					return
				}
				if r.Method == http.MethodPost {
					posts++
				}
				w.WriteHeader(403)
				w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"secret-certificate"}]}`))
			}))
			defer provider.Close()
			m.DNSAPIBase = provider.URL
			id, err := m.ProvisionOwnership(context.Background(), "demo.example.com", "challenge")
			if id != "" || !errors.Is(err, ErrBrowserDNSUnavailable) || strings.Contains(err.Error(), "secret-certificate") {
				t.Fatal("denial reported as success or secret leaked", id, err)
			}
			if denyZone && posts != 0 {
				t.Fatal("wrote before selected-zone safety could be checked")
			}
		})
	}
}
