# Authentication product polish

Baseline: main @ ab68530. The user identified the login view as looking less polished than a commercial product. This is a local refinement of approved Workspace B; the library and administration world are preserved.

## Change

Removed the two-pane marketing composition, illustrative placeholder artifacts and MIT slogan from authentication. A centered 420px form, compact wordmark, clear labels, neutral canvas and restrained border now place signing in first. Mobile uses an edge-free white form. First-run setup shares the same hierarchy at 460px desktop width. Password visibility is an accessible button that does not submit the form. Account help explains the existing administrator-created account/reset flow without inventing SSO or self-service recovery.

References inspected directly: [Vercel login](https://vercel.com/login), [Resend login](https://resend.com/login), [Linear login](https://linear.app/login). The first two expose focused sign-in tasks; Linear's retrieved page did not expose its loaded form, so no form behavior was inferred from that response. Actual Folio auth, fields, API calls, state and locally bundled design tokens were inspected before editing.

## Verification

- TypeScript and production build passed.
- Real Chromium screenshots inspected at 1440×960, 2048×1365, 768×1024 and 390×844. No horizontal overflow.
- Setup, invalid-password feedback, password display/hide, and expandable help verified in an isolated installation; no page errors.
- The existing 20-workflow browser suite passed; password-toggle assertions were added to the real member sign-in flow.
- Impeccable detector found no issues. No external runtime assets or authentication providers were added.

## Delivery evidence

- Source: `9473294146fb8dc3f5951884080343fa03e28c5a`, tagged `v0.2.1`. [Source CI](https://github.com/YangTaeyoung/folio/actions/runs/43724803845) passed all required checks.
- The dedicated [HTTPS installation](https://folio.melona.cloud) runs the clean tagged native build (`vcs.modified=false`). Fresh anonymous contexts verified the 420px desktop form, mobile layout, removal of the previous story pane, password visibility and account help. The existing administrator account successfully signed in; no page errors occurred.
- The release build is tracked in [this workflow](https://github.com/YangTaeyoung/folio/actions/runs/43725029106); public distribution is available through the [v0.2.1 release page](https://github.com/YangTaeyoung/folio/releases/tag/v0.2.1).
- Existing accounts, artifact data, domains, content-origin separation and public-demo history are preserved. This patch changes only the authentication presentation and its local interactions.

