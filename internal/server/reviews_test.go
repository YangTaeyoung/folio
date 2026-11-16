package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func reviewDraft(t *testing.T, c *client, id, base string) Version {
	t.Helper()
	var body bytes.Buffer
	m := multipart.NewWriter(&body)
	meta, _ := json.Marshal(map[string]any{"title": "Review fixture", "baseVersionId": base, "draft": true, "files": []map[string]any{{"path": "index.html"}}})
	m.WriteField("metadata", string(meta))
	part, _ := m.CreateFormFile("file0", "index.html")
	part.Write([]byte("<html><body>Draft</body></html>"))
	m.Close()
	endpoint := "/api/artifacts"
	if id != "" {
		endpoint += "/" + id + "/versions"
	}
	req := httptest.NewRequest("POST", "http://app.test"+endpoint, &body)
	req.Header.Set("Content-Type", m.FormDataContentType())
	req.Header.Set("Origin", "http://app.test")
	req.Header.Set("X-CSRF-Token", c.csrf)
	req.AddCookie(c.cookie)
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("draft upload: %d %s", w.Code, w.Body.String())
	}
	var out struct{ Version Version }
	json.Unmarshal(w.Body.Bytes(), &out)
	return out.Version
}
func reviewIdentity(t *testing.T, c *client) string {
	t.Helper()
	code, out, _ := c.call("GET", "/api/me", nil)
	if code != 200 {
		t.Fatal(code, out)
	}
	return out["user"].(map[string]any)["id"].(string)
}
func reviewHTTP(t *testing.T, c *client, method, path string, body any, want int) map[string]any {
	t.Helper()
	status, out, w := c.call(method, path, body)
	if status != want {
		t.Fatalf("%s %s = %d, want %d: %s", method, path, status, want, w.Body.String())
	}
	return out
}

func TestVersionReviewsDraftIsolationAndPromotion(t *testing.T) {
	admin := fresh(t)
	admin.setup()
	owner := admin.member("review-owner@test.invalid")
	reviewer := admin.member("reviewer@test.invalid")
	reader := admin.member("reader@test.invalid")
	_, initial := owner.upload("", "", "index.html", "<html>Published</html>")
	id := initial["artifact"].(map[string]any)["id"].(string)
	published := initial["version"].(map[string]any)["id"].(string)
	base := "/api/artifacts/" + id
	reviewHTTP(t, owner, "PATCH", base+"/sharing", map[string]any{"visibility": "organization"}, 200)
	draft := reviewDraft(t, owner, id, published)
	// An ordinary member cannot infer draft review metadata, locations, or comments.
	for _, route := range []string{"/comments", "/review"} {
		reviewHTTP(t, reader, "GET", base+route+"?versionId="+draft.ID, nil, 404)
	}
	reviewHTTP(t, &client{t: t, s: admin.s}, "GET", base+"/comments?versionId="+published, nil, 404)
	reviewHTTP(t, owner, "GET", base+"/review", nil, 400)
	reviewHTTP(t, owner, "PATCH", base+"/reviewers", map[string]any{"reviewerIds": []string{reviewIdentity(t, reviewer)}}, 200)
	reviewHTTP(t, reviewer, "GET", base+"/review?versionId="+draft.ID, nil, 200)
	// Approval on another version does not authorize this draft.
	reviewHTTP(t, owner, "POST", base+"/promote", map[string]any{"versionId": draft.ID, "basePublishedVersionId": published}, 409)
	reviewHTTP(t, reviewer, "PUT", base+"/review", map[string]any{"versionId": draft.ID, "status": "approved"}, 200)
	reviewHTTP(t, reviewer, "POST", base+"/promote", map[string]any{"versionId": draft.ID, "basePublishedVersionId": published}, 403)
	newer := reviewDraft(t, owner, id, draft.ID)
	reviewHTTP(t, owner, "POST", base+"/promote", map[string]any{"versionId": draft.ID}, 400)
	reviewHTTP(t, owner, "POST", base+"/promote", map[string]any{"versionId": draft.ID, "basePublishedVersionId": "stale"}, 409)
	reviewHTTP(t, owner, "POST", base+"/promote", map[string]any{"versionId": draft.ID, "basePublishedVersionId": published}, 200)
	a := admin.s.st.Artifacts[id]
	if a.PublishedVersionID != draft.ID || a.CurrentVersionID != newer.ID {
		t.Fatalf("publication overwrote working pointer: %+v", a)
	}
	out := reviewHTTP(t, reader, "GET", base, nil, 200)
	if out["artifact"].(map[string]any)["currentVersionId"] != draft.ID {
		t.Fatal("reader saw working version", out)
	}
	reviewHTTP(t, reader, "GET", base+"/review?versionId="+newer.ID, nil, 404)
	reviewHTTP(t, reader, "GET", base+"/comments?versionId="+draft.ID, nil, 200)
	// Explicit review assignment never expands the underlying sharing ACL.
	reviewHTTP(t, owner, "PATCH", base+"/sharing", map[string]any{"visibility": "private"}, 200)
	reviewHTTP(t, reviewer, "GET", base+"/review?versionId="+newer.ID, nil, 404)
	reviewHTTP(t, owner, "PATCH", base+"/sharing", map[string]any{"visibility": "organization"}, 200)
	reviewHTTP(t, owner, "PATCH", base+"/reviewers", map[string]any{"reviewerIds": []string{}}, 200)
	reviewHTTP(t, reviewer, "GET", base+"/review?versionId="+newer.ID, nil, 404)
}

func TestReviewThreadAuthorPermissionsAndReadOnlyToken(t *testing.T) {
	admin := fresh(t)
	admin.setup()
	owner := admin.member("thread-owner@test.invalid")
	author := admin.member("thread-author@test.invalid")
	_, initial := owner.upload("", "", "index.html", "<html>Published</html>")
	id := initial["artifact"].(map[string]any)["id"].(string)
	v := initial["version"].(map[string]any)["id"].(string)
	base := "/api/artifacts/" + id
	reviewHTTP(t, owner, "PATCH", base+"/sharing", map[string]any{"visibility": "organization"}, 200)
	legacyReview := reviewHTTP(t, author, "GET", base+"/review?versionId="+v, nil, 200)
	if legacyReview["review"].(map[string]any)["status"] != "approved" {
		t.Fatal("legacy publication lost approved state")
	}
	reviewHTTP(t, author, "PUT", base+"/review", map[string]any{"versionId": v, "status": "changes_requested"}, 200)
	if admin.s.st.Reviews[v].Status != "changes_requested" || admin.s.st.Artifacts[id].CurrentVersionID != v {
		t.Fatal("published version review state was not independently editable")
	}
	reviewHTTP(t, author, "POST", base+"/comments", map[string]any{"versionId": v, "body": "Inspect this", "path": "missing.html"}, 400)
	root := reviewHTTP(t, author, "POST", base+"/comments", map[string]any{"versionId": v, "body": "Inspect this", "path": "index.html", "anchor": "heading"}, 201)["comment"].(map[string]any)["id"].(string)
	reply := reviewHTTP(t, owner, "POST", base+"/comments", map[string]any{"versionId": v, "parentId": root, "body": "Updated"}, 201)["comment"].(map[string]any)["id"].(string)
	reviewHTTP(t, owner, "POST", base+"/comments", map[string]any{"versionId": v, "parentId": reply, "body": "Nested"}, 400)
	reviewHTTP(t, owner, "PATCH", base+"/comments/"+root, map[string]any{"body": "Unauthorized edit"}, 403)
	reviewHTTP(t, owner, "DELETE", base+"/comments/"+root, nil, 403)
	reviewHTTP(t, author, "PATCH", base+"/comments/"+root, map[string]any{"body": "Please inspect"}, 200)
	reviewHTTP(t, owner, "PATCH", base+"/comments/"+root, map[string]any{"resolved": true}, 200)
	reviewHTTP(t, author, "POST", base+"/comments", map[string]any{"versionId": v, "parentId": root, "body": "One more"}, 409)
	reviewHTTP(t, author, "PATCH", base+"/comments/"+root, map[string]any{"resolved": false}, 200)
	reviewHTTP(t, admin, "PATCH", base+"/comments/"+reply, map[string]any{"body": "Admin edit"}, 200)
	token := reviewHTTP(t, author, "POST", "/api/tokens", map[string]any{"name": "Read only", "scopes": []string{"read"}}, 201)["token"].(string)
	for _, route := range []string{"/comments", "/review", "/promote"} {
		raw, _ := json.Marshal(map[string]any{"versionId": v, "body": "Token write", "status": "approved", "basePublishedVersionId": v})
		method := "POST"
		if route == "/review" {
			method = "PUT"
		}
		req := httptest.NewRequest(method, "http://app.test"+base+route, bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		admin.s.Handler().ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatalf("readonly token %s: %d %s", route, w.Code, w.Body.String())
		}
	}
	reviewHTTP(t, author, "DELETE", base+"/comments/"+root, nil, 204)
	out := reviewHTTP(t, owner, "GET", base+"/comments?versionId="+v, nil, 200)["comments"].([]any)
	if len(out) != 2 {
		t.Fatal("delete lost replies", out)
	}
	for _, raw := range out {
		c := raw.(map[string]any)
		if c["id"] == root && (c["deleted"] != true || c["body"] != "" || c["path"] != nil || c["anchor"] != nil) {
			t.Fatal("deleted text/location retained", c)
		}
	}
	if len(admin.s.st.Audit) == 0 {
		t.Fatal("missing audit")
	}
}

func TestReviewMutationRollbackPreservesDecisionAndPublication(t *testing.T) {
	c := fresh(t)
	c.setup()
	_, initial := c.upload("", "", "index.html", "<html>Published</html>")
	id := initial["artifact"].(map[string]any)["id"].(string)
	v := initial["version"].(map[string]any)["id"].(string)
	base := "/api/artifacts/" + id
	draft := reviewDraft(t, c, id, v)
	reviewHTTP(t, c, "PUT", base+"/review", map[string]any{"versionId": draft.ID, "status": "approved"}, 200)
	auditCount := len(c.s.st.Audit)
	c.s.db.Close()
	reviewHTTP(t, c, "PUT", base+"/review", map[string]any{"versionId": draft.ID, "status": "changes_requested"}, 500)
	if c.s.st.Reviews[draft.ID].Status != "approved" || len(c.s.st.Audit) != auditCount {
		t.Fatal("failed review mutation leaked")
	}
	reviewHTTP(t, c, "POST", base+"/promote", map[string]any{"versionId": draft.ID, "basePublishedVersionId": v}, 500)
	if c.s.st.Artifacts[id].PublishedVersionID != v || len(c.s.st.Audit) != auditCount {
		t.Fatal("failed publication mutation leaked")
	}
	for _, version := range c.s.st.Versions[id] {
		if version.ID == draft.ID && version.Published {
			t.Fatal("failed publication exposed draft")
		}
	}
}

func TestFirstDraftReviewerAndEmptyPublicationBase(t *testing.T) {
	owner := fresh(t)
	owner.setup()
	reviewer := owner.member("first-draft-reviewer@test.invalid")
	reader := owner.member("first-draft-reader@test.invalid")
	draft := reviewDraft(t, owner, "", "")
	base := "/api/artifacts/" + draft.ArtifactID
	reviewHTTP(t, owner, "PATCH", base+"/sharing", map[string]any{"visibility": "organization"}, 200)
	reviewHTTP(t, reader, "GET", base, nil, 404)
	reviewHTTP(t, owner, "PATCH", base+"/reviewers", map[string]any{"reviewerIds": []string{reviewIdentity(t, reviewer)}}, 200)
	reviewHTTP(t, reviewer, "GET", base+"/review?versionId="+draft.ID, nil, 200)
	reviewHTTP(t, reviewer, "POST", base+"/comments", map[string]any{"versionId": draft.ID, "body": "First publication checked"}, 201)
	reviewHTTP(t, reviewer, "PUT", base+"/review", map[string]any{"versionId": draft.ID, "status": "approved"}, 200)
	reviewHTTP(t, owner, "POST", base+"/promote", map[string]any{"versionId": draft.ID, "basePublishedVersionId": ""}, 200)
	reviewHTTP(t, reader, "GET", base, nil, 200)
}
