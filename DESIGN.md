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
  hover-border: "#6290f8"
  secondary-border: "#d6d9df"
  secondary-hover: "#f6f7f9"
  scope-background: "#f1f3f7"
  scope-text: "#515b6b"
  danger: "#bd3038"
typography:
  headline:
    fontFamily: 'Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif'
    fontSize: "38px"
    fontWeight: 720
    lineHeight: 1.3
    letterSpacing: "-1.1px"
  headline-mobile:
    fontFamily: 'Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif'
    fontSize: "26px"
    fontWeight: 720
    lineHeight: 1.3
    letterSpacing: "-1.1px"
  section-title:
    fontFamily: 'Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif'
    fontSize: "18px"
    fontWeight: 650
    lineHeight: 1.4
    letterSpacing: "-0.3px"
  card-title:
    fontFamily: 'Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif'
    fontSize: "19px"
    fontWeight: 650
    lineHeight: 1.5
    letterSpacing: "-0.4px"
  card-title-mobile:
    fontFamily: 'Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif'
    fontSize: "17px"
    fontWeight: 650
    lineHeight: 1.5
    letterSpacing: "-0.4px"
  body:
    fontFamily: 'Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif'
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.55
  button:
    fontFamily: 'Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif'
    fontSize: "14px"
    fontWeight: 550
    lineHeight: 1.4
  metadata:
    fontFamily: 'Pretendard, -apple-system, BlinkMacSystemFont, "Apple SD Gothic Neo", system-ui, sans-serif'
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.55
  code:
    fontFamily: "ui-monospace, SFMono-Regular, Consolas, monospace"
    fontSize: "12px"
rounded:
  control: "7px"
  preview: "6px"
  panel: "8px"
  card: "9px"
  dialog: "12px"
  scope: "20px"
spacing:
  tight: "8px"
  compact: "12px"
  regular: "16px"
  grid-gap: "20px"
  panel-padding: "24px"
  desktop-gutter: "30px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.paper}"
    typography: "{typography.button}"
    rounded: "{rounded.control}"
    padding: "9px 15px"
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
  button-secondary:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.button}"
    rounded: "{rounded.control}"
    padding: "9px 15px"
  button-danger:
    backgroundColor: "{colors.danger}"
    textColor: "{colors.paper}"
    typography: "{typography.button}"
    rounded: "{rounded.control}"
    padding: "9px 15px"
  field:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "10px 12px"
  navigation-active:
    backgroundColor: "{colors.active-background}"
    textColor: "{colors.active-text}"
    rounded: "{rounded.control}"
    padding: "9px 12px"
  scope-label:
    backgroundColor: "{colors.scope-background}"
    textColor: "{colors.scope-text}"
    typography: "{typography.metadata}"
    rounded: "{rounded.scope}"
    padding: "4px 8px"
  artifact-card:
    backgroundColor: "{colors.paper}"
    rounded: "{rounded.card}"
  panel:
    backgroundColor: "{colors.paper}"
    rounded: "{rounded.panel}"
    padding: "{spacing.panel-padding}"
---

# Design System: Folio

## Overview

**Creative North Star: "The Artifact Workspace"**

Folio is a practical, contemporary workspace whose artifacts carry the visual interest. Cool gray navigation frames a white working canvas; clear sans-serif hierarchy, dark primary actions and blue selection make finding, opening and sharing work easy to scan. The shell is calm and precise, with restrained borders and enough room for real artifact previews.

This captures the built replacement world approved by the user as concept B on 2026-11-15: “B괜찮네 B로 가자, 이제 좀 프로덕트 같다.” The previous sage/ivory palette, bookish styling, serif wordmark, tiny labels and poetic library copy were rejected. The approved world is a product interface, with direct task labels and visible access state.

**Key Characteristics:**
- Neutral operating chrome around visually diverse artifacts.
- Strong headings, readable cards and concise Korean task labels.
- Black primary actions; blue selection, focus and active navigation.
- Real previews and explicit audiences, with honest loading and failure states.
- Locally bundled typography and assets.

Extracted from the reviewed local redesign, subsequently committed as `ad23c57` without further UI changes. The built implementation is authoritative for this document. The world and seed `4054feac` are recorded in `web/index.html:15`. The [finish review](docs/verification/2026-11-15-design-review.md) records `disposition: ship` with F1/F2/F3 resolved; it concerns the local redesign, not release or deployed-runtime verification. The document structure follows the [DESIGN.md format](https://raw.githubusercontent.com/google-labs-code/design.md/main/docs/spec.md).

## Colors

The palette is cool and neutral, with blue reserved for orientation and interaction. Frontmatter values are normative; the source palette lives in `web/src/style.css:7`.

### Primary

- **Action Black** (`primary`): Filled primary actions, including create, save and login; its lighter hover state retains the same character (`web/src/style.css:193`).
- **Interaction Blue** (`accent`): Keyboard focus outlines, active tabs and native selection controls. **Selection Wash** and **Selection Blue** (`active-background`, `active-text`) distinguish the current navigation item (`web/src/style.css:352`).

### Neutral

- **Canvas White** (`paper`): Main workspace, form fields, dialogs and cards.
- **Sidebar Gray** (`nav`): Navigation's quiet, persistent frame.
- **Working Ink** (`ink`): Core text. **Slate Metadata** (`muted`) supports secondary explanation without competing with titles.
- **Divider Gray** (`line`): Panel borders, separators and table structure. Field and card borders preserve their observed, slightly stronger strokes.
- **Scope Mist / Scope Slate** (`scope-background`, `scope-text`): Compact audience labels; the audience remains readable as text and icon.

Destructive actions use the dedicated red `danger`, while errors and warnings have their own pale status surfaces (`web/src/style.css:211`, `:976`). Do not turn status colors into general decoration.

**The Action and State Rule.** Use black for the main action and blue for selection, focus and active state; preserve their separate jobs.

## Typography

**Display and Body Font:** Locally served Pretendard Variable with Apple/system sans-serif fallbacks. The bundled WOFF2 supports weights (100–900), with `font-display: swap` (`web/src/style.css:1`).

**Code Font:** System monospace, reserved for code, commands, keys and configuration (`web/src/style.css:138`).

**Character:** One clear sans-serif voice supports both Korean and Latin content. Weight and scale create hierarchy; the wordmark is bold sans-serif, with the Folio mark as a separate asset (`web/src/Auth.tsx:13`, `web/src/style.css:252`).

### Hierarchy

- **Headline:** Page heading; desktop role is defined in frontmatter. At widths up to (900px), headings become (30px); up to (600px), use `headline-mobile` (`web/src/style.css:76`, `:2120`, `:2198`).
- **Card title:** Grid artifact titles use `card-title`, then `card-title-mobile` on small screens; retain their prominent scale. Long titles truncate with an ellipsis. List titles intentionally use (15px) desktop and (14px) mobile (`web/src/style.css:687`, `:820`, `:2254`).
- **Section title:** Default secondary heading uses `section-title`; specific panel titles can be (16px). Authentication form headings use (24px), increasing to (25px) on mobile.
- **Body:** Base body role is defined in frontmatter. Paragraphs use a looser line height (1.65); page descriptions have a maximum line length (72ch). Supporting descriptions commonly use (13px).
- **Metadata:** Audience, owner, timestamps and hints use `metadata`; small type supports primary labels rather than replacing them.
- **Buttons:** Medium-weight labels remain on one line; list actions must not break into individual Korean characters (`web/src/style.css:759`, `:836`).

**The Hierarchy Rule.** Preserve the approved heading and card-title hierarchy when adding screens; compact metadata must not set the scale of the whole interface.

## Layout

The desktop shell uses a fixed left sidebar (244px), a breadcrumb bar (56px), and a centered workspace capped at (1500px). Standard workspace padding is (34px 30px 50px). The artifact grid uses three equal columns with (20px) gaps, with preview surfaces at a (16:10) ratio. Header title and create action sit above a search/sort/view toolbar; actual content follows immediately (`web/src/style.css:281`, `:452`, `:485`, `:609`).

The responsive implementation changes structure rather than merely shrinking everything:

- At widths at least (1750px), workspace padding becomes (40px), and grid gaps become (24px).
- Up to (1250px), the sidebar becomes (225px), the grid becomes two columns, and main padding becomes (30px 25px).
- Up to (900px), the sidebar becomes an off-canvas (244px) navigation dialog, the main margin disappears, and a (58px) mobile header replaces the breadcrumb bar.
- Up to (600px), the grid becomes one column with (18px) gaps. Workspace padding becomes (26px 18px 35px); search takes its own full row. Form grids collapse to one column; panels use (20px 16px) padding.
- List view is a bordered row layout with small previews and dedicated action space. On small screens, metadata is hidden and actions stack, while action labels remain unbroken.

Sharing uses a right drawer (440px); detail uses a wide drawer (670px). Both fill the viewport height, with separately scrolling bodies and visible footers. Mobile drawers occupy the full viewport width. Standard dialogs are (470px), wide dialogs (680px), and mobile dialogs have (12px) outer gutters (`web/src/style.css:1018`, `:1076`, `:2307`).

Authentication uses a single centered white form (420px including 36px padding), a cool near-white canvas, and a compact top-left wordmark. The form heading is (24px); inputs and the primary button are (44px) high. Account guidance and an expandable help section sit below a divider. There is no marketing story pane or illustrative artifact stack. Up to (600px), the panel border/shadow are removed and the form uses the available width with (24px) page gutters and a (25px) heading. First-run setup reuses the same system at (460px) desktop width (`web/src/Auth.tsx`, `web/src/style.css`).

The [interactive showcase](examples/folio-showcase/index.html) is a separate product-tour artifact. It shares the cool neutral material, black actions, blue state and local Pretendard, but has its own type scale and responsive navigation. Its illustrative cards and explicitly labeled local access simulation are tour content, not a substitute for actual library data.

## Elevation & Depth

Borders and tonal separation establish the resting hierarchy. The sidebar is a tonal layer; white cards are lightly outlined. Keep depth restrained: primary buttons have a tiny shadow, artifact cards acquire a blue border/ring on hover or focus within, and blocking dialogs receive the strongest shadow. Authentication's centered form has a subtle downward shadow. Exact shadow values are carried in `.impeccable/design.json` (`web/src/style.css:197`, `:205`, `:624`, `:1031`, `:1874`).

**The Quiet Depth Rule.** Use borders and tone for ordinary organization; reserve stronger elevation for blocking overlays and transient feedback.

## Shapes

Controls have gently curved corners (`control`); panels are slightly softer (`panel`), artifact cards use `card`, and inset previews use `preview`. Dialogs are more rounded (`dialog`), while side drawers meet the viewport edge squarely. Audience labels are pills (`scope`); avatars are circles. These are distinct component roles, not one universal radius (`web/src/style.css:105`, `:176`, `:614`, `:730`, `:895`, `:1029`, `:1097`).

Use thin, cool gray borders to define edges. Preview clipping keeps real artifact content inside its thumbnail; text truncation prevents long titles, owner names and account information from distorting the layout.

## Components

### Buttons

Direct and familiar. Primary actions use Action Black with white text; secondary actions remain white with a gray border; destructive actions use red. Normal buttons have a minimum height (40px), compact buttons (32px), and authentication controls (44px). At small widths, normal button text becomes (13px). Disabled controls have reduced opacity (0.48) and a disabled cursor. Hover changes fill or border over (0.15s); keyboard focus uses a blue outline (2px), offset (3px) (`web/src/style.css:54`, `:62`, `:176`, `:216`, `:1959`).

### Inputs / Fields

White fields use the stronger gray field border and control radius, with minimum height (40px) and padding (10px 12px). Labels sit above fields with hints below; errors appear near the relevant task. Search includes an inset icon and a keyboard shortcut hint. Native checkboxes and radios use blue selection. Preserve visible labels, disabled surfaces and the shared keyboard outline (`web/src/style.css:105`, `:122`, `:529`, `:923`).

### Navigation

A bold Folio wordmark anchors the gray sidebar. Navigation items combine icons with readable labels; hover is gray and active state is a blue wash with stronger blue text. Workspace context sits above the task groups; account and storage information sit below (`web/src/App.tsx:52`, `web/src/style.css:293`, `:328`).

On compact screens, the closed sidebar is `inert`; the open sidebar has dialog semantics and makes covered content `inert`. Opening focuses the close button, Tab wraps at both boundaries, Escape closes, and closing restores the previous focus. Keep the visible close button and scroll lock (`web/src/App.tsx:287`, `:418`, `:524`).

### Audience Labels and Sharing Options

Audience pills combine text and an icon. The sharing drawer has four radio options: private, organization, selected members and public link. Selection adds a blue border and pale blue fill; the access note states who can open the artifact. Private copy explicitly includes the owner and administrators. Selected sharing exposes a member picker; expiry and link controls are part of the same task (`web/src/Artifacts.tsx:35`, `:219`, `:317`; `web/src/style.css:1149`).

### Artifact Cards and Previews

Cards prioritize actual artifact content, followed by title, description, owner, audience and time. The bottom action strip exposes opening and sharing. Real previews load the artifact's launch URL in an iframe; loading and failed-preview states are explicit. Grid thumbnails suppress pointer interaction; the larger detail preview enables it. Do not replace this with invented dashboard art or activity (`web/src/Artifacts.tsx:51`, `:132`; `web/src/style.css:639`, `:667`, `:752`).

### Panels, Tables and Administrative Tasks

Administration reuses the same page heading, bordered white panels, fields, notices and primary actions. Member lists use tables; storage and domain pages use labeled connections and configuration forms; agent setup uses token scopes and copyable code. Tables retain their scroll container on narrow screens. Counts, capacity and connection statuses must come from actual state (`web/src/Admin.tsx:89`, `:395`, `:1008`, `:1469`; `web/src/style.css:895`, `:1400`).

### Dialogs, Feedback and Motion

Dialogs and drawers keep their heading, close affordance and action footer visible while the body scrolls. Notices, error boxes and warnings state the actual outcome, with text accompanying color. Toasts use a dark surface for transient feedback. The drawer enters over (0.22s) with a short horizontal movement; the compact sidebar transitions over (0.2s). Reduced-motion preference removes animations, transitions and smooth scrolling (`web/src/style.css:949`, `:1018`, `:1093`, `:1793`, `:2526`).

## Do's and Don'ts

### Do:

- **Do** keep the gray navigation shell, white canvas, black primary actions and blue active state together.
- **Do** preserve the established desktop and mobile heading/card-title hierarchy.
- **Do** show actual artifact previews, explicit audience labels and honest loading, empty and error states.
- **Do** keep local fonts and assets available without a runtime CDN.
- **Do** retain keyboard focus, compact-navigation inertness, focus trapping and focus restoration.
- **Do** identify showcase simulations and illustrative content as examples.

### Don't:

- **Don't** restore the rejected sage/ivory, serif, bookish visual world or poetic library labels.
- **Don't** fill the workspace with fabricated activity, usage, customers or health metrics.
- **Don't** reduce primary task labels to metadata scale or let Korean action labels break character by character.
- **Don't** express sharing scope or status through color alone.
- **Don't** let a visually closed mobile sidebar remain keyboard reachable.
- **Don't** turn the tour's composition or simulation into the authenticated workspace's source of truth.
