# Compact identity and collection editors

The identity editor is one section. A 72px photo and its explicit upload/delete actions sit beside the editable name. Personal email and role appear once as read-only metadata below the name. Organization settings use the same composition for logo and organization name. At narrow widths the photo controls become one short row above the fields.

Photo selection remains a draft until **Save photo** is pressed. The existing JPEG/PNG and size checks, object-URL cleanup, upload/delete endpoints, server errors, and save status remain in place. Saving a photo does not submit an edited name; saving a name does not submit a selected photo. These actions remain explicit because their APIs are separate and either request can fail.

The collection editor is capped at 720px. Its main flow is name → selected artifacts in display order → search and available artifacts → save. Description and sharing use secondary disclosures. The current sharing scope stays visible in the disclosure summary. Selecting a row moves it into the selected list; a visible check, order number, title, preview, type, and scope explain its state. Named up/down/remove controls support keyboard ordering. Empty selection has a short bordered instruction. There is one cancel/save footer and an icon-only, named close control above.

The prior composition placed full-width name, description, and sharing controls before the actual selection task, while selected artifacts appeared as a plain list of names. This increased scanning distance and left selection and ordering less visible. The revised list uses compact rows rather than additional cards. Collection detail rows link directly to the artifact reader.

## Implementation contracts

- `UserProfileSettings({ user, onSaved })` retains its public props.
- `OrganizationIdentitySettings({ organization, onSaved })` calls `onSaved` with the returned `Organization` after a name or photo mutation. The existing `OrganizationPhotoSettings` export remains available for older callers.
- Name writes retain `/api/me` and `/api/admin/settings`; photo writes retain `/api/me/photo` and `/api/organization/photo`.
- Collection save still sends name, description, visibility, ordered artifact IDs, and `baseUpdatedAt`. Ownership, administrator permissions, the 500-item limit, and collection/artifact permission separation are unchanged.
- Decorative editor previews are `inert` and hidden from assistive technology; visible row metadata and named controls carry the interaction.

## References checked

- [MDN: details disclosure element](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/details), checked 2026-11-19. Native summary controls provide an expandable secondary region.
- `web/src/ui.tsx`: `Field` pairs its generated ID with its label and hint. The editor retains that pairing.
- `web/node_modules/@types/react/index.d.ts`: React HTML attributes include `inert?: boolean`; editor previews use that attribute to keep embedded preview content outside the interaction sequence.
- `internal/server/auth.go`: `adminSettings` accepts `organizationName` and returns the settings object containing the organization.
