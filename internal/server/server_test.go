package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type client struct {
	t      *testing.T
	s      *Server
	cookie *http.Cookie
	csrf   string
}

func fresh(t *testing.T) *client {
	t.Helper()
	s, e := New(Config{DataDir: t.TempDir(), AppURL: "http://app.test", ContentURL: "http://{artifact}.content.test", BootstrapToken: "test-setup"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return &client{t: t, s: s}
}
func (c *client) call(method, path string, body any) (int, map[string]any, *httptest.ResponseRecorder) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, "http://app.test"+path, r)
	req.Header.Set("Content-Type", "application/json")
	if c.cookie != nil {
		req.AddCookie(c.cookie)
		req.Header.Set("Origin", "http://app.test")
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, req)
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w
}
func (c *client) setup() {
	code, out, w := c.call("POST", "/api/setup", map[string]any{"token": "test-setup", "organizationName": "Team", "name": "Admin", "email": "admin@test.invalid", "password": "strong-password-123"})
	if code != 200 {
		c.t.Fatalf("setup %d %s", code, w.Body)
	}
	c.cookie = w.Result().Cookies()[0]
	c.csrf = out["csrfToken"].(string)
}
func (c *client) upload(id, base, path, content string) (int, map[string]any) {
	c.t.Helper()
	b := new(bytes.Buffer)
	m := multipart.NewWriter(b)
	meta := map[string]any{"title": "Test artifact", "baseVersionId": base, "files": []map[string]any{{"path": path}}}
	raw, _ := json.Marshal(meta)
	m.WriteField("metadata", string(raw))
	p, _ := m.CreateFormFile("file0", "ignored.html")
	p.Write([]byte(content))
	m.Close()
	url := "/api/artifacts"
	if id != "" {
		url += "/" + id + "/versions"
	}
	req := httptest.NewRequest("POST", "http://app.test"+url, b)
	req.Header.Set("Content-Type", m.FormDataContentType())
	req.Header.Set("Origin", "http://app.test")
	req.Header.Set("X-CSRF-Token", c.csrf)
	req.AddCookie(c.cookie)
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, req)
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}
func TestBootstrapCSRFAndPathAtomicity(t *testing.T) {
	c := fresh(t)
	c.setup()
	req := httptest.NewRequest("POST", "http://app.test/api/auth/logout", nil)
	req.AddCookie(c.cookie)
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("missing CSRF %d", w.Code)
	}
	for _, p := range []string{"../index.html", "/etc/passwd", ".env", "x/../index.html", "x\\index.html"} {
		code, _ := c.upload("", "", p, "bad")
		if code != 400 {
			t.Fatalf("path %s status %d", p, code)
		}
	}
	code, out, _ := c.call("GET", "/api/artifacts", nil)
	if code != 200 || len(out["artifacts"].([]any)) != 0 {
		t.Fatal(out)
	}
}
func TestGrantHostReplayAndCredentialRevocation(t *testing.T) {
	c := fresh(t)
	c.setup()
	code, out := c.upload("", "", "index.html", "<script type=module src='./app.js'></script>")
	if code != 201 {
		t.Fatalf("upload %d %v", code, out)
	}
	a := out["artifact"].(map[string]any)
	id := a["id"].(string)
	code, out, _ = c.call("POST", "/api/artifacts/"+id+"/launch", map[string]any{})
	if code != 200 {
		t.Fatal(out)
	}
	grant := out["url"].(string)
	wrong := httptest.NewRequest("GET", grant, nil)
	wrong.Host = "wrong.content.test"
	w := httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, wrong)
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, httptest.NewRequest("GET", grant, nil))
	if w.Code != 303 {
		t.Fatalf("exchange %d %s", w.Code, w.Body)
	}
	vc := w.Result().Cookies()[0]
	location := w.Header().Get("Location")
	w2 := httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w2, httptest.NewRequest("GET", grant, nil))
	if w2.Code != 404 {
		t.Fatal("replayed grant")
	}
	req := httptest.NewRequest("GET", strings.Split(grant, "/grant/")[0]+location, nil)
	req.AddCookie(vc)
	w2 = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w2, req)
	if w2.Code != 200 {
		t.Fatalf("content %d %s", w2.Code, w2.Body)
	}
	c.call("POST", "/api/auth/logout", map[string]any{})
	w2 = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w2, req)
	if w2.Code != 404 {
		t.Fatalf("revoked %d", w2.Code)
	}
}
func TestVersionsQuotaArchiveAndSharing(t *testing.T) {
	c := fresh(t)
	c.setup()
	code, out := c.upload("", "", "index.html", "one")
	if code != 201 {
		t.Fatal(code, out)
	}
	a := out["artifact"].(map[string]any)
	id := a["id"].(string)
	base := a["currentVersionId"].(string)
	code, _ = c.upload(id, "stale", "index.html", "two")
	if code != 409 {
		t.Fatal(code)
	}
	code, out = c.upload(id, base, "index.html", "two")
	if code != 201 {
		t.Fatal(code, out)
	}
	code, _, _ = c.call("POST", "/api/artifacts/"+id+"/versions/"+base+"/restore", map[string]any{})
	if code != 200 {
		t.Fatal(code)
	}
	c.call("POST", "/api/artifacts/"+id+"/archive", map[string]any{})
	code, _, _ = c.call("PATCH", "/api/admin/settings", map[string]any{"policy": map[string]any{"quotaBytes": 6, "maxUploadBytes": 100, "retentionDays": 1, "overflowPolicy": "delete_oldest"}})
	if code != 200 {
		t.Fatal(code)
	}
	code, _ = c.upload("", "", "index.html", "no room")
	if code != 413 {
		t.Fatal("archived quota", code)
	}
	anon := &client{t: t, s: c.s}
	code, _, _ = anon.call("GET", "/api/share/"+a["slug"].(string), nil)
	if code != 404 {
		t.Fatal(code)
	}
	c.call("PATCH", "/api/artifacts/"+id+"/sharing", map[string]any{"visibility": "public", "memberIds": []string{}, "expiresAt": nil})
	code, _, _ = anon.call("GET", "/api/share/"+a["slug"].(string), nil)
	if code != 200 {
		t.Fatal(code)
	}
}

func (c *client) member(email string) *client {
	c.t.Helper()
	code, out, _ := c.call("POST", "/api/admin/members", map[string]any{"name": email, "email": email, "role": "member", "password": "temporary-password-123"})
	if code != 201 {
		c.t.Fatal(code, out)
	}
	m := &client{t: c.t, s: c.s}
	code, out, w := m.call("POST", "/api/auth/login", map[string]any{"email": email, "password": "temporary-password-123"})
	if code != 200 {
		c.t.Fatal(code, out)
	}
	m.cookie = w.Result().Cookies()[0]
	m.csrf = out["csrfToken"].(string)
	code, out, w = m.call("POST", "/api/auth/password", map[string]any{"currentPassword": "temporary-password-123", "newPassword": "permanent-password-123"})
	if code != 200 {
		c.t.Fatal(code, out)
	}
	m.cookie = w.Result().Cookies()[0]
	m.csrf = out["csrfToken"].(string)
	return m
}
func TestPermissionMatrixAndImmediateRevocation(t *testing.T) {
	admin := fresh(t)
	admin.setup()
	owner := admin.member("owner@test.invalid")
	selected := admin.member("selected@test.invalid")
	other := admin.member("other@test.invalid")
	_, me, _ := selected.call("GET", "/api/me", nil)
	selectedID := me["user"].(map[string]any)["id"].(string)
	code, out := owner.upload("", "", "index.html", "private")
	if code != 201 {
		t.Fatal(code, out)
	}
	a := out["artifact"].(map[string]any)
	id := a["id"].(string)
	slug := a["slug"].(string)
	anon := &client{t: t, s: admin.s}
	for _, scope := range []string{"private", "organization", "selected", "public"} {
		code, _, _ = owner.call("PATCH", "/api/artifacts/"+id+"/sharing", map[string]any{"visibility": scope, "memberIds": []string{selectedID}, "expiresAt": nil})
		if code != 200 {
			t.Fatal(code)
		}
		for name, c := range map[string]*client{"admin": admin, "owner": owner, "selected": selected, "other": other, "anonymous": anon} {
			want := name == "admin" || name == "owner" || scope == "public" || (scope == "organization" && name != "anonymous") || (scope == "selected" && name == "selected")
			code, _, _ = c.call("GET", "/api/share/"+slug, nil)
			if (code == 200) != want {
				t.Fatalf("scope=%s role=%s status=%d expected read=%v", scope, name, code, want)
			}
		}
	}
	code, _, _ = other.call("PATCH", "/api/artifacts/"+id+"/sharing", map[string]any{"visibility": "public", "memberIds": []string{}, "expiresAt": nil})
	if code != 403 {
		t.Fatal("recipient write", code)
	}
	owner.call("PATCH", "/api/artifacts/"+id+"/sharing", map[string]any{"visibility": "private", "memberIds": []string{}, "expiresAt": nil})
	code, out, _ = other.call("GET", "/api/artifacts?view=shared&q=Test", nil)
	if code != 200 || len(out["artifacts"].([]any)) != 0 {
		t.Fatal("private leaked", out)
	}
	code, _, _ = admin.call("PATCH", "/api/admin/members/"+admin.s.st.Users[admin.s.st.Sessions[digest(admin.cookie.Value)].UserID].ID, map[string]any{"role": "member"})
	if code != 409 {
		t.Fatal("last admin", code)
	}
}
func TestTokenScopesExpiryAndRevocation(t *testing.T) {
	c := fresh(t)
	c.setup()
	code, out := c.upload("", "", "index.html", "secret")
	if code != 201 {
		t.Fatal(out)
	}
	id := out["artifact"].(map[string]any)["id"].(string)
	code, out, _ = c.call("POST", "/api/tokens", map[string]any{"name": "Reader", "scopes": []string{"read"}})
	if code != 201 {
		t.Fatal(out)
	}
	plain := out["token"].(string)
	tokenID := out["record"].(map[string]any)["id"].(string)
	req := httptest.NewRequest("POST", "http://app.test/api/artifacts/"+id+"/launch", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+plain)
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("bearer launch %d %s", w.Code, w.Body)
	}
	var launch map[string]any
	json.Unmarshal(w.Body.Bytes(), &launch)
	grant := launch["url"].(string)
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, httptest.NewRequest("GET", grant, nil))
	if w.Code != 303 {
		t.Fatal(w.Code)
	}
	cookie := w.Result().Cookies()[0]
	location := w.Header().Get("Location")
	req = httptest.NewRequest("POST", "http://app.test/api/artifacts/"+id+"/archive", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+plain)
	w = httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("read token write", w.Code)
	}
	record := c.s.st.Tokens[tokenID]
	expired := now().AddDate(0, 0, -1)
	record.ExpiresAt = &expired
	c.s.st.Tokens[tokenID] = record
	expiredReq := httptest.NewRequest("GET", strings.Split(grant, "/grant/")[0]+location, nil)
	expiredReq.AddCookie(cookie)
	expiredResponse := httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(expiredResponse, expiredReq)
	if expiredResponse.Code != 404 {
		t.Fatal("expired token viewer", expiredResponse.Code)
	}
	record.ExpiresAt = nil
	c.s.st.Tokens[tokenID] = record
	c.call("DELETE", "/api/tokens/"+tokenID, nil)
	req = httptest.NewRequest("GET", strings.Split(grant, "/grant/")[0]+location, nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatal("token viewer survived revocation", w.Code)
	}
}
func TestUploadLimitsAndConcurrentVersionConflict(t *testing.T) {
	c := fresh(t)
	c.setup()
	c.call("PATCH", "/api/admin/settings", map[string]any{"policy": map[string]any{"quotaBytes": 100, "maxUploadBytes": 4, "retentionDays": 0, "overflowPolicy": "reject"}})
	code, _ := c.upload("", "", "index.html", "12345")
	if code != 413 {
		t.Fatal(code)
	}
	code, out := c.upload("", "", "index.html", "1234")
	if code != 201 {
		t.Fatal(code, out)
	}
	a := out["artifact"].(map[string]any)
	id := a["id"].(string)
	base := a["currentVersionId"].(string)
	ch := make(chan int, 2)
	for range 2 {
		go func() { code, _ := c.upload(id, base, "index.html", "new"); ch <- code }()
	}
	one, two := <-ch, <-ch
	if !((one == 201 && two == 409) || (one == 409 && two == 201)) {
		t.Fatal(one, two)
	}
	_, out, _ = c.call("GET", "/api/artifacts/"+id, nil)
	if len(out["versions"].([]any)) != 2 {
		t.Fatal(out)
	}
}
func TestPersistenceAndEncryptedSecrets(t *testing.T) {
	c := fresh(t)
	c.setup()
	code, out, _ := c.call("POST", "/api/storage", map[string]any{"name": "Private config", "provider": "local", "accessKeyId": "public-id-marker", "secretAccessKey": "never-persist-this-secret"})
	if code != 201 {
		t.Fatal(code, out)
	}
	raw, _ := json.Marshal(out)
	if bytes.Contains(raw, []byte("never-persist")) || bytes.Contains(raw, []byte("public-id-marker")) {
		t.Fatal("secret in response")
	}
	var persisted []byte
	if err := c.s.db.QueryRow("SELECT value FROM state WHERE id=1").Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(persisted, []byte("never-persist")) || bytes.Contains(persisted, []byte("strong-password")) {
		t.Fatal("plaintext persisted")
	}
	dir := c.s.cfg.DataDir
	c.s.Close()
	s, err := New(Config{DataDir: dir, AppURL: "http://app.test", ContentURL: "http://{artifact}.content.test"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := &client{t: t, s: s, cookie: c.cookie, csrf: c.csrf}
	code, _, _ = d.call("GET", "/api/me", nil)
	if code != 200 {
		t.Fatal("session not persisted", code)
	}
	if s.BootstrapToken() != "" {
		t.Fatal("setup reopened")
	}
	code, out, _ = d.call("GET", "/api/storage", nil)
	if code != 200 || len(out["backends"].([]any)) != 2 {
		t.Fatal(out)
	}
}

func TestRelativeModuleAssetRequiresLiveOriginatingSession(t *testing.T) {
	c := fresh(t)
	c.setup()
	b := new(bytes.Buffer)
	m := multipart.NewWriter(b)
	m.WriteField("metadata", `{"title":"Module artifact","files":[{"path":"index.html"},{"path":"app.js"}]}`)
	p, _ := m.CreateFormFile("file0", "ignored.html")
	p.Write([]byte(`<script type="module" src="./app.js"></script>`))
	p, _ = m.CreateFormFile("file1", "ignored.js")
	p.Write([]byte(`document.body.dataset.module="loaded"`))
	m.Close()
	req := httptest.NewRequest("POST", "http://app.test/api/artifacts", b)
	req.Header.Set("Content-Type", m.FormDataContentType())
	req.Header.Set("Origin", "http://app.test")
	req.Header.Set("X-CSRF-Token", c.csrf)
	req.AddCookie(c.cookie)
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	var uploaded map[string]any
	json.Unmarshal(w.Body.Bytes(), &uploaded)
	a := uploaded["artifact"].(map[string]any)
	_, launch, _ := c.call("POST", "/api/artifacts/"+a["id"].(string)+"/launch", map[string]any{})
	grant := launch["url"].(string)
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, httptest.NewRequest("GET", grant, nil))
	cookie := w.Result().Cookies()[0]
	htmlURL := strings.Split(grant, "/grant/")[0] + w.Header().Get("Location")
	assetURL := strings.TrimSuffix(htmlURL, "index.html") + "app.js"
	for _, url := range []string{htmlURL, assetURL} {
		req = httptest.NewRequest("GET", url, nil)
		req.AddCookie(cookie)
		w = httptest.NewRecorder()
		c.s.ContentHandler().ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatal(url, w.Code, w.Body)
		}
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "javascript") {
		t.Fatal(w.Header())
	}
	c.call("POST", "/api/auth/logout", map[string]any{})
	req = httptest.NewRequest("GET", assetURL, nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatal("module remained readable after session revoked", w.Code)
	}
}
