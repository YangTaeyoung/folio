# Appearance, controls and profile release verification

## Delivered behavior

[v0.6.0](https://github.com/YangTaeyoung/folio/releases/tag/v0.6.0) adds a persistent system/light/dark preference across authentication, workspace, shared and guest chrome. The Folio fold mark now supplies matching SVG/ICO, Apple touch and application icons. Local service marks identify storage, Google/Cloudflare configuration and agent clients. The shared Radix Select replaces 17 native application dropdowns with styled menus, service icons, keyboard navigation, selected indicators, viewport limits and visible required-field feedback.

Members can edit their own name/photo under **My account**; administrators can edit the organization photo under **Organization settings**. JPEG/PNG uploads are bounded, resized and re-encoded on the server, stored locally, delivered only to authenticated members, and included in encrypted recovery. Workspace/member/author avatars use the saved image with an initials fallback. Anonymous artifact projections omit private photo URLs.

The **Agents** page now provides registration and verification commands for Codex, Claude Code, Gemini CLI, VS Code/Copilot, Cursor and OpenCode, with separate terminal-environment guidance. Cursor retains an explicit existing-file merge step; commands do not silently overwrite another client's configuration or embed Folio tokens. [Appearance references](../references/appearance.md), [photo contract](../references/profile-photos.md), [agent command evidence](../references/agent-connections.md) and [service asset provenance](../design/service-icons.md) record the underlying sources and bounds. All five application languages remain supported.

## Source and release evidence

- [PR #2](https://github.com/YangTaeyoung/folio/pull/2) passed [CI](https://github.com/YangTaeyoung/folio/actions/runs/44124705618) at `ceb06105189f6a2d45f09ffecf88cf3077694cb7`. The merged source and annotated `v0.6.0` tag point to `a70526c75625820ee72860381a798a68bed81ec4`; reviewed and merged tree IDs match exactly.
- [Main CI](https://github.com/YangTaeyoung/folio/actions/runs/44125841280) and [release workflow](https://github.com/YangTaeyoung/folio/actions/runs/44125859598) passed. Checks include Go race/vet, production build, installer/CLI smoke, existing 20-flow browser, Google configuration, collaboration and responsive suites, five-language checks, command-generation tests and the new appearance/agent browser suites.
- All six downloaded public archives matched the complete checksum manifest and included the code/font license entries. The tag-pinned public installer produced the same macOS arm64 executable as the downloaded release archive.
- Native executable SHA-256: `49d9363b2287207eb7ec7aaf3f88d562dbfe3e226af4686f68213dd34cda48af`. Its Go build metadata identifies the tagged source revision and `vcs.modified=false`.
- The actual downloaded executable also passed the isolated appearance/photo/menu browser suite, not just the development build.

## Contrast and interaction checks

The existing light palette produced several sub-threshold supporting labels and control boundaries. Those were corrected with semantic text and field-stroke roles, while decorative dividers retain a quieter treatment. A final account-avatar contrast regression was caught and corrected against its actual button background. Review also corrected a dark capacity meter with indistinguishable fill/track, and a required Select placeholder which bypassed native validation. Invalid selection now keeps constraint validation while directing focus and localized feedback to the visible trigger.

The final integrated suite passed **56 rendered states** in light/dark mode with zero axe text-contrast violations, zero incomplete text-contrast results and zero page errors. A separate six-client guide suite passed **36 states** across both themes and 1440/390/320px layouts with zero text-contrast violations. The application suite additionally measures field boundaries, placeholders, icon controls, meter fill and keyboard focus from computed colors using the WCAG luminance formula.

| Measured role | Lowest light ratio | Lowest dark ratio | Check threshold |
| --- | ---: | ---: | ---: |
| Form boundary | 3.4295:1 | 4.4994:1 | 3:1 |
| Placeholder text | 6.0453:1 | 8.4914:1 | 4.5:1 |
| Icon control | 6.0453:1 | 7.7206:1 | 3:1 |
| Keyboard focus | 5.2580:1 | 9.3953:1 | 3:1 |
| Capacity fill against track | 4.6281:1 | 7.3368:1 | 3:1 |

Normal text uses the WCAG AA 4.5:1 threshold; sufficiently large text uses 3:1. Disabled controls, decorative strokes and logos are evaluated according to their applicable roles rather than requiring every color pair to meet a text threshold. Tests cover actual popup selection/Escape/focus return, required-field prevention before API submission, persistent/cross-tab/system theme changes, unavailable browser storage, theme before React starts, image loading, photo upload/reload/delete and immediate owner-avatar refresh. Uploaded artifact styles and pixels are not inverted by the shell.

Independent code review found no Critical/Important issues; its two bounded integration findings above were fixed and confirmed. The photo backend received a separate authorization/validation/recovery review without material findings. Existing design targets were rechecked using unchanged free LayerDocs; these unanchored kits are not pixel-fidelity or accessibility certificates, and no new paid asset generation was used.

## Deployment and runtime evidence

Mac Studio runs the verified public macOS arm64 executable under its existing user LaunchAgent. Its deployed binary hash matches the released archive. Before replacement, a complete stopped backup of data, encryption key, executable and launch configuration passed integrity and file-hash checks. The management/content listeners and existing launch configuration remain unchanged. No DNS records, tunnel routes or shared connector were changed or restarted.

The public HTTPS installation was checked after deployment:

- Public login switches and persists both themes; both login modes, including the configured Google sign-in surface, had zero text-contrast violations.
- Favicon, ICO, touch/application icons, theme bootstrap and service assets matched the released source bytes.
- The authenticated personal and organization photo editors render correctly; anonymous organization-photo access is denied. No production profile image or name was modified for this check.
- All five external storage presets display their actual local marks; all six client guides show their registration/verification content against the real installation origin.
- Narrow account/organization pages have no horizontal document overflow. No browser page errors occurred.
- The released CLI authenticates with the preexisting token, and its actual stdio MCP session discovers all 23 tools and calls live capabilities successfully.
- Existing four users, two artifacts, five versions, old API fields and Google configuration match the immediate pre-upgrade snapshot. All twelve original object files and the encryption key passed post-upgrade hash comparison. Production photo values and client installations were left untouched.

These checks cover the Folio shell's contrast and interaction flows, not complete WCAG certification, every assistive technology or arbitrary uploaded documents. Photo mutation/recovery was exercised in isolated installations, including the actual release executable; no full production restore was attempted. External client commands were checked against official documentation, current CLI help and disposable command stubs, not registered into every live client. Windows/client-runtime and live cloud-account setup are outside this validation. The existing Google audience and user-login LaunchAgent autostart behavior are unchanged; no machine-reboot or boot-before-login test is claimed.
