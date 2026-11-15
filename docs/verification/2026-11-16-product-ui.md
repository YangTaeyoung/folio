# Product UI redesign verification

## Implementation

The v0.4.0 interface follows the [product research and composition strategy](../design/2026-11-16-product-experience-strategy.md). The scope includes authentication, artifact collections and list view, reading and versions, sharing, publishing, members, storage and retention, domains, agents, organization/Google settings, account and audit views.

The login uses an original transparent folded-paper asset on a page canvas. The library puts actual artifact previews before compact metadata. Reading uses a wide interactive canvas with contextual properties. Sharing distinguishes saved access from the pending choice. Administrative sections use aligned rows and progressive setup help. All assets and fonts remain embedded; reference-product screenshots and generated UI mockups are not runtime assets.

The API, persistence, role rules, content-host isolation, Google OAuth backend and cleanup/archive semantics are unchanged. The layout correction for narrow membership tables, storage summaries and scrolling provider forms is covered by a new behavioral browser suite, included in CI. Thumbnail composition remains stable between grid and list layouts.

## Source validation

Completed against a combined `make build VERSION=v0.4.0` binary:

- Go race tests and vet passed.
- Frontend typecheck and production build passed.
- Installer checksum/idempotency/existing-file/upgrade checks passed.
- CLI setup, doctor, publish/revision/share/archive/restore, skill install and shutdown checks passed.
- Existing 20-flow browser suite passed, including selected/organization/public sharing, revocation, relative modules, workers, versions and archive.
- Nine Google authentication/configuration checks passed, including conditional sign-in, secret redaction/retention, Google-only provisioning and authorization-start state/nonce/PKCE inspection.
- Responsive administration checks passed at 1440, 390 and 320 CSS pixels for membership controls, usage text and AWS/R2/GCS/Azure form footers.
- A separate authored-fixture capture pass covered 32 desktop/mobile states and nine disclosures, with no page errors, unexpected console errors, broken local images, external requests or horizontal document overflow.

Public [library](../images/library-desktop.png), [sharing](../images/sharing-desktop.png) and [login](../images/login-desktop.png) captures are actual local application screens with authored artifacts and test identities. Optional Google sign-in is enabled in the login example. The wider private capture set does not include real installation data or credentials.

## Design review

Four library alternatives were inspected before selecting the preview-led composition. Six sibling screens and their HTML exports were obtained before the design service allowance was exhausted. No additional balance was purchased. Seven matching-target alignment kits reused unchanged converted LayerDocs; their image-input plans are unanchored specifications, not automated DOM/pixel-fidelity scores. Actual application captures and behavior checks supply the delivery evidence.

The independent finish review found no material UI defects. Its documentation reconciliation is handled separately from runtime code; the final design reference records the built interface.

## Deployment record

- Reviewed implementation: `7ff5f60c5cdf912fa832f6b2e132f6ae9030e7dd`; [source CI](https://github.com/YangTaeyoung/folio/actions/runs/43745719674) passed.
- [v0.4.0 release](https://github.com/YangTaeyoung/folio/releases/tag/v0.4.0) and [release workflow](https://github.com/YangTaeyoung/folio/actions/runs/43745960639) completed. All six public archive checksums and code/font license entries passed. The public tag-pinned installer produced the same native binary as the downloaded archive; installer, CLI, browser, Google and responsive checks passed against that released binary.
- Mac Studio now runs the downloaded macOS arm64 release under the existing scoped user LaunchAgent. Live binary SHA-256: `9ecd3e78f51e1129010957061a1475cb1f28f15806ce75d0a662ea3ebe315357`. Both existing management/content listeners remain loopback-only. Cloudflare DNS/tunnel configuration and unrelated services were not modified.
- A complete stopped data/binary/plist backup was retained privately. An initial backup-reader check stopped the upgrade before binary replacement; the previous service was restored, the copied snapshot was verified with SQLite immutable read mode, and the upgrade was completed. The successful backup passed integrity, key comparison and complete file-hash checks. Immutable mode was used only on a stopped copied snapshot without a WAL sidecar, never on the live database; see [SQLite URI semantics](https://www.sqlite.org/uri.html) and [WAL read-only considerations](https://www.sqlite.org/wal.html).
- Public HTTPS renders the new login artwork and library. Existing four users, two artifacts, five revisions and the Google configuration compare unchanged against the pre-upgrade API snapshot. Private anonymous access is denied, authenticated content stays on its canonical isolated hostname, and logout revokes private viewer access.
- A fresh sign-in through the existing Chrome Google account completed after deployment. The account page confirmed the Google-only administrator, and the server audit recorded a new Google login after the upgrade. No OAuth client, scopes or Google policy settings were changed.
- The released CLI authenticates against the public installation; its MCP server lists all nine tools and successfully inspects an existing artifact without publishing or changing its access.

The Google project's existing Testing audience and user-login LaunchAgent autostart configuration remain unchanged. This UI release does not claim a machine reboot test or boot-before-login availability.
