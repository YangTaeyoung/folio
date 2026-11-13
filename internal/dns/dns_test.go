package dns

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeResolver struct {
	txt    map[string][]string
	cnames map[string]string
	ips    map[string][]net.IPAddr
}

func (r fakeResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	v, ok := r.txt[name]
	if !ok {
		return nil, &net.DNSError{IsNotFound: true}
	}
	return v, nil
}
func (r fakeResolver) LookupCNAME(_ context.Context, name string) (string, error) {
	return r.cnames[name], nil
}
func (r fakeResolver) LookupIPAddr(_ context.Context, name string) ([]net.IPAddr, error) {
	return r.ips[name], nil
}
func TestVerifyRequiresOwnershipAndExactRoute(t *testing.T) {
	ctx := context.Background()
	r := fakeResolver{txt: map[string][]string{"_folio.demo.example.com": {"owned"}}, cnames: map[string]string{"demo.example.com": "target.example.com."}}
	for _, tc := range []struct {
		challenge, target string
		want              bool
	}{{"owned", "TARGET.example.com", true}, {"wrong", "target.example.com", false}, {"owned", "elsewhere.example.com", false}} {
		got, err := VerifyWithResolver(ctx, r, "demo.example.com", tc.challenge, tc.target)
		if err != nil || got != tc.want {
			t.Fatalf("verify %v: %v, %v", tc, got, err)
		}
	}
	r.ips = map[string][]net.IPAddr{"demo.example.com": {{IP: net.ParseIP("192.0.2.1")}}}
	ok, err := VerifyWithResolver(ctx, r, "demo.example.com", "owned", "192.0.2.1")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if _, err = VerifyWithResolver(ctx, r, "*.example.com", "owned", "target.example.com"); err == nil {
		t.Fatal("wildcard accepted")
	}
}
func TestCloudflareCollisionOwnershipAndRedaction(t *testing.T) {
	collision := true
	posts, deletes := 0, 0
	record := cfRecord{ID: "owned", Type: "CNAME", Name: "demo.example.com", Content: "target.example.com", Comment: ownedComment}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Error("missing authorization")
		}
		var result any
		switch {
		case strings.HasSuffix(r.URL.Path, "/dns_records") && r.Method == "GET":
			if r.URL.Query().Get("name") == "" {
				t.Error("unscoped lookup")
			}
			if collision {
				result = []cfRecord{record}
			} else {
				result = []cfRecord{}
			}
		case r.Method == "POST":
			posts++
			var input map[string]any
			json.NewDecoder(r.Body).Decode(&input)
			if input["comment"] != ownedComment || input["proxied"] != false {
				t.Error(input)
			}
			result = record
		case r.Method == "DELETE":
			deletes++
			result = map[string]string{"id": "owned"}
		default:
			result = record
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
	}))
	defer srv.Close()
	cf := &Cloudflare{APIToken: "secret-token", APIBase: srv.URL}
	ctx := context.Background()
	if _, err := cf.Provision(ctx, "zone", "demo.example.com", "target.example.com"); !errors.Is(err, ErrCollision) || posts != 0 {
		t.Fatal(err, posts)
	}
	if ok, err := cf.VerifyRoute(ctx, "zone", "demo.example.com", "target.example.com"); err != nil || !ok {
		t.Fatal("authenticated exact route", ok, err)
	}
	if ok, err := cf.VerifyRoute(ctx, "zone", "demo.example.com", "other.example.com"); err != nil || ok {
		t.Fatal("wrong authenticated route accepted", ok, err)
	}
	collision = false
	if id, err := cf.Provision(ctx, "zone", "demo.example.com", "target.example.com"); err != nil || id != "owned" || posts != 1 {
		t.Fatal(id, err, posts)
	}
	if err := cf.DeleteOwned(ctx, "zone", "owned", "wrong.example.com", "target.example.com"); !errors.Is(err, ErrNotOwned) || deletes != 0 {
		t.Fatal(err, deletes)
	}
	record.Comment = "someone else's record"
	if err := cf.DeleteOwned(ctx, "zone", "owned", "demo.example.com", "target.example.com"); !errors.Is(err, ErrNotOwned) {
		t.Fatal(err)
	}
	record.Comment = ownedComment
	if err := cf.DeleteOwned(ctx, "zone", "owned", "demo.example.com", "TARGET.example.com."); err != nil || deletes != 1 {
		t.Fatal(err, deletes)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"success":false,"errors":[{"code":1000,"message":"secret-token"}]}`))
	}))
	defer bad.Close()
	cf.APIBase = bad.URL
	_, err := cf.Zones(ctx)
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatal("token leaked", err)
	}
}
func TestCloudflarePaginationAndTXT(t *testing.T) {
	calls := 0
	txt := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any
		pages := 2
		if strings.HasSuffix(r.URL.Path, "/zones") {
			calls++
			result = []Zone{{ID: r.URL.Query().Get("page"), Name: "example.com"}}
		} else if r.Method == "GET" {
			result = []cfRecord{}
		} else {
			var v map[string]any
			json.NewDecoder(r.Body).Decode(&v)
			txt = v["type"] == "TXT" && v["name"] == "_folio.demo.example.com" && v["content"] == "challenge"
			result = cfRecord{ID: "txt"}
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result, "result_info": map[string]int{"total_pages": pages}})
	}))
	defer srv.Close()
	cf := &Cloudflare{APIToken: "token", APIBase: srv.URL}
	zones, err := cf.Zones(context.Background())
	if err != nil || len(zones) != 2 || calls != 2 {
		t.Fatal(zones, err, calls)
	}
	if _, err = cf.ProvisionOwnership(context.Background(), "zone", "demo.example.com", "challenge"); err != nil || !txt {
		t.Fatal(err, txt)
	}
}
