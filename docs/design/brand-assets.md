# Authentication brand assets

The authentication surface keeps Workspace B typography, neutral controls and cobalt accents. It adds an original folded-paper illustration and a clearly labeled example of the real library, without inventing product data or customer claims. Mobile presents the login form first. All assets and fonts are embedded and need no runtime CDN.

- `web/public/images/artifact-space-v1.webp`: original generated brand artwork, 1448 × 1086, 113,218 bytes. Illustration, not a product screenshot.
- `web/public/images/workspace-preview-v1.png`: real example library capture from `docs/images/library-desktop.png`. Explicitly labeled as a product example.
- `web/public/images/google-signin-icon.png`: official Google light square sign-in asset, central mark shown without reshaping or recoloring.
- `web/public/fonts/GoogleSans.ttf`: locally bundled Google Sans, used only for the Google sign-in button; see `THIRD_PARTY_NOTICES.md`.

## Generation record

Mode: built-in `image_gen`, new image, opaque background, no reference images. The selected source was copied into the workspace and converted to WebP for web delivery; image content was not edited.

Final prompt:

> Create a premium original SaaS brand illustration asset, not a website mockup. Landscape 4:3 composition for the welcoming visual panel of a self-hosted artifact-sharing workspace. A striking architectural still life made of tactile folded document sheets, two elegant file trays and a continuous ultramarine-blue folded paper path connecting them. Ivory-white paper planes, precise charcoal folded corners, one large sculptural cobalt folder-like arch framing a small collection of carefully arranged pages. Convey making, collecting and sharing creative work. Modern industrial product photography meets sophisticated 3D editorial illustration: matte ceramic-like blue surfaces, real fine paper texture, crisp folded edges, believable soft directional shadows, physically coherent objects resting on a surface. Bold, memorable composition with scale variation and a clear focal point. Colors restricted to Workspace B: rich cobalt #2258e8, cool pale periwinkle-blue ground, white and small charcoal accents. Large sculptural forms occupy about 75% of the canvas; slightly elevated three-quarter camera, objects clearly legible at a 600px display width. Complete image with a uniform pale periwinkle-blue studio background that extends to every edge. No letters, text, numbers, logo, watermarks, people, dashboard, interface screenshot, glassmorphism, glow, gradients, floating generic app icons, fake charts or pictograms. This is finished production brand artwork to bring character to an otherwise clear black/white/blue login experience.
