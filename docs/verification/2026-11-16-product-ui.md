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

Runtime deployment and public release evidence will be recorded here after the reviewed source is published and the release archive is installed. Source checks above are not a claim that the public installation has already been upgraded.
