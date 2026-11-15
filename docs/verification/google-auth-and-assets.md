# Optional Google authentication and brand assets

Validated 2026-11-16 on the change based on `ce769a0`.

- Full Go race suite and vet passed; final Google-specific race and whole server suites passed after correcting reverse-proxy retry isolation.
- Independent review found a shared-IP Google retry quota; replaced it with per-browser soft friction plus bounded pending/concurrent work. Re-review found no remaining material issue.
- Existing 20-flow real Chromium suite passed with no page errors.
- New `web/tests/google_auth.py` passed nine checks against an isolated actual server: default off/password access, redacted settings, secret retention, Google-only member creation, state/nonce/PKCE redirect inspection, safe error rendering, responsive settings/login and zero external browser requests. This test intentionally does not contact Google and does not establish real OAuth-client success.
- Installer checksum/idempotency/preservation/upgrade and CLI/MCP-skill smoke checks passed.
- New generated illustration and local fonts/assets were inspected at desktop/mobile widths; horizontal overflow and JavaScript errors were absent. Impeccable detector reported no findings on changed authentication surfaces.

Actual Google client issuance, live account login and Mac Studio runtime checks are deployment gates and must be reported separately. See [Google setup](../GOOGLE_LOGIN.md) and [asset generation record](../design/brand-assets.md).
