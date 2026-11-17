package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func photoBytes(t *testing.T, format string, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var b bytes.Buffer
	var e error
	if format == "jpeg" {
		e = jpeg.Encode(&b, img, nil)
	} else {
		e = png.Encode(&b, img)
	}
	if e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func (c *client) photo(method, path, kind string, body []byte) *httptest.ResponseRecorder {
	c.t.Helper()
	r := httptest.NewRequest(method, "http://app.test"+path, bytes.NewReader(body))
	r.Header.Set("Content-Type", kind)
	if c.cookie != nil {
		r.AddCookie(c.cookie)
		r.Header.Set("Origin", "http://app.test")
		r.Header.Set("X-CSRF-Token", c.csrf)
	}
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, r)
	return w
}
func TestProfilePhotosPrivacyACLAndValidation(t *testing.T) {
	admin := fresh(t)
	admin.setup()
	member := admin.member("photo@test.invalid")
	other := admin.member("other-photo@test.invalid")
	raw := append(photoBytes(t, "png", 512, 300), []byte("<script>metadata that must be stripped</script>")...)
	w := member.photo("PUT", "/api/me/photo", "image/png", raw)
	if w.Code != 200 {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var out struct{ User User }
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.User.AvatarURL == "" || strings.Contains(w.Body.String(), "Data") || strings.Contains(w.Body.String(), "passwordHash") {
		t.Fatal("bad public user", w.Body)
	}
	path := out.User.AvatarURL
	for _, c := range []*client{admin, member, other} {
		w = c.photo("GET", path, "", nil)
		if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("Cache-Control") != "no-store" || bytes.Contains(w.Body.Bytes(), []byte("script")) {
			t.Fatalf("read: %d %s", w.Code, w.Body)
		}
		cfg, _, e := image.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
		if e != nil || cfg.Width != 256 || cfg.Height != 150 {
			t.Fatal(cfg, e)
		}
	}
	if w = member.photo("HEAD", path, "", nil); w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal("photo HEAD", w.Code, w.Body)
	}
	w = member.photo("PUT", "/api/me/photo", "image/jpeg", append(photoBytes(t, "jpeg", 64, 64), []byte("trailing-private-metadata")...))
	var replacement struct{ User User }
	json.Unmarshal(w.Body.Bytes(), &replacement)
	if w.Code != 200 || replacement.User.AvatarURL == path || replacement.User.AvatarURL == "" {
		t.Fatal("replacement revision did not change", w.Code, w.Body)
	}
	w = member.photo("GET", replacement.User.AvatarURL, "", nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/jpeg" || bytes.Contains(w.Body.Bytes(), []byte("trailing-private-metadata")) {
		t.Fatal("JPEG re-encoding", w.Code)
	}
	anonymous := &client{t: t, s: admin.s}
	if w = anonymous.photo("GET", path, "", nil); w.Code != 401 {
		t.Fatal("public access", w.Code)
	}
	if w = other.photo("PUT", strings.Split(path, "?")[0], "image/png", raw); w.Code != 403 {
		t.Fatal("other write", w.Code)
	}
	if w = member.photo("PUT", "/api/organization/photo", "image/png", raw); w.Code != 403 {
		t.Fatal("org ACL", w.Code)
	}
	badCSRF := *member
	badCSRF.csrf = "wrong"
	if w = badCSRF.photo("DELETE", "/api/me/photo", "", nil); w.Code != 403 {
		t.Fatal("csrf", w.Code)
	}
	for _, tc := range []struct {
		data   []byte
		kind   string
		status int
	}{
		{[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), "image/svg+xml", 415},
		{[]byte("<html>not a png</html>"), "image/png", 415},
		{photoBytes(t, "png", 2, 2), "image/jpeg", 415},
		{photoBytes(t, "png", 2049, 1), "image/png", 400},
		{photoBytes(t, "png", 2, 2)[:20], "image/png", 400},
		{append(photoBytes(t, "png", 2, 2), make([]byte, photoUploadMax)...), "image/png", 413},
	} {
		if w = member.photo("PUT", "/api/me/photo", tc.kind, tc.data); w.Code != tc.status {
			t.Fatalf("validation got %d want %d: %s", w.Code, tc.status, w.Body)
		}
	}
	if w = member.photo("GET", path, "", nil); w.Code != 200 {
		t.Fatal("invalid write lost existing photo")
	}
	if w = admin.photo("PUT", "/api/organization/photo", "image/jpeg", photoBytes(t, "jpeg", 64, 64)); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w = member.photo("GET", "/api/organization/photo", "", nil); w.Code != 200 {
		t.Fatal("org read", w.Code)
	}
	// A temporary password cannot change a profile; disabled sessions cannot read photos.
	uid := out.User.ID
	u := admin.s.st.Users[uid]
	u.MustChangePassword = true
	admin.s.st.Users[uid] = u
	if w = member.photo("PUT", "/api/me/photo", "image/png", raw); w.Code != 403 {
		t.Fatal("password boundary", w.Code)
	}
	u.MustChangePassword = false
	u.Disabled = true
	admin.s.st.Users[uid] = u
	if w = member.photo("GET", "/api/organization/photo", "", nil); w.Code != 401 {
		t.Fatal("disabled reader", w.Code)
	}
	if w = other.photo("GET", path, "", nil); w.Code != 404 {
		t.Fatal("disabled target", w.Code)
	}
	u.Disabled = false
	admin.s.st.Users[uid] = u
	if w = member.photo("DELETE", "/api/me/photo", "", nil); w.Code != 200 {
		t.Fatal("remove", w.Code)
	}
	if w = member.photo("GET", path, "", nil); w.Code != 404 {
		t.Fatal("removed photo readable", w.Code)
	}
	if w = admin.photo("DELETE", "/api/organization/photo", "", nil); w.Code != 200 {
		t.Fatal("org remove", w.Code)
	}
	if w = admin.photo("GET", "/api/organization/photo", "", nil); w.Code != 404 {
		t.Fatal("removed org readable", w.Code)
	}
}
func TestProfileNameAndTokenBoundaries(t *testing.T) {
	c := fresh(t)
	c.setup()
	p := opsAdmin(c)
	before := c.s.st.Users[p.user.ID]
	status, out, _ := c.call("PATCH", "/api/me", map[string]any{"name": "  New 이름  "})
	if status != 200 || out["user"].(map[string]any)["name"] != "New 이름" {
		t.Fatal(status, out)
	}
	after := c.s.st.Users[p.user.ID]
	if after.Hash != before.Hash || after.Email != before.Email || after.Role != before.Role || len(c.s.st.Sessions) != 1 {
		t.Fatal("profile changed credentials")
	}
	for _, name := range []string{"", strings.Repeat("a", 101), "control\nname"} {
		if status, _, _ = c.call("PATCH", "/api/me", map[string]any{"name": name}); status != 400 {
			t.Fatal("name validation", status)
		}
	}
	if status, _, _ = c.call("PATCH", "/api/me", map[string]any{"name": "x", "role": "admin"}); status != 400 {
		t.Fatal("mass assignment", status)
	}
	_, out, _ = c.call("POST", "/api/tokens", map[string]any{"name": "read token", "scopes": []string{"read"}})
	token, _ := out["token"].(string)
	if token == "" {
		t.Fatal("no token", out)
	}
	_, noRead, _ := c.call("POST", "/api/tokens", map[string]any{"name": "publish only", "scopes": []string{"publish"}})
	noReadRequest := httptest.NewRequest("GET", "http://app.test/api/me/photo", nil)
	noReadRequest.Header.Set("Authorization", "Bearer "+noRead["token"].(string))
	noReadResponse := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(noReadResponse, noReadRequest)
	if noReadResponse.Code != 403 {
		t.Fatal("photo read bypassed read scope", noReadResponse.Code)
	}
	raw := photoBytes(t, "png", 4, 4)
	if w := c.photo("PUT", "/api/me/photo", "image/png", raw); w.Code != 200 {
		t.Fatal(w.Code)
	}
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/api/me/photo", 200}, {"PUT", "/api/me/photo", 403}, {"DELETE", "/api/organization/photo", 403}, {"PATCH", "/api/me", 403}} {
		r := httptest.NewRequest(tc.method, "http://app.test"+tc.path, bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "image/png")
		w := httptest.NewRecorder()
		c.s.Handler().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("token %s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
	}
	// Guest credentials never become an organization member principal.
	guest := principal{kind: "guest", guestLinkID: "guest"}
	w := httptest.NewRecorder()
	if e := c.s.profilePhotos(w, httptest.NewRequest("GET", "http://app.test/api/organization/photo", nil), guest); e == nil {
		t.Fatal("guest access")
	}
}

func TestProfilePhotoStateBudgetAndCommitRollback(t *testing.T) {
	c := fresh(t)
	c.setup()
	data := photoBytes(t, "png", 10, 10)
	if w := c.photo("PUT", "/api/me/photo", "image/png", data); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	uid := opsAdmin(c).user.ID
	before := c.s.st.Users[uid]
	c.s.st.OrganizationPhoto = &profilePhoto{Data: make([]byte, photoStateMax), MIME: "image/png"}
	if w := c.photo("PUT", "/api/me/photo", "image/png", data); w.Code != 413 {
		t.Fatal("photo state budget bypassed", w.Code)
	}
	if e := validatePhotoStateSize(c.s.st); e == nil {
		t.Fatal("merged backup photo budget bypassed")
	}
	c.s.st.OrganizationPhoto = nil
	if e := c.s.db.Close(); e != nil {
		t.Fatal(e)
	}
	if w := c.photo("DELETE", "/api/me/photo", "", nil); w.Code != 500 {
		t.Fatal("failed SQLite commit reported success", w.Code)
	}
	after := c.s.st.Users[uid]
	if after.Photo == nil || !bytes.Equal(before.Photo.Data, after.Photo.Data) || before.AvatarURL != after.AvatarURL || before.Hash != after.Hash {
		t.Fatal("failed photo deletion did not roll back")
	}
}
func TestProfilePhotosPersistenceBackupAndRecovery(t *testing.T) {
	c := fresh(t)
	c.setup()
	member := c.member("persist-photo@test.invalid")
	for _, entry := range []struct {
		c    *client
		path string
	}{{c, "/api/organization/photo"}, {c, "/api/me/photo"}, {member, "/api/me/photo"}} {
		if w := entry.c.photo("PUT", entry.path, "image/png", photoBytes(t, "png", 12, 10)); w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	p := opsAdmin(c)
	uid := p.user.ID
	adminHash := c.s.st.Users[uid].Hash
	info, e := c.s.createBackup(context.Background(), "backup-password-123", p)
	if e != nil {
		t.Fatal(e)
	}
	backup, e := os.ReadFile(filepath.Join(c.s.backupDir(), info.ID+".folio"))
	if e != nil {
		t.Fatal(e)
	}
	cfg := c.s.cfg
	if e = c.s.Close(); e != nil {
		t.Fatal(e)
	}
	reopened, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { reopened.Close() })
	c.s = reopened
	member.s = reopened
	for _, path := range []string{"/api/me/photo", "/api/organization/photo"} {
		if w := c.photo("GET", path, "", nil); w.Code != 200 {
			t.Fatal("restart lost photo", path, w.Code)
		}
	}
	c.photo("DELETE", "/api/organization/photo", "", nil)
	// Current user profile wins during restore, exactly like current credentials.
	c.photo("DELETE", "/api/me/photo", "", nil)
	if e = c.s.restoreBackup(context.Background(), bytes.NewReader(backup), "backup-password-123", opsAdmin(c)); e != nil {
		t.Fatal(e)
	}
	if w := c.photo("GET", "/api/organization/photo", "", nil); w.Code != 200 {
		t.Fatal("backup lost org", w.Code)
	}
	if w := c.photo("GET", "/api/me/photo", "", nil); w.Code != 404 {
		t.Fatal("restore overwrote current profile", w.Code)
	}
	if c.s.st.Users[uid].Hash != adminHash {
		t.Fatal("restore changed credentials")
	}
	// Restoring into another installation imports historical profile bytes while
	// current matching-email identities retain their destination credentials/photo.
	dest := fresh(t)
	dest.setup()
	if e = dest.s.restoreBackup(context.Background(), bytes.NewReader(backup), "backup-password-123", opsAdmin(dest)); e != nil {
		t.Fatal(e)
	}
	for _, u := range dest.s.st.Users {
		if u.Email == "persist-photo@test.invalid" {
			if u.Photo == nil || u.AvatarURL == "" {
				t.Fatal("imported photo missing")
			}
			w := dest.photo("GET", u.AvatarURL, "", nil)
			if w.Code != 200 {
				t.Fatal("imported photo inaccessible", w.Code)
			}
		}
	}
	// Backup-provided URLs, MIME values and trailing payloads are never trusted.
	st := state{Organization: Organization{AvatarURL: "https://evil.invalid/image"}, Users: map[string]userRecord{"u": {User: User{ID: "u", AvatarURL: "javascript:evil"}, Photo: &profilePhoto{Data: append(photoBytes(t, "png", 4, 4), []byte("trailing")...), MIME: "text/html"}}}}
	if e = normalizeBackupPhotos(&st); e != nil {
		t.Fatal(e)
	}
	if st.Organization.AvatarURL != "" || st.Users["u"].Photo.MIME != "image/png" || !strings.HasPrefix(st.Users["u"].AvatarURL, "/api/members/u/photo?") || bytes.Contains(st.Users["u"].Photo.Data, []byte("trailing")) {
		t.Fatal("unsafe backup photo")
	}
	st.OrganizationPhoto = &profilePhoto{Data: []byte("<svg/>"), MIME: "image/png"}
	if e = normalizeBackupPhotos(&st); e == nil {
		t.Fatal("unsafe backup photo accepted")
	}
}
