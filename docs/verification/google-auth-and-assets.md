# Optional Google authentication and brand assets

Validated 2026-11-16 on the change based on `ce769a0`.

- Full Go race suite and vet passed; final Google-specific race and whole server suites passed after correcting reverse-proxy retry isolation.
- Independent review found a shared-IP Google retry quota; replaced it with per-browser soft friction plus bounded pending/concurrent work. Re-review found no remaining material issue.
- Existing 20-flow real Chromium suite passed with no page errors.
- New `web/tests/google_auth.py` passed nine checks against an isolated actual server: default off/password access, redacted settings, secret retention, Google-only member creation, state/nonce/PKCE redirect inspection, safe error rendering, responsive settings/login and zero external browser requests. This test intentionally does not contact Google and does not establish real OAuth-client success.
- Installer checksum/idempotency/preservation/upgrade and CLI/MCP-skill smoke checks passed.
- New generated illustration and local fonts/assets were inspected at desktop/mobile widths; horizontal overflow and JavaScript errors were absent. Impeccable detector reported no findings on changed authentication surfaces.

## Deployment verification

Source `3dfab2f` was released as `v0.3.0`. Source CI `43732757200` and release `43732760644` succeeded. All six public archive checksums and both bundled font licenses were verified. The public one-command installer was exercised on the development Mac and Mac Studio; their installed binary SHA-256 matched. The published binary passed both the 20-flow regression and nine-flow Google settings suites.

The existing installation was stopped cleanly before its database, encryption key and content were copied. SQLite integrity and all 18 copied file hashes passed. All three existing users, two artifacts and five retained versions survived the migration. A dedicated native user LaunchAgent runs the verified public binary on Mac Studio; it starts at user login and restarts the application. Host reboot before user login was not tested. Only the four existing Folio DNS records were migrated; unrelated tunnel ingress rules were preserved.

Actual HTTPS checks on `folio.melona.cloud` passed for password administration, both canonical artifact hosts, anonymous private-content denial, public demo, private viewer revocation after logout, generated assets and mobile form order.

A separate Google Cloud project and web OAuth client were created by the operator. The operator's precreated Google-only administrator completed real Google account selection and consent, returned to Folio, and showed administrator status and Google account management on the Account page. The server persisted the subject binding and matching `login.google` audit event. Configuration and audit responses did not expose the client secret. Private identity, credential and account captures remain outside the repository. The Google app is currently in Testing mode with the operator registered as a test user; expanding Google-side access requires its audience configuration in Google Cloud.

See [Google setup](../GOOGLE_LOGIN.md) and [asset generation record](../design/brand-assets.md).
