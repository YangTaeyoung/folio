---
name: Folio
description: A clear workspace for artifacts on your infrastructure.
colors:
  primary: '#17181b'
  primary-hover: '#32343a'
  ink: '#18191c'
  accent: '#2258e8'
  soft: '#edf2ff'
  active-background: '#dce7ff'
  active-text: '#174fce'
  paper: '#fff'
  nav: '#f3f4f6'
  line: '#e2e4e9'
  muted: '#626976'
  field-border: '#cdd1d9'
  card-border: '#d9dde5'
  hover-border: '#91a8d4'
  secondary-border: '#d6d9df'
  secondary-hover: '#f6f7f9'
  scope-background: '#f1f3f7'
  scope-text: '#515b6b'
  danger: '#bd3038'
typography:
  headline:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 32px
    fontWeight: 720
    lineHeight: 1.3
    letterSpacing: -0.035em
  headline-mobile:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 27px
    fontWeight: 720
    lineHeight: 1.3
    letterSpacing: -0.035em
  section-title:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 18px
    fontWeight: 650
    lineHeight: 1.4
    letterSpacing: -0.3px
  card-title:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 17px
    fontWeight: 650
    lineHeight: 1.4
    letterSpacing: -0.025em
  body:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.55
  button:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 14px
    fontWeight: 550
    lineHeight: 1.4
  metadata:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 12px
    fontWeight: 400
    lineHeight: 1.55
  code:
    fontFamily: ui-monospace, SFMono-Regular, Consolas, monospace
    fontSize: 12px
  headline-compact:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 28px
    fontWeight: 720
    lineHeight: 1.3
    letterSpacing: -0.035em
  admin-section-title:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 16px
    fontWeight: 650
    lineHeight: 1.5
    letterSpacing: -0.015em
rounded:
  control: 7px
  preview: 6px
  panel: 8px
  card: 8px
  dialog: 12px
  scope: 20px
spacing:
  tight: 8px
  compact: 12px
  regular: 16px
  grid-gap: 22px
  panel-padding: 24px
  desktop-gutter: 28px
components:
  button-primary:
    backgroundColor: '{colors.primary}'
    textColor: '{colors.paper}'
    typography: '{typography.button}'
    rounded: '{rounded.control}'
    padding: 9px 15px
  button-primary-hover:
    backgroundColor: '{colors.primary-hover}'
  button-secondary:
    backgroundColor: '{colors.paper}'
    textColor: '{colors.ink}'
    typography: '{typography.button}'
    rounded: '{rounded.control}'
    padding: 9px 15px
  button-danger:
    backgroundColor: '{colors.danger}'
    textColor: '{colors.paper}'
    typography: '{typography.button}'
    rounded: '{rounded.control}'
    padding: 9px 15px
  field:
    backgroundColor: '{colors.paper}'
    textColor: '{colors.ink}'
    rounded: '{rounded.control}'
    padding: 10px 12px
  navigation-active:
    backgroundColor: '{colors.active-background}'
    textColor: '{colors.active-text}'
    rounded: '{rounded.control}'
    padding: 9px 12px
  scope-label:
    backgroundColor: '{colors.scope-background}'
    textColor: '{colors.scope-text}'
    typography: '{typography.metadata}'
    rounded: '{rounded.scope}'
    padding: 4px 8px
  artifact-card:
    backgroundColor: '{colors.paper}'
    rounded: '{rounded.card}'
---

# Design System: Folio

## Overview

**Creative North Star: "The Artifact Workspace"**

Folio is a practical, contemporary workspace whose artifacts carry the visual interest. Cool gray navigation frames a white working canvas; clear sans-serif hierarchy, black primary actions and cobalt selection make finding, opening and sharing work easy to scan. Actual artifact previews lead the library, the reading canvas leads detail, and aligned flat sections make administration legible.

This records the built composition overhaul within the user's approved B light-neutral, black-and-cobalt world. Folio keeps its own folded-page mark and locally bundled Pretendard. At login, an original transparent blue-and-white paper sculpture shares the page canvas and crops freely across its edge; the form remains unboxed. On mobile the form takes priority and the artwork stage disappears.

**Key Characteristics:**

- Neutral operating chrome around visually diverse artifacts.
- Compact, preview-led browsing with contextual sharing and explicit access.
- A broad reading plane with a separate metadata aside.
- Flat, aligned administration and progressive setup guidance.
- Local typography and brand assets; direct Korean task labels.

This is a scan of the current reviewed working-tree implementation, including its surface overrides; it does not assert release or deployed-runtime status. Source anchors below identify the effective implementation. Private review evidence is the 32-screen set in `.work/design-research/product-after-final/` and its `capture-metadata.json`, plus Google-enabled login captures in `.work/design-research/final-source-google/`. Those are temporary loopback installations with authored fixtures, not production activity. Earlier implementation history is not the authority for the current composition. The document uses the [official DESIGN.md format](https://raw.githubusercontent.com/google-labs-code/design.md/main/docs/spec.md), verified during this refresh.

## Colors

The palette is cool and neutral. Black establishes action hierarchy; cobalt establishes orientation and interaction. Frontmatter values are normative. Base colors come from [style.css:7](web/src/style.css#L7); preview hover and scope variants come from [ProductSurface.css:39](web/src/ProductSurface.css#L39).

### Primary

- **Action Black** (`primary`, `primary-hover`): Filled create, save and login actions, with a lighter hover state.
- **Interaction Blue** (`accent`): Keyboard focus, active tabs and native selection controls.
- **Selection Wash / Selection Blue** (`active-background`, `active-text`): The current navigation item.
- **Soft Blue** (`soft`): Restrained interactive emphasis.

### Neutral

- **Canvas White** (`paper`): The main working canvas, form fields, dialogs and artifact tiles.
- **Sidebar Gray** (`nav`): The persistent navigation frame.
- **Working Ink / Slate Metadata** (`ink`, `muted`): Primary text and supporting context.
- **Divider Gray** (`line`): Section separators and table structure. Field, card and secondary-control strokes retain their stronger observed roles.
- **Scope Mist / Scope Slate** (`scope-background`, `scope-text`): Private access labels. Organization and selected-member labels use a blue tint; public labels use a green status tint, always accompanied by text and an icon ([ProductSurface.css:101](web/src/ProductSurface.css#L101)).

Destructive Red (`danger`) belongs to destructive actions. Authentication uses closely related local neutrals for its form and pale artwork canvas; those surface-specific values do not introduce another product palette ([AuthSurface.css:1](web/src/AuthSurface.css#L1)).

**The Action and State Rule.** Use black for the main action and blue for selection, focus and active state; preserve their separate jobs.

## Typography

**Display and Body Font:** Locally served Pretendard Variable with Apple/system sans-serif fallbacks. The local WOFF2 declares weights (100–900) and `font-display: swap` ([style.css:1](web/src/style.css#L1)).

**Code Font:** System monospace for commands, keys and configuration ([style.css:138](web/src/style.css#L138)).

**Character:** One clear sans-serif voice supports Korean and Latin content. Scale and weight establish hierarchy. The bold sans-serif wordmark accompanies Folio's own folded-page SVG ([Auth.tsx:7](web/src/Auth.tsx#L7)). Artifact content retains its own typography inside the preview.

### Hierarchy

- **Headline:** Use `headline` for desktop page headings, `headline-compact` up to (900px), and `headline-mobile` up to (600px). Their effective sizes are in frontmatter, replacing the older global heading defaults ([ProductSurface.css:16](web/src/ProductSurface.css#L16)).
- **Card title:** Use `card-title` in the grid at desktop and mobile. Two-line clamping protects the compact tile; the description is omitted in grid view. List titles use (15px) desktop and (14px) mobile, with a supporting description ([ProductSurface.css:53](web/src/ProductSurface.css#L53), [style.css:814](web/src/style.css#L814), [style.css:2502](web/src/style.css#L2502)).
- **Section title:** General secondary headings use `section-title`; flat administration uses `admin-section-title`. Login form headings use (29px), becoming (27px) up to (760px) ([AdminSurface.css:18](web/src/AdminSurface.css#L18), [AuthSurface.css:128](web/src/AuthSurface.css#L128)).
- **Body:** Base body is defined in frontmatter. Paragraphs use line height (1.65); supporting descriptions commonly use (13px). Login fields use (16px) text on mobile ([AuthSurface.css:343](web/src/AuthSurface.css#L343)).
- **Metadata and buttons:** Owner, access and hints stay subordinate to titles. Tile timestamps use (11px); ordinary metadata uses `metadata`. Button labels remain on one line ([ProductSurface.css:93](web/src/ProductSurface.css#L93), [style.css:176](web/src/style.css#L176)).

**The Hierarchy Rule.** Preserve the established heading and card-title hierarchy; compact metadata must not set the scale of the whole interface.

## Layout

The desktop shell uses a fixed sidebar (250px), a breadcrumb bar (56px), and a centered workspace capped at (1500px). Effective workspace padding is (28px 28px 48px). Surface overrides follow the base stylesheet, so their geometry takes precedence over the earlier (244px) sidebar and wider workspace padding ([main.tsx:4](web/src/main.tsx#L4), [ProductSurface.css:2](web/src/ProductSurface.css#L2), [style.css:485](web/src/style.css#L485)).

The library puts its title and count on the left, with create and discovery controls aligned on the right. Below (1200px), these stack into a heading and toolbar. Search, sort and grid/list controls remain close to the work. Mobile exposes the create action in the shared header, rather than repeating it in the library heading ([ProductSurface.css:418](web/src/ProductSurface.css#L418)).

The grid uses three equal columns and `grid-gap`; up to (1250px) it uses two columns, and up to (600px) one column with (16px) gaps. Thumbnail apertures retain (16:10). Up to (900px), workspace padding becomes (24px 22px 40px), the breadcrumb bar yields to a (58px) mobile header, and navigation becomes an off-canvas drawer. Up to (600px), padding becomes (22px 18px 32px). The compact navigation drawer remains (244px) wide ([ProductSurface.css:157](web/src/ProductSurface.css#L157), [style.css:2223](web/src/style.css#L2223), [style.css:2319](web/src/style.css#L2319)).

List view uses flat separated rows with a small preview, title/description and contextual access/share area. At mobile width the access/share area moves below the preview/title row. Both library modes render the same thumbnail document viewport (1024 × 640), scaled into their respective apertures; switching view changes presentation rather than the artifact's internal layout ([Artifacts.tsx:51](web/src/Artifacts.tsx#L51), [ProductSurface.css:118](web/src/ProductSurface.css#L118)).

Detail is a wide reading canvas, with width `min(1440px, calc(100vw - 250px))`. A flexible reading column and (250px) metadata aside are separated by (28px). Below (1100px), metadata follows the reading plane; up to (900px), detail fills the viewport width. Its interactive iframe uses the actual reading viewport without thumbnail scaling. The desktop reading height is `max(480px, calc(100dvh - 280px))`; mobile uses (58dvh) with a (360px) minimum ([ProductSurface.css:218](web/src/ProductSurface.css#L218)).

Sharing remains a contextual right side panel (490px), filling the viewport width up to (600px). Detail and sharing retain full-height drawers with separately scrolling bodies and visible action footers. Standard dialogs are (470px), wide dialogs (680px); small-screen modal gutters are (12px) ([ProductSurface.css:309](web/src/ProductSurface.css#L309), [style.css:1018](web/src/style.css#L1018)).

Authentication is an independent two-column page. A pale canvas supports the free-cropped transparent paper sculpture on the left; the bare white form occupies the right. There is no embedded library screenshot. The usual form width is (400px), growing to (420px) on very wide screens. Fields and actions have a minimum height (48px). At (760px) and below, the artwork stage is hidden, with the brand and form presented first and page gutters (28px), reducing to (22px) up to (360px). First-run setup omits the artwork and centers an unboxed (440px) form ([Auth.tsx:57](web/src/Auth.tsx#L57), [AuthSurface.css:36](web/src/AuthSurface.css#L36)).

Administration uses flat sections separated by horizontal rules. Description columns (220px) align with working forms, with a (44px) gap; forms use natural maximum widths. Below (1150px), section labels narrow and agent setup stacks. Below (700px), section headings precede their forms. Members become labeled, stacked rows below (600px). Storage dialogs keep only their field body scrolling, with heading and footer visible ([AdminSurface.css:5](web/src/AdminSurface.css#L5), [ResponsiveAdmin.css:6](web/src/ResponsiveAdmin.css#L6)).

## Elevation & Depth

Tone and fine rules do most of the organizing. Artifact tiles have light outlines and a restrained hover/focus shadow; flat list rows and administrative sections do not become floating panels. Login and first-run forms remain unboxed and shadow-free. Blocking dialogs and drawers carry the strongest elevation; transient feedback also floats above the workspace ([ProductSurface.css:39](web/src/ProductSurface.css#L39), [AdminSurface.css:5](web/src/AdminSurface.css#L5), [AuthSurface.css:115](web/src/AuthSurface.css#L115), [style.css:1028](web/src/style.css#L1028)). Exact shadow and motion values live in the sidecar.

**The Quiet Depth Rule.** Use borders and tone for ordinary organization; reserve stronger elevation for blocking overlays and transient feedback.

## Shapes

Controls use `control`, artifact tiles use `card`, and thumbnail apertures use `preview`. Dialogs use `dialog`, while side panels meet the viewport edge squarely. Audience labels use `scope`; avatars are circles. Generic inset panels retain `panel`, but administrative section containers are square, transparent and open to the page ([ProductSurface.css:39](web/src/ProductSurface.css#L39), [AdminSurface.css:5](web/src/AdminSurface.css#L5), [style.css:727](web/src/style.css#L727)).

The folded-page mark is Folio's reusable identity. The login sculpture is page material, not a framed preview or a prescribed container for all brand imagery. Actual artifacts are clipped only within their preview aperture; their own visual worlds are not shell design tokens.

## Components

### Buttons

Direct and familiar. Primary actions use Action Black with white text; secondary actions are white with a gray stroke; destructive actions use red. Ordinary buttons have minimum height (40px), compact buttons (32px), and authentication controls (48px). Disabled controls retain reduced opacity (0.48) and a disabled cursor. Hover changes fill or border over (0.15s); keyboard focus uses a blue outline (2px), offset (3px). Contextual icon buttons keep an accessible task label ([style.css:54](web/src/style.css#L54), [style.css:176](web/src/style.css#L176), [Artifacts.tsx:183](web/src/Artifacts.tsx#L183)).

### Inputs / Fields

White fields use the stronger field border, control radius, minimum height (40px) and padding (10px 12px). Labels sit above fields with hints below. Search has an inset icon and shortcut hint. Native checkboxes/radios use blue selection. Login has the larger field treatment and an explicit password-visibility control ([style.css:105](web/src/style.css#L105), [Auth.tsx:181](web/src/Auth.tsx#L181), [AuthSurface.css:159](web/src/AuthSurface.css#L159)).

### Navigation

The Folio mark and bold wordmark anchor the gray sidebar. Navigation combines outlined icons and readable labels; hover uses gray, active state a blue wash and stronger blue text. Workspace context sits above task groups; account and actual storage state sit below ([style.css:281](web/src/style.css#L281), [style.css:328](web/src/style.css#L328)). Compact navigation retains the existing modal focus and inertness behavior; preserve this shared navigation rather than making route-specific copies ([App.tsx](web/src/App.tsx)).

### Artifact Cards and Previews

A compact tile leads with the real artifact, then a title and a single metadata area: owner/avatar, modification time, audience and contextual share button. Opening is attached to the preview/title. Grid descriptions stay hidden; list descriptions remain visible. There is no repeated open/share footer strip. Loading and failed previews are explicit. Library iframes suppress pointer interaction; the detail iframe is interactive at its native reading viewport ([Artifacts.tsx:51](web/src/Artifacts.tsx#L51), [Artifacts.tsx:159](web/src/Artifacts.tsx#L159), [ProductSurface.css:53](web/src/ProductSurface.css#L53)).

### Audience Labels and Sharing

Audience pills combine text and an icon, with private, organization/selected and public tints. The side panel shows the saved current audience separately from editable radio choices. The four choices form flat separated rows; selection uses a pale blue wash. Member selection and expiry belong to the same task. The footer distinguishes draft changes from saved access and states that copying a link does not change permissions. Private access copy includes the owner and administrators ([Artifacts.tsx:222](web/src/Artifacts.tsx#L222), [Artifacts.tsx:255](web/src/Artifacts.tsx#L255), [Artifacts.tsx:405](web/src/Artifacts.tsx#L405), [ProductSurface.css:315](web/src/ProductSurface.css#L315)).

### Administrative Sections and Disclosures

Use aligned headings, actual statuses, flat rows, restrained fields and existing task actions. Member tables become labeled rows on narrow screens. Storage connections, retention policy, domain configuration and token scopes preserve their real controls and guards. Setup instructions use native `details`/`summary` disclosures with clear titles, supporting context and visible expansion state; Google setup, CLI guides and installation references follow this pattern. Forms remain the primary task, with long guidance progressively disclosed ([Admin.tsx:1223](web/src/Admin.tsx#L1223), [Admin.tsx:1659](web/src/Admin.tsx#L1659), [GoogleAuthSettings.tsx:192](web/src/GoogleAuthSettings.tsx#L192), [AdminSurface.css:93](web/src/AdminSurface.css#L93)).

### Authentication, Feedback and Motion

Optional Google sign-in appears above the email divider only when enabled by the provider response, and never on first-run setup. The form keeps the existing password flow, password visibility, account help and errors. Google configuration remains an optional administrative form with progressive guidance; the visual overhaul does not imply a new authentication feature ([Auth.tsx:40](web/src/Auth.tsx#L40), [Auth.tsx:127](web/src/Auth.tsx#L127), [GoogleAuthSettings.tsx:82](web/src/GoogleAuthSettings.tsx#L82)). Fonts, mark, artwork and Google button image use local paths; asset provenance and licenses are in [brand-assets.md](docs/design/brand-assets.md).

Notices and errors state the actual outcome with text as well as color. Toasts use a dark surface. Drawer entrance takes (0.22s) with short horizontal motion; compact navigation takes (0.2s). Reduced-motion preference removes animation, transition and smooth scrolling ([style.css:1093](web/src/style.css#L1093), [style.css:1793](web/src/style.css#L1793), [style.css:2731](web/src/style.css#L2731)).

## Do's and Don'ts

### Do:

- **Do** keep the gray navigation shell, white canvas, black primary actions and blue active state together.
- **Do** preserve the established desktop/mobile heading scale and compact preview-led tiles.
- **Do** keep owner, modification time, audience and contextual sharing close to each artifact.
- **Do** give reading content the broad canvas and keep metadata contextual.
- **Do** use flat aligned administrative sections and progressively disclosed setup guidance.
- **Do** keep local fonts/assets, visible focus, honest states and the existing authorization controls.

### Don't:

- **Don't** restore the rejected sage/ivory, serif, bookish visual world or poetic library labels.
- **Don't** confine all brand imagery to decorative cards or bring the login artwork stage onto mobile.
- **Don't** restore repeated open/share tile footers or make a thumbnail viewport change with library view mode.
- **Don't** substitute fabricated activity, customers, usage or health metrics for actual state.
- **Don't** express sharing scope through color alone or imply that copying a link saves draft permissions.
- **Don't** treat uploaded artifacts, research references or example-tour styling as shell design tokens.
