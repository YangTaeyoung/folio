package server

import (
	"bytes"
	"context"
	"errors"
	"github.com/YangTaeyoung/folio/internal/blob"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type faultStore struct {
	mu                  sync.Mutex
	objects             map[string][]byte
	failPut, failDelete bool
	putStarted          chan struct{}
	putContinue         chan struct{}
	afterPut            func()
}

func (f *faultStore) Put(ctx context.Context, key string, r io.Reader, n int64, m string) error {
	b, e := io.ReadAll(r)
	if e != nil {
		return e
	}
	f.mu.Lock()
	f.objects[key] = b
	fail := f.failPut
	start, cont := f.putStarted, f.putContinue
	hook := f.afterPut
	f.mu.Unlock()
	if start != nil {
		start <- struct{}{}
		<-cont
	}
	if hook != nil {
		hook()
	}
	if fail {
		return errors.New("injected put failure after remote write")
	}
	return nil
}
func (f *faultStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[key]
	if !ok {
		return nil, errors.New("missing")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (f *faultStore) Delete(ctx context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failDelete {
		return errors.New("provider-delete-failure-sensitive-marker")
	}
	delete(f.objects, key)
	return nil
}
func (f *faultStore) Test(context.Context) error { return nil }
func freshFault(t *testing.T) (*client, *faultStore) {
	t.Helper()
	f := &faultStore{objects: map[string][]byte{}}
	s, e := New(Config{DataDir: t.TempDir(), AppURL: "http://app.test", ContentURL: "http://{artifact}.content.test", BootstrapToken: "test-setup", BlobFactory: func(blob.Config) (blob.Store, error) { return f, nil }})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	c := &client{t: t, s: s}
	c.setup()
	return c, f
}
func policy(c *client, quota int64, retention int, overflow string) {
	c.t.Helper()
	code, out, _ := c.call("PATCH", "/api/admin/settings", map[string]any{"policy": map[string]any{"quotaBytes": quota, "maxUploadBytes": 1000, "retentionDays": retention, "overflowPolicy": overflow}})
	if code != 200 {
		c.t.Fatal(code, out)
	}
}
func TestFailedStagingDoesNotEvictAndFailedGCRetries(t *testing.T) {
	c, f := freshFault(t)
	code, out := c.upload("", "", "index.html", "old")
	if code != 201 {
		t.Fatal(out)
	}
	old := out["artifact"].(map[string]any)["id"].(string)
	policy(c, 3, 0, "delete_oldest")
	f.mu.Lock()
	f.failPut = true
	f.mu.Unlock()
	code, _ = c.upload("", "", "index.html", "new")
	if code != 502 {
		t.Fatal(code)
	}
	if _, exists := c.s.st.Artifacts[old]; !exists {
		t.Fatal("failed stage evicted valid publication")
	}
	f.mu.Lock()
	if len(f.objects) != 1 {
		t.Fatalf("failed key not deleted, objects=%d", len(f.objects))
	}
	f.failPut = false
	f.failDelete = true
	f.mu.Unlock()
	code, out = c.upload("", "", "index.html", "new")
	if code != 201 {
		t.Fatal(code, out)
	}
	if _, exists := c.s.st.Artifacts[old]; exists {
		t.Fatal("old not evicted")
	}
	if c.s.used() != 3 || len(c.s.st.Garbage) != 1 {
		t.Fatal(c.s.used(), c.s.st.Garbage)
	}
	if c.s.st.Garbage[0].Error != "Storage deletion failed" {
		t.Fatal("provider error leak")
	}
	f.mu.Lock()
	f.failDelete = false
	f.mu.Unlock()
	result, e := c.s.Maintenance(context.Background(), false)
	if e != nil || result.PendingDeletionBytes != 0 || len(c.s.st.Garbage) != 0 {
		t.Fatal(result, e)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.objects) != 1 {
		t.Fatal(len(f.objects))
	}
}
func TestRetentionDryRunArchiveAndInFlightUpdate(t *testing.T) {
	c, f := freshFault(t)
	_, out := c.upload("", "", "index.html", "old")
	a := out["artifact"].(map[string]any)
	id := a["id"].(string)
	base := a["currentVersionId"].(string)
	_, out = c.upload("", "", "index.html", "archive")
	archiveID := out["artifact"].(map[string]any)["id"].(string)
	c.call("POST", "/api/artifacts/"+archiveID+"/archive", map[string]any{})
	policy(c, 1000, 1, "reject")
	c.s.mu.Lock()
	for key, v := range c.s.st.Artifacts {
		v.UpdatedAt = now().Add(-48 * time.Hour)
		c.s.st.Artifacts[key] = v
	}
	c.s.save()
	c.s.mu.Unlock()
	result, e := c.s.Maintenance(context.Background(), true)
	if e != nil || len(result.Candidates) != 1 || result.Candidates[0].ID != id {
		t.Fatal(result, e)
	}
	if len(c.s.st.Artifacts) != 2 {
		t.Fatal("dry-run mutated")
	}
	f.mu.Lock()
	f.putStarted = make(chan struct{}, 1)
	f.putContinue = make(chan struct{})
	f.mu.Unlock()
	ch := make(chan int, 1)
	go func() { code, _ := c.upload(id, base, "index.html", "updated"); ch <- code }()
	<-f.putStarted
	result, e = c.s.Maintenance(context.Background(), false)
	if e != nil || len(result.Candidates) != 0 {
		t.Fatal("in-flight update was a candidate", result, e)
	}
	close(f.putContinue)
	if code := <-ch; code != 201 {
		t.Fatal(code)
	}
	result, e = c.s.Maintenance(context.Background(), false)
	if e != nil || len(result.Candidates) != 0 || len(c.s.st.Artifacts) != 2 {
		t.Fatal(result, e)
	}
}
func TestTransactionFailurePreservesOldPublication(t *testing.T) {
	c, f := freshFault(t)
	_, out := c.upload("", "", "index.html", "old")
	id := out["artifact"].(map[string]any)["id"].(string)
	policy(c, 3, 0, "delete_oldest")
	f.mu.Lock()
	f.afterPut = func() { c.s.db.Close() }
	f.mu.Unlock()
	code, _ := c.upload("", "", "index.html", "new")
	if code != 500 {
		t.Fatal(code)
	}
	if len(c.s.st.Artifacts) != 1 || c.s.st.Artifacts[id].ID != id {
		t.Fatal("transaction failure changed metadata")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.objects) != 1 {
		t.Fatal("transaction failure left staged bytes", len(f.objects))
	}
}

type fakeResolver struct {
	mu    sync.Mutex
	txt   map[string][]string
	cname map[string]string
}

func (f *fakeResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.txt[name]...), nil
}
func (f *fakeResolver) LookupCNAME(ctx context.Context, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cname[name], nil
}
func (f *fakeResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) { return nil, nil }
func TestDomainProofHostBindingAndPrivateRedirect(t *testing.T) {
	c := fresh(t)
	c.setup()
	_, out := c.upload("", "", "index.html", "private")
	a := out["artifact"].(map[string]any)
	id := a["id"].(string)
	slug := a["slug"].(string)
	res := &fakeResolver{txt: map[string][]string{}, cname: map[string]string{}}
	c.s.cfg.DNSResolver = res
	code, out, _ := c.call("POST", "/api/domains", map[string]any{"hostname": "preview.example.test", "artifactId": id, "provider": "manual", "target": slug + ".content.test"})
	if code != 201 {
		t.Fatal(code, out)
	}
	d := out["domain"].(map[string]any)
	domainID := d["id"].(string)
	code, out, _ = c.call("POST", "/api/domains/"+domainID+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] == "verified" {
		t.Fatal(code, out)
	}
	stored := c.s.st.Domains[domainID]
	res.txt["_folio.preview.example.test"] = []string{stored.Challenge}
	res.cname["preview.example.test"] = slug + ".content.test."
	code, out, _ = c.call("POST", "/api/domains/"+domainID+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] != "verified" {
		t.Fatal(code, out)
	}
	req := httptest.NewRequest("GET", "https://preview.example.test/", nil)
	w := httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, req)
	if w.Code != 303 || w.Header().Get("Location") != "http://app.test/s/"+slug+"?domain=preview.example.test" {
		t.Fatal(w.Code, w.Header())
	}
	code, out, _ = c.call("POST", "/api/artifacts/"+id+"/launch", map[string]any{"hostname": "attacker.example.test"})
	if code != 400 {
		t.Fatal(code, out)
	}
	code, out, _ = c.call("POST", "/api/artifacts/"+id+"/launch", map[string]any{"hostname": "preview.example.test"})
	if code != 200 {
		t.Fatal(code, out)
	}
	grant := out["url"].(string)
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, httptest.NewRequest("GET", grant, nil))
	if w.Code != 303 {
		t.Fatal(w.Code)
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.Secure || cookie.Domain != "" || !cookie.HttpOnly {
		t.Fatal(cookie)
	}
	c.call("DELETE", "/api/domains/"+domainID, nil)
	req = httptest.NewRequest("GET", "https://preview.example.test"+w.Header().Get("Location"), nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatal("detached domain still serves", w.Code)
	}
}
func TestContentModulesCrossArtifactAndSharingExpiry(t *testing.T) {
	c := fresh(t)
	c.setup()
	code, out := c.upload("", "", "index.html", "<script type=module>fetch('./data.json')</script>")
	if code != 201 {
		t.Fatal(out)
	}
	a := out["artifact"].(map[string]any)
	id := a["id"].(string)
	slug := a["slug"].(string)
	_, other := c.upload("", "", "index.html", "other")
	otherA := other["artifact"].(map[string]any)
	c.call("PATCH", "/api/artifacts/"+id+"/sharing", map[string]any{"visibility": "public", "memberIds": []string{}, "expiresAt": nil})
	req := httptest.NewRequest("GET", "http://"+slug+".content.test/render/"+otherA["id"].(string)+"/"+otherA["currentVersionId"].(string)+"/index.html", nil)
	w := httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatal("cross artifact read", w.Code)
	}
	req = httptest.NewRequest("GET", "http://"+slug+".content.test/", nil)
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, req)
	if w.Code != 200 || !stringsContains(w.Header().Get("Content-Security-Policy"), "allow-same-origin") {
		t.Fatal(w.Code, w.Header())
	}
	a2 := c.s.st.Artifacts[id]
	past := now().Add(-time.Second)
	a2.ExpiresAt = &past
	c.s.st.Artifacts[id] = a2
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, req)
	if w.Code != 303 {
		t.Fatal("expired public root", w.Code)
	}
	code, _, _ = c.call("GET", "/api/share/"+slug, nil)
	if code != 200 {
		t.Fatal("owner lost expired publication", code)
	}
}
func stringsContains(s, x string) bool { return bytes.Contains([]byte(s), []byte(x)) }

func TestRecoverPersistedStagedOrphanAfterRestart(t *testing.T) {
	c, f := freshFault(t)
	f.objects["versions/interrupted"] = []byte("orphan")
	backendID := ""
	for id := range c.s.st.Backends {
		backendID = id
	}
	c.s.st.Garbage = append(c.s.st.Garbage, Garbage{BackendID: backendID, Key: "versions/interrupted", Size: 6, UploadID: "interrupted-before-commit"})
	if e := c.s.save(); e != nil {
		t.Fatal(e)
	}
	dir := c.s.cfg.DataDir
	c.s.Close()
	s, e := New(Config{DataDir: dir, AppURL: "http://app.test", ContentURL: "http://{artifact}.content.test", BlobFactory: func(blob.Config) (blob.Store, error) { return f, nil }})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	result, e := s.Maintenance(context.Background(), false)
	if e != nil || result.PendingDeletionBytes != 0 || len(f.objects) != 0 {
		t.Fatal(result, e, len(f.objects))
	}
}
func TestBackendSwitchKeepsHistoricalContent(t *testing.T) {
	c := fresh(t)
	c.setup()
	_, out := c.upload("", "", "index.html", "old")
	a := out["artifact"].(map[string]any)
	id := a["id"].(string)
	oldVersion := a["currentVersionId"].(string)
	oldBackend := out["version"].(map[string]any)["backendId"].(string)
	code, out, _ := c.call("POST", "/api/storage", map[string]any{"name": "Next backend", "provider": "local"})
	if code != 201 {
		t.Fatal(code, out)
	}
	newBackend := out["backend"].(map[string]any)["id"].(string)
	code, _, _ = c.call("POST", "/api/storage/"+newBackend+"/default", map[string]any{})
	if code != 200 {
		t.Fatal(code)
	}
	code, out = c.upload(id, oldVersion, "index.html", "new")
	if code != 201 || out["version"].(map[string]any)["backendId"] != newBackend {
		t.Fatal(code, out)
	}
	code, _, _ = c.call("DELETE", "/api/storage/"+oldBackend, nil)
	if code != 409 {
		t.Fatal("deleted referenced backend", code)
	}
	code, out, _ = c.call("POST", "/api/artifacts/"+id+"/launch", map[string]any{"versionId": oldVersion})
	if code != 200 {
		t.Fatal(code, out)
	}
	grant := out["url"].(string)
	w := httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, httptest.NewRequest("GET", grant, nil))
	cookie := w.Result().Cookies()[0]
	req := httptest.NewRequest("GET", stringsSplit(grant, "/grant/")+w.Header().Get("Location"), nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, req)
	if w.Code != 200 || w.Body.String() != "old" {
		t.Fatal("historical backend broken", w.Code, w.Body)
	}
}
func stringsSplit(s, sep string) string {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return s[:i]
		}
	}
	return s
}
func TestSuspensionAndPasswordResetRevokeCredentials(t *testing.T) {
	admin := fresh(t)
	admin.setup()
	m := admin.member("member@test.invalid")
	_, me, _ := m.call("GET", "/api/me", nil)
	uid := me["user"].(map[string]any)["id"].(string)
	_, out := m.upload("", "", "index.html", "secret")
	id := out["artifact"].(map[string]any)["id"].(string)
	_, out, _ = m.call("POST", "/api/tokens", map[string]any{"name": "Agent", "scopes": []string{"read"}})
	token := out["token"].(string)
	_, out, _ = m.call("POST", "/api/artifacts/"+id+"/launch", map[string]any{})
	grant := out["url"].(string)
	w := httptest.NewRecorder()
	m.s.ContentHandler().ServeHTTP(w, httptest.NewRequest("GET", grant, nil))
	cookie := w.Result().Cookies()[0]
	contentURL := stringsSplit(grant, "/grant/") + w.Header().Get("Location")
	admin.call("PATCH", "/api/admin/members/"+uid, map[string]any{"disabled": true})
	req := httptest.NewRequest("GET", contentURL, nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	m.s.ContentHandler().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatal("suspended viewer", w.Code)
	}
	req = httptest.NewRequest("GET", "http://app.test/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	m.s.Handler().ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("suspended token", w.Code)
	}
	admin.call("PATCH", "/api/admin/members/"+uid, map[string]any{"disabled": false})
	code, _, _ := m.call("GET", "/api/me", nil)
	if code != 401 {
		t.Fatal("reactivation restored old session", code)
	}
	code, out, _ = admin.call("POST", "/api/admin/members/"+uid+"/reset-password", map[string]any{})
	if code != 200 || out["temporaryPassword"] == nil {
		t.Fatal(code, out)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProxiedDomainRequiresOwnershipAndHTTPSOriginProof(t *testing.T) {
	c := fresh(t)
	c.setup()
	_, out := c.upload("", "", "index.html", "private")
	id := out["artifact"].(map[string]any)["id"].(string)
	_, out, _ = c.call("POST", "/api/domains", map[string]any{"hostname": "proxy.example.test", "artifactId": id, "provider": "cloudflare"})
	domainID := out["domain"].(map[string]any)["id"].(string)
	d := c.s.st.Domains[domainID]
	d.TunnelRoute = true
	d.Target = "dedicated.cfargotunnel.com"
	c.s.st.Domains[domainID] = d
	res := &fakeResolver{txt: map[string][]string{}, cname: map[string]string{}}
	c.s.cfg.DNSResolver = res
	c.s.cfg.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		c.s.ContentHandler().ServeHTTP(w, r)
		return w.Result(), nil
	})}
	code, out, _ := c.call("POST", "/api/domains/"+domainID+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] == "verified" {
		t.Fatal("verified missing TXT", code, out)
	}
	res.txt["_folio.proxy.example.test"] = []string{d.Challenge}
	code, out, _ = c.call("POST", "/api/domains/"+domainID+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] != "verified" {
		t.Fatal("HTTPS proof failed", code, out)
	}
	c.s.cfg.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		w.Header().Set("Location", "https://attacker.example.test/")
		w.WriteHeader(302)
		return w.Result(), nil
	})}
	code, out, _ = c.call("POST", "/api/domains/"+domainID+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] == "verified" {
		t.Fatal("trusted redirect instead of origin", code, out)
	}
}
