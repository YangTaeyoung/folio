# Path-based Artifact Content Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve artifact content at `<app-url>/c/{artifact}/` on the management host as the default, keeping per-artifact hostnames as an opt-in mode.

**Architecture:** The content handler stops keying on `r.Host` and keys on a *site key*: the lowercase host plus an optional path prefix (`app.test/c/demo`). In subdomain mode the key is the bare host, so stored `Viewer.Host` values and all existing behaviour are unchanged. In path mode the management `Handler()` strips `/c/<segment>` and calls the same content core with that key and prefix; requests whose host is not the app host (verified custom domains) go to the host-keyed content core.

**Tech Stack:** Go 1.26 (`net/http`, `httptest`), React/TypeScript (Vite) web UI.

**Spec:** `docs/superpowers/specs/2026-11-18-path-content-design.md`

## Global Constraints

- One `{artifact}` placeholder; its position (host vs path) selects the mode.
- Path mode: template scheme+host equal `--app-url`'s; placeholder is the last, whole path segment; prefix non-empty; prefix first segment not reserved.
- Default content URL when unset: `<app-url>/c/{artifact}` in CLI flag, `server.New` and `compose.yaml`.
- Path mode keeps content CSP exactly as `internal/server/content.go:141`, including `allow-same-origin`.
- Subdomain mode behaviour and all existing tests stay unchanged.
- Viewer cookies in path mode: `Path=/c/<segment>/`, name prefix `__Secure-` on HTTPS, none on HTTP.
- No mode is stored or detected from existing data.
- Code comments describe current behaviour only (no history, rationale or measurements).

## Review Focus

1. Path-mode request for `/c/<slug>` without trailing slash → redirect to `/c/<slug>/` so relative assets resolve under the prefix (test in Task 3).
2. A session cookie for the management app sent along with `/c/...` requests must never authorize content by itself; only viewer cookies or public visibility do (test in Task 3).
3. Slug segment containing uppercase or percent-encoding (`/c/Demo/`, `/c/%64emo/`) → 404, no alternative spelling of a site key (test in Task 3).
4. Private artifact opened anonymously at `/c/<slug>/` → 303 to `<app-url>/s/<slug>` without a `domain` parameter (test in Task 3).
5. Diff view in path mode shows no "preview domain" help with empty or duplicate hosts (Task 5).

---

### Task 1: Content URL template parsing and defaults

**Files:**
- Create: `internal/server/content_site.go`
- Modify: `internal/server/server.go:256-277` (`New` defaults and validation), `Server` struct (add fields)
- Test: `internal/server/content_site_test.go`

**Interfaces:**
- Produces:
  - `func ResolveContentURL(appURL, contentURL string) (ContentLayout, error)` (exported, used by `cmd/folio`)
  - `type ContentLayout struct { Template string; PathMode bool; Prefix string }` — `Template` is the effective template; `Prefix` is e.g. `"/c"` in path mode, `""` otherwise.
  - `Server` fields `pathMode bool`, `contentPrefix string`.

- [ ] **Step 1: Write the failing test** `TestResolveContentURLModes` (table-driven, app URL `http://app.test`):

| contentURL | want err | PathMode | Prefix | Template |
|---|---|---|---|---|
| `""` | no | true | `/c` | `http://app.test/c/{artifact}` |
| `http://app.test/c/{artifact}` | no | true | `/c` | same |
| `http://app.test/view/c/{artifact}` | no | true | `/view/c` | same |
| `http://{artifact}.content.test` | no | false | `""` | same |
| `http://other.test/c/{artifact}` | yes | | | |
| `https://app.test/c/{artifact}` | yes (scheme differs) | | | |
| `http://app.test/{artifact}` | yes (empty prefix) | | | |
| `http://app.test/api/{artifact}` | yes (reserved) | | | |
| `http://app.test/s/{artifact}`, `/g/`, `/assets/` | yes (reserved) | | | |
| `http://app.test/c/{artifact}/x` | yes | | | |
| `http://app.test/c/x{artifact}` | yes | | | |
| `http://app.test/c/{artifact}?q=1` | yes | | | |
| `http://app.test/c/{artifact}/{artifact}` | yes | | | |

Also `TestResolveContentURLKeepsHostModeRules`: `http://{artifact}.app.test` with app `http://app.test` is accepted only if today's rule accepts it (hostname differs from app hostname), and `http://app.test` (no placeholder) errors.

- [ ] **Step 2: Run** `go test ./internal/server -run TestResolveContentURL -v` → FAIL (undefined `ResolveContentURL`).

- [ ] **Step 3: Implement `ResolveContentURL` in `content_site.go`.** Host mode keeps the exact checks now at `server.go:271-276` (move them). Path mode: parse with placeholder replaced by `sample`, require `Path == prefix+"/sample"` with no query/fragment/user; reserved first segments are `api`, `s`, `g`, `assets`. Error messages: `"ContentURL path mode must use the management origin"`, `"ContentURL path placeholder must be the last whole segment after a non-empty prefix"`, `"ContentURL path prefix is reserved by the management app"`.

- [ ] **Step 4: Wire into `New`.** Replace the default at `server.go:263-265` and the validation at `271-276` with `ResolveContentURL(c.AppURL, c.ContentURL)` after `AppURL` is trimmed and validated; set `c.ContentURL = layout.Template`, `s.pathMode`, `s.contentPrefix`.

- [ ] **Step 5: Run** `go test ./internal/server/...` → all PASS (existing tests use explicit host templates).

- [ ] **Step 6: Commit** `feat(server): parse path-mode content URL templates`.

### Task 2: Site-key content core (no behaviour change in subdomain mode)

**Files:**
- Modify: `internal/server/content.go` (whole file), `internal/server/domains.go:223,319` (renamed helper), `internal/server/diff.go:187` (renamed helper only)
- Test: existing `internal/server/*_test.go` (regression)

**Interfaces:**
- Consumes: `s.pathMode`, `s.contentPrefix` (Task 1).
- Produces:
  - `func siteKey(u *url.URL) string` — `strings.ToLower(u.Host) + strings.TrimRight(u.Path, "/")`.
  - `func (s *Server) canonical(a Artifact) *url.URL` / `canonicalVersion(a, versionID)` — unchanged signatures; in path mode they return `<app-scheme>://<app-host><prefix>/<slug>` and `.../v--<digest32>` (same digest expression as today).
  - `func (s *Server) versionForSite(a Artifact, key string) string` (replaces `versionForHost`)
  - `func (s *Server) artifactForSite(key string) (Artifact, bool)` (replaces `artifactForHost`)
  - `func (s *Server) hostFor(a Artifact, hostname string) (*url.URL, bool)` — same signature; compares with `siteKey`, custom-domain branch unchanged.
  - `func (s *Server) contentSecure(a Artifact, key string) bool`
  - `func (s *Server) serveContent(w http.ResponseWriter, r *http.Request, key, prefix string)` — the body of today's `ContentHandler` closure with `r.Host` replaced by `key`, every redirect `"/render/..."` prefixed with `prefix`, and cookie `Path` set to `prefix+"/"` (or `"/"` when prefix is empty).
  - `func viewerCookieName(secure, pathScoped bool) string` — `__Host-folio_viewer` (secure, not path-scoped), `__Secure-folio_viewer` (secure, path-scoped), `folio_viewer` (insecure); `viewerVersionCookieName(secure, pathScoped bool, versionID string)` likewise.
  - `ContentHandler()` becomes: headers/method/closed checks are inside `serveContent`; handler calls `s.serveContent(w, r, strings.ToLower(r.Host), "")`.
  - `launch` stores `Viewer.Host = siteKey(host)` and returns `url: host.String() + "/grant/" + nonce`, `hostname: host.Host`.

- [ ] **Step 1: Refactor as above.** Lock handling stays as today (`s.mu` taken once in `serveContent`).
- [ ] **Step 2: Run** `go test ./...` → all PASS, unchanged test files. `git diff --stat -- '*_test.go'` → empty.
- [ ] **Step 3: Commit** `refactor(server): key artifact content by site instead of host`.

### Task 3: Path-mode dispatch in the management handler

**Files:**
- Modify: `internal/server/server.go:651-680` (`Handler`)
- Modify: `internal/server/content.go` (private-root redirect omits `domain` when `key` is a path-mode canonical site)
- Create: `internal/server/path_content_test.go`

**Interfaces:**
- Consumes: `serveContent`, `siteKey`, `s.pathMode`, `s.contentPrefix` (Tasks 1–2).
- Produces: test helper `func freshPath(t *testing.T) *client` — same as `fresh` with `ContentURL: "http://app.test/c/{artifact}"`; `client.call` already targets `Handler()`.

Dispatch rule, placed before the app-host check and before taking `s.mu`:
- path mode and `lower(r.Host) != s.appHost` → `s.ContentHandler().ServeHTTP(w, r)`.
- path mode and `r.URL.Path == prefix+"/"+seg` (no trailing slash) → 308 to `prefix+"/"+seg+"/"` (preserve query).
- path mode and `r.URL.Path` has prefix `prefix+"/"` → `seg` = next segment; reject if `seg` is empty or `seg != strings.ToLower(seg)` or `r.URL.RawPath != ""` (404); clone request with `URL.Path` = remainder (leading `/`), call `s.serveContent(w, r2, s.appHost+prefix+"/"+seg, prefix+"/"+seg)`.

- [ ] **Step 1: Write failing tests** in `path_content_test.go`, each using `freshPath` and `c.setup()`, requests through `c.s.Handler()`:
  - `TestPathModeGrantServesModulesAndRelativeAssets`: upload `index.html` = `<script type=module src='./app.js'></script>`, `app.js` = `import('./data.js')`, `data.js` = `export default 1`; launch → `url` has prefix `http://app.test/c/<slug>/grant/`; GET grant → 303, `Location` = `/c/<slug>/render/<id>/<vid>/index.html`, viewer cookie `Path` = `/c/<slug>/`, name `folio_viewer`; GET `/c/<slug>/render/<id>/<vid>/app.js` with cookie → 200 body `import('./data.js')`, CSP header equals subdomain-mode CSP string.
  - `TestPathModeRejectsCrossArtifactAndUnknownSites`: with artifact A public, GET `/c/<slugA>/render/<idB>/<vidB>/index.html` → 404; `/c/nope/` → 404; `/c/Demo/` (A's slug uppercased) → 404; `/c/%64emo/`-style percent-encoded slug → 404; `/c/<slugA>/render/<idA>/<vidA>/../x` → 404.
  - `TestPathModeTrailingSlashRedirect`: GET `/c/<slug>` → 308 `Location` `/c/<slug>/`.
  - `TestPathModeManagementSessionDoesNotAuthorizeContent`: private artifact; GET `/c/<slug>/index.html` with only the admin session cookie → 404; GET `/c/<slug>/` with only the session cookie → 303 `Location` `http://app.test/s/<slug>` (no `domain=`).
  - `TestPathModeVersionIsolation`: two versions, `isolatedVersion:true` launches → URLs under `/c/v--<digest>/grant/`, distinct; v1 cookie on `/c/v--<d1>/render/<id>/<v2>/index.html` → 404.
  - `TestPathModeAssetRequiresLiveSession`: mirror `TestRelativeModuleAssetRequiresLiveOriginatingSession` (`server_test.go:386`) through `/c/` URLs: after `POST /api/logout`, asset → 404.
  - `TestPathModeGuestLink`: create guest link with password via the API used in `internal/server/guests.go`, exchange it, launch → URL under `/c/<slug>/`, asset → 200.
  - `TestPathModeCustomDomainStillHostRouted`: reuse the verified-domain setup from `TestDomainProofHostBindingAndPrivateRedirect` (`integration_test.go:221`), request `http://<custom-host>/` through `Handler()` → same status as the subdomain test expects.
  - `TestPathModeManagementRoutesUnaffected`: `GET /api/me` → 200, `GET /s/<slug>` → SPA HTML.
- [ ] **Step 2: Run** `go test ./internal/server -run TestPathMode -v` → FAIL.
- [ ] **Step 3: Implement the dispatch rule and the redirect change.**
- [ ] **Step 4: Run** `go test ./... && go test -race ./internal/server` → PASS.
- [ ] **Step 5: Commit** `feat(server): serve artifact content under the management origin path`.

### Task 4: CLI, container and tunnel wiring

**Files:**
- Modify: `cmd/folio/main.go:211-268`
- Modify: `compose.yaml`, `Dockerfile` (CMD line)
- Test: `cmd/folio/main_test.go` (create if absent; follow existing `cmd/folio/*_test.go` style if present)

**Interfaces:**
- Consumes: `server.ResolveContentURL` (Task 1).

- [ ] **Step 1: Write failing test** `TestServePathModeSkipsContentListener`: start `serve` with `--data-dir t.TempDir() --listen 127.0.0.1:0 --content-listen 127.0.0.1:<port held by test listener> --dev` and no `--content-url`; it must start (no bind error on the held port) and print `Content: http://folio.localhost:4310/c/{artifact}`. If driving `serve` in-process is impractical, extract `func contentListenerNeeded(layout server.ContentLayout) bool` and test that instead.
- [ ] **Step 2: Run** `go test ./cmd/folio -v` → FAIL.
- [ ] **Step 3: Implement.** `--content-url` default `env("FOLIO_CONTENT_URL", "")`; call `server.ResolveContentURL` right after flag parsing; in path mode do not bind `--content-listen`, print one stderr line `Content listener disabled: content is served under the management origin` when `--content-listen`/`FOLIO_CONTENT_LISTEN` was set explicitly, and pass `ContentOrigin: listenerOrigin(l.Addr())` to `dns.Manager` so tunnel ingress for custom domains targets the management listener. Print the effective template.
- [ ] **Step 4: Container files.** `compose.yaml`: `FOLIO_CONTENT_URL: ${FOLIO_CONTENT_URL:-}`, drop the `4311` port mapping. `Dockerfile` CMD: `["serve", "--listen", "0.0.0.0:4310", "--data-dir", "/data"]`, `EXPOSE 4310`.
- [ ] **Step 5: Run** `go test ./... && docker compose config` → PASS, rendered config has one port.
- [ ] **Step 6: Commit** `feat(cli): default to path-mode content and skip the content listener`.

### Task 5: Web UI for path mode

**Files:**
- Modify: `internal/server/diff.go:187` (`previewHosts` is `[]string{}` in path mode)
- Modify: `web/src/WorkspaceFeatures.tsx:1075` (render the preview-domain help only when `previewHosts.length > 0`)
- Test: `internal/server/path_content_test.go` (`TestPathModeDiffHasNoPreviewHosts`: `GET /api/artifacts/:id/diff` → `previewHosts` is an empty array)

- [ ] **Step 1: Write the failing Go test; run** `go test ./internal/server -run TestPathModeDiff -v` → FAIL.
- [ ] **Step 2: Implement both changes.**
- [ ] **Step 3: Run** `go test ./... && (cd web && npm ci && npm run typecheck)` → PASS.
- [ ] **Step 4: Commit** `feat(web): hide content-domain help in path mode`.

### Task 6: Documentation

**Files:**
- Modify: `AGENTS.md:10`, `docs/superpowers/specs/2026-11-14-folio-design.md:23-25`, `README.md` (Build and run, Domains bullet), `INSTALL.md` (First run, LAN deployment, Docker, add "Upgrading to path mode" section), `docs/API.md:224` and the launch/`contentUrl` descriptions, `docs/dns.md` (content wildcard only in subdomain mode), `docs/COLLABORATION.md` (version preview domains)

- [ ] **Step 1: Edit.** `AGENTS.md:10` states both modes: path mode (default) runs artifact HTML on the management origin with `allow-same-origin`, an accepted risk for trusted publishers; subdomain mode keeps per-artifact origins and every isolation rule. `INSTALL.md` upgrade section: installations that never set `--content-url` switch to `<app-url>/c/{artifact}` and old `*.content.folio.localhost:4311` links stop working; set `--content-url 'http://{artifact}.content.folio.localhost:4311'` (and keep `--content-listen`) to stay on subdomain mode. One paragraph on the trade-off, copied from the spec's accepted-risk list.
- [ ] **Step 2: Verify** `grep -rn "content.folio.localhost:4311" README.md INSTALL.md docs/ AGENTS.md` → only in subdomain-mode and upgrade text.
- [ ] **Step 3: Commit** `docs: document path-mode content as the default`.

### Task 7: Real build and browser check

**Files:** none (results go to `~/dev/_qa/folio-path-content/`, outside the repo)

- [ ] **Step 1:** `make build && make test` → PASS.
- [ ] **Step 2:** `./dist/folio serve --data-dir <tmp>`; open the setup URL, create admin, publish an artifact with an ES module, a relative `fetch('./data.json')` and a `localStorage` write; open `/s/<slug>` in a real browser → content renders inside the viewer, module and fetch succeed, no console errors; open `/c/<slug>/` directly as the owner → renders; sign out → private content denied.
- [ ] **Step 3:** Restart with `--content-url 'http://{artifact}.content.folio.localhost:4311'` on the same data dir → subdomain viewer works as before.
- [ ] **Step 4:** Push branch, open PR to `main` with spec link, accepted-risk summary and the upgrade note.
