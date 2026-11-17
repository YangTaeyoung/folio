# Third-party notices

Folio's original source is licensed under MIT. Bundled third-party assets retain their own licenses.

## Pretendard

The administration UI and example product tour bundle Pretendard Variable by Kil Hyung-jin, distributed under the SIL Open Font License 1.1. The full license and upstream notices are included in `web/public/fonts/OFL.txt` and `examples/folio-showcase/fonts/OFL.txt`, and in the embedded UI at `/fonts/OFL.txt`.

Source: https://github.com/orioncactus/pretendard

## Google Sans

The optional Google sign-in button bundles Google Sans from Google Fonts under the SIL Open Font License 1.1. The full license is in `web/public/fonts/GoogleSans-OFL.txt`, embedded at `/fonts/GoogleSans-OFL.txt`, and included as `GOOGLE-SANS-OFL.txt` in release archives.

Source: https://github.com/google/fonts/tree/main/ofl/googlesans

## Google sign-in identity asset

The optional Google sign-in button uses Google's official sign-in asset. Google identity marks remain Google trademarks and are used according to the Google Sign-In branding guidelines: https://developers.google.com/identity/branding-guidelines

## Service identity icons

Selected unmodified SVGs from `@lobehub/icons-static-svg` 1.103.0 are bundled in `web/public/services`. The package is distributed under MIT, Copyright (c) 2023 LobeHub. Its full notice is included at `web/public/services/LOBE-ICONS-LICENSE.txt`. Source: https://github.com/lobehub/lobe-icons

The R2 icon is an unmodified asset from Cloudflare's `cloudflare-docs` repository, commit `8b688cd54b99da6d3a300ceaf6b1a8942d6ded72`, `src/icons/r2.svg`. Attribution: Cloudflare. The repository's CC BY 4.0 license is included at `web/public/services/CLOUDFLARE-DOCS-LICENSE.txt`. Source: https://github.com/cloudflare/cloudflare-docs

The Visual Studio Code stable blue icon comes from Microsoft's official brand download. Its original notice is included at `web/public/services/VS-CODE-README.txt`. Visual Studio Code and its icon are Microsoft trademarks. Guidelines: https://code.visualstudio.com/brand

The MinIO stork PNG is the unmodified favicon asset linked by MinIO's official brand page. MinIO brand assets are not relicensed under MIT. Guidelines: https://www.min.io/logo

Package copyright licenses do not transfer ownership of the depicted service names or trademarks. Service assets identify integrations and client compatibility; no endorsement is implied. Exact URLs, upstream versions, and SHA-256 hashes are recorded in `web/public/services/sources.json` and explained in `docs/design/service-icons.md`.

## Radix Select

`@radix-ui/react-select` and its Radix primitive dependencies are MIT licensed by WorkOS. Source and licenses: https://github.com/radix-ui/primitives . Folio bundles these primitives locally for its Select controls; no Radix hosted service is used. axe-core (https://github.com/dequelabs/axe-core, MPL-2.0) is a development-only accessibility test dependency and is not included in the application runtime bundle.
