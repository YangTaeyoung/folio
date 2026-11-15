# Folio v1 integration contract

JSON camelCase. Error responses `{error:{code,message}}`, appropriate 400/401/403/404/409/413/429/5xx. No secrets in returned config/list/detail objects. Unauthorized artifact identifiers return 404. IDs opaque strings. Dates RFC3339 UTC. Metadata operations are JSON; uploads multipart.

## JSON models
User: `{id,email,name,role:"admin"|"member",disabled,mustChangePassword,createdAt,loginMethod?:"google",googleLinked?:boolean}`. Omitted `loginMethod` means password account (including password accounts linked to Google); `google` means Google-only, with no local password.
Organization: `{id,name}` (one per installation).
Artifact: `{id,slug,title,description,ownerId,ownerName,visibility:"private"|"organization"|"selected"|"public",memberIds:[],archived,expiresAt:null|string,createdAt,updatedAt,sizeBytes,currentVersionId,shareUrl,kind}`.
Version: `{id,artifactId,number,createdAt,sizeBytes,backendId,files:[{path,mimeType,size}],message}`.
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
- POST /api/auth/logout -> 204
- POST /api/auth/password `{currentPassword,newPassword}` -> `{user,csrfToken}` (session rotated)
- GET /api/artifacts?view=mine|shared|archive|all&q=&sort=updated|title -> `{artifacts,usage:{usedBytes,quotaBytes},policy}`. all admin only; archive contains only readable archived items.
- POST /api/artifacts: multipart field `metadata` JSON `{title,description,slug?,visibility?,memberIds?,expiresAt?,message?,files:[{path,mimeType?}]}`; numbered parts `file0`,`file1`,... -> `{artifact,version}`.
- GET /api/artifacts/{id} -> `{artifact,versions}`
- POST /api/artifacts/{id}/versions: multipart same metadata plus `baseVersionId` -> `{artifact,version}`; stale base 409.
- PATCH /api/artifacts/{id} `{title?,description?}` -> `{artifact}`
- PATCH /api/artifacts/{id}/sharing `{visibility,memberIds,expiresAt}` -> `{artifact}`
- POST /api/artifacts/{id}/archive -> `{artifact}`
- POST /api/artifacts/{id}/restore -> `{artifact}`
- DELETE /api/artifacts/{id} -> 204
- POST /api/artifacts/{id}/versions/{versionId}/restore -> `{artifact}`
- GET /api/artifacts/{id}/download -> zip attachment (authorized, never inline HTML)
- POST /api/artifacts/{id}/launch `{versionId?,hostname?}` -> `{url}` short-lived single-use content grant, bound to artifact/version/principal/originating-credential/exact destination host. hostname must be the canonical artifact-isolated host or a verified domain for that same artifact. Read-authorized, cookie CSRF protected when authenticated. Anonymous only for public artifacts.
- GET /api/share/{slug} -> `{artifact}` for permitted user or anonymous public. No content leaks on denial.
- GET /s/{slug} -> trusted frontend viewer route (normal app shell assets); fetched metadata checks access.
- Content listener /grant/{nonce} -> exchange once on nonce's exact hostname, artifact-scoped host-only viewer cookie, redirect /render/{artifactId}/{versionId}/index.html. EVERY file checks original session/token remains active, scope and incoming host matches this artifact. Each canonical hostname resolves exactly one artifact from ContentURL template `{artifact}`. Host-mapped verified custom domain `/...` resolves that artifact root under same checks; private unauthenticated root redirects to AppURL/s/{slug}?domain=hostname for trusted login/grant exchange.
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
- POST /api/tokens `{name,scopes:["read","publish","share"],expiresAt?}` -> `{token,record}` one-time plaintext
- DELETE /api/tokens/{id} -> 204 self only
- GET /api/capabilities -> version, offline mode, upload limits, share modes and CLI/MCP setup metadata
- Domain/DNS endpoints below use admin authentication and CSRF for browser mutations. Implemented by internal/dns service mounted via server Config.DNSHandler? See interface below.
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
