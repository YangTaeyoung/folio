# Local organization and profile photos

Verified 2026-11-18 after fetching `origin/main` at `96d06c1abfd61394b8fb34c239c441b997b20ce0`. The implementation builds on the existing SQLite state mutation, member authentication, Origin/CSRF and encrypted backup contracts. No remote avatar URL, provider photo fetch, new dependency, cloud runtime, or credential change is required.

## Reference checks

- [Go image security considerations](https://pkg.go.dev/image@go1.26.0#hdr-Security_Considerations): call DecodeConfig and bound dimensions before full decoding. Confirmed installed Go 1.26.0 `src/image/image.go:23–37`; Folio also compares decoded dimensions/format to the header.
- [Go http.MaxBytesReader](https://pkg.go.dev/net/http#MaxBytesReader): bounded request reader reports an oversized body; installed `src/net/http/request.go:1176–1200` inspected. Folio reads at most 5 MiB before decoding and returns 413 on `MaxBytesError`.
- [Go http.DetectContentType](https://pkg.go.dev/net/http#DetectContentType): server-side sniffing is compared with the supplied media type and decoded format; filenames never select decoding or paths.
- [Go image/jpeg.Encode](https://pkg.go.dev/image/jpeg@go1.26.0#Encode) and [Go PNG encoder source](https://github.com/golang/go/blob/go1.26.0/src/image/png/writer.go#L584): encoding takes decoded image pixels. Installed `src/image/jpeg/writer.go:575–659` and `src/image/png/writer.go:584–660` inspected. Folio creates a new byte stream from decoded pixels, so uploaded EXIF/PNG metadata and appended payloads are discarded. Only JPEG/PNG are accepted; SVG, HTML, GIF and WebP are rejected.
- Existing repository sources inspected: `internal/server/server.go` (`require`, `csrf`, `Handler`, `save`, `mutate`), `internal/server/auth.go` (`members`, `adminSettings`, `/api/me`), `internal/server/backups.go` (`createBackup`, `restoreBackup`, validation) and `internal/server/restore_identity.go` (current destination identities win).

## Implementation evidence

`internal/server/profile.go` validates input bytes and dimensions, resizes proportionally to a 256-pixel bounding box, re-encodes pixels, stores private bytes, enforces active-member reads/session-only writes/admin-only organization mutation, audits changes and normalizes recovered photos. `User` and `Organization` expose only optional local `avatarUrl` values. Authenticated artifact projections also carry the enabled owner's local `ownerAvatarUrl`; anonymous/public projections omit it, and supplied stored/backup artifact URLs are cleared. URLs include a digest revision; reads retain the management server's `no-store` and `nosniff` headers and add a same-origin resource policy.

The aggregate sanitized photo budget is 8 MiB, independent of artifact quota. Individual sanitized records are limited to 300 KiB. Keeping bytes in the SQLite snapshot avoids dangling external files on commit failure; encrypted portable backup includes the bytes. Existing destination user profiles/photos win identity merging, imported-only historical user photos survive, and organization photos restore with organization metadata. Normalization strips any backup-supplied external URL or MIME value and rejects non-raster data. The merged destination photo budget is checked before commit.

`web/src/ProfilePhotos.tsx` provides a local preview with cancel/save/remove controls, accessible file labels, progress/error/status feedback, editable own name, and organization controls. It uses existing same-origin API/CSRF handling and existing theme variables. New messages are translated into Korean, English, Japanese, simplified Chinese and French. The server is authoritative for validation and permissions.

## Verification

`internal/server/profile_test.go` exercises real HTTP handlers for upload/replace/remove, metadata stripping and resize, active member reads, public/guest/disabled/temp-password/token/CSRF restrictions, mismatched type, SVG/HTML/truncation/oversized dimensions/body, private JSON shape, own-name validation and credential preservation, restart persistence, encrypted backup round trip and cross-installation identity merging, malicious backup URL/MIME/image rejection, aggregate photo budget and SQLite failure rollback. These tests use isolated temporary state only. Root integration additionally verifies the mounted controls, global avatars, responsive layout, localization and contrast in a real browser.
