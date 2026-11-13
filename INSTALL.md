# Install and operate Folio

## Release binary

Download the archive for Linux, macOS or Windows (amd64/arm64) and `checksums.txt` from the same [GitHub release](https://github.com/YangTaeyoung/folio/releases). Verify SHA-256 with `sha256sum` (Linux), `shasum -a 256` (macOS), or `Get-FileHash -Algorithm SHA256` (PowerShell). Compare the archive's full digest with its exact filename in the manifest before extracting it. Windows archives contain `folio.exe`; Unix archives contain `folio`.

Unix installer:

```sh
curl -fsSL https://raw.githubusercontent.com/YangTaeyoung/folio/main/scripts/install.sh -o /tmp/folio-install.sh
# Read the script, then:
FOLIO_VERSION=v0.1.0 FOLIO_INSTALL_DIR="$HOME/.local/bin" sh /tmp/folio-install.sh
folio version
folio serve --data-dir ./folio-data
```

Omit FOLIO_VERSION to select the latest release. Existing identical binaries are accepted; a different existing binary is preserved unless `FOLIO_INSTALL_FORCE=1` is set. Stop the service and back up its data before an upgrade.

The installer downloads only the executable and verifies the release manifest; it does not start services, configure MCP clients, register DNS or open a tunnel. macOS may ask you to approve running a downloaded executable through its normal security UI.

## Offline and air-gap installation

On an Internet-connected machine, download the release archive and checksum manifest, verify them, then transfer both through your normal approved process. Verify the digest again on the target, extract, and run `folio serve`. No dependency download, license server, telemetry, external fonts, CDN, model API or email service is needed by the core runtime. Optional cloud storage, public DNS and tunnel connections require access to their providers.

For offline Docker, build/pull the image on a connected machine, `docker save folio:TAG -o folio-image.tar`, transfer it and `docker load -i folio-image.tar`. Use the same compose volumes and explicitly select that loaded image instead of rebuilding.

## First run and users

Folio binds management to `127.0.0.1:4310` and artifact content to `127.0.0.1:4311`. The advertised defaults are `http://folio.localhost:4310` and `http://{artifact}.content.folio.localhost:4311`. Use a browser/resolver that resolves `.localhost` names to loopback, or configure your local resolver.

The terminal displays a private setup URL containing a one-time token. The same token is stored in `bootstrap.token` in the private data directory until setup completes. An operator may supply `FOLIO_BOOTSTRAP_TOKEN` through a protected environment instead. Never expose the setup token in a public log. Open the setup URL, create the organization and admin, then create members in the backoffice. Temporary passwords require a first-login change. Administrators can manage every artifact; ordinary recipients can only read artifacts shared with them.

Create scoped API tokens in **Agents & API keys** and save them using `folio login --token-stdin`. The default local config is `folio/config.json` under the OS user config directory and is written mode 0600. `FOLIO_CONFIG` selects another file. `FOLIO_URL` and `FOLIO_TOKEN` override saved values. Keep token files in your user account; do not mount an admin config into a shared agent container.

## LAN deployment and HTTPS

Choose a management hostname and a different per-artifact content template, then configure DNS to resolve them to the server/reverse proxy. For example:

```sh
folio serve --data-dir /srv/folio/data \
  --listen 0.0.0.0:4310 --content-listen 0.0.0.0:4311 \
  --app-url https://folio.example.internal \
  --content-url 'https://{artifact}.artifacts.example.internal'
```

Route management to port4310 and the content hosts to port4311. Preserve each incoming Host header and exact scheme/origin configuration. Terminate TLS at your trusted reverse proxy, use appropriate certificates and firewall rules, and expose only the routes you intend. Do not map management and uploaded artifacts to the same hostname or serve multiple artifacts on one execution origin. Authentication relies on the configured origins and host-only cookies.

The same options accept `FOLIO_DATA_DIR`, `FOLIO_LISTEN`, `FOLIO_CONTENT_LISTEN`, `FOLIO_APP_URL` and `FOLIO_CONTENT_URL`. Client FOLIO_URL is separate from server FOLIO_APP_URL. Bind addresses control reachability; advertised URLs control links and security checks.

Optional provider settings: `--cloudflare-cert /private/path/cert.pem` (`FOLIO_CLOUDFLARE_CERT`) explicitly imports an existing official cloudflared certificate into the installation without overwriting its original. `--cloudflared /path/to/cloudflared` (`FOLIO_CLOUDFLARED`) chooses the official executable. Tunnel upstreams derive from the actual listener addresses/ports, with unspecified binds translated to loopback.

For a real provider-approved Domain Connect template, supply all three: `--domainconnect-provider PROVIDER_ID`, `--domainconnect-service SERVICE_ID`, and `--domainconnect-allowed-hosts EXACT_AUTHORITY,EXACT_AUTHORITY`. Environment equivalents are `FOLIO_DOMAINCONNECT_PROVIDER`, `FOLIO_DOMAINCONNECT_SERVICE` and `FOLIO_DOMAINCONNECT_ALLOWED_HOSTS`. Only explicitly trusted HTTPS authorities are contacted. These values require a template registered with the DNS provider; they do not create universal login support. Without this configuration unsupported providers receive manual DNS instructions.

Public sharing does not open a firewall. Internet ingress is optional. [DNS and Cloudflare guide](docs/dns.md) covers manual records, token provisioning, official browser login and explicit tunnel start/stop. Install the official `cloudflared` executable in PATH for browser login and owned tunnel control. The default container includes the offline Folio core only; a derived image can add a trusted cloudflared binary and persist its normal login certificate location as well as Folio's `/data`. Existing login certificates are never overwritten by Folio.

## Docker

```sh
docker compose up --build -d
docker compose logs folio
```

The supplied compose file publishes loopback-only ports and persists `/data` in a named volume. It runs the image as a non-root user. To support a LAN or reverse proxy, deliberately change port bindings and advertised URLs together. Save the actual data volume before upgrades; do not use `docker compose down -v` unless you intend to delete the installation.

## Storage, retention and archives

Filesystem is the default. Configure a private bucket/container in **Storage & retention** for AWS S3, S3-compatible services (R2/MinIO), Google Cloud Storage or Azure Blob. Supply explicit credentials or a supported provider default credential chain as described in [storage adapter documentation](internal/blob/README.md), test the connection, then select it as default. Folio performs a bounded temporary write/read/delete; the account must permit those operations. Historical revisions keep their original backend ID; do not remove external buckets containing retained objects.

Quota measures retained artifact bytes. All retained revisions and archived artifacts count. Archive preserves content and sharing and excludes the artifact from automatic cleanup. Retention is disabled by default; quota overflow rejects by default. Delete-oldest overflow is an explicit policy choice. Preview cleanup candidates before running manual cleanup. Scheduled maintenance runs hourly while the server is running. Failed physical deletion remains pending and is retried; a metadata deletion is not proof that provider bytes have been reclaimed.

## Backup and restore

1. Stop Folio gracefully (SIGINT/SIGTERM, or `docker compose stop`). Ensure no other Folio process uses the same data directory.
2. Copy the **complete data directory** with its restrictive permissions, including `folio.db` and any SQLite sidecars, `data.key`, `objects/`, `dns/` and all other installation files. Keep the installation key and encrypted database together in a private encrypted backup. Losing `data.key` makes stored provider secrets unreadable.
3. Snapshot/copy external buckets and containers too. A local backup does not contain cloud objects. Preserve matching versions and backend configuration.
4. Restore the directory and cloud objects, ensure the service user owns the directory, then start the same Folio release with the same origin configuration. Verify login, a private artifact, a historical revision, cloud access and `folio doctor` before reopening ingress.

Do not copy a running SQLite file alone or run two servers against one data directory. There is no automatic external bucket migration or automatic cross-install key recovery.

## Build and releases

Go1.26+ and Node24+:

```sh
make build
make test
make release VERSION=v0.1.0
```

`make build` builds the UI, copies it into the embed directory, and builds the combined executable. A source-only `go build ./cmd/folio` compiles, but serve refuses an absent embedded UI unless `--dev` is explicitly supplied. Development can run Vite separately. Release scripts build CGO-disabled Linux/macOS/Windows amd64/arm64 binaries with trimmed paths and a fixed version, then create archives and SHA-256 manifests. Build with the same toolchain/dependency lock files for reproducible binaries. Archive metadata is not claimed to be byte-for-byte reproducible.

The tag release workflow tests Go, builds frontend and release artifacts, then publishes archives/checksums. A release job succeeding is separate from deployment validation.
