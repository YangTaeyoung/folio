# Folio

MIT, self-hosted artifact publishing for people and AI agents. The user approved the Folio name and requested autonomous implementation and notification when all requested features work.

## Requirements
- Offline/on-prem core: no cloud, telemetry, CDN, email, or model provider needed at runtime.
- Go server/CLI/MCP in one executable; React/TypeScript web UI embedded at build time; SQLite metadata and filesystem default storage.
- Respect `docs/superpowers/specs/2026-11-14-folio-design.md` and `docs/API.md` as integration contracts.
- Consult official docs AND actual dependency code/types before making technical claims. Record sources in docs/references.
- Content has two modes, selected by where ContentURL puts `{artifact}` (`docs/superpowers/specs/2026-11-18-path-content-design.md`). Path mode (default, `<app-url>/c/{artifact}`) runs artifact HTML on the management origin with sandbox allow-same-origin: an accepted risk for trusted publishers, since artifact JavaScript can call management APIs with the viewer's session and artifacts share one origin. Subdomain mode gives every artifact its own execution hostname and must never run artifact HTML on the authenticated management origin. In both modes, check site/artifact binding and authorization for every asset; never rely on an unguessable URL.
- Default visibility private. Sharing: owner/private, organization, specific active members, public link. Administrators can manage all artifacts and users, clearly documented.
- Archive is protected from automatic cleanup; quota eviction is opt-in; all mutations audited.
- Never commit generated credentials, tokens, local databases, runtime data, private reports, or screenshots of private user surfaces.
- Keep each subsystem focused. Agents must edit only their assigned files, never spawn further agents, never commit another worker's changes, never push or deploy.
- Prefer meaningful integration tests over tests mirroring implementation. Finish with real UI and public-domain validation.
- Keep durable progress and rulings in `.work/progress.md` (gitignored).

## Delivery
Public MIT repository under YangTaeyoung/folio, reproducible release binaries, Docker, README one-prompt install, real test subdomains on the user's Cloudflare zone melona.cloud. Do not modify preexisting production records. Public deployment must preserve authorization for non-public content.
