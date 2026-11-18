# Workspace activity and security history

Verified 2026-11-19 against the implementation and [OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html).

OWASP distinguishes business/audit trails from security event logging under “Purpose”, recommends recording attributable actor/action/object context under “Event attributes”, and lists passwords, access tokens, session identifiers and encryption secrets under “Data to exclude”. Folio retains existing security events and presents them through a separate filter rather than deleting them. This is an application activity history, not a signed or tamper-evident compliance log.

## Recorded facts

`internal/server/audit.go` explicitly selects fields from the old and new persisted state. `Server.mutate` captures the state before mutation, enriches newly appended events before saving, and rolls back both the mutation and the audit changes on function or SQLite failure. Historical names and selected before/after values survive renaming and deleting a target. Recovery enriches only the newly appended `backup.restored` event; imported historical records are never retroactively given snapshots.

Selected snapshots include artifact title, visibility, sharing member and reviewer names, archive/trash state, version numbers and tags; member name, role and disabled/password-change-required state; collection name, visibility and contained artifact names; storage name, provider and default flag; domain hostname, status and target artifact ID; guest expiration, pinned version and protection/revocation flags; token name, scopes and expiry; organization name, capacity/retention policy and photo-presence flag; Google enabled and allowed-domain settings; explicit review status and comment resolved/deleted flags.

The snapshot deliberately excludes artifact/description/comment bodies, provenance text or URLs, blob contents, storage locations/configs, credentials, token values and hashes, passwords/verifiers, Google secrets/client IDs, DNS challenges, session identifiers and profile image bytes. Event labels still describe changes that have no allowlisted before/after values, such as password changes, image replacement, comments or creation-context updates. Existing raw `detail` remains backward compatible in the API; the new activity UI does not render or search it.

Old events retain their recorded actor, action, target ID, timestamp and detail. Their missing historical values remain absent. A target name resolved from the current state is explicitly marked `targetNameCurrent`; the UI calls it a current name. No historical before/after values are inferred from current state. Links are resolved from the current state and are omitted for trashed/purged artifacts. Artifact links use `/s/{slug}`. Administrative targets link to the relevant settings view.

## API

`GET /api/admin/audit` remains administrator-only. Calls with no new filter return all categories, newest first, with the existing default limit 100 and valid range 1–1000. The existing `{events:[...]}` envelope is preserved with additive metadata.

Query parameters:

- `category`: `workspace` (all except security), `all`, `content`, `sharing`, `members`, `settings`, `security`, `storage`, `domain`. Empty preserves the legacy all-category behavior.
- `actor`: exact actor ID; `__system__` selects events without a user actor.
- `q`: case-insensitive actor name, target name/ID, action identifier and allowlisted change value search. Raw detail is excluded.
- `from`, `to`: RFC3339 inclusive lower and exclusive upper time bounds. The UI converts local calendar dates to these boundaries, with the selected end day included.
- `limit`, `cursor`: page size and opaque event-ID cursor returned by the previous page. Filtering happens before limiting. New events do not displace a cursor's boundary.

The response adds `total` (all matching records, independent of cursor), `nextCursor` when more matches remain, `actors` for actor selection, and `X-Total-Count`. Invalid categories, dates/ranges and unknown or malformed cursors return 400. Existing event fields remain; optional additions are `category`, `targetType`, `targetName`, `targetUrl`, `targetNameCurrent`, `snapshot` and `changes:[{field,before,after}]`. Unknown or inapplicable values are `null`, not fabricated historical values.

## UI and verification

`web/src/AuditActivity.tsx` has an independent fetch effect, 50-row incremental pagination, actor/category/date/search controls, localized semantic labels and expandable changes. The default workspace category keeps routine authentication out of the activity view; the security category includes authentication, token and password/security-settings events. Rows are compact with responsive horizontal overflow, shared theme tokens, explicit loading/error/empty states and focusable links/buttons. The CSV action is labelled as exporting displayed records; it quotes cells and prefixes formula-leading values so untrusted names do not become spreadsheet formulas.

Integration coverage in `internal/server/audit_test.go` exercises actual authenticated routes, filtering before limiting despite 130 newer login events, cursor navigation and total count, date/actor/search combinations, legacy behavior, preserved security history, admin authorization, denied writes, persisted snapshots through rename/purge/restart, member/policy attribution, allowlist secrecy, mutation/SQLite failure rollback, and imported/legacy snapshot honesty. Browser acceptance of the complete settings shell remains part of the main task's UI validation.
