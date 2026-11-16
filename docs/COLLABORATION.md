# Collaboration and recovery

Folio keeps working drafts, published revisions and feedback together without giving a collection, reviewer assignment or guest session broader artifact access. These features work with local storage and the offline core. Optional remote storage still needs its provider connection.

## Draft, review, publish

An artifact has a latest working `currentVersionId` and an official `publishedVersionId`. Existing artifacts keep automatic publication until you opt into review. Publish a new draft from the UI, or use:

```sh
folio inspect ARTIFACT_ID
folio publish --artifact ARTIFACT_ID --base-version CURRENT_VERSION_ID --draft ./report
```

The base must be the working version returned by inspect. Flags precede the local path. A stale base returns a conflict instead of overwriting someone else's revision. After review mode is enabled, later uploads remain drafts even if `--draft` is omitted. The official link stays on the previous publication until an owner/admin promotes an approved exact version. A new draft-only artifact has no official publication yet. ([CLI source](../cmd/folio/main.go#L89), [publication rules](../internal/server/artifacts.go#L436).)

A practical review loop:

1. Share the artifact privately, with selected active members or with the organization as needed.
2. Assign explicit reviewers. Reviewer assignment permits draft reading only for members who also have ordinary sharing access; it does not turn a private artifact into a share.
3. Publish a draft, open that exact version and add comments. A thread can refer to a file and text anchor. Replies stay on the same version; resolved threads must be reopened before replying.
4. Record `in_review`, `changes_requested` or `approved` for that exact version. Approval records a decision; it does not publish content by itself.
5. An owner/admin promotes the approved version using the official base observed before the decision. A competing promotion returns a conflict.

Comments, review state, launch, downloads, content files, comparison and search all check version access. Members outside the owner/admin/reviewer group see published revisions under the artifact's sharing policy. Guest links are explicit grants to one selected version. ([Version authorization](../internal/server/feature_types.go#L70), [review endpoints](../internal/server/reviews.go#L75).)

Version links use `/s/SLUG?version=VERSION_ID`. They keep the selected bytes and metadata snapshot while still checking current sharing permission. Each version records title, description, tags and provenance. Changing working context does not rewrite retained snapshots. On upgrade, old revisions receive the metadata available at migration time; earlier historical metadata cannot be recovered automatically. ([Snapshot migration](../internal/server/feature_types.go#L26).)

## Organize and find work

Create a private or organization-visible collection, then add readable artifacts in the order you want. A team collection can include restricted artifacts, but each viewer receives only the items they can already read. Collection membership never grants access. Favorites belong to the current user; tags belong to the artifact's working metadata. ([Collections](../internal/server/workspace.go#L108).)

Body search runs locally, combines the effective version's text with title, description and tags, and checks permission before returning a match or snippet. All whitespace-separated terms must match without case sensitivity. The index reads at most 2 MiB of supported HTML/text per version. It extracts text without running scripts and excludes script/style/template content. This is bounded source extraction, not browser rendering: CSS layout and runtime-generated text are not a search engine input. Metadata search remains useful when content cannot be read. ([Index and filtering](../internal/server/search.go#L14).)

Compare two readable versions side by side. The file list uses actual SHA-256 hashes to show added, removed, modified and unchanged files. Select a file to view text changes. Text is limited to 256 KiB and 1,000 lines per side; larger files, binary data and invalid UTF-8 retain their hashes without a line diff. No AI service is involved. ([Comparison](../internal/server/diff.go#L24).)

Provenance is optional: agent name, model, request summary, repository URL, commit, task URL and source links. Enter only the context you intend to share. Folio does not collect full chat transcripts, environment variables or credentials. Context changes apply to future version snapshots. ([Validation](../internal/server/workspace.go#L35).)

## Guest links

In artifact sharing settings, choose a retained version, a future expiry within one year and an optional password. A link can deliberately share a draft. It grants read access to that one version and no member-directory or management access. Its password is supplied separately and never embedded in the URL. Passwords require 12–72 bytes. ([Guest creation](../internal/server/guests.go#L98).)

Recipients open `/g/LINK_ID`. Protected links reveal no artifact title before unlock. Even an unprotected link establishes a separate guest session. The guest can open only the pinned version; a new artifact publication does not move the link. The selected canonical/custom hostname must belong to the same artifact.

Revoking a link invalidates its existing content sessions. Expiry, owner disablement and trashing also deny subsequent asset requests. Guest sessions last at most 12 hours and never outlast the link. Password failures are rate limited. If you trash and restore an artifact, its previous guest links stay revoked; create new links deliberately. ([Guest/session checks](../internal/server/guests.go#L31), [soft-delete invalidation](../internal/server/lifecycle.go#L116).)

Guest expiry belongs to the explicit guest grant. Ordinary artifact sharing settings do not replace a guest link's selected version or revoke it; revoke the guest link itself when access should end.

## Archive and trash

Archive keeps an artifact and its existing sharing available, while protecting it from automatic retention and quota eviction. It still counts toward quota. `folio archive ID` and `folio restore ID` are the archive/unarchive commands.

Delete moves an artifact into trash, records who deleted it and when, and denies ordinary artifact/content access. The owner or an administrator can restore it or permanently purge it. Trash keeps its bytes and consumes quota until purged. Maintenance deletes non-archived trash after 30 days. Archived trash remains protected from automatic purge; an owner/admin can still explicitly purge it. An in-flight upload prevents a conflicting deletion.

Archive restoration and trash restoration are separate operations. The MCP `folio_trash` tool takes `action:"trash"` or `action:"restore"`; permanent purge is intentionally absent from the agent tools. ([Lifecycle API](API.md#trash), [implementation](../internal/server/lifecycle.go#L13).)

## Portable backups and restore

Administrators can create and download a password-encrypted `.folio` file in the backup settings. The snapshot includes metadata and every retained version's objects, including files stored in external backends. It needs a 12–256-byte backup password. Keep that password separately; Folio cannot recover it.

Limits apply to the uncompressed archive and its metadata:

| Limit                       | Maximum |
| --------------------------- | ------: |
| Archive, including manifest | 512 MiB |
| Distinct retained objects   |  50,000 |
| Serialized state            |  16 MiB |
| Manifest                    |  32 MiB |

The state includes stored body indexes, so an installation can reach the metadata limit before reaching the object-byte limit. Oversized snapshots are rejected. Use the full-installation procedure in [INSTALL.md](../INSTALL.md#backup-and-restore) for larger installations. ([Bounds](../internal/server/backups.go#L36), [export](../internal/server/backups.go#L286).)

Snapshot and restore hold the server mutex during file/provider I/O. Other requests wait until the operation finishes. Active uploads return a conflict instead of allowing a restore to replace their state. Run a backup when this pause is acceptable. At the maximum size, reserve approximately 1 GiB of extra free disk for staging and output/restored copies, in addition to existing data and saved backups. Saved `.folio` files accumulate in the data directory's `backups/` folder; there is no automatic backup rotation.

To restore:

1. Create and download a backup of the current installation first.
2. Select the `.folio` file and enter its password in backup settings.
3. Read the replacement notice and confirm it. Validation/decryption and object staging finish before the server installs the imported state.
4. Refresh the workspace and inspect the restored artifacts and their permissions. Recreate API tokens and guest links that you still need.

The restore boundary is deliberately explicit:

| Data                                                                                | Portable restore behavior                                |
| ----------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Artifacts, versions, comments, reviews, collections, tags, favorites and provenance | Replaced with the snapshot                               |
| Retained local/remote object bytes                                                  | Copied into a fresh local backend under new opaque keys  |
| Original provider objects and existing local object directories                     | Not written or deleted                                   |
| Imported users                                                                      | Retained; current same-ID accounts/credentials win       |
| Current users and sessions                                                          | Preserved, including the administrator's current session |
| Current Google sign-in config and identity bindings                                 | Preserved                                                |
| Current DNS config/domain records and provider config/secrets                       | Preserved; no DNS provisioning action is run             |
| Installation encryption key                                                         | Preserved                                                |
| Imported external-provider secrets                                                  | Not activated; imported object references become local   |
| API tokens, guest links/sessions and content viewers/grants                         | Cleared; recreate access intentionally                   |
| Imported garbage queue                                                              | Cleared; restore never schedules source cloud deletion   |

A snapshot with an invalid artifact owner/reference, wrong password, modified/truncated encrypted data, unsafe ZIP entry or mismatched object checksum is rejected before state installation. Imported provider secrets remain encrypted inside the archive but are not imported with another installation's key. Recovery of the entire original deployment, including its exact provider credentials and encryption key, remains a separate stopped-service data-directory backup procedure. ([Restore](../internal/server/backups.go#L407), [validation](../internal/server/backups.go#L546).)

The HTTP restore body is the raw encrypted file. Its password is carried in a request header, not a URL. Unicode passwords use base64 of UTF-8 with an encoding header. Browser admin authentication and CSRF apply; API tokens and MCP cannot create/download/restore backups. See [API details](API.md#portable-backup-api).

## Languages

Korean (`ko`) is the default. The language selector also offers English (`en`), Japanese (`ja`), Simplified Chinese (`zh-CN`) and French (`fr`) on sign-in and workspace surfaces. Choose manually; the browser stores the preference and uses it on later visits and other tabs. Date/number formatting and document language follow the selection. Catalogs ship locally, so no translation API or CDN is required. Uploaded content, code and user-entered titles/comments are not translated. ([Locale store](../web/src/i18n.tsx#L8), [date formatting](../web/src/api.ts#L57).)

## CLI and 23 MCP tools

The CLI adds `--draft` and comma-separated `--tags` to publication. The MCP server exposes the workflow below; all calls use the configured user's scoped token and ordinary artifact permission. Review writes accept `review` or `publish`; owner/admin and `share` are still needed when changing reviewers or guest sharing. Agent guidance requires actual human authorization before recording an approval or promoting it. ([Tool registration](../internal/agent/mcp.go#L49), [workspace tools](../internal/agent/workspace.go#L84).)

| Tools                                                                                    | Purpose                                                                            |
| ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `folio_capabilities`, `folio_list`, `folio_inspect`, `folio_members`                     | Discover limits and authorized work/recipients                                     |
| `folio_publish`, `folio_share`, `folio_archive`, `folio_restore_version`, `folio_launch` | Publish, share, archive/unarchive, roll back and preview                           |
| `folio_feedback`, `folio_comment`, `folio_resolve_thread`, `folio_review`                | Read feedback, comment/reply, resolve/reopen and record exact-version review state |
| `folio_promote`, `folio_reviewers`                                                       | Publish an approved exact version and assign draft reviewers                       |
| `folio_compare_versions`, `folio_set_context`                                            | Compare real files and set explicit tags/provenance                                |
| `folio_collections`, `folio_save_collection`                                             | Read/save private or team collections with ordered items                           |
| `folio_guest_links`, `folio_create_guest_link`, `folio_revoke_guest_link`                | Manage explicit version-pinned guest grants                                        |
| `folio_trash`                                                                            | Trash or restore; never permanently purge                                          |

An agent review handoff can say:

```text
Inspect this artifact and read feedback for its exact working version. Apply the
requested changes in a new draft using the observed baseVersionId, preserve the
existing sharing, and record the explicit request summary as provenance. Request
review, then report the new version link. Wait for my approval before approving
or promoting that version. Do not put credentials or the full chat in provenance.
```

Core metadata, objects, review state and locale catalogs remain self-hosted. [Technical references](references/collaboration.md) tie the implementation to the consulted official documentation. This guide describes behavior and limits; release/deployment verification belongs in the release notes and verification records.

### Version preview domains

Side-by-side previews request `isolatedVersion:true` from the launch endpoint. Each retained version has a stable execution hostname made by substituting `v--` followed by a digest into the configured ContentURL placeholder. The server binds that hostname to exactly one artifact/version, including root-absolute scripts, styles and delayed fetches. New artifact slugs beginning with `v--` are reserved. Existing canonical artifact links remain unchanged. The comparison response includes `previewHosts`; the UI exposes them under **Preview domains**.

Route the content wildcard to the content listener on an internal installation, or register those exact version hostnames using the same DNS/tunnel setup as artifact domains. A hostname which has not been routed cannot render the preview; file comparisons remain available on the management origin. Do not point content hosts at the authenticated management listener.

Grant nonces exchange once into a browser credential. A consumed grant URL can redirect that same still-authorized browser credential again when an iframe reloads, without creating another viewer or extending expiry. A different browser, revoked credential, expired viewer, wrong host or wrong version still receives 404.

Guest unlock reuses an existing active guest session. New successful unlocks consume the same 15-minute attempt budgets as failures (10 per IP/link, 50 per link), with at most 1,000 active sessions per link and 5,000 installation-wide. Maintenance and new unlocks remove expired or revoked guest sessions. Restore reuses current identities matched by email and remaps imported ownership and ACL references, preserving current credentials.
