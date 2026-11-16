package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func opsAdmin(c *client) principal {
	r := httptest.NewRequest("GET", "http://app.test/api/me", nil)
	r.AddCookie(c.cookie)
	return c.s.principal(r)
}
func opsArtifact(t *testing.T, c *client) Artifact {
	t.Helper()
	code, out := c.upload("", "", "index.html", "<h1>Portable private bytes</h1>")
	if code != 201 {
		t.Fatalf("publish %d %v", code, out)
	}
	return c.s.st.Artifacts[out["artifact"].(map[string]any)["id"].(string)]
}
func TestOpsTrashRestoreAndArchiveProtection(t *testing.T) {
	c := fresh(t)
	c.setup()
	a := opsArtifact(t, c)
	p := opsAdmin(c)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "http://app.test/api/artifacts/"+a.ID, nil)
	if e := c.s.trash(w, r, p, a); e != nil {
		t.Fatal(e)
	}
	a = c.s.st.Artifacts[a.ID]
	if a.DeletedAt == nil || a.DeletedBy != p.user.ID {
		t.Fatal("missing deletion attribution")
	}
	if c.s.read(p, a) {
		t.Fatal("deleted artifact readable")
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "http://app.test/api/trash/"+a.ID+"/restore", nil)
	if e := c.s.lifecycle(w, r, p); e != nil {
		t.Fatal(e)
	}
	if c.s.st.Artifacts[a.ID].DeletedAt != nil {
		t.Fatal("not restored")
	}
	old := now().Add(-31 * 24 * time.Hour)
	a = c.s.st.Artifacts[a.ID]
	a.DeletedAt = &old
	a.Archived = true
	c.s.st.Artifacts[a.ID] = a
	if e := c.s.purgeExpiredTrash(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, ok := c.s.st.Artifacts[a.ID]; !ok {
		t.Fatal("archived trash purged")
	}
	a.Archived = false
	c.s.st.Artifacts[a.ID] = a
	if e := c.s.purgeExpiredTrash(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, ok := c.s.st.Artifacts[a.ID]; ok {
		t.Fatal("expired trash retained")
	}
}
func TestOpsGuestPasswordSessionRevokeAndVersionBoundary(t *testing.T) {
	c := fresh(t)
	c.setup()
	a := opsArtifact(t, c)
	p := opsAdmin(c)
	raw, _ := json.Marshal(map[string]any{"versionId": a.CurrentVersionID, "expiresAt": now().Add(time.Hour), "password": "guest-password-123"})
	r := httptest.NewRequest("POST", "http://app.test/api/artifacts/"+a.ID+"/guest-links", bytes.NewReader(raw))
	w := httptest.NewRecorder()
	if e := c.s.guestLinks(w, r, p, a); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(w.Body.String(), "passwordHash") || strings.Contains(w.Body.String(), "$2") {
		t.Fatal("password verifier leaked")
	}
	var out struct{ GuestLink GuestLink }
	json.Unmarshal(w.Body.Bytes(), &out)
	g := c.s.st.GuestLinks[out.GuestLink.ID]
	persisted, _ := json.Marshal(c.s.st)
	var restored state
	json.Unmarshal(persisted, &restored)
	if restored.GuestLinks[g.ID].PasswordHash == "" {
		t.Fatal("password verifier not persisted")
	}
	get := httptest.NewRequest("GET", "http://app.test/api/guest/"+g.ID, nil)
	w = httptest.NewRecorder()
	if e := c.s.guest(w, get); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(w.Body.String(), a.Title) {
		t.Fatal("protected title disclosed")
	}
	unlock := func(password, origin string) (*httptest.ResponseRecorder, error) {
		raw, _ := json.Marshal(map[string]string{"password": password})
		r := httptest.NewRequest("POST", "http://app.test/api/guest/"+g.ID+"/unlock", bytes.NewReader(raw))
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		return w, c.s.guest(w, r)
	}
	if _, e := unlock("guest-password-123", ""); e == nil {
		t.Fatal("unlock without Origin accepted")
	}
	if _, e := unlock("wrong", "http://app.test"); e == nil {
		t.Fatal("wrong password accepted")
	}
	w, e := unlock("guest-password-123", "http://app.test")
	if e != nil {
		t.Fatal(e)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].Domain != "" {
		t.Fatal("guest cookie scope")
	}
	get.AddCookie(cookies[0])
	gp := c.s.guestPrincipal(get)
	if !c.s.guestCanRead(gp, a, g.VersionID) || c.s.guestCanRead(gp, a, "another-version") {
		t.Fatal("version pin boundary")
	}
	if c.s.read(principal{}, a) {
		t.Fatal("guest enabled anonymous read")
	}
	if c.s.require(get, gp, false, "read") == nil {
		t.Fatal("guest obtained organization access")
	}
	v := Viewer{ArtifactID: a.ID, VersionID: g.VersionID, Kind: "guest", Credential: gp.credential, ExpiresAt: now().Add(time.Hour)}
	if _, ok := c.s.guestViewerPrincipal(v); !ok {
		t.Fatal("active viewer rejected")
	}
	revoked := now()
	g.RevokedAt = &revoked
	c.s.st.GuestLinks[g.ID] = g
	if _, ok := c.s.guestViewerPrincipal(v); ok {
		t.Fatal("revoked existing asset viewer accepted")
	}
	if c.s.guestCanRead(gp, a, g.VersionID) {
		t.Fatal("revoked link readable")
	}
}
func TestOpsBackupRoundTripTamperAndWrongPassword(t *testing.T) {
	c := fresh(t)
	c.setup()
	a := opsArtifact(t, c)
	p := opsAdmin(c)
	originalVersion := c.s.st.Versions[a.ID][0]
	originalKey := originalVersion.Files[0].Key
	originalBackend := originalVersion.BackendID
	info, e := c.s.createBackup(context.Background(), "backup-password-123", p)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(c.s.backupDir(), info.ID+".folio"))
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte("Portable private bytes")) {
		t.Fatal("backup contains plaintext")
	}
	c.s.st.Artifacts[a.ID] = Artifact{}
	delete(c.s.st.Artifacts, a.ID)
	delete(c.s.st.Versions, a.ID)
	before, _ := json.Marshal(c.s.st)
	if e = c.s.restoreBackup(context.Background(), bytes.NewReader(raw), "wrong-password", p); e == nil {
		t.Fatal("wrong password accepted")
	}
	after, _ := json.Marshal(c.s.st)
	if !bytes.Equal(before, after) {
		t.Fatal("failed restore mutated state")
	}
	corrupt := bytes.Clone(raw)
	corrupt[len(corrupt)-1] ^= 1
	if e = c.s.restoreBackup(context.Background(), bytes.NewReader(corrupt), "backup-password-123", p); e == nil {
		t.Fatal("tampered backup accepted")
	}
	if e = c.s.restoreBackup(context.Background(), bytes.NewReader(raw[:len(raw)-20]), "backup-password-123", p); e == nil {
		t.Fatal("truncated backup accepted")
	}
	adminBefore := c.s.st.Users[p.user.ID]
	if e = c.s.restoreBackup(context.Background(), bytes.NewReader(raw), "backup-password-123", p); e != nil {
		t.Fatal(e)
	}
	v := c.s.st.Versions[a.ID][0]
	if v.BackendID == originalBackend || v.Files[0].Key == originalKey {
		t.Fatal("restore reused source keys")
	}
	if c.s.st.Users[p.user.ID].Hash != adminBefore.Hash {
		t.Fatal("admin password replaced")
	}
	st, e := c.s.store(v.BackendID)
	if e != nil {
		t.Fatal(e)
	}
	rd, e := st.Open(context.Background(), v.Files[0].Key)
	if e != nil {
		t.Fatal(e)
	}
	content, e := io.ReadAll(rd)
	rd.Close()
	if e != nil || !bytes.Contains(content, []byte("Portable private bytes")) {
		t.Fatalf("restored bytes %q %v", content, e)
	}
	source, e := c.s.store(originalBackend)
	if e != nil {
		t.Fatal(e)
	}
	rd, e = source.Open(context.Background(), originalKey)
	if e != nil {
		t.Fatal("restore deleted original object")
	}
	rd.Close()
}
func TestOpsBackupRejectsTraversalAndCompressedEntries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method uint16
	}{{"../outside", zip.Store}, {"manifest.json", zip.Deflate}} {
		var b bytes.Buffer
		zw := zip.NewWriter(&b)
		f, e := zw.CreateHeader(&zip.FileHeader{Name: tc.name, Method: tc.method})
		if e != nil {
			t.Fatal(e)
		}
		f.Write([]byte(`{}`))
		zw.Close()
		zr, e := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
		if e != nil {
			t.Fatal(e)
		}
		if _, _, e = validateBackup(zr); e == nil {
			t.Fatalf("unsafe archive accepted: %s", tc.name)
		}
	}
}
func TestOpsBackupAdminAndActiveUploadGuards(t *testing.T) {
	c := fresh(t)
	c.setup()
	p := opsAdmin(c)
	r := httptest.NewRequest("GET", "http://app.test/api/backups", nil)
	if e := c.s.backups(httptest.NewRecorder(), r, principal{}); e == nil {
		t.Fatal("anonymous backup access")
	}
	c.s.inFlight["active"] = 1
	if e := c.s.restoreBackup(context.Background(), strings.NewReader(""), "backup-password-123", p); e == nil {
		t.Fatal("restore raced active publish")
	}
}

func TestOpsGuestHandlerLoggedInBrowserAndAssetRevocation(t *testing.T) {
	c := fresh(t)
	c.setup()
	a := opsArtifact(t, c)
	code, out := c.upload(a.ID, a.CurrentVersionID, "index.html", "<h1>Guest draft</h1>")
	if code != 201 {
		t.Fatal(code, out)
	}
	a = c.s.st.Artifacts[a.ID]
	a.ReviewEnabled = true
	a.PublishedVersionID = c.s.st.Versions[a.ID][0].ID
	c.s.st.Artifacts[a.ID] = a
	code, out, _ = c.call("POST", "/api/artifacts/"+a.ID+"/guest-links", map[string]any{"versionId": a.CurrentVersionID, "expiresAt": now().Add(time.Hour), "password": "guest-password-123"})
	if code != 201 {
		t.Fatal(code, out)
	}
	id := out["guestLink"].(map[string]any)["id"].(string)
	// Admin session cookie is intentionally present: guest-CSRF owns this route.
	code, out, w := c.call("POST", "/api/guest/"+id+"/unlock", map[string]any{"password": "guest-password-123"})
	if code != 200 {
		t.Fatal(code, out)
	}
	gc := w.Result().Cookies()[0]
	csrf := out["csrfToken"].(string)
	r := httptest.NewRequest("POST", "http://app.test/api/guest/"+id+"/launch", strings.NewReader(`{}`))
	r.Header.Set("Origin", "http://app.test")
	r.Header.Set("X-CSRF-Token", csrf)
	r.AddCookie(c.cookie)
	r.AddCookie(gc)
	w = httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("guestCSRF blocked by admin session: %d %s", w.Code, w.Body)
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	r = httptest.NewRequest("GET", out["url"].(string), nil)
	w = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, r)
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body)
	}
	vc := w.Result().Cookies()[0]
	location := w.Header().Get("Location")
	contentURL := "http://" + a.Slug + ".content.test" + location
	read := func(url string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", url, nil)
		r.AddCookie(vc)
		w := httptest.NewRecorder()
		c.s.ContentHandler().ServeHTTP(w, r)
		return w
	}
	if w = read(contentURL); w.Code != 200 || !strings.Contains(w.Body.String(), "Guest draft") {
		t.Fatalf("pinned draft inaccessible: %d %s", w.Code, w.Body)
	}
	wrongURL := "http://" + a.Slug + ".content.test/render/" + a.ID + "/" + a.PublishedVersionID + "/index.html"
	if w = read(wrongURL); w.Code != 404 {
		t.Fatal("guest escaped version pin", w.Code)
	}
	code, out, _ = c.call("POST", "/api/artifacts/"+a.ID+"/guest-links/"+id+"/revoke", map[string]any{})
	if code != 204 {
		t.Fatal(code, out)
	}
	if w = read(contentURL); w.Code != 404 {
		t.Fatal("revoked cookie still reads asset", w.Code)
	}
}

type opsRemoteSnapshot struct {
	bytes         []byte
	puts, deletes int
}

func (st *opsRemoteSnapshot) Put(context.Context, string, io.Reader, int64, string) error {
	st.puts++
	return nil
}
func (st *opsRemoteSnapshot) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(st.bytes)), nil
}
func (st *opsRemoteSnapshot) Delete(context.Context, string) error { st.deletes++; return nil }
func (st *opsRemoteSnapshot) Test(context.Context) error           { return nil }
func TestOpsBackupIncludesRemoteObjectsWithoutSourceMutation(t *testing.T) {
	c := fresh(t)
	c.setup()
	a := opsArtifact(t, c)
	p := opsAdmin(c)
	remote := &opsRemoteSnapshot{bytes: []byte("<h1>Remote snapshot bytes</h1>")}
	c.s.st.Backends["remote-test"] = backendRecord{Backend: Backend{ID: "remote-test", Provider: "s3", Name: "Remote source"}}
	c.s.stores["remote-test"] = remote
	v := c.s.st.Versions[a.ID][0]
	v.BackendID = "remote-test"
	v.Files[0].Key = "remote/original"
	v.Files[0].Size = int64(len(remote.bytes))
	v.SizeBytes = int64(len(remote.bytes))
	c.s.st.Versions[a.ID] = []Version{v}
	a.SizeBytes = v.SizeBytes
	c.s.st.Artifacts[a.ID] = a
	info, e := c.s.createBackup(context.Background(), "backup-password-123", p)
	if e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(filepath.Join(c.s.backupDir(), info.ID+".folio"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = c.s.restoreBackup(context.Background(), f, "backup-password-123", p); e != nil {
		t.Fatal(e)
	}
	restored := c.s.st.Versions[a.ID][0]
	if restored.BackendID == "remote-test" || c.s.st.Backends[restored.BackendID].Provider != "local" {
		t.Fatal("remote restore not portable")
	}
	local, e := c.s.store(restored.BackendID)
	if e != nil {
		t.Fatal(e)
	}
	rd, e := local.Open(context.Background(), restored.Files[0].Key)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(rd)
	rd.Close()
	if !bytes.Equal(raw, remote.bytes) {
		t.Fatal("remote bytes missing")
	}
	if remote.puts != 0 || remote.deletes != 0 {
		t.Fatal("backup/restore mutated source cloud storage")
	}
}
func TestOpsTrashRestoreDoesNotReactivateGuestLinks(t *testing.T) {
	c := fresh(t)
	c.setup()
	a := opsArtifact(t, c)
	p := opsAdmin(c)
	g := GuestLink{ID: "testlink", ArtifactID: a.ID, VersionID: a.CurrentVersionID, CreatedAt: now(), ExpiresAt: now().Add(time.Hour)}
	c.s.st.GuestLinks[g.ID] = g
	c.s.st.GuestSessions["credential"] = GuestSession{LinkID: g.ID, ExpiresAt: now().Add(time.Hour)}
	if e := c.s.trash(httptest.NewRecorder(), httptest.NewRequest("DELETE", "http://app.test/api/artifacts/"+a.ID, nil), p, a); e != nil {
		t.Fatal(e)
	}
	if c.s.st.GuestLinks[g.ID].RevokedAt == nil {
		t.Fatal("trash left guest link active")
	}
	if e := c.s.lifecycle(httptest.NewRecorder(), httptest.NewRequest("POST", "http://app.test/api/trash/"+a.ID+"/restore", nil), p); e != nil {
		t.Fatal(e)
	}
	if c.s.guestActive(c.s.st.GuestLinks[g.ID]) {
		t.Fatal("restore reactivated prior guest access")
	}
	if len(c.s.st.GuestSessions) > 0 {
		t.Fatal("trash retained guest sessions")
	}
}
