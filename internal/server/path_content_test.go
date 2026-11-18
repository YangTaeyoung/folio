package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const pathCSP = "sandbox allow-scripts allow-forms allow-downloads allow-same-origin; default-src 'self' data: blob:; script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https: http:; connect-src 'self'; worker-src 'self' blob:; object-src 'none'; base-uri 'self'"

func freshPath(t *testing.T) *client {
	t.Helper()
	s, e := New(Config{DataDir: t.TempDir(), AppURL: "http://app.test", ContentURL: "http://app.test/c/{artifact}", BootstrapToken: "test-setup"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return &client{t: t, s: s}
}

func (c *client) get(rawURL string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	c.t.Helper()
	r := httptest.NewRequest("GET", rawURL, nil)
	for _, co := range cookies {
		r.AddCookie(co)
	}
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, r)
	return w
}

func (c *client) pathArtifact(paths, bodies []string) (id, slug, versionID string) {
	c.t.Helper()
	code, out := c.publishFiles("", "", paths, bodies)
	if code != 201 {
		c.t.Fatal(code, out)
	}
	a := out["artifact"].(map[string]any)
	return a["id"].(string), a["slug"].(string), a["currentVersionId"].(string)
}

func (c *client) pathGrant(body map[string]any, id string) (string, *httptest.ResponseRecorder) {
	c.t.Helper()
	out := reviewHTTP(c.t, c, "POST", "/api/artifacts/"+id+"/launch", body, 200)
	grant := out["url"].(string)
	return grant, c.get(grant)
}

func TestPathModeGrantServesModulesAndRelativeAssets(t *testing.T) {
	c := freshPath(t)
	c.setup()
	id, slug, vid := c.pathArtifact([]string{"index.html", "app.js", "data.js"}, []string{"<script type=module src='./app.js'></script>", "import('./data.js')", "export default 1"})
	grant, w := c.pathGrant(map[string]any{}, id)
	if !strings.HasPrefix(grant, "http://app.test/c/"+slug+"/grant/") {
		t.Fatal(grant)
	}
	if w.Code != 303 || w.Header().Get("Location") != "/c/"+slug+"/render/"+id+"/"+vid+"/index.html" {
		t.Fatal(w.Code, w.Header())
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no viewer cookie")
	}
	for _, co := range cookies {
		if co.Path != "/c/"+slug+"/" || !strings.HasPrefix(co.Name, "folio_viewer") || !co.HttpOnly {
			t.Fatal(co)
		}
	}
	w = c.get("http://app.test/c/"+slug+"/render/"+id+"/"+vid+"/app.js", cookies...)
	if w.Code != 200 || w.Body.String() != "import('./data.js')" {
		t.Fatal(w.Code, w.Body)
	}
	if w.Header().Get("Content-Security-Policy") != pathCSP {
		t.Fatal(w.Header().Get("Content-Security-Policy"))
	}
}

func TestPathModeRejectsCrossArtifactAndUnknownSites(t *testing.T) {
	c := freshPath(t)
	c.setup()
	id, slug, vid := c.pathArtifact([]string{"index.html"}, []string{"a"})
	otherID, _, otherVID := c.pathArtifact([]string{"index.html"}, []string{"b"})
	c.call("PATCH", "/api/artifacts/"+id+"/sharing", map[string]any{"visibility": "public", "memberIds": []string{}, "expiresAt": nil})
	if w := c.get("http://app.test/c/" + slug + "/"); w.Code != 200 || w.Body.String() != "a" {
		t.Fatal("public root", w.Code, w.Body)
	}
	encoded := "%" + strings.ToUpper(hexByte(slug[0])) + slug[1:]
	for _, u := range []string{
		"/c/" + slug + "/render/" + otherID + "/" + otherVID + "/index.html",
		"/c/nope/",
		"/c/" + strings.ToUpper(slug) + "/",
		"/c/" + encoded + "/",
		"/c/" + slug + "/render/" + id + "/" + vid + "/../x",
		"/c//",
	} {
		if w := c.get("http://app.test" + u); w.Code != 404 {
			t.Fatal(u, w.Code)
		}
	}
}

func hexByte(b byte) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[b>>4], digits[b&15]})
}

func TestPathModeTrailingSlashRedirect(t *testing.T) {
	c := freshPath(t)
	c.setup()
	_, slug, _ := c.pathArtifact([]string{"index.html"}, []string{"a"})
	w := c.get("http://app.test/c/" + slug + "?x=1")
	if w.Code != 308 || w.Header().Get("Location") != "/c/"+slug+"/?x=1" {
		t.Fatal(w.Code, w.Header())
	}
}

func TestPathModeManagementSessionDoesNotAuthorizeContent(t *testing.T) {
	c := freshPath(t)
	c.setup()
	_, slug, _ := c.pathArtifact([]string{"index.html"}, []string{"private"})
	if w := c.get("http://app.test/c/"+slug+"/index.html", c.cookie); w.Code != 404 {
		t.Fatal("session read content", w.Code)
	}
	w := c.get("http://app.test/c/"+slug+"/", c.cookie)
	if w.Code != 303 || w.Header().Get("Location") != "http://app.test/s/"+slug {
		t.Fatal(w.Code, w.Header())
	}
}

func TestPathModeVersionIsolation(t *testing.T) {
	c := freshPath(t)
	c.setup()
	a, v1 := workspaceUpload(t, c, "", map[string]any{"title": "Versions"}, map[string]string{"index.html": "ONE"})
	_, v2 := workspaceUpload(t, c, a.ID, map[string]any{"baseVersionId": v1.ID}, map[string]string{"index.html": "TWO"})
	g1, w1 := c.pathGrant(map[string]any{"versionId": v1.ID, "isolatedVersion": true}, a.ID)
	g2, _ := c.pathGrant(map[string]any{"versionId": v2.ID, "isolatedVersion": true}, a.ID)
	site1 := strings.Split(g1, "/grant/")[0]
	if !strings.HasPrefix(site1, "http://app.test/c/v--") || site1 == strings.Split(g2, "/grant/")[0] {
		t.Fatal(g1, g2)
	}
	if w := c.get(site1+"/render/"+a.ID+"/"+v1.ID+"/index.html", w1.Result().Cookies()...); w.Code != 200 || w.Body.String() != "ONE" {
		t.Fatal(w.Code, w.Body)
	}
	if w := c.get(site1+"/render/"+a.ID+"/"+v2.ID+"/index.html", w1.Result().Cookies()...); w.Code != 404 {
		t.Fatal("cross version", w.Code)
	}
}

func TestPathModeAssetRequiresLiveSession(t *testing.T) {
	c := freshPath(t)
	c.setup()
	id, slug, vid := c.pathArtifact([]string{"index.html", "app.js"}, []string{"<script type=module src='./app.js'></script>", "1"})
	_, w := c.pathGrant(map[string]any{}, id)
	cookies := w.Result().Cookies()
	asset := "http://app.test/c/" + slug + "/render/" + id + "/" + vid + "/app.js"
	if w := c.get(asset, cookies...); w.Code != 200 {
		t.Fatal(w.Code)
	}
	c.call("POST", "/api/auth/logout", map[string]any{})
	if w := c.get(asset, cookies...); w.Code != 404 {
		t.Fatal("asset readable after logout", w.Code)
	}
}

func TestPathModeGuestLink(t *testing.T) {
	c := freshPath(t)
	c.setup()
	id, slug, vid := c.pathArtifact([]string{"index.html"}, []string{"guest view"})
	code, out, _ := c.call("POST", "/api/artifacts/"+id+"/guest-links", map[string]any{"versionId": vid, "expiresAt": now().Add(time.Hour), "password": "guest-password-123"})
	if code != 201 {
		t.Fatal(code, out)
	}
	gid := out["guestLink"].(map[string]any)["id"].(string)
	code, out, w := c.call("POST", "/api/guest/"+gid+"/unlock", map[string]any{"password": "guest-password-123"})
	if code != 200 {
		t.Fatal(code, out)
	}
	gc := w.Result().Cookies()[0]
	r := httptest.NewRequest("POST", "http://app.test/api/guest/"+gid+"/launch", strings.NewReader(`{}`))
	r.Header.Set("Origin", "http://app.test")
	r.Header.Set("X-CSRF-Token", out["csrfToken"].(string))
	r.AddCookie(gc)
	w = httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	grant := out["url"].(string)
	if !strings.HasPrefix(grant, "http://app.test/c/"+slug+"/grant/") {
		t.Fatal(grant)
	}
	w = c.get(grant)
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	if w2 := c.get("http://app.test"+w.Header().Get("Location"), w.Result().Cookies()...); w2.Code != 200 || w2.Body.String() != "guest view" {
		t.Fatal(w2.Code, w2.Body)
	}
}

func TestPathModeCustomDomainStillHostRouted(t *testing.T) {
	c := freshPath(t)
	c.setup()
	id, slug, _ := c.pathArtifact([]string{"index.html"}, []string{"private"})
	res := &fakeResolver{txt: map[string][]string{}, cname: map[string]string{}}
	c.s.cfg.DNSResolver = res
	code, out, _ := c.call("POST", "/api/domains", map[string]any{"hostname": "preview.example.test", "artifactId": id, "provider": "manual", "target": "app.test"})
	if code != 201 {
		t.Fatal(code, out)
	}
	domainID := out["domain"].(map[string]any)["id"].(string)
	res.txt["_folio.preview.example.test"] = []string{c.s.st.Domains[domainID].Challenge}
	res.cname["preview.example.test"] = "app.test."
	code, out, _ = c.call("POST", "/api/domains/"+domainID+"/verify", map[string]any{})
	if code != 200 || out["domain"].(map[string]any)["status"] != "verified" {
		t.Fatal(code, out)
	}
	w := c.get("https://preview.example.test/")
	if w.Code != 303 || w.Header().Get("Location") != "http://app.test/s/"+slug+"?domain=preview.example.test" {
		t.Fatal(w.Code, w.Header())
	}
}

func TestPathModeManagementRoutesUnaffected(t *testing.T) {
	c := freshPath(t)
	c.setup()
	if code, _, w := c.call("GET", "/api/me", nil); code != 200 {
		t.Fatal(code, w.Body)
	}
	if w := c.get("http://app.test/s/anything"); w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Fatal(w.Code, w.Header())
	}
}
