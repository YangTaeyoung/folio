# Folio HTTP integration contract (v0.5)

JSON camelCase. Error responses `{error:{code,message}}`, appropriate 400/401/403/404/409/413/429/5xx. No secrets in returned config/list/detail objects. Unauthorized artifact identifiers return 404. IDs opaque strings. Dates RFC3339 UTC. Metadata operations are JSON; uploads multipart.

## JSON models

User: `{id,email,name,role:"admin"|"member",disabled,mustChangePassword,createdAt,loginMethod?:"google",googleLinked?:boolean,avatarUrl?:string}`. Omitted `loginMethod` means password account (including password accounts linked to Google); `google` means Google-only, with no local password.
Organization: `{id,name,avatarUrl?:string}` (one per installation).
Artifact: `{id,slug,title,description,ownerId,ownerName,visibility:"private"|"organization"|"selected"|"public",memberIds:[],archived,expiresAt:null|string,createdAt,updatedAt,sizeBytes,currentVersionId,publishedVersionId,reviewEnabled,reviewerIds:[],shareUrl,kind,tags:[],provenance,favorite?,searchSnippet?,deletedAt?,deletedBy?}`.
Version: `{id,artifactId,number,createdAt,sizeBytes,backendId,files:[{path,mimeType,size}],message,published,provenance,title,description,tags:[],metadataSnapshot?}`.
Policy: `{quotaBytes,maxUploadBytes,retentionDays,overflowPolicy:"reject"|"delete_oldest"}`.
StorageBackend public: `{id,name,provider:"local"|"s3"|"gcs"|"azure",bucket,region,endpoint,prefix,accountName,credentialsConfigured,isDefault,createdAt}`.
Domain: `{id,hostname,artifactId,provider:"manual"|"cloudflare"|"domainconnect",status:"pending"|"verified"|"error",records:[{type,name,value}],lastError,verifiedAt}`.

## HTTP API

- GET /api/bootstrap -> `{needsSetup,version}`
- POST /api/setup `{token,organizationName,name,email,password}` -> `{user,csrfToken,organization}` + session cookie. Token from config/file, one-time bootstrap.
- POST /api/auth/login `{email,password}` -> `{user,csrfToken,organization}` + session cookie
- GET /api/auth/providers -> `{google:{enabled:boolean,startUrl:"/api/auth/google/start"}}` (public; no provider network request).
- GET /api/auth/google/start -> Google authorization-code redirect with browser-bound, five-minute single-use state, nonce, and S256 PKCE; scopes `openid email`. Retry friction is per opaque browser cookie: 10 starts per 15 minutes, reset on successful login; no shared peer-IP quota or trusted forwarded headers. Hard bounds: 1,000 live pending states and 16 concurrent token/JWKS exchanges. An over-cap callback consumes its state and redirects `google_rate_limited`; start again to retry.
- GET /api/auth/google/callback -> existing session cookie and `/#library`; errors redirect to `/?auth_error=SAFE_CODE` without provider details. Codes: `google_disabled`, `google_rate_limited`, `google_state_invalid`, `google_denied`, `google_failed`, `google_account_denied`. No signup: only existing active members with verified authoritative Gmail/Workspace emails; persistent issuer/subject binding rejects reassignment.
- GET /api/admin/auth/google -> `{enabled,clientId,clientSecretConfigured,allowedDomains:[],redirectURI}`; admin session required.
- PATCH /api/admin/auth/google `{enabled,clientId,clientSecret?:string,allowedDomains:[]}` -> same; admin session + CSRF. Omitted/blank secret retains encrypted stored secret. Nonempty hosted-domain restriction uses signed `hd`, never email suffix. Disabling/changing settings preserves at least one active admin with an available sign-in method; pending logins are invalidated.
- GET /api/me -> `{user,csrfToken,organization}`; 401 otherwise
- PATCH /api/me `{name}` -> `{user}`; current member session + CSRF only, 1–100 Unicode characters without control characters. Credentials, role, email and sessions unchanged.
- PUT /api/me/photo -> `{user}`; current member session + CSRF only. Raw `image/jpeg` or `image/png` body, matching sniffed type, at most 5 MiB and 2048 × 2048 pixels. Re-encoded and resized proportionally into 256 × 256; no uploaded metadata/trailing bytes retained.
- DELETE /api/me/photo -> `{user}`; current member session + CSRF only; clears avatarUrl.
- PUT /api/organization/photo -> `{organization}`; administrator session + CSRF only; same image limits.
- DELETE /api/organization/photo -> `{organization}`; administrator session + CSRF only.
- GET|HEAD /api/me/photo, /api/members/{id}/photo, /api/organization/photo -> binary sanitized image; active authenticated member only, API token requires `read`; guests/anonymous denied, missing/disabled target 404. `avatarUrl` values are local authenticated URLs with content revision queries; responses are `no-store` and `nosniff`. No raw photo bytes in public JSON models. Aggregate sanitized photo bytes capped at 8 MiB independently of artifact storage quota. Image/photo limits use `invalid_photo`, `photo_too_large`, `photo_storage_full`; invalid name uses `invalid_profile_name`.
- POST /api/auth/logout -> 204
- POST /api/auth/password `{currentPassword,newPassword}` -> `{user,csrfToken}` (session rotated)
- GET /api/artifacts?view=library|mine|shared|archive|all&q=&sort=updated|title&tag=&favorite=true -> `{artifacts,usage:{usedBytes,quotaBytes},policy}`. all admin only; archive contains only readable archived items.
- POST /api/artifacts: multipart field `metadata` JSON `{title,description,slug?,visibility?,memberIds?,expiresAt?,message?,draft?,tags?,provenance?,files:[{path,mimeType?}]}`; numbered parts `file0`,`file1`,... -> `{artifact,version}`.
- GET /api/artifacts/{id} -> `{artifact,versions}`
- POST /api/artifacts/{id}/versions: multipart same metadata plus `baseVersionId` -> `{artifact,version}`; stale base 409.
- PATCH /api/artifacts/{id} `{title?,description?}` -> `{artifact}`
- PATCH /api/artifacts/{id}/sharing `{visibility,memberIds,expiresAt}` -> `{artifact}`
- POST /api/artifacts/{id}/archive -> `{artifact}`
- POST /api/artifacts/{id}/restore -> `{artifact}`
- DELETE /api/artifacts/{id} -> 204; owner/admin soft deletion into 30-day trash, not permanent deletion.
- POST /api/artifacts/{id}/versions/{versionId}/restore -> `{artifact}`
- GET /api/artifacts/{id}/download?versionId=VERSION_ID -> zip attachment (authorized, never inline HTML)
- POST /api/artifacts/{id}/launch `{versionId?,hostname?}` -> `{url}` short-lived single-use content grant, bound to artifact/version/principal/originating-credential/exact destination host. hostname must be the canonical artifact-isolated host or a verified domain for that same artifact. Read-authorized, cookie CSRF protected when authenticated. Anonymous only for public artifacts.
- GET /api/share/{slug}?version=VERSION_ID -> `{artifact}` for permitted user or anonymous public. No content leaks on denial.
- GET /s/{slug} -> trusted frontend viewer route (normal app shell assets); fetched metadata checks access.
- Content site /grant/{nonce} -> exchange once on the nonce's exact content site, artifact-scoped viewer cookie, redirect /render/{artifactId}/{versionId}/index.html. A content site is `<app-url>/c/{slug}` in path mode (cookie `Path=/c/{slug}/`) or the artifact's own hostname in subdomain mode (host-only cookie); the launch `url` is absolute in both. EVERY file checks original session/token remains active, scope and that the incoming site matches this artifact. Each canonical site resolves exactly one artifact from ContentURL template `{artifact}`. Host-mapped verified custom domain `/...` resolves that artifact root under same checks; private unauthenticated root redirects to AppURL/s/{slug}?domain=hostname for trusted login/grant exchange.
- GET /api/members -> `{members}` active member directory for sharing (admin gets disabled too)
- POST /api/admin/members `{name,email,role,password?,loginMethod?:"password"|"google"}` -> `{user,temporaryPassword?}` displayed once. Google-only requires enabled Google provider and an email; password must be omitted, no temporary password or password-change gate. Existing users keep password behavior.
- PATCH /api/admin/members/{id} `{role?,disabled?,name?}` -> `{user}`
- POST /api/admin/members/{id}/reset-password -> `{temporaryPassword}` displayed once; revoke sessions/tokens. Google-only returns 409 `google_only_account`; Google-only also cannot use password login/change.
- GET /api/admin/settings -> `{organization,policy,appUrl,contentUrl}`
- PATCH /api/admin/settings `{organizationName?,policy?}` -> same
- POST /api/admin/cleanup `{dryRun:true|false}` -> `{candidates:[{id,title,reason,sizeBytes}],freedBytes}`
- GET /api/admin/audit?limit=100 -> `{events:[{id,actorId,actorName,action,targetId,createdAt,detail}]}`
- GET /api/storage -> `{backends}` (admin only)
- POST /api/storage `{name,provider,bucket,region,endpoint,prefix,accountName,accessKeyId?,secretAccessKey?,sessionToken?,serviceAccountJson?,connectionString?,accountKey?}` -> `{backend}` (encrypted credentials)
- POST /api/storage/{id}/test -> `{ok,message}`; perform bounded put/get/delete of random probe key
- POST /api/storage/{id}/default -> `{backend}`
- DELETE /api/storage/{id} -> 204 or 409 if referenced/default
- GET /api/tokens -> `{tokens:[{id,name,createdAt,expiresAt,lastUsedAt,scopes}]}` for self
- POST /api/tokens `{name,scopes:["read","publish","share","review"],expiresAt?}` -> `{token,record}` one-time plaintext
- DELETE /api/tokens/{id} -> 204 self only
- GET /api/capabilities -> version, offline mode, upload limits, share modes and CLI/MCP setup metadata
- Domain/DNS endpoints below use admin authentication and CSRF for browser mutations. Implemented by internal/server domain routes and the internal/dns service.
- GET /api/domains -> `{domains}`
- POST /api/domains `{hostname,artifactId,provider,target?,cloudflareZoneId?}` -> `{domain}` with ownership challenge and desired record
- POST /api/domains/{id}/verify -> `{domain}`; verify TXT + routing DNS using resolver; injected resolver for tests
- DELETE /api/domains/{id} -> 204 (remove only Folio-owned record if explicit `?deleteDns=true`, otherwise detach)
- GET /api/admin/dns/providers -> `{providers}` (manual, Cloudflare, Domain Connect capabilities honestly reported)
- POST /api/admin/dns/cloudflare/connect `{apiToken?}` -> `{connected,zones}`; credentials encrypted, no echo
- POST /api/admin/dns/cloudflare/login -> `{id,authorizationUrl,status}` invokes official cloudflared login
- GET /api/admin/dns/cloudflare/login/{id} -> `{status,error?}`
- GET /api/admin/dns/cloudflare/tunnel -> `{configured,running,healthy,id?,name?,hostnames:[]}`
- POST /api/admin/dns/cloudflare/tunnel `{name,hostnames:[...],create:true}` -> same, create/select only Folio-dedicated tunnel and exact ingress routes. Existing DNS collisions refused.
- POST /api/admin/dns/cloudflare/tunnel/start -> same; start this installation's connector only
- POST /api/admin/dns/cloudflare/tunnel/stop -> same; stop its owned connector only
- POST /api/domains/{id}/provision -> `{domain}` uses configured provider and refuses collisions; Cloudflare API or official logged-in tunnel DNS route
- POST /api/admin/dns/domainconnect/discover `{domain}` -> `{supported,providerName,authorizationUrl?,instructions?}`; templates/provider IDs are configured, no arbitrary callback URL trust
- POST /api/domains/{id}/authorize -> `{authorizationUrl,stateExpiresAt}` for supported configured Domain Connect template; callback GET /api/dns/domainconnect/callback?state=... validates single-use state, then verifies DNS. Unsupported provider gets manual fallback instead.

## Collaboration, recovery and locale API additions

Browser session mutations require the management `Origin` and `X-CSRF-Token`; API tokens use their granted scopes. Guests use the separate unlock/CSRF flow below. Owner/admin rights are additional to token scopes. Administrator operations require a browser session and reject API tokens.

### Versions and review

`currentVersionId` is the working version. `publishedVersionId` is the official version. Legacy artifacts publish automatically until the first `draft:true` upload or explicit promotion enables review. Thereafter every upload remains a draft until promoted. Previously published versions remain readable under the artifact sharing policy. Owners/admins can read drafts; explicit `reviewerIds` permit draft access only when the member also has ordinary artifact sharing access. Guest links are the explicit version-bound exception. Lists, downloads, content assets, search, diff and comments apply the same version rules. Public version JSON omits private storage keys and the internal search index.

| Method and route                                  | Input / response                                               | Permission                                                                                          |
| ------------------------------------------------- | -------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| GET `/api/artifacts/{id}/comments?versionId=...`  | `{comments:[Comment]}`                                         | `read`, readable exact version                                                                      |
| POST `/api/artifacts/{id}/comments`               | `{versionId,body,parentId?,path?,anchor?}` → `{comment}` (201) | `review` or `publish`, readable exact version                                                       |
| PATCH `/api/artifacts/{id}/comments/{commentId}`  | `{body?,resolved?}` → `{comment}`                              | `review` or `publish`; author/admin edits body; thread author or artifact owner/admin resolves root |
| DELETE `/api/artifacts/{id}/comments/{commentId}` | 204; retains deleted-thread marker                             | `review` or `publish`; author/admin                                                                 |
| GET `/api/artifacts/{id}/review?versionId=...`    | `{review}`                                                     | `read`, readable exact version                                                                      |
| PUT or PATCH `/api/artifacts/{id}/review`         | `{versionId,status}` → `{review}`                              | `review` or `publish`, readable exact version                                                       |
| PATCH `/api/artifacts/{id}/reviewers`             | `{reviewerIds:[]}` → `{artifact}`                              | owner/admin + `share`                                                                               |
| POST `/api/artifacts/{id}/promote`                | `{versionId,basePublishedVersionId}` → `{artifact,review}`     | owner/admin + `publish`; exact version approved; base required, including `""` when unpublished     |

Review status is `draft`, `in_review`, `approved` or `changes_requested`. A base mismatch returns 409. Approval does not itself promote a version. The API records who set the status; an agent must not claim human approval without it. Comment bodies contain 1–20,000 bytes. A reply targets a root thread on the same version, inherits its location and requires the thread to be open. An optional root `path` must exist in that version; `anchor` needs a path and is limited to 1,000 bytes without control characters.

`Review`: `{artifactId,versionId,status,updatedBy,updatedByName,updatedAt}`.
`Comment`: `{id,artifactId,versionId,parentId?,authorId,authorName,body,path?,anchor?,resolved,deleted,createdAt,updatedAt}`.

Version metadata snapshots include title, description, tags and provenance. Legacy versions without snapshots receive the artifact metadata available during migration; Folio cannot reconstruct earlier historical values. Working metadata edits do not rewrite retained snapshots. `/s/{slug}?version=VERSION_ID` selects an authorized immutable revision; sharing still applies.

### Collections, tags, favorites and provenance

| Method and route                     | Input / response                                                                | Permission                             |
| ------------------------------------ | ------------------------------------------------------------------------------- | -------------------------------------- |
| GET `/api/collections`               | `{collections}`                                                                 | `read`                                 |
| GET `/api/collections/{id}`          | `{collection,artifacts}`                                                        | `read`, visible collection             |
| POST `/api/collections`              | `{name,description?,visibility?,artifactIds?}` → `{collection}` (201)           | `publish`                              |
| PATCH `/api/collections/{id}`        | `{name?,description?,visibility?,artifactIds?,baseUpdatedAt?}` → `{collection}` | owner/admin + `publish`                |
| DELETE `/api/collections/{id}`       | 204                                                                             | owner/admin + `publish`                |
| PUT `/api/artifacts/{id}/favorite`   | `{favorite:boolean}` → `{artifact}`                                             | `publish`, readable artifact; per-user |
| PATCH `/api/artifacts/{id}/metadata` | `{tags?:[],provenance?:Provenance}` → `{artifact}`                              | owner/admin + `publish`                |

`Collection`: `{id,name,description,ownerId,visibility:"private"|"organization",artifactIds:[],createdAt,updatedAt}`. Artifact IDs preserve order; at most 500 supplied IDs. Each item is filtered by the caller’s artifact permission. Collection visibility never grants artifact access. Updating a visible subset retains hidden existing items. A supplied stale `baseUpdatedAt` returns 409. Tags allow at most 30 distinct entries of 1–40 characters.

`Provenance`: `{agent?,model?,summary?,repository?,commit?,taskUrl?,sources?:[]}`. Links must be HTTP(S) without userinfo credentials; source links are limited to 20. Agent/model/commit each allow 200 bytes, summary 8,000 bytes. Folio records only explicit inputs; it does not ingest conversations or environment variables.

### Search and comparison

GET `/api/artifacts` supports `q`, `tag` and `favorite=true`, alongside view/sort. Search combines effective version body text with authorized title/description/tags. Whitespace-separated query terms must all match, case-insensitively. Results contain bounded `searchSnippet` text, never executable markup. The local index reads at most 2 MiB of supported text/HTML per version; scripts/styles/templates are excluded and no scripts run. Storage failure leaves available metadata search as a fallback. No model, external search service or runtime network translation is needed.

GET `/api/artifacts/{id}/diff?from=VERSION_ID&to=VERSION_ID&path=OPTIONAL_FILE` requires `read` and access to both versions. Returns `{from,to,files,summary,textLimitBytes:262144,textLimitLines:1000}`. Each file includes `{path,status,beforeBytes,afterBytes,beforeHash?,afterHash?,lines?,textOmitted?}`; status is `added`, `removed`, `modified` or `unchanged`. Hashes are SHA-256 over stored bytes. Optional selected-file lines use `{kind:"context"|"added"|"removed",text}`. Binary/invalid UTF-8/large files keep their hashes and omit text. Text limits are 256 KiB and 1,000 lines per side.

### Trash

| Method and route               | Response                                             | Permission              |
| ------------------------------ | ---------------------------------------------------- | ----------------------- |
| GET `/api/trash`               | `{artifacts,retentionDays:30}`; only own/admin items | `read`                  |
| POST `/api/trash/{id}/restore` | `{artifact}`                                         | owner/admin + `publish` |
| DELETE `/api/trash/{id}`       | 204; permanent purge                                 | owner/admin + `publish` |

Soft deletion sets `deletedAt`/`deletedBy`, hides the artifact from ordinary access and revokes its existing guest links and content sessions. Restore clears the deletion marker; prior guest links stay revoked. Trash still consumes quota until purged. Archive is separate: archived artifacts retain access and are protected from automatic retention/quota eviction and expired-trash purge. Explicit permanent purge is still allowed for archived trash. Maintenance or non-dry-run admin cleanup purges non-archived trash after 30 days; an active upload blocks destructive lifecycle operations.

### Guest links

| Method and route                                       | Input / response                                                                            | Permission                                            |
| ------------------------------------------------------ | ------------------------------------------------------------------------------------------- | ----------------------------------------------------- |
| GET `/api/artifacts/{id}/guest-links`                  | `{guestLinks}`                                                                              | owner/admin + `share`                                 |
| POST `/api/artifacts/{id}/guest-links`                 | `{versionId,expiresAt,password?}` → `{guestLink}` (201)                                     | owner/admin + `share`                                 |
| POST `/api/artifacts/{id}/guest-links/{linkId}/revoke` | 204                                                                                         | owner/admin + `share`                                 |
| GET `/api/guest/{linkId}`                              | `{id,passwordProtected,unlocked,expiresAt}`; unlocked adds `{artifact,versionId,csrfToken}` | valid link; no account required                       |
| POST `/api/guest/{linkId}/unlock`                      | `{password?}` → `{artifact,versionId,csrfToken}` and guest cookie                           | exact management `Origin`, valid password if set      |
| POST `/api/guest/{linkId}/launch`                      | `{hostname?}` → `{url}`                                                                     | exact `Origin`, guest cookie and guest `X-CSRF-Token` |

`GuestLink` public: `{id,artifactId,versionId,createdBy,createdAt,expiresAt,revokedAt?,passwordProtected,url}`. URL is `/g/{linkId}`; the password is never in it. Choose an explicit retained version and a future expiry within 365 days. Optional password is 12–72 bytes. Even unprotected links establish a guest session through POST unlock. Protected metadata reveals no title before unlock. Guest metadata omits organization directory IDs and sessions have no member/control API rights.

Credentials are stored as hashes and cookie scope is host-only, HttpOnly, SameSite=Lax (Secure on HTTPS). Password failures are limited per link/peer (10) and per link globally (50) over 15 minutes. Guest sessions expire within 12 hours or the link expiry, whichever is earlier. Revoke, link expiry, soft deletion and owner disablement invalidate existing content access; every asset rechecks the exact pinned version. Custom host selection must still belong to this artifact.

### Portable backup API

All routes require an administrator browser session; mutations also require its normal CSRF headers.

| Method and route            | Input / response                                                      |
| --------------------------- | --------------------------------------------------------------------- |
| GET `/api/backups`          | `{backups:[{id,createdAt,sizeBytes,scope}],scope,maxBytes:536870912}` |
| POST `/api/backups`         | JSON `{password}` → `{backup}` (201)                                  |
| GET `/api/backups/{id}`     | encrypted `.folio` attachment                                         |
| POST `/api/backups/restore` | raw `.folio` body, `X-Backup-Password` → `{restored:true,scope}`      |

For Unicode password headers, use `X-Backup-Password-Encoding: base64` and base64 of UTF-8 bytes in `X-Backup-Password`. Plain header values remain supported. Passwords are 12–256 bytes and must not be included in query strings or command logs.

The encrypted archive contains the state snapshot and every retained-version object, including remote objects. Limit: 512 MiB uncompressed ZIP including manifest, 50,000 objects, 16 MiB serialized state and 32 MiB manifest. The format uses scrypt-derived AES-256-GCM records with an authenticated end marker. Restore rejects malformed paths, non-regular/compressed/duplicate ZIP entries, excessive sizes, invalid state references, wrong passwords, truncation and modified data; staged object SHA-256 checks complete before state installation.

Snapshot and restore hold the server state mutex during I/O, so they block other requests. Active uploads return 409 instead of racing restoration. Files are disk-staged; reserve space for the archive and staging alongside existing objects. Backups accumulate locally until the operator removes saved files.

Restore replaces artifact/version/collaboration state using a fresh local backend and newly generated object keys. It does not write/delete source cloud objects. Imported users are retained, but current same-ID users and credentials win. Current sessions, Google authentication/recovery bindings, DNS config/domains, provider config/secrets and the installation encryption key are preserved. Imported API tokens, guest links/sessions, viewer/grant sessions and garbage queue are cleared. Imported external-provider secrets are not activated or decrypted with the destination key. A restored artifact with a missing owner is rejected. See [operational steps and scope](COLLABORATION.md#portable-backups-and-restore).

### Locales

`GET /api/capabilities` advertises `locales:["ko","en","ja","zh-CN","fr"]`, feature names and `tokenScopes:["read","publish","share","review"]`. Locale selection is a browser preference, stored locally as `folio.locale`; Korean is the default. UI catalogs ship with the executable. Locale switches update document language and date/number formatting. Uploaded content, user metadata and code are not translated.

## Go package ownership and interfaces

Module `github.com/YangTaeyoung/folio`; Go floor 1.26. Root owns go.mod/go.sum dependency changes.

`internal/blob` implements:

```
type Config struct { ID,Name,Provider,Root,Bucket,Region,Endpoint,Prefix,AccountName,AccessKeyID,SecretAccessKey,SessionToken,ServiceAccountJSON,ConnectionString,AccountKey string }
type Store interface { Put(context.Context,string,io.Reader,int64,string) error; Open(context.Context,string)(io.ReadCloser,error); Delete(context.Context,string) error; Test(context.Context) error }
func New(Config)(Store,error)
```

All object keys generated by server, not raw user paths. New validates provider config. Test verifies write/read/delete. Server injects BlobFactory `func(blob.Config)(blob.Store,error)` for tests. Local backend root defaults DataDir/objects. Adapter never logs config/credentials.

`internal/server` implements:

```
type Config struct { DataDir,AppURL,ContentURL,BootstrapToken,Version string; WebFS fs.FS; BlobFactory func(blob.Config)(blob.Store,error); DNS DNSService }
func New(Config)(*Server,error)
func (s *Server) Handler() http.Handler
func (s *Server) ContentHandler() http.Handler
func (s *Server) Close() error
func (s *Server) BootstrapToken() string
func (s *Server) Maintenance(ctx context.Context,dryRun bool)(CleanupResult,error)
```

Server includes DNS/domain persistence and HTTP authorization; call internal/dns provider functions for network/provider operations. No circular imports. If DNS provider package not ready, declare an interface in server or import internal/dns once agent provides it; coordinate exact interface promptly.

`internal/dns` pure service (no server imports), proposed:

```
type Record struct {Type,Name,Value string}
type Cloudflare struct {APIToken,APIBase string; HTTPClient *http.Client}
func (*Cloudflare) Zones(context.Context)([]Zone,error)
func (*Cloudflare) Provision(context.Context,zoneID,hostname,target string)(recordID string,err error)
func (*Cloudflare) DeleteOwned(context.Context,zoneID,recordID,hostname,target string) error
func Verify(ctx context.Context,hostname,challenge,target string)(bool,error)
```

DNS contributor may extend these interfaces before core integration, recording exact exports in docs/API.md. Core owns storing encrypted DNS config/domain records; provider code is independently testable.

`internal/agent` and cmd/folio implement CLI serve, publish, login/token config, artifacts list, inspect, share, archive/restore, doctor, mcp, skills install. API client uses the HTTP routes above. Commands must not output secrets except intentionally generated token/password once to trusted local console; no secrets in flags when stdin/config alternative exists. MCP stdout JSON-RPC only; portable skill advertises both publish and sharing.

Root builds web/dist then copies it to internal/webassets/dist and Go embeds `dist/*`. Package `internal/webassets` exposes `FS() fs.FS` (sub FS). Build placeholder via .gitkeep until frontend exists. cmd/folio owns serve listeners and graceful shutdown, does not launch any external cloud services by default.

Version-isolated previews: `POST /api/artifacts/:id/launch` accepts optional `isolatedVersion:true` together with an authorized `versionId`; omit `hostname` in this mode. The result includes `url`, `hostname`, `isolatedVersion`. In path mode the version site is `<app-url>/c/v--{digest}` and needs no routing. In subdomain mode the hostname must be routed to the content listener, using a content wildcard or exact DNS/tunnel registration. `GET /api/artifacts/:id/diff` also returns `previewHosts`: the two version hostnames in subdomain mode, an empty array in path mode. Host/version binding applies to every file, including root-absolute paths. See [version domains](COLLABORATION.md#version-preview-domains).


## Profile photo persistence and recovery

Profile and organization photos are stored inside the SQLite metadata snapshot, so existing mutation rollback and encrypted portable backup cover them atomically. Restore validates and re-encodes imported photos and regenerates local URLs rather than trusting backup-supplied URLs/MIME values. Existing destination user profiles and photos win during identity merge, preserving destination credentials and disabled state. Imported-only historical users retain their sanitized photos; the organization photo restores with imported organization metadata. A merged restore exceeding the 8 MiB photo budget is rejected. [Implementation and source references](references/profile-photos.md).
