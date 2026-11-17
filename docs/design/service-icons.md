# Service identity assets

The integrations/settings and agent connection surfaces share `web/src/ServiceIcon.tsx` and its `SERVICE_ICONS` registry. Vendor marks are local files in `web/public/services`, included by Vite in the embedded web build. They do not require a CDN or a runtime icon package. The existing Google identity PNG in `web/public/images/google-signin-icon.png` is reused unchanged.

## Use

```tsx
import { ServiceIcon, resolveStorageService } from "./ServiceIcon";

<ServiceIcon service="r2" />
<span>Cloudflare R2</span>

// Lists of saved backends must resolve the actual endpoint, not a display name.
<ServiceIcon service={resolveStorageService(backend)} />
```

Icons beside visible names are decorative. Only standalone icons supply `label`, for example `<ServiceIcon service="google" label="Google" />`. The `size` prop defaults to 20 CSS pixels. Preserve proportions and brand colors. Monochrome marks use a white plate in both themes; no filter, inversion, or recoloring is applied to brand artwork. Product names remain adjacent to icons. Generic disk/cloud/network icons follow the UI foreground color and are not vendor marks.

Registry IDs: `aws`, `r2`, `minio`, `gcs`, `azure`, `cloudflare`, `google`, `codex`, `claudecode`, `geminicli`, `vscode`, `copilot`, `cursor`, `opencode`, `mcp`, `local`, `generic`, `dns`, `domainconnect`.

`resolveStorageService` accepts `{ provider, endpoint?, name? }`. Explicit vendor provider IDs and recognized vendor hosts select the corresponding mark. A custom S3 endpoint, an invalid endpoint, or an arbitrary display name does not establish a vendor identity. Generic S3-compatible endpoints use the neutral cloud symbol. The MinIO preset can use `minio`, but saved arbitrary compatible endpoints remain generic. Domain Connect uses the neutral network symbol with its visible name; this is not presented as an official Domain Connect logo.

## Provenance and rights notices

Downloaded on 2026-11-18. The exact source URLs, package archive paths, unmodified status, upstream commits where available, and per-file SHA-256 hashes are in [`web/public/services/sources.json`](../../web/public/services/sources.json).

| Assets | Source and accompanying notice |
| --- | --- |
| AWS, Azure, Cloudflare, Google Cloud, Codex, Claude Code, Gemini CLI, Cursor, OpenCode, GitHub Copilot, MCP | Selected original SVG files from [`@lobehub/icons-static-svg@1.103.0`](https://registry.npmjs.org/@lobehub/icons-static-svg/1.103.0), upstream commit [`573cb307`](https://github.com/lobehub/lobe-icons/tree/573cb307cbaf10dca7c471a634a4a6d3192e701b). The package declares MIT; the upstream MIT notice is bundled as `LOBE-ICONS-LICENSE.txt`. This describes the icon package license, not ownership of the depicted company marks. |
| R2 | Unmodified [`src/icons/r2.svg`](https://github.com/cloudflare/cloudflare-docs/blob/8b688cd54b99da6d3a300ceaf6b1a8942d6ded72/src/icons/r2.svg), Cloudflare documentation commit `8b688cd5`. This is the actual current R2 storage icon used in the documentation repository. Its repository [CC BY 4.0 license](https://github.com/cloudflare/cloudflare-docs/blob/8b688cd54b99da6d3a300ceaf6b1a8942d6ded72/LICENSE) is bundled as `CLOUDFLARE-DOCS-LICENSE.txt`; attribution: Cloudflare, cloudflare-docs, unchanged. [Cloudflare trademark guidelines](https://www.cloudflare.com/trademark/) are separate. |
| MinIO | Genuine 32px stork PNG linked as the favicon by the current official [MinIO logo page](https://www.min.io/logo), copied byte-for-byte from its website CDN. Brand guidelines apply; this asset is not relicensed under MIT. The small standalone stork avoids compressing the full wordmark into an unreadable square. |
| Visual Studio Code | Stable blue SVG from the [official brand download](https://code.visualstudio.com/assets/branding/visual-studio-code-icons.zip), with the original `VS-CODE-README.txt`. [Brand guidelines](https://code.visualstudio.com/brand) identify the stable blue symbol and the Microsoft trademark. |
| Google identity | Existing official sign-in PNG retained unchanged. [Google Sign-In branding guidelines](https://developers.google.com/identity/branding-guidelines). |
| Local storage, generic object storage, DNS, Domain Connect fallback | Neutral `HardDrive`, `Cloud`, and `Network` components from the existing `lucide-react` dependency. These are not service brand assets. |

Reference investigation also covered the official [Google Cloud icon library](https://cloud.google.com/icons), [AWS architecture icons](https://aws.amazon.com/architecture/icons/), [Azure architecture icon guidelines](https://learn.microsoft.com/en-us/azure/architecture/icons/), and [Domain Connect](https://www.domainconnect.org/). Azure architecture asset download terms describe architecture diagrams, training, and documentation; Folio does not represent those assets or Microsoft marks as MIT-licensed. The registry uses the separately attributed LobeHub Azure SVG.

These marks identify optional interoperability and client instructions. They do not replace Folio's identity or assert sponsorship. Third-party trademarks remain with their respective owners; retaining an icon package license does not grant rights to the depicted trademarks.

## Updating

Replace assets from their recorded sources without modifying their geometry or color. Preserve notices and update `sources.json` hashes. Review the resulting light/dark render at actual UI sizes. Keep requests local, and inspect newly downloaded SVGs for scripts, external references, and embedded resources before adding them.
