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

func workspaceUpload(t *testing.T, c *client, id string, meta map[string]any, files map[string]string) (Artifact, Version) {
	t.Helper()
	var b bytes.Buffer
	m := multipart.NewWriter(&b)
	paths := []string{"index.html"}
	for p := range files {
		if p != "index.html" {
			paths = append(paths, p)
		}
	}
	ff := []map[string]string{}
	for _, p := range paths {
		ff = append(ff, map[string]string{"path": p})
	}
	meta["files"] = ff
	raw, _ := json.Marshal(meta)
	m.WriteField("metadata", string(raw))
	for i, p := range paths {
		part, _ := m.CreateFormFile("file"+string(rune('0'+i)), p)
		io.WriteString(part, files[p])
	}
	m.Close()
	endpoint := "/api/artifacts"
	if id != "" {
		endpoint += "/" + id + "/versions"
	}
	r := httptest.NewRequest("POST", "http://app.test"+endpoint, &b)
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("Origin", "http://app.test")
	r.Header.Set("X-CSRF-Token", c.csrf)
	r.AddCookie(c.cookie)
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("upload %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Artifact Artifact
		Version  Version
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	return out.Artifact, out.Version
}
func workspaceContent(c *client, a Artifact, vid string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "http://"+a.Slug+".content.test/render/"+a.ID+"/"+vid+"/index.html", nil)
	w := httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(w, r)
	return w
}
func TestWorkspacePublishedIsolationSearchAndRealDiff(t *testing.T) {
	admin := fresh(t)
	admin.setup()
	owner := admin.member("workspace-owner@test.invalid")
	reader := admin.member("workspace-reader@test.invalid")
	a, v1 := workspaceUpload(t, owner, "", map[string]any{"title": "Search fixture", "visibility": "public", "provenance": map[string]any{"agent": "Local agent", "summary": "First request"}}, map[string]string{"index.html": "<h1>Visible apricot</h1>\n<p>Before</p>"})
	a, v2 := workspaceUpload(t, owner, a.ID, map[string]any{"baseVersionId": v1.ID, "draft": true, "title": "Secret draft title", "description": "Secret draft description", "provenance": map[string]any{"agent": "Local agent", "summary": "Private revision"}}, map[string]string{"index.html": "<h1>Secret dragonfruit</h1>\n<p>After</p>", "notes.txt": "draft-only-nectarine"})
	if !a.ReviewEnabled || a.PublishedVersionID != v1.ID || a.CurrentVersionID != v2.ID {
		t.Fatal("official pointer changed", a)
	}
	anon := &client{t: t, s: admin.s}
	out := reviewHTTP(t, anon, "GET", "/api/share/"+a.Slug, nil, 200)
	if out["artifact"].(map[string]any)["currentVersionId"] != v1.ID {
		t.Fatal("shared draft pointer")
	}
	if strings.Contains(string(mustJSON(out)), "Secret draft") {
		t.Fatal("draft metadata leaked", out)
	}
	if w := workspaceContent(admin, a, v2.ID); w.Code != 404 {
		t.Fatal("public explicit draft path readable", w.Code)
	}
	if w := workspaceContent(admin, a, v1.ID); w.Code != 200 || !strings.Contains(w.Body.String(), "apricot") {
		t.Fatal("official inaccessible", w.Code)
	}
	reviewHTTP(t, reader, "POST", "/api/artifacts/"+a.ID+"/launch", map[string]string{"versionId": v2.ID}, 404)
	reviewHTTP(t, reader, "GET", "/api/artifacts/"+a.ID+"/download?versionId="+v2.ID, nil, 404)
	items := reviewHTTP(t, reader, "GET", "/api/artifacts?q=dragonfruit", nil, 200)["artifacts"].([]any)
	if len(items) != 0 {
		t.Fatal("draft index leaked")
	}
	items = reviewHTTP(t, reader, "GET", "/api/artifacts?q=apricot", nil, 200)["artifacts"].([]any)
	if len(items) != 1 {
		t.Fatal("published body not searched")
	}
	items = reviewHTTP(t, owner, "GET", "/api/artifacts?q=nectarine", nil, 200)["artifacts"].([]any)
	if len(items) != 1 || !strings.Contains(items[0].(map[string]any)["searchSnippet"].(string), "nectarine") {
		t.Fatal("text body missing", items)
	}
	out = reviewHTTP(t, owner, "GET", "/api/artifacts/"+a.ID+"/diff?from="+v1.ID+"&to="+v2.ID+"&path=index.html", nil, 200)
	ff := out["files"].([]any)
	if ff[0].(map[string]any)["status"] != "modified" || len(ff[0].(map[string]any)["lines"].([]any)) < 2 {
		t.Fatal("not real line diff", out)
	}
	reviewHTTP(t, reader, "GET", "/api/artifacts/"+a.ID+"/diff?from="+v1.ID+"&to="+v2.ID, nil, 404)
	detail := reviewHTTP(t, reader, "GET", "/api/artifacts/"+a.ID, nil, 200)
	if len(detail["versions"].([]any)) != 1 {
		t.Fatal("draft metadata exposed")
	}
	if strings.Contains(string(mustJSON(detail)), "searchText") {
		t.Fatal("index exposed")
	}
	// The stored context of an immutable revision is unaffected by metadata edits.
	reviewHTTP(t, owner, "PATCH", "/api/artifacts/"+a.ID+"/metadata", map[string]any{"tags": []string{"team", "demo"}, "provenance": map[string]any{"summary": "Future context"}}, 200)
	if admin.s.st.Versions[a.ID][0].Provenance.Summary != "First request" || admin.s.st.Versions[a.ID][1].Provenance.Summary != "Private revision" {
		t.Fatal("version context mutated")
	}
	reviewHTTP(t, owner, "PATCH", "/api/artifacts/"+a.ID+"/metadata", map[string]any{"provenance": map[string]string{"repository": "https://user:secret@example.test/repo"}}, 400)
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func TestWorkspaceCollectionsNeverGrantAndFavoriteIsolation(t *testing.T) {
	c := fresh(t)
	c.setup()
	owner := c.member("collection-owner@test.invalid")
	reader := c.member("collection-reader@test.invalid")
	private, _ := workspaceUpload(t, owner, "", map[string]any{"title": "Hidden result"}, map[string]string{"index.html": "<h1>private-body-kiwi</h1>"})
	shared, _ := workspaceUpload(t, owner, "", map[string]any{"title": "Team result", "visibility": "organization"}, map[string]string{"index.html": "<h1>Shared</h1>"})
	out := reviewHTTP(t, owner, "POST", "/api/collections", map[string]any{"name": "Project", "visibility": "organization", "artifactIds": []string{private.ID, shared.ID}}, 201)
	id := out["collection"].(map[string]any)["id"].(string)
	out = reviewHTTP(t, reader, "GET", "/api/collections/"+id, nil, 200)
	if len(out["artifacts"].([]any)) != 1 || strings.Contains(string(mustJSON(out)), private.ID) {
		t.Fatal("collection widened permission", out)
	}
	reviewHTTP(t, reader, "GET", "/api/artifacts/"+private.ID, nil, 404)
	reviewHTTP(t, reader, "PATCH", "/api/collections/"+id, map[string]any{"name": "Hijack"}, 403)
	reviewHTTP(t, reader, "POST", "/api/collections", map[string]any{"name": "Guessed", "artifactIds": []string{private.ID}}, 404)
	if items := reviewHTTP(t, reader, "GET", "/api/artifacts?q=kiwi", nil, 200)["artifacts"].([]any); len(items) != 0 {
		t.Fatal("private body searched")
	}
	reviewHTTP(t, reader, "PUT", "/api/artifacts/"+shared.ID+"/favorite", map[string]bool{"favorite": true}, 200)
	if items := reviewHTTP(t, reader, "GET", "/api/artifacts?favorite=true", nil, 200)["artifacts"].([]any); len(items) != 1 {
		t.Fatal("favorite missing")
	}
	if items := reviewHTTP(t, owner, "GET", "/api/artifacts?favorite=true", nil, 200)["artifacts"].([]any); len(items) != 0 {
		t.Fatal("favorite leaked across users")
	}
	reviewHTTP(t, owner, "PATCH", "/api/collections/"+id, map[string]any{"artifactIds": []string{shared.ID, private.ID}}, 200)
	out = reviewHTTP(t, owner, "GET", "/api/collections/"+id, nil, 200)
	if out["collection"].(map[string]any)["artifactIds"].([]any)[0] != shared.ID {
		t.Fatal("ordering ignored")
	}
	reviewHTTP(t, owner, "DELETE", "/api/artifacts/"+shared.ID, nil, http.StatusNoContent)
	if out := reviewHTTP(t, reader, "GET", "/api/collections/"+id, nil, 200); len(out["artifacts"].([]any)) != 0 {
		t.Fatal("trash visible in collection")
	}
	reviewHTTP(t, owner, "POST", "/api/trash/"+shared.ID+"/restore", nil, 200)
	if out := reviewHTTP(t, reader, "GET", "/api/collections/"+id, nil, 200); len(out["artifacts"].([]any)) != 1 {
		t.Fatal("restored collection lost")
	}
}
func TestSearchHTMLSkipsExecutableCodeAndUnicode(t *testing.T) {
	text := visibleText([]byte("<script>hidden-secret</script><style>.secret{}</style><h1>日本語 &amp; Français</h1><template>hidden-template</template>"), true)
	if strings.Contains(text, "hidden") || !strings.Contains(text, "日本語 & Français") {
		t.Fatal(text)
	}
}
