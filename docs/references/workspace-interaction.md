# Workspace interaction and control system

The v0.8 correction follows the owner's annotated review: the library opens the actual reader first; metadata is secondary, hosted content does not offer Folio language/theme selectors, navigation is consolidated, identity editing is adjacent, and agent connection is a single copy action.

## Reference decisions

- [Linear workspace settings](https://linear.app/docs/workspaces) and [sidebar/settings separation](https://linear.app/changelog/2024-12-18-personalized-sidebar): frequent workspace destinations remain in the main sidebar; administration lives inside settings.
- [Radix Select](https://www.radix-ui.com/primitives/docs/components/select) and its installed `@radix-ui/react-select` types/source in `web/node_modules/@radix-ui/react-select`: retain the real trigger/listbox, keyboard navigation, typeahead, portal and form validation. The common component supports an icon-only language trigger without a fabricated select implementation.
- [Radix button variants](https://www.radix-ui.com/themes/docs/components/button): primary action, quiet secondary controls and icon-only utility actions have distinct jobs. These are visual references; Folio does not add the Radix Themes runtime.
- [WCAG text contrast](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html) and [non-text contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html): normal text remains at least 4.5:1; necessary control boundaries, functional icons and focus indicators use the 3:1 checks. Decorative card separators and ghost control borders are not treated as mandatory input boundaries.
- [OWASP logging](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html): actionable target and change attribution, bounded filters, and explicit exclusion of credentials and content. See [audit contract](audit-activity.md).

## Implementation boundaries

`WorkspaceSurface.css` owns the compact operating scale across both themes: 224px sidebar, 56px workspace header, 24px page headings, 13px controls, 32–36px control heights, one cobalt primary action role and quiet secondary actions. Mobile text inputs retain 16px text. User-authored iframe content keeps its own theme and language.

`App.tsx` keeps the existing hash destinations valid while showing four main destinations. Library filters and settings subsections are contextual. Language is an icon-triggered Radix menu; theme is an icon button cycling system/light/dark with a descriptive title. Both remain on Folio authentication/workspace chrome, not the `/s/` or `/g/` readers.

`Artifacts.tsx` uses real anchor links for cards, including native browser open-in-new-tab behavior. The reader's info button explicitly fetches the authenticated management record for its detail dialog; shared published-version metadata remains governed by the existing share endpoint contract. Sharing stays subject to server-side owner/admin checks. Metadata mutations refresh the management record independently. Reloading does not remount a consumed content grant, and library navigation refreshes data before mounting previews from a previous resource revision.

Profile/collection editor composition is documented in [editor layout](../design/editor-layout.md). Quick connection deliberately creates credentials only on an explicit copy action; see [client references](agent-connections.md). Installation choices and their validation limits are in [deployment references](deployment-options.md).
