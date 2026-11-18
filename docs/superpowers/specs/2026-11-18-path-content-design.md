# Path-based artifact content

## Intent

Folio serves artifact content today on a per-artifact hostname (`{artifact}.content.example`).
Every installation therefore needs wildcard DNS, a wildcard TLS certificate and a second
ingress route. Behind Cloudflare Tunnel on a subdomain such as `folio.example.com`, the
content hosts become second-level names (`{artifact}.folio.example.com`) that Universal SSL
does not cover, so a paid certificate or a manually renewed one is required just to view an
artifact.

The owner decided that path-based content on the management host is the primary mode:

```
https://folio.example.com/c/{artifact}/...
```

One hostname, one certificate, one ingress route. Per-artifact hostnames remain available as
an opt-in mode.

## Approaches considered

| Approach | URL shape | Trade-off | Outcome |
|---|---|---|---|
| A. Dedicated content host + path | `https://folio-content.example.com/{artifact}/` | Management origin stays isolated; artifacts share one origin with each other; needs a second hostname | Not chosen |
| **B. Management host + path** | `https://folio.example.com/c/{artifact}/` | One hostname; artifact JS shares the management origin | **Chosen by the owner, with `allow-same-origin` kept (B1)** |
| B2. B with opaque-origin sandbox | same as B | Management API protected; breaks `localStorage`, credentialed ES modules and relative `fetch`; asset auth must move to path tokens | Not chosen |
| C. A with opaque-origin sandbox | same as A | Full isolation; same breakage as B2 | Not chosen |

## Decision on origin isolation (accepted risk)

Path mode serves artifact HTML on the **same origin as the management UI and API**, and keeps
the sandbox `allow-same-origin` token so artifacts keep working unchanged (ES modules,
relative `fetch`, `localStorage`).

Consequences the owner accepted:

- Artifact JavaScript runs with the viewer's management session. A malicious artifact opened
  by an administrator can call management APIs as that administrator (members, tokens,
  sharing, deletion).
- Artifacts share one origin with each other: one artifact can read another's `localStorage`
  and fetch other artifacts the viewer can read.

Rationale: Folio is used inside one team and by that team's agents; publishers are trusted.
Operators who need isolation choose subdomain mode. This decision replaces the
`AGENTS.md` requirement "Arbitrary artifact HTML must never run on the authenticated management
origin" for path mode only; subdomain mode keeps the original guarantees.

## Goals

- Path mode works with a single management hostname and no wildcard DNS or certificate.
- Path mode is the default for new installations.
- Subdomain installations with an explicit host-placeholder `--content-url` keep working after
  upgrade without config changes.
- Sharing, visibility, guest links, version links, review drafts, diff previews, ZIP
  download and custom domains behave the same in both modes from the user's point of view.

## Non-goals

- Mitigating same-origin risk in path mode (no opaque-origin sandbox, no CSRF hardening
  specific to artifact JS beyond what exists).
- Migrating an installation between modes automatically.
- Supporting root-absolute asset URLs (`/style.css`) inside artifacts in path mode.

- Changing or redeploying the melona.cloud installation (owner decision: out of scope). Real
  HTTPS validation of path mode happens on the target on-prem install
  (`folio.trychainshift.ai`) after release.

## Anti-goals

- Breaking a subdomain installation whose `--content-url` is set explicitly.
- Weakening subdomain mode's isolation guarantees or tests.

## Configuration contract

`--content-url` / `FOLIO_CONTENT_URL` keeps one `{artifact}` placeholder. Its position selects
the mode:

| Placeholder position | Mode | Example |
|---|---|---|
| Path | path mode | `https://folio.example.com/c/{artifact}` |
| Host | subdomain mode (current) | `https://{artifact}.content.example.com` |

Path mode rules:

- The template's scheme and host must equal `--app-url`'s scheme and host.
- The placeholder must be one whole path segment; the prefix before it must be non-empty and
  must not collide with management routes (`/api`, `/s`, static UI assets, setup paths).
- Default when `--content-url` is unset: `<app-url>/c/{artifact}`.

Subdomain mode keeps today's validation (`internal/server/server.go:271-276`).

### Defaults and upgrade (owner decision)

The default changes in all three places that pin it today: the CLI flag default
(`cmd/folio/main.go:217`), `server.New` (`internal/server/server.go:263-265`) and
`compose.yaml:10`. The `Dockerfile` CMD and compose port mappings stop requiring the content
listener. No mode is stored or detected from existing data: an installation that relied on the
old implicit default switches to path mode on upgrade, and its `*.content.folio.localhost:4311`
links stop working. The release notes and `INSTALL.md` tell operators to set `--content-url`
explicitly to keep subdomain mode. Installations that already set `--content-url` with a host
placeholder are unaffected.

The separate content listener (`--content-listen`) is unused in path mode. Custom domains
still arrive by `Host` and are served by the content handler (see below), so the management
listener dispatches non-app hosts to the content handler in path mode.

## Behaviour in path mode

- Canonical artifact URL: `<app-url>/c/<slug>/`. Version-isolated URL:
  `<app-url>/c/v--<digest32>/` (same digest as today's version host label,
  `internal/server/content.go:19`).
- The management handler routes `/c/...` to the content handler; the content handler resolves
  the artifact or version from the path segment instead of `r.Host`.
- Grant exchange, `/render/<artifactID>/<versionID>/...`, viewer re-check on every asset and
  path-traversal checks keep their current logic, relative to the `/c/<segment>` prefix.
- Viewer cookies are scoped with `Path=/c/<segment>/`. The `__Host-` prefix requires
  `Path=/`, so path mode uses `__Secure-` (HTTPS) or no prefix (HTTP), keeping
  HttpOnly/Secure/SameSite/Partitioned as today.
- Content response headers stay as today (`content.go:139-143`), including
  `sandbox ... allow-same-origin`.
- The management UI and API expose content locations as URLs, not hosts. Places that expose
  hosts today (`previewHosts` in `internal/server/diff.go:187`, `contentUrl` in the
  organization payload at `server.go:577`, launch responses at `content.go:93`) return values
  that work in both modes.
- Custom domains: a verified custom domain still binds one artifact and is served at its root,
  host-based, in both modes. Folio cannot add isolation the operator did not configure: in
  path mode, the custom domain is an extra host, not a replacement for the management origin.
- DNS/tunnel manager: in path mode, ingress needs only the management hostname plus any custom
  domains; no content wildcard.

## Documentation changes

- `AGENTS.md` requirement on origin isolation: state both modes and the accepted risk above.
- `docs/superpowers/specs/2026-11-14-folio-design.md`: the architecture section on distinct
  origins references this document for path mode.
- `README.md`, `INSTALL.md`, `docs/API.md`, `docs/dns.md`: path mode as default; subdomain mode
  as the isolation option, with the trade-off stated in one paragraph.

## Testing

- Integration tests for path mode covering: publish → share → launch → grant → render with
  relative assets and an ES module; version link; guest link with password; draft preview;
  diff preview; revocation between HTML and asset; path traversal under `/c/<segment>/`;
  unknown segment returns 404; management routes unaffected.
- Existing subdomain-mode tests keep passing unchanged.
- Config validation tests for both template shapes and for prefix collisions.
- Real UI check in a browser for path mode on a local install.

## Left to the builder

- Exact list of reserved path prefixes and the error message for collisions.
- How `--content-listen` is reported or rejected when set in path mode.
- Internal refactor of `canonical`/`canonicalVersion`/`hostFor`/`artifactForHost` into a
  mode-aware locator.
- Web UI changes needed to consume URLs instead of hosts.
