# Collaboration and language release verification

## Delivered behavior

[v0.5.0](https://github.com/YangTaeyoung/folio/releases/tag/v0.5.0) adds version-bound comments and review, draft/official publication, ordered collections with tags and favorites, side-by-side previews and file/text differences, permission-filtered local body search, explicit per-version provenance, trash/recovery and encrypted portable backup/restore, and expiring/revocable password-protected guest links. The [collaboration guide](../COLLABORATION.md) records access rules and operational limits; the [API reference](../API.md) records the integration contract.

Korean remains the default. English, Japanese, Simplified Chinese and French are selectable throughout the application. All five dictionaries and fonts ship with the application; the preference persists locally and updates document language and application date/number formatting. Native operating-system controls retain their system language. User-authored content and diagnostic identifiers are not automatically translated.

The CLI and MCP server ship in the same executable. The MCP server exposes 23 tools, including the new collaboration workflows, and the embedded and portable agent skills describe the same tool contract.

## Source and release validation

- Reviewed [PR #1](https://github.com/YangTaeyoung/folio/pull/1) was merged as `1bf92710e371a571b46a9b35eac3631e85b0d8d5`. Its final reviewed tree and the release source tree match exactly. The annotated `v0.5.0` tag points to that commit.
- [Main CI](https://github.com/YangTaeyoung/folio/actions/runs/43911625415) and the [release workflow](https://github.com/YangTaeyoung/folio/actions/runs/43911724089) passed. Coverage includes Go race tests and vet, the production frontend build, installer/CLI checks, existing browser/authentication/responsive suites, and the new collaboration and language suites.
- All six downloaded public platform archives matched `checksums.txt` and contained the application/code/font license files. The tag-pinned public installer produced the same native executable as the downloaded macOS arm64 archive.
- Native release executable SHA-256: `1097d98c0f026c89732623faa6e42240988fcb790066d049c0d47b89ef7b7a39`.
- The new collaboration browser suite passed 19 checks, including an actual encrypted export and restore of fixture comments, ordered collections, version HTML and object bytes. Recovery handler tests also cover invalid passwords, corruption/truncation, identity remapping and remote-source snapshots without remote object writes or deletion.
- All five language catalogs passed key/interpolation checks. Desktop and narrow-screen browser runs checked the translated interface, document language, focus, overflow and preview behavior without external runtime requests. Existing browser and Google configuration regressions passed.
- Fresh independent code and visual finish reviews closed their material findings. Responsive checks covered 1440, 900, 390 and 320 CSS pixels. The visual review found no remaining material UI defects; unchanged free design alignment targets were inspected without a new paid generation.

## Deployment and live checks

Mac Studio runs the public macOS arm64 v0.5.0 executable under its existing scoped user LaunchAgent. The deployed executable hash matches the verified public archive. Management and content listeners remain separate and loopback-only. A complete stopped backup of the previous executable, launch configuration and data was retained before atomic binary replacement; the copied snapshot passed SQLite integrity and full-file/key comparisons.

After deployment and again after test cleanup, the existing four users, two artifacts and five versions matched the pre-upgrade API snapshot, including the old artifact/version fields and Google configuration. All twelve original object files and the encryption key were preserved. A fresh Google sign-in through the existing browser account completed after the upgrade; the account screen showed the Google-only administrator and a new server login audit event confirmed the fresh session. OAuth client, scopes and policy settings were not changed.

A disposable artifact with two versions was then tested through the public HTTPS installation and the actual released stdio MCP executable:

- All 23 MCP tools were discoverable; publishing, comments, collection creation, review and official-version promotion completed.
- An anonymous public reader received the official version and could not fetch draft assets or draft comments. The existing isolated member/selected-recipient authorization suites also passed; this production smoke did not create new organization members.
- Two concurrent private previews used different version execution origins. Each loaded its own root-absolute JavaScript module, stylesheet and delayed fetch under strict TLS validation.
- A password-protected guest opened only its pinned version, had no organization API access, and lost an already-open asset grant immediately after link revocation.
- Switching the fixture back to private denied anonymous access. Trash and restore preserved its working version. No page errors occurred.
- Public login headings and document language were checked in all five languages; the French mobile login had no horizontal document overflow.

The synthetic artifact, versions, collection, guest grants and temporary API token were removed. Its three temporary DNS records and tunnel routes were also removed. Original inventory and object counts were rechecked after cleanup. The shared tunnel connector was not restarted, and all preexisting ingress entries and DNS records were preserved.

## DNS and remaining operating boundaries

Five exact content hostnames were added for the five existing versions, using the same tunnel/content listener as the existing artifact hosts. No zone-wide wildcard was added. Future canonical artifact hosts and isolated version hosts need routing through the installation's normal DNS/tunnel setup; this deployment does **not** claim automatic registration for every future version. The UI exposes comparison hosts under **Preview domains**, and the [routing instructions](../COLLABORATION.md#version-preview-domains) cover internal wildcard or exact-host setup. File/text comparisons do not require content-host routing. Content requests still enforce exact artifact/version host binding and authorization on every asset.

Cloudflare's [wildcard DNS reference](https://developers.cloudflare.com/dns/manage-dns-records/reference/wildcard-dns-records/) and [tunnel ingress reference](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/do-more-with-tunnels/local-management/configuration-file/) were consulted before retaining the scoped exact-host configuration. Folio's actual binding is implemented in [content.go](../../internal/server/content.go#L17).

Encrypted restore was exercised against isolated installations and fixtures, not by replacing production data. Live third-party storage-account validation, a machine reboot, boot-before-login availability and a change to the existing Google Testing audience are outside this release's validation. Private production captures, identities, credentials and backup locations are excluded from the public repository.
