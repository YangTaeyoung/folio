---
name: Folio
description: A clear workspace for artifacts on your infrastructure.
colors:
  primary: "#17181b"
  primary-hover: "#32343a"
  ink: "#18191c"
  accent: "#2258e8"
  soft: "#edf2ff"
  active-background: "#dce7ff"
  active-text: "#174fce"
  paper: "#fff"
  nav: "#f3f4f6"
  line: "#e2e4e9"
  muted: "#626976"
  field-border: "#cdd1d9"
  card-border: "#d9dde5"
  hover-border: "#91a8d4"
  secondary-border: "#d6d9df"
  secondary-hover: "#f6f7f9"
  scope-background: "#f1f3f7"
  scope-text: "#515b6b"
  danger: "#bd3038"
  status-success: "#21683e"
  review-changes: "#9c3d24"
  review-changes-background: "#fff6f0"
  diff-added-background: "#e3f3e9"
  diff-added-text: "#205c37"
  diff-removed-background: "#fce5e6"
  diff-removed-text: "#9b242c"
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
  language-label:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 12px
  language-label-mobile:
    fontFamily: Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif
    fontSize: 16px
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
    backgroundColor: "{colors.primary}"
    textColor: "{colors.paper}"
    typography: "{typography.button}"
    rounded: "{rounded.control}"
    padding: 9px 15px
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
  button-secondary:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.button}"
    rounded: "{rounded.control}"
    padding: 9px 15px
  button-danger:
    backgroundColor: "{colors.danger}"
    textColor: "{colors.paper}"
    typography: "{typography.button}"
    rounded: "{rounded.control}"
    padding: 9px 15px
  field:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: 10px 12px
  navigation-active:
    backgroundColor: "{colors.active-background}"
    textColor: "{colors.active-text}"
    rounded: "{rounded.control}"
    padding: 9px 12px
  scope-label:
    backgroundColor: "{colors.scope-background}"
    textColor: "{colors.scope-text}"
    typography: "{typography.metadata}"
    rounded: "{rounded.scope}"
    padding: 4px 8px
  artifact-card:
    backgroundColor: "{colors.paper}"
    rounded: "{rounded.card}"
  language-picker:
    textColor: "{colors.ink}"
    typography: "{typography.language-label}"
    rounded: "{rounded.control}"
    padding: 6px 22px 6px 7px
---

# Design System: Folio

## Overview

**Creative North Star: "The Artifact Workspace"**

Folio is a practical, contemporary workspace whose artifacts carry the visual interest. Cool gray navigation frames a white working canvas; clear sans-serif hierarchy, cobalt primary actions and cobalt selection make finding, opening and sharing work easy to scan. Actual artifact previews lead the library, the reading canvas leads detail, and aligned flat sections make administration legible.

This records the built composition overhaul within the user's approved B light-neutral, black-and-cobalt world. Folio keeps its own folded-page mark and locally bundled Pretendard. At login, an original transparent blue-and-white paper sculpture shares the page canvas and crops freely across its edge; the form remains unboxed. On mobile the form takes priority and the artwork stage disappears.

**Key Characteristics:**

- Neutral operating chrome around visually diverse artifacts.
- Compact, preview-led browsing with contextual sharing and explicit access.
- A broad reading plane with a separate metadata aside.
- Flat, aligned administration and progressive setup guidance.
- Local typography and brand assets; direct task labels across five interface languages.

This records the reviewed working tree on `feat/collaboration-and-i18n`, based on `0f121646a59af74b1676c0769e343c686a897b19` and its collaboration/localization changes. It captures implemented visual behavior, not release or deployment status. Source anchors identify the effective styles and markup. The document follows the [official DESIGN.md format](https://raw.githubusercontent.com/google-labs-code/design.md/main/docs/spec.md).

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

Review and file changes add semantic colors, not another brand accent. **Success Green** (`status-success`) marks approved/resolved feedback and saved outcomes. **Revision Amber** (`review-changes`, `review-changes-background`) marks requested changes. **Added / Removed Text and Wash** (`diff-added-*`, `diff-removed-*`) distinguish code additions and removals; visible change labels and line prefixes carry the same meaning ([ReviewPanel.css:35](web/src/ReviewPanel.css#L35), [WorkspaceFeatures.css:554](web/src/WorkspaceFeatures.css#L554), [WorkspaceFeatures.tsx:1201](web/src/WorkspaceFeatures.tsx#L1201)).

Destructive Red (`danger`) belongs to destructive actions. Authentication uses closely related local neutrals for its form and pale artwork canvas; those surface-specific values do not introduce another product palette ([AuthSurface.css:1](web/src/AuthSurface.css#L1)).

**The Action and State Rule.** Use black for the main action and blue for selection, focus and active state; preserve their separate jobs.

## Typography

**Display and Body Font:** Locally served Pretendard Variable with Apple/system sans-serif fallbacks. The local WOFF2 declares weights (100–900) and `font-display: swap` ([style.css:1](web/src/style.css#L1)).

**Code Font:** System monospace for commands, keys and configuration ([style.css:138](web/src/style.css#L138)).

**Character:** One clear sans-serif voice supports Korean and Latin content. Scale and weight establish hierarchy. The bold sans-serif wordmark accompanies Folio's own folded-page SVG ([Auth.tsx:9](web/src/Auth.tsx#L9)). Korean, English, Japanese, Simplified Chinese and French share the same shell hierarchy; uploaded artifact content retains its own language and typography inside the preview ([i18n.tsx:8](web/src/i18n.tsx#L8)).

### Hierarchy

- **Headline:** Use `headline` for desktop page headings, `headline-compact` up to (900px), and `headline-mobile` up to (600px). Their effective sizes are in frontmatter, replacing the older global heading defaults ([ProductSurface.css:16](web/src/ProductSurface.css#L16)).
- **Card title:** Use `card-title` in the grid at desktop and mobile. Two-line clamping protects the compact tile; the description is omitted in grid view. List titles use (15px) desktop and (14px) mobile, with a supporting description ([ProductSurface.css:53](web/src/ProductSurface.css#L53), [style.css:814](web/src/style.css#L814), [style.css:2502](web/src/style.css#L2502)).
- **Section title:** General secondary headings use `section-title`; flat administration uses `admin-section-title`. Login form headings use (29px), becoming (27px) up to (760px) ([AdminSurface.css:18](web/src/AdminSurface.css#L18), [AuthSurface.css:128](web/src/AuthSurface.css#L128)).
- **Body:** Base body is defined in frontmatter. Paragraphs use line height (1.65); supporting descriptions commonly use (13px). Login fields use (16px) text on mobile ([AuthSurface.css:343](web/src/AuthSurface.css#L343)).
- **Metadata and buttons:** Owner, access and hints stay subordinate to titles. Tile timestamps use (11px); ordinary metadata uses `metadata`. Button labels remain on one line ([ProductSurface.css:93](web/src/ProductSurface.css#L93), [style.css:176](web/src/style.css#L176)).

Review comments use body text (14px), line height (1.65) and a (75ch) measure; path, author, time and location metadata use (12px). Diff content uses monospace (12px) with line height (1.7), remaining horizontally scrollable rather than wrapping code. The language picker uses `language-label`, switching to `language-label-mobile` up to (760px) ([ReviewPanel.css:155](web/src/ReviewPanel.css#L155), [WorkspaceFeatures.css:533](web/src/WorkspaceFeatures.css#L533), [LanguageSelector.css:9](web/src/LanguageSelector.css#L9)).

**The Hierarchy Rule.** Preserve the established heading and card-title hierarchy; compact metadata must not set the scale of the whole interface.

## Layout

The desktop shell uses a fixed sidebar (250px), a breadcrumb bar (56px), and a centered workspace capped at (1500px). Effective workspace padding is (28px 28px 48px). Surface overrides follow the base stylesheet, so their geometry takes precedence over the earlier (244px) sidebar and wider workspace padding ([main.tsx:4](web/src/main.tsx#L4), [ProductSurface.css:2](web/src/ProductSurface.css#L2), [style.css:485](web/src/style.css#L485)).

The library puts its title and count on the left, with create and discovery controls aligned on the right. Below (1200px), these stack into a heading and toolbar. Search, sort and grid/list controls remain close to the work. Mobile exposes the create action in the shared header, rather than repeating it in the library heading ([ProductSurface.css:418](web/src/ProductSurface.css#L418)).

The grid uses three equal columns and `grid-gap`; up to (1250px) it uses two columns, and up to (600px) one column with (16px) gaps. Thumbnail apertures retain (16:10). Up to (900px), workspace padding becomes (24px 22px 40px), the breadcrumb bar yields to a (58px) mobile header, and navigation becomes an off-canvas drawer. Up to (600px), padding becomes (22px 18px 32px). The compact navigation drawer remains (244px) wide ([ProductSurface.css:157](web/src/ProductSurface.css#L157), [style.css:2223](web/src/style.css#L2223), [style.css:2319](web/src/style.css#L2319)).

List view uses flat separated rows with a small preview, title/description and contextual access/share area. At mobile width the access/share area moves below the preview/title row. Both library modes render the same thumbnail document viewport (1024 × 640), scaled into their respective apertures; switching view changes presentation rather than the artifact's internal layout ([Artifacts.tsx:64](web/src/Artifacts.tsx#L64), [ProductSurface.css:118](web/src/ProductSurface.css#L118)).

Detail is a wide reading canvas, with width `min(1440px, calc(100vw - 250px))`. A flexible reading column and (250px) metadata aside are separated by (28px). Below (1100px), metadata follows the reading plane; up to (900px), detail fills the viewport width. Its interactive iframe uses the actual reading viewport without thumbnail scaling. The desktop reading height is `max(480px, calc(100dvh - 280px))`; mobile uses (58dvh) with a (360px) minimum ([ProductSurface.css:218](web/src/ProductSurface.css#L218)).

Sharing remains a contextual right side panel (490px), filling the viewport width up to (600px). Detail and sharing retain full-height drawers with separately scrolling bodies and visible action footers. Standard dialogs are (470px), wide dialogs (680px); small-screen modal gutters are (12px) ([ProductSurface.css:309](web/src/ProductSurface.css#L309), [style.css:1018](web/src/style.css#L1018)).

Authentication is an independent two-column page. A pale canvas supports the free-cropped transparent paper sculpture on the left; the bare white form occupies the right. There is no embedded library screenshot. The usual form width is (400px), growing to (420px) on very wide screens. Fields and actions have a minimum height (48px). At (760px) and below, the artwork stage is hidden, with the brand and form presented first and page gutters (28px), reducing to (22px) up to (360px). First-run setup omits the artwork and centers an unboxed (440px) form ([Auth.tsx:55](web/src/Auth.tsx#L55), [AuthSurface.css:36](web/src/AuthSurface.css#L36)).

Administration uses flat sections separated by horizontal rules. Description columns (220px) align with working forms, with a (44px) gap; forms use natural maximum widths. Below (1150px), section labels narrow and agent setup stacks. Below (700px), section headings precede their forms. Members become labeled, stacked rows below (600px). Storage dialogs keep only their field body scrolling, with heading and footer visible ([AdminSurface.css:5](web/src/AdminSurface.css#L5), [ResponsiveAdmin.css:6](web/src/ResponsiveAdmin.css#L6)).

Collections use a separate selection rail (230px) and content column with a (28px) gap, narrowing the rail to (190px) up to (1100px). Up to (700px), the rail becomes a horizontal scrolling row above the content. Collection artifact rows use small real previews and separate audience/actions; the editor stays within (760px) ([WorkspaceFeatures.css:89](web/src/WorkspaceFeatures.css#L89), [WorkspaceFeatures.css:562](web/src/WorkspaceFeatures.css#L562)).

Detail keeps the version selector above five horizontally scrollable sections: preview, versions, review and feedback, version comparison, and provenance. Mobile section buttons do not shrink. Comparison places two native interactive previews side by side at (400px) height, stacking at (360px) height up to (700px). Its file rail (200px), narrowing to (160px) up to (1100px), then stacks with a (180px) scroll limit up to (700px). Diff content has its own scrolling region capped at (600px). These are task-specific regions inside the reading plane, not extra floating cards ([Artifacts.tsx:760](web/src/Artifacts.tsx#L760), [ProductSurface.css:501](web/src/ProductSurface.css#L501), [WorkspaceFeatures.css:391](web/src/WorkspaceFeatures.css#L391)).

Review forms and thread rows remain flat. At (600px), comment/publish footers stack, primary submit actions fill their row, location fields and reviewer choices become one column, and reply indentation reduces. Provenance and comparison selectors use paired fields, becoming one column up to (500px). Operations use (16px) form gaps, a (560px) form maximum and (16px) separated list rows; up to (680px), actions follow the item details and forms fill available width ([ReviewPanel.css:257](web/src/ReviewPanel.css#L257), [WorkspaceFeatures.css:628](web/src/WorkspaceFeatures.css#L628), [OperationsFeatures.css:27](web/src/OperationsFeatures.css#L27), [OperationsFeatures.css:202](web/src/OperationsFeatures.css#L202)).

The guest reader has independent minimal chrome with brand, read-only label and language picker. Its centered canvas caps at (1400px), with (24px 32px) padding, becoming (20px 16px) up to (680px). The entry form is unboxed and capped at (400px); opened content uses the available reader height with a (70dvh) iframe minimum, reduced to (65dvh) on compact screens ([OperationsFeatures.css:123](web/src/OperationsFeatures.css#L123)).

## Elevation & Depth

Tone and fine rules do most of the organizing. Artifact tiles have light outlines and a restrained hover/focus shadow; flat list rows and administrative sections do not become floating panels. Login and first-run forms remain unboxed and shadow-free. Blocking dialogs and drawers carry the strongest elevation; transient feedback also floats above the workspace ([ProductSurface.css:39](web/src/ProductSurface.css#L39), [AdminSurface.css:5](web/src/AdminSurface.css#L5), [AuthSurface.css:115](web/src/AuthSurface.css#L115), [style.css:1028](web/src/style.css#L1028)). Exact shadow and motion values live in the sidecar.

**The Quiet Depth Rule.** Use borders and tone for ordinary organization; reserve stronger elevation for blocking overlays and transient feedback.

## Shapes

Controls use `control`, artifact tiles use `card`, and thumbnail apertures use `preview`. Dialogs use `dialog`, while side panels meet the viewport edge squarely. Audience labels use `scope`; avatars are circles. Generic inset panels retain `panel`, but administrative section containers are square, transparent and open to the page ([ProductSurface.css:39](web/src/ProductSurface.css#L39), [AdminSurface.css:5](web/src/AdminSurface.css#L5), [style.css:727](web/src/style.css#L727)).

The folded-page mark is Folio's reusable identity. The login sculpture is page material, not a framed preview or a prescribed container for all brand imagery. Actual artifacts are clipped only within their preview aperture; their own visual worlds are not shell design tokens.

## Components

### Buttons

Direct and familiar. Primary actions use Action Black with white text; secondary actions are white with a gray stroke; destructive actions use red. Ordinary buttons have minimum height (40px), compact buttons (32px), and authentication controls (48px). Disabled controls retain reduced opacity (0.48) and a disabled cursor. Hover changes fill or border over (0.15s); keyboard focus uses a blue outline (2px), offset (3px). Contextual icon buttons keep an accessible task label ([style.css:54](web/src/style.css#L54), [style.css:176](web/src/style.css#L176), [Artifacts.tsx:180](web/src/Artifacts.tsx#L180)).

### Inputs / Fields

White fields use the stronger field border, control radius, minimum height (40px) and padding (10px 12px). Labels sit above fields with hints below. Search has an inset icon and shortcut hint. Native checkboxes/radios use blue selection. Login has the larger field treatment and an explicit password-visibility control ([style.css:105](web/src/style.css#L105), [Auth.tsx:176](web/src/Auth.tsx#L176), [AuthSurface.css:159](web/src/AuthSurface.css#L159)).

### Navigation

The Folio mark and bold wordmark anchor the gray sidebar. Navigation combines outlined icons and readable labels; hover uses gray, active state a blue wash and stronger blue text. Workspace context sits above task groups; account and actual storage state sit below ([style.css:281](web/src/style.css#L281), [style.css:328](web/src/style.css#L328)). Compact navigation retains the existing modal focus and inertness behavior; preserve this shared navigation rather than making route-specific copies ([App.tsx](web/src/App.tsx)).

### Artifact Cards and Previews

A compact tile leads with the real artifact, then a title and a single metadata area: owner/avatar, modification time, audience and contextual share button. Opening is attached to the preview/title. Search includes locally extracted body text from the accessible version alongside title, description and tags ([search.go:90](internal/server/search.go#L90)). Grid descriptions stay hidden during ordinary browsing; actual search snippets are the exception and occupy at most two lines. List descriptions remain visible ([ProductSurface.css:543](web/src/ProductSurface.css#L543)). There is no repeated open/share footer strip. Loading and failed previews are explicit. Library iframes suppress pointer interaction; the detail iframe is interactive at its native reading viewport ([Artifacts.tsx:64](web/src/Artifacts.tsx#L64), [Artifacts.tsx:170](web/src/Artifacts.tsx#L170), [ProductSurface.css:53](web/src/ProductSurface.css#L53)).

### Audience Labels and Sharing

Audience pills combine text and an icon, with private, organization/selected and public tints. The side panel shows the saved current audience separately from editable radio choices. The four choices form flat separated rows; selection uses a pale blue wash. Member selection and expiry belong to the same task. The footer distinguishes draft changes from saved access and states that copying a link does not change permissions. Private access copy includes the owner and administrators ([Artifacts.tsx:306](web/src/Artifacts.tsx#L306), [Artifacts.tsx:313](web/src/Artifacts.tsx#L313), [Artifacts.tsx:460](web/src/Artifacts.tsx#L460), [ProductSurface.css:315](web/src/ProductSurface.css#L315)).

### Administrative Sections and Disclosures

Use aligned headings, actual statuses, flat rows, restrained fields and existing task actions. Member tables become labeled rows on narrow screens. Storage connections, retention policy, domain configuration and token scopes preserve their real controls and guards. Setup instructions use native `details`/`summary` disclosures with clear titles, supporting context and visible expansion state; Google setup, CLI guides and installation references follow this pattern. Forms remain the primary task, with long guidance progressively disclosed ([Admin.tsx:1310](web/src/Admin.tsx#L1310), [Admin.tsx:1777](web/src/Admin.tsx#L1777), [GoogleAuthSettings.tsx:203](web/src/GoogleAuthSettings.tsx#L203), [AdminSurface.css:93](web/src/AdminSurface.css#L93)).

### Review, Comments and Publication

The selected version remains explicit above the reading sections; option text distinguishes the official version, published versions and drafts. Upload has a native draft checkbox and explanatory text. Review shows a text status badge, state selector and save action; publication is a separate black action enabled after approval. Keep draft, review and official-publication states distinguishable in text ([Artifacts.tsx:582](web/src/Artifacts.tsx#L582), [Artifacts.tsx:760](web/src/Artifacts.tsx#L760), [ReviewPanel.tsx:195](web/src/ReviewPanel.tsx#L195)).

Comments use author/time metadata above readable plain-text bodies, optional file/location anchors, inline edit/delete/reply controls, a resolved-state label and lightly indented replies. Reviewer assignment and location fields use native disclosures; show-resolved uses a checkbox. The composer uses a resizable textarea, a minimum height (80px) and explicit submit feedback ([ReviewPanel.css:155](web/src/ReviewPanel.css#L155), [ReviewPanel.css:202](web/src/ReviewPanel.css#L202), [ReviewPanel.css:222](web/src/ReviewPanel.css#L222), [ReviewPanel.tsx:260](web/src/ReviewPanel.tsx#L260)).

### Collections, Tags, Favorites and Provenance

Collections use the shared navigation selection colors and flat preview-led rows. Editing uses ordinary fields, ordered rows with explicit move/remove controls, searchable artifact choices and a save/cancel footer. Keep the actual access note beside the collection content ([WorkspaceFeatures.tsx:285](web/src/WorkspaceFeatures.tsx#L285), [WorkspaceFeatures.tsx:350](web/src/WorkspaceFeatures.tsx#L350), [WorkspaceFeatures.tsx:516](web/src/WorkspaceFeatures.tsx#L516)).

Tags are compact neutral labels with control-shaped corners. Favorites use a star with a pressed state and an explicit add/remove label; they also have a library navigation entry. Provenance uses definition-list metadata and underlined source links, an edit form and a disclosure for the selected version's snapshot. Summaries preserve line breaks and long values wrap inside their column ([WorkspaceFeatures.tsx:653](web/src/WorkspaceFeatures.tsx#L653), [WorkspaceFeatures.tsx:815](web/src/WorkspaceFeatures.tsx#L815), [WorkspaceFeatures.css:312](web/src/WorkspaceFeatures.css#L312)).

### Version Comparison

Labeled before/after selectors lead the paired previews. Each preview requests its selected version with version isolation; preserve that selection and the existing content-origin boundary when reusing the preview component. Domain help is progressively disclosed with copyable values from the current response. Below the previews, text change counts lead a selectable file rail, size comparison, SHA-256 disclosure and bounded code diff. Non-text/oversized and unchanged files have explicit explanatory states ([WorkspaceFeatures.tsx:1041](web/src/WorkspaceFeatures.tsx#L1041), [WorkspaceFeatures.tsx:1074](web/src/WorkspaceFeatures.tsx#L1074), [WorkspaceFeatures.tsx:1093](web/src/WorkspaceFeatures.tsx#L1093), [WorkspaceFeatures.tsx:1129](web/src/WorkspaceFeatures.tsx#L1129)).

### Trash, Backup and Guest Access

Trash uses dated flat rows with size and restore actions. Permanent deletion first reveals an inline warning, destructive confirmation and cancel action. Backup creation uses a password field and black action, then dated download rows. Restore is its own ruled section with a native file picker, password, explanation, acknowledgment checkbox and guarded red action. Preserve the explanatory content and confirmation next to the consequential task ([OperationsFeatures.tsx:118](web/src/OperationsFeatures.tsx#L118), [OperationsFeatures.tsx:432](web/src/OperationsFeatures.tsx#L432), [OperationsFeatures.tsx:470](web/src/OperationsFeatures.tsx#L470)).

Guest-link management extends the sharing panel with a selected-version form, expiry, optional password and flat link-status rows with copy/revoke actions. The separate reader presents a read-only label, optional password gate, expiry/error copy and the artifact iframe, without management navigation ([Artifacts.tsx:494](web/src/Artifacts.tsx#L494), [OperationsFeatures.tsx:246](web/src/OperationsFeatures.tsx#L246), [OperationsFeatures.tsx:577](web/src/OperationsFeatures.tsx#L577)).

### Language Picker

A language icon and the shared Radix Select offer each language under its own name. Use an icon-only picker in authentication and the workspace header. Hosted and guest readers omit language and appearance utilities. It has a transparent fill, divider stroke, control radius, minimum height (36px), and the shared keyboard focus treatment. The current locale persists locally and updates the document language. French navigation labels and toolbars can wrap; keep translated controls within their region rather than shrinking labels ([LanguageSelector.tsx:13](web/src/LanguageSelector.tsx#L13), [LanguageSelector.css:1](web/src/LanguageSelector.css#L1), [i18n.tsx:27](web/src/i18n.tsx#L27)).

### Authentication, Feedback and Motion

Optional Google sign-in appears above the email divider only when enabled by the provider response, and never on first-run setup. The form keeps the existing password flow, password visibility, account help and errors. Google configuration remains an optional administrative form with progressive guidance; the visual overhaul does not imply a new authentication feature ([Auth.tsx:41](web/src/Auth.tsx#L41), [Auth.tsx:116](web/src/Auth.tsx#L116), [GoogleAuthSettings.tsx:82](web/src/GoogleAuthSettings.tsx#L82)). Fonts, mark, artwork and Google button image use local paths; asset provenance and licenses are in [brand-assets.md](docs/design/brand-assets.md).

Notices and errors state the actual outcome with text as well as color. Toasts use a dark surface. Drawer entrance takes (0.22s) with short horizontal motion; compact navigation takes (0.2s). Reduced-motion preference removes animation, transition and smooth scrolling ([style.css:1093](web/src/style.css#L1093), [style.css:1793](web/src/style.css#L1793), [style.css:2731](web/src/style.css#L2731)).

### Appearance and identity (v0.6)

The approved B composition supports a system default and explicit light/dark choices. Appearance controls sit in the authentication and workspace headers; hosted and guest readers omit them. `web/public/theme.js` resolves preference before the application paints; `Theme.css` maps canvas, navigation, text, selection, state, action and field-stroke roles. Dark mode keeps the cobalt accent, original paper sculpture and uploaded artifact colors. Stronger supporting text and field strokes satisfy the measured contrast targets in both modes; see [appearance references](docs/references/appearance.md).

Dropdowns use the common Radix trigger and portalled listbox: aligned labels/icons, a chevron, selected checkmark, visible focus, typeahead, scroll limits and reduced-motion support. They preserve keyboard and modal focus behavior. Integration marks identify actual services without substituting company logos for unknown compatible endpoints. Personal and organization photo editors use a local preview and explicit save/cancel/remove actions; private images replace initials in the workspace shell and member selectors, with an initials fallback.

## Do's and Don'ts

### Do:

- **Do** keep the gray navigation shell, white canvas, cobalt primary actions and blue active state together.
- **Do** preserve the established desktop/mobile heading scale and compact preview-led tiles.
- **Do** keep owner, modification time, audience and contextual sharing close to each artifact.
- **Do** give reading content the broad canvas and keep metadata contextual.
- **Do** use flat aligned administrative sections and progressively disclosed setup guidance.
- **Do** keep local fonts/assets, visible focus, honest states and the existing authorization controls.
- **Do** preserve explicit version, review and access labels, and let translated controls wrap or scroll within their region.

### Don't:

- **Don't** restore the rejected sage/ivory, serif, bookish visual world or poetic library labels.
- **Don't** confine all brand imagery to decorative cards or bring the login artwork stage onto mobile.
- **Don't** restore repeated open/share tile footers or make a thumbnail viewport change with library view mode.
- **Don't** substitute fabricated activity, customers, usage or health metrics for actual state.
- **Don't** express sharing scope through color alone or imply that copying a link saves draft permissions.
- **Don't** treat uploaded artifacts, research references or example-tour styling as shell design tokens.
- **Don't** turn operational screens into a marketing landing page or replace real task content with decorative cards.


## v0.8 operating-surface correction

This revision supersedes earlier scale and interaction notes: a 224px sidebar exposes Library, Collections, Agents and Settings; library filters and administrative tabs are contextual. Header utilities use an icon language menu and a theme-cycle button. The consistent action is cobalt in both modes, with quiet secondary controls and visible focus. Page headings are 24px; controls 32–36px with 13px text; mobile inputs 16px.

Cards are native links into the hosted reader. The reader has explicit information and sharing actions; it has no language or appearance selector that could imply transforming uploaded content. Account and organization editors place photo and editable identity together. Collection metadata is compact, selection/order explicit, optional fields disclosed. Agent setup is client selection and one token-inclusive command copy; Antigravity CLI (`agy`) replaces the old Gemini CLI primary choice. Audit is a compact filtered activity stream with attributed targets and allowlisted changes, not raw authentication event codes. Source/reference contracts: `docs/references/workspace-interaction.md`.
