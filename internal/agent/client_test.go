package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/YangTaeyoung/folio/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func fixture(t *testing.T) (*Client, func()) {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	origin := "http://" + l.Addr().String()
	s, e := server.New(server.Config{DataDir: t.TempDir(), AppURL: origin, ContentURL: "http://{artifact}.content.test", BootstrapToken: "setup-test"})
	if e != nil {
		t.Fatal(e)
	}
	hs := httptest.NewUnstartedServer(s.Handler())
	hs.Listener = l
	hs.Start()
	raw, _ := json.Marshal(map[string]string{"token": "setup-test", "organizationName": "Test", "name": "Admin", "email": "admin@example.test", "password": "test-password-123"})
	req, _ := http.NewRequest("POST", origin+"/api/setup", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	var setup map[string]any
	if e = json.NewDecoder(res.Body).Decode(&setup); e != nil {
		t.Fatal(e)
	}
	if res.StatusCode != 200 && res.StatusCode != 201 {
		t.Fatalf("setup %d %v", res.StatusCode, setup)
	}
	makeToken := func(scopes []string) string {
		raw, _ = json.Marshal(map[string]any{"name": "test", "scopes": scopes})
		req, _ = http.NewRequest("POST", origin+"/api/tokens", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		req.Header.Set("X-CSRF-Token", setup["csrfToken"].(string))
		for _, cookie := range res.Cookies() {
			req.AddCookie(cookie)
		}
		r, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		var out map[string]any
		json.NewDecoder(r.Body).Decode(&out)
		if r.StatusCode != 201 && r.StatusCode != 200 {
			t.Fatalf("token %d %v", r.StatusCode, out)
		}
		return out["token"].(string)
	}
	c, _ := NewClient(Config{origin, makeToken([]string{"read", "publish", "share"})})
	return c, func() { hs.Close(); s.Close() }
}
func TestMCPStdioProcess(t *testing.T) {
	c, closeFixture := fixture(t)
	defer closeFixture()
	site := t.TempDir()
	if e := os.Mkdir(filepath.Join(site, "assets"), 0700); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(site, "index.html"), []byte(`<script type="module" src="assets/app.js"></script>`), 0600)
	os.WriteFile(filepath.Join(site, "assets", "app.js"), []byte(`document.body.textContent="Folio MCP"`), 0600)
	// The test subprocess runs the actual SDK stdio server. Only JSON-RPC is
	// written to stdout, and the official client performs initialize/list/call.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMCPHelperProcess$")
	cmd.Env = append(os.Environ(), "FOLIO_MCP_HELPER=1", "FOLIO_URL="+c.URL, "FOLIO_TOKEN="+c.Token)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	session, e := mcp.NewClient(&mcp.Implementation{Name: "folio-test", Version: "1"}, nil).Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if e != nil {
		t.Fatalf("stdio initialize: %v %s", e, stderr.String())
	}
	defer session.Close()
	tools, e := session.ListTools(ctx, nil)
	if e != nil || len(tools.Tools) < 23 {
		t.Fatalf("list tools: %v %+v", e, tools)
	}
	resources, e := session.ListResources(ctx, nil)
	if e != nil || len(resources.Resources) != 1 {
		t.Fatalf("resources: %v", e)
	}
	prompts, e := session.ListPrompts(ctx, nil)
	if e != nil || len(prompts.Prompts) != 1 {
		t.Fatalf("prompts: %v", e)
	}
	call := func(name string, in any) map[string]any {
		t.Helper()
		out, e := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: in})
		if e != nil || out.IsError {
			t.Fatalf("%s: %v %+v", name, e, out)
		}
		raw, _ := json.Marshal(out.StructuredContent)
		var v map[string]any
		if e = json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	call("folio_capabilities", Empty{})
	created := call("folio_publish", PublishInput{Path: site, Title: "MCP integration"})
	a := created["artifact"].(map[string]any)
	id := a["id"].(string)
	if a["visibility"] != "private" {
		t.Fatal("publish default is not private")
	}
	detail := call("folio_inspect", IDInput{ID: id})
	versions := detail["versions"].([]any)
	if len(versions) != 1 || len(versions[0].(map[string]any)["files"].([]any)) != 2 {
		t.Fatal("relative files missing")
	}
	call("folio_share", ShareInput{ID: id, Visibility: "organization"})
	call("folio_archive", ArchiveInput{ID: id})
	call("folio_archive", ArchiveInput{ID: id, Restore: true})
	updated := call("folio_publish", PublishInput{Path: site, Title: "MCP revision", ArtifactID: id, BaseVersionID: a["currentVersionId"].(string)})
	if updated["version"].(map[string]any)["number"] != float64(2) {
		t.Fatal("revision not published")
	}

	official := updated["version"].(map[string]any)["id"].(string)
	os.WriteFile(filepath.Join(site, "index.html"), []byte("<h1>Revised text for review</h1>"), 0600)
	draft := call("folio_publish", PublishInput{Path: site, ArtifactID: id, BaseVersionID: official, Draft: true, Provenance: &CreationContext{Agent: "test agent", Summary: "Explicit request summary"}})
	vid := draft["version"].(map[string]any)["id"].(string)
	if draft["artifact"].(map[string]any)["publishedVersionId"] != official {
		t.Fatal("draft changed official link")
	}
	comment := call("folio_comment", CommentInput{ID: id, VersionID: vid, Body: "Human fixture requests spacing correction"})
	cid := comment["comment"].(map[string]any)["id"].(string)
	feedback := call("folio_feedback", FeedbackInput{ID: id, VersionID: vid})
	if len(feedback["comments"].([]any)) != 1 {
		t.Fatal("feedback missing")
	}
	call("folio_resolve_thread", ResolveInput{ID: id, CommentID: cid, Resolved: true})
	call("folio_review", ReviewInput{ID: id, VersionID: vid, Status: "in_review"})
	// Explicit approval fixture; the workflow tells real agents never to invent this decision.
	call("folio_review", ReviewInput{ID: id, VersionID: vid, Status: "approved"})
	promoted := call("folio_promote", PromoteInput{ID: id, VersionID: vid, BasePublishedVersionID: official})
	if promoted["artifact"].(map[string]any)["publishedVersionId"] != vid {
		t.Fatal("promotion missing")
	}
	diff := call("folio_compare_versions", DiffInput{ID: id, From: official, To: vid, Path: "index.html"})
	if diff["summary"].(map[string]any)["modified"] != float64(1) {
		t.Fatal("real comparison missing")
	}
	call("folio_save_collection", CollectionInput{Name: "MCP project", Visibility: "organization", ArtifactIDs: []string{id}})
	if len(call("folio_collections", Empty{})["collections"].([]any)) != 1 {
		t.Fatal("collection missing")
	}
	guest := call("folio_create_guest_link", GuestInput{ID: id, VersionID: vid, ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Password: "guest-test-password"})
	gid := guest["guestLink"].(map[string]any)["id"].(string)
	call("folio_revoke_guest_link", GuestRevokeInput{ID: id, LinkID: gid})
	call("folio_trash", TrashInput{ID: id, Action: "trash"})
	call("folio_trash", TrashInput{ID: id, Action: "restore"})
}
func TestMCPHelperProcess(t *testing.T) {
	if os.Getenv("FOLIO_MCP_HELPER") != "1" {
		return
	}
	cfg, _ := LoadConfig()
	c, e := NewClient(cfg)
	if e != nil {
		os.Exit(2)
	}
	if e = NewMCP(c, "test").Run(context.Background(), &mcp.StdioTransport{}); e != nil {
		os.Exit(3)
	}
	os.Exit(0)
}
func TestConfigAndFiles(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config", "config.json")
	t.Setenv("FOLIO_CONFIG", p)
	t.Setenv("FOLIO_URL", "")
	t.Setenv("FOLIO_TOKEN", "")
	want := Config{"http://localhost:4310", "secret-value"}
	if e := SaveConfig(want); e != nil {
		t.Fatal(e)
	}
	got, e := LoadConfig()
	if e != nil || got != want {
		t.Fatal(got, e)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0600 {
		t.Fatal(fi.Mode())
	}
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "index.html"), []byte("hi"), 0600)
	os.WriteFile(filepath.Join(d, ".env"), []byte("secret"), 0600)
	files, e := collectFiles(d)
	if e != nil || len(files) != 1 {
		t.Fatal(files, e)
	}
	if e = os.Symlink(filepath.Join(d, "index.html"), filepath.Join(d, "link.html")); e == nil {
		if _, e = collectFiles(d); e == nil {
			t.Fatal("symlink accepted")
		}
	}
	for _, u := range []string{"http://token@localhost", "https://example.test/path", "file:///tmp/test"} {
		if _, e = NewClient(Config{URL: u}); e == nil {
			t.Fatal("bad origin accepted", u)
		}
	}
}
func TestHTTPErrorNoCredentialLeak(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, `{"error":{"code":"unauthorized","message":"Sign in"}}`)
	}))
	defer s.Close()
	c, _ := NewClient(Config{s.URL, "do-not-print"})
	if _, e := c.JSON(context.Background(), "GET", "/api/me", nil); e == nil || bytes.Contains([]byte(e.Error()), []byte("do-not-print")) {
		t.Fatal(e)
	}
}

func TestRevisionPreservesUnspecifiedMetadata(t *testing.T) {
	c, closeFixture := fixture(t)
	defer closeFixture()
	site := t.TempDir()
	if e := os.WriteFile(filepath.Join(site, "index.html"), []byte("<h1>revision</h1>"), 0600); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	created, e := c.Publish(ctx, PublishInput{Path: site, Title: "Original", Description: "Keep this description"})
	if e != nil {
		t.Fatal(e)
	}
	a := created["artifact"].(map[string]any)
	id := a["id"].(string)
	retitled, e := c.Publish(ctx, PublishInput{Path: site, ArtifactID: id, BaseVersionID: a["currentVersionId"].(string), Title: "Renamed"})
	if e != nil {
		t.Fatal(e)
	}
	a = retitled["artifact"].(map[string]any)
	if a["title"] != "Renamed" || a["description"] != "Keep this description" {
		t.Fatalf("retitle lost metadata: %+v", a)
	}
	unchanged, e := c.Publish(ctx, PublishInput{Path: site, ArtifactID: id, BaseVersionID: a["currentVersionId"].(string)})
	if e != nil {
		t.Fatal(e)
	}
	a = unchanged["artifact"].(map[string]any)
	if a["title"] != "Renamed" || a["description"] != "Keep this description" {
		t.Fatalf("content-only revision lost metadata: %+v", a)
	}
	updated, e := c.Publish(ctx, PublishInput{Path: site, ArtifactID: id, BaseVersionID: a["currentVersionId"].(string), Description: "Changed description"})
	if e != nil {
		t.Fatal(e)
	}
	a = updated["artifact"].(map[string]any)
	if a["title"] != "Renamed" || a["description"] != "Changed description" {
		t.Fatalf("description edit lost title: %+v", a)
	}
}
