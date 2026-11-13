# Domains, DNS, and optional public ingress

Folio's core works without cloud services. A custom domain binds one artifact;
DNS configuration does not change artifact access rules. Administrators configure
DNS, and the server activates a domain only after ownership and routing checks.
The provider package never activates domains or bypasses the content grant.

## Manual DNS and API tokens

`dns.DesiredRecords(hostname, challenge, target)` returns an ownership TXT at
`_folio.<hostname>` plus an exact CNAME (or A/AAAA for an IP target). Publish both
records at your DNS provider. `Verify` / `VerifyWithResolver` checks the exact TXT
and CNAME/address target. There is no registrar login assumption.

`dns.Cloudflare` accepts an API token through trusted server configuration. Grant
Zone Read and DNS Edit for only the intended zone. `Zones` paginates the API;
`Provision` and `ProvisionOwnership` inspect the exact name and create a new record
only when absent. Existing records, including identical preexisting records, are
refused. There is no implicit update, overwrite, or adoption. API-created records
are DNS-only and carry a `Managed by Folio` comment. Provider failures report HTTP
status and numeric error code, never token or provider response text.

`DeleteOwned` first reads the record ID and requires the exact expected name,
content, CNAME/TXT type, and management comment. The caller must store IDs returned
by successful creation. A mismatch prevents deletion. Detaching a domain does
not imply DNS deletion. These checks do not offer transactional protection against
an independent administrator changing a record between the provider read and
provider delete; coordinate changes to Folio-owned records.

Cloudflare proxied records can hide their CNAME in public DNS answers. Do not
accept shared proxy IP addresses as routing proof. `VerifyOwnershipWithResolver`
checks the public TXT independently, and `Cloudflare.VerifyRoute` checks the exact
CNAME through the authenticated provider API. The server may instead verify its
own exact-host HTTPS challenge before activating a tunnel-backed custom domain.
The challenge endpoint must not expose uploaded artifact files.

## Official browser login

`dns.Manager.StartLogin` executes the installed official `cloudflared tunnel
login`, captures only an official `https://dash.cloudflare.com/argotunnel?...`
authorization URL, and tracks `pending`, `complete`, `error`, or `expired` status.
`Login(id)` polls the attempt. The default timeout is five minutes. Completion
requires a valid newly-written account certificate, not a successful exit alone.
The login URL is authorization material; show it to the initiating administrator,
and do not put it in analytics or access logs.

The installed cloudflared 2026.10.3 login implementation chooses its own default
`~/.cloudflared/cert.pem` write location. `--origincert` does **not** change that
login output path. Folio preserves HOME and refuses login if that default file
already exists. After official login completes, Folio copies the certificate to
`<DataDir>/dns/cert.pem` and preserves the official default file. An operator can
explicitly import an existing certificate with `ImportCertificate(path)`. Folio
never moves, deletes, or silently replaces an existing default certificate.
All subsequent cloudflared operations use the dedicated `--origincert` path.

The dedicated directory is mode 0700; account certificate, tunnel credentials,
ingress configuration, and metadata are 0600. Raw command diagnostics are discarded
because they may contain secrets. Ambient `TUNNEL_*` / `CLOUDFLARED_*` options are
stripped from child environments; system HOME is unchanged. A private empty
command configuration prevents adopting unrelated cloudflared configuration.

Browser login does not create a tunnel, start a connector, or make a local-only
installation publicly reachable. Explicit `Manager.ProvisionOwnership` can try
collision-safe ownership TXT creation using the certificate's contained API token,
without asking the administrator to copy that token. `SelectedZone` first reads
the certificate-selected zone through the generic API; every hostname must equal
that zone or lie below its exact DNS label boundary. The same zone check precedes
official tunnel DNS routing, preventing a foreign name from being treated as a
relative name inside the selected zone.

Cloudflared's source confirms the certificate contains ZoneID/AccountID/APIToken
and sends the token as Bearer authorization, but the official documented tunnel
route endpoint is distinct from generic DNS record CRUD. Its published contract
does not guarantee the generated token's generic Zone Read / DNS Edit policy.
Folio therefore reports successful automatic ownership only after the API creates
the TXT record and returns its ID. A 401/403 returns `ErrBrowserDNSUnavailable` with
manual/API-token fallback; it never treats CNAME-only provisioning as complete.
`Manager.VerifyRoute` can read the exact CNAME through the same private certificate
client when authorized. No exported Manager operation returns the token.

## Dedicated tunnel lifecycle

`ConfigureTunnel(ctx, name, hostnames)` is an explicit external mutation. It
creates one random `folio-<installation>-<name>` tunnel and retains its ID. Retries
select only the recorded Folio tunnel; arbitrary existing tunnel names are never
adopted. Creation requests a dedicated credential file and checks that its tunnel
ID matches the returned ID.

Each explicitly listed hostname is routed with official `cloudflared tunnel
route dns --overwrite-dns=false`. Wildcards and invalid names are rejected. DNS
collisions stop configuration without overwriting the existing record. A failed
multi-host configuration may leave the newly-created stopped tunnel and the
successfully-created subset of new routes; private metadata records the owned ID
and successful names so the administrator can inspect and retry. Folio does not
delete an unrelated tunnel or roll back another administrator's DNS changes.
Removing a hostname from ingress leaves its DNS record intact and serves the
catch-all 404. DNS cleanup is a separate explicit operation.

The exact AppURL hostname routes to `AppOrigin`, default
`http://127.0.0.1:4310`; other approved content/custom hostnames route to
`ContentOrigin`, default `http://127.0.0.1:4311`. Configure explicit upstreams when
listeners use different ports or addresses. The final ingress rule is always
`http_status:404`. ContentURL still governs artifact isolation in the server;
adding tunnel ingress does not authorize an arbitrary artifact hostname.

`StartTunnel` starts only this instance's connector, with a loopback-only metrics
address. `Tunnel` reports process liveness separately from `/ready` health; neither
login nor file creation implies health. `StopTunnel` signals and, if necessary,
kills only the process held by this Manager. It never uses process-name matching,
service shutdown, or a PID loaded from disk. Construct one Manager per installation
and call `Close` at shutdown. Startup does not automatically start a connector.

## Domain Connect

Domain Connect is a DNS provisioning protocol. Folio implements its synchronous
browser apply flow only for an operator-configured registered `ProviderID` and
`ServiceID` template. It does not present every registrar as an OAuth provider.

`Discover` walks the requested hostname's DNS suffixes for `_domainconnect` TXT,
fetches real provider settings at `https://<TXT>/v2/<zone>/settings`, and checks the
configured template through the provider's `urlAPI`. The provider must advertise
a synchronous `urlSyncUX`, and its template-support endpoint must return 2xx.
Missing discovery, unconfigured template IDs, unsupported templates, or invalid
settings result in manual fallback instructions.

The operator must configure `AllowedProviderHosts`: exact HTTPS authorities for
settings, API, and browser destinations. Discovery never fetches an arbitrary
address from DNS. URLs with user information, a query, a fragment, or non-HTTPS
scheme are rejected; provider HTTP redirects are not followed. CallbackURL is a
fixed trusted HTTPS endpoint configured by the application, not user input. The
registered template must permit that callback host through `syncRedirectDomain`
unless the provider's template contract requires a separately signed apply URL.
Folio currently does not generate provider-specific signed-template requests;
providers requiring them need a suitable registered template or manual fallback.

`Authorize(ctx, hostname, adminID, domainID, variables)` rediscovers support and
builds the actual `/v2/domainTemplates/providers/.../services/.../apply` URL. The
server supplies the intended target and ownership challenge as template variables;
reserved parameters cannot be overwritten. State is HMAC signed with a private
key of at least 32 bytes, random, bound to admin/domain/hostname, expires in ten
minutes by default (maximum fifteen), and is accepted once by `ConsumeState`.
Pending states exist only in memory, so restart invalidates outstanding flows.
The callback must check the initiating administrator, consume state, handle the
provider's denial, and then independently verify TXT and routing before activation.
A successful browser redirect alone never activates a domain.

## Verified references and test boundary

Official references inspected on 2026-11-14:

- [Cloudflare DNS create API](https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/create/).
- [Locally managed tunnel creation](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/create-local-tunnel/).
- [Tunnel certificate permissions](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/tunnel-permissions/).
- [Proxied DNS CNAME visibility](https://developers.cloudflare.com/tunnel/troubleshooting/https-origins/).
- [cloudflared 2026.10.3 login source](https://github.com/cloudflare/cloudflared/blob/2026.10.3/cmd/cloudflared/tunnel/login.go#L65-L157): fixed default write path, overwrite refusal, private certificate file.
- [cloudflared tunnel creation source](https://github.com/cloudflare/cloudflared/blob/2026.10.3/cmd/cloudflared/tunnel/subcommand_context.go#L121-L190): explicit credential file and JSON create output.
- [cloudflared certificate schema](https://github.com/cloudflare/cloudflared/blob/2026.10.3/credentials/origin_cert.go#L20-L25) and [private API client construction](https://github.com/cloudflare/cloudflared/blob/2026.10.3/credentials/credentials.go#L44-L58).
- [cloudflared Bearer authorization and selected-zone tunnel endpoint](https://github.com/cloudflare/cloudflared/blob/2026.10.3/cfapi/base_client.go#L44-L107).
- [Selected zone details API](https://developers.cloudflare.com/api/resources/zones/methods/get/).
- [Cloudflare permission definitions](https://developers.cloudflare.com/fundamentals/api/reference/permissions/).
- [cloudflared DNS route implementation](https://github.com/cloudflare/cloudflared/blob/2026.10.3/cfapi/hostname.go#L35-L100): overwrite flag and unchanged-route result.
- [cloudflared readiness command](https://github.com/cloudflare/cloudflared/blob/2026.10.3/cmd/cloudflared/tunnel/subcommands.go#L479-L503).
- [Domain Connect specification](https://github.com/Domain-Connect/spec/blob/master/Domain%20Connect%20Spec%20Draft.adoc): discovery, provider settings, template support, synchronous apply, callback and state.
- [Domain Connect IETF draft](https://www.ietf.org/archive/id/draft-ietf-dconn-domainconnect-04.html).

`go test -race ./internal/dns` uses httptest provider servers, a fake resolver, and
a real child executable standing in for cloudflared. It exercises collision
refusal, exact record deletion, pagination, TXT binding, destination refusal,
unsupported templates, signed state tampering/replay/expiry, browser-login
completion/timeout, private file modes, dedicated connector start/health/stop,
ambient overwrite-option isolation, certificate-authenticated TXT/route API calls,
selected-zone boundary checks, permission-denied fallback, and diagnostic redaction. `go vet` also passes.
The installed official cloudflared 2026.10.3 accepted a generated JSON ingress
configuration with `tunnel --config <file> ingress validate` (offline, result OK).
These package tests do not prove live account authorization, a registered Domain
Connect template, public DNS propagation, or live tunnel HTTPS reachability.
