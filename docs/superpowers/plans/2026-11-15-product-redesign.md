# Folio Product Redesign Implementation Plan

> **For agentic workers:** Use native execution; independent public tour ownership and a fresh final reviewer. Preserve the existing API and data.

**Goal:** Replace the rejected visual world across the functioning app and its public sample, then deliver a tested usable result.
**Architecture:** Existing React/TS app, embedded Go delivery, unchanged HTTP authorization and content origins. Shared CSS/components carry the redesign across all views.
**Tech Stack:** React, TypeScript, Vite, Lucide, locally bundled Pretendard, existing Go/SQLite/MCP.
**Spec:** ../specs/2026-11-15-product-redesign.md

## Global constraints
- No backend/API/permission relaxation, no fake state, no runtime CDN.
- Artifact content remains isolated and all current scopes retain exact semantics.
- The user explicitly approved concept B (Workspace) on 2026-11-15.

## Review focus
- Real data and small libraries must look finished; no invented content/counts to fill a mockup.
- Long Korean titles and sidebar labels must remain readable at1440/390px.
- Scope switching, recipients, expiry and read-only users must stay truthful.
- Modal focus, mobile overlays, upload/versions/download/archive must still work.
- Public sample must explain/show Folio rather than render a generic fake KPI chart.

## Task1: Design system and working shell
Files: web/src/style.css, App.tsx, Auth.tsx, ui.tsx, public/favicon.svg, public/fonts, index.html.
- [x] Replace typography, tokens, shell/navigation/header, concise copy, login/setup and shared controls.
- [x] Preserve routes and existing API calls; use real library state; no duplicate navigation.
- [x] Build and check component semantics and current source types.

## Task2: Artifact gallery/sharing/administration
Files: web/src/Artifacts.tsx, Admin.tsx, style.css.
- [x] Larger real previews, simpler metadata, usable list mode; retain owner-only actions.
- [x] Four accessible sharing options, member picker, expiry and stable-link/save actions.
- [x] Apply coherent tables/forms/connections/versions/upload/empty/error styles.
- [x] Update E2E selectors only for intended copy/semantic changes, run full20 workflows.

## Task3: Public product tour
Independent owner: examples/folio-showcase/** only; consume existing product truth and B system. No API/server changes.
- [x] Create a polished responsive interactive static artifact with actual install/CLI/MCP links and clearly labeled simulations.
- [x] Include no fake product analytics, commercial claims or nonfunctional CTAs.
- [x] Root publishes as a revision of its own existing demo after review, retaining rollback.

## Task4: Inspect, review, deliver
- [x] One batched desktop/mobile capture of library/share/auth/storage and public tour; fix material gaps in one batch and confirm.
- [ ] Run detector once, fresh finish review with screenshot/comp/source packet; resolve its material findings, document DESIGN.md.
- [ ] Build combined binary; validate real API-backed UI workflows and live HTTPS with existing private credentials kept local.
- [ ] Save real screenshots; push/CI and publish updated distribution after validated changes. Never rewrite v0.1.0 or delete user data.
