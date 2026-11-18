# Install and operate Folio

## Release binary

Download the archive for Linux, macOS or Windows (amd64/arm64) and `checksums.txt` from the same [GitHub release](https://github.com/YangTaeyoung/folio/releases). Verify SHA-256 with `sha256sum` (Linux), `shasum -a 256` (macOS), or `Get-FileHash -Algorithm SHA256` (PowerShell). Compare the archive's full digest with its exact filename in the manifest before extracting it. Windows archives contain `folio.exe`; Unix archives contain `folio`.

Unix installer:

```sh
curl -fsSL https://raw.githubusercontent.com/YangTaeyoung/folio/main/scripts/install.sh -o /tmp/folio-install.sh
# Read the script, then:
FOLIO_VERSION=v0.8.0 FOLIO_INSTALL_DIR="$HOME/.local/bin" sh /tmp/folio-install.sh
folio version
folio serve --data-dir ./folio-data
```

Omit FOLIO_VERSION to select the latest release. Existing identical binaries are accepted; a different existing binary is preserved unless `FOLIO_INSTALL_FORCE=1` is set. Stop the service and back up its data before an upgrade.

The installer downloads only the executable and verifies the release manifest; it does not start services, configure MCP clients, register DNS or open a tunnel. macOS may ask you to approve running a downloaded executable through its normal security UI.

## Offline and air-gap installation

On an Internet-connected machine, download the release archive and checksum manifest, verify them, then transfer both through your normal approved process. Verify the digest again on the target, extract, and run `folio serve`. No dependency download, license server, telemetry, external fonts, CDN, model API or email service is needed by the core runtime. Optional cloud storage, public DNS and tunnel connections require access to their providers.

For offline Docker, build on a connected machine for the target CPU, then transfer the image and a checksum:

```sh
# On a reviewed source checkout; replace the example tag with your reviewed ref.
docker build --build-arg VERSION=v0.8.0 -t folio:v0.8.0 .
docker save folio:v0.8.0 -o folio-image.tar
sha256sum folio-image.tar > folio-image.tar.sha256
# On the disconnected Docker host, after transferring both files:
sha256sum -c folio-image.tar.sha256
docker load -i folio-image.tar
FOLIO_IMAGE=folio:v0.8.0 docker compose up -d --no-build --pull never
```

Use `shasum -a 256` on macOS instead of `sha256sum`. Transfer the Compose file too. Do not label a working-tree build as a released version: check out the exact reviewed tag in a separate source directory first. `docker build --platform linux/amd64 ...` selects an amd64 target; build for `linux/arm64` for ARM nodes. Cross-platform builds require a builder able to execute the target build stages. A Docker archive imported on a workstation does not load Kubernetes nodes: push it to an internal registry reachable by every node, or use your cluster distribution's node-image import procedure. The target needs no npm/Go downloads; source-image builds do.

## First run and users

Folio binds management to `127.0.0.1:4310` and advertises `http://folio.localhost:4310`. Artifact content is served on the same origin under `http://folio.localhost:4310/c/{artifact}/`. Use a browser/resolver that resolves `.localhost` names to loopback, or configure your local resolver.

### Content modes

`--content-url` (`FOLIO_CONTENT_URL`) contains one `{artifact}` placeholder, and its position selects the mode:

- **Path mode (default):** `https://folio.example.com/c/{artifact}`. The placeholder is the last path segment on the management origin. One hostname, one certificate and one ingress route; `--content-listen` is not used. Artifact JavaScript runs on the management origin with the viewer's session, so an artifact can call management APIs as the person viewing it, and artifacts share one origin with each other. Use it when publishers are trusted.
- **Subdomain mode:** `https://{artifact}.artifacts.example.com`. Every artifact gets its own execution hostname on the content listener (`--content-listen`, default `127.0.0.1:4311`). Needs wildcard DNS and TLS for the content hosts, and keeps artifacts isolated from the management origin and from each other.

### Upgrading to path mode

Installations that never set `--content-url` switch to path mode on upgrade, and old `*.content.folio.localhost:4311` links stop working. To stay on subdomain mode, set `--content-url 'http://{artifact}.content.folio.localhost:4311'` (or your existing template) explicitly and keep `--content-listen`. Installations that already set a host template are unaffected.

The terminal displays a private setup URL containing a one-time token. The same token is stored in `bootstrap.token` in the private data directory until setup completes. An operator may supply `FOLIO_BOOTSTRAP_TOKEN` through a protected environment instead. Never expose the setup token in a public log. Open the setup URL, create the organization and admin, then create members in the backoffice. Temporary passwords require a first-login change. Administrators can manage every artifact; ordinary recipients can only read artifacts shared with them.

Create scoped API tokens in **Agents & API keys** and save them using `folio login --token-stdin`. The default local config is `folio/config.json` under the OS user config directory and is written mode 0600. `FOLIO_CONFIG` selects another file. `FOLIO_URL` and `FOLIO_TOKEN` override saved values. Keep token files in your user account; do not mount an admin config into a shared agent container.

## LAN deployment and HTTPS

In path mode, configure only the management hostname: `--app-url https://folio.example.internal` serves artifacts at `https://folio.example.internal/c/{artifact}/`. For subdomain mode, choose a management hostname and a different per-artifact content template, then configure DNS to resolve them to the server/reverse proxy. For example:

```sh
folio serve --data-dir /srv/folio/data \
  --listen 0.0.0.0:4310 --content-listen 0.0.0.0:4311 \
  --app-url https://folio.example.internal \
  --content-url 'https://{artifact}.artifacts.example.internal'
```

Route management to port4310 and, in subdomain mode, the content hosts to port4311. Preserve each incoming Host header and exact scheme/origin configuration. Terminate TLS at your trusted reverse proxy, use appropriate certificates and firewall rules, and expose only the routes you intend. In subdomain mode, do not map management and uploaded artifacts to the same hostname or serve multiple artifacts on one execution origin. Authentication relies on the configured origins and host-only cookies.

The same options accept `FOLIO_DATA_DIR`, `FOLIO_LISTEN`, `FOLIO_CONTENT_LISTEN`, `FOLIO_APP_URL` and `FOLIO_CONTENT_URL`. Client FOLIO_URL is separate from server FOLIO_APP_URL. Bind addresses control reachability; advertised URLs control links and security checks.

Optional provider settings: `--cloudflare-cert /private/path/cert.pem` (`FOLIO_CLOUDFLARE_CERT`) explicitly imports an existing official cloudflared certificate into the installation without overwriting its original. `--cloudflared /path/to/cloudflared` (`FOLIO_CLOUDFLARED`) chooses the official executable. Tunnel upstreams derive from the actual listener addresses/ports, with unspecified binds translated to loopback.

For a real provider-approved Domain Connect template, supply all three: `--domainconnect-provider PROVIDER_ID`, `--domainconnect-service SERVICE_ID`, and `--domainconnect-allowed-hosts EXACT_AUTHORITY,EXACT_AUTHORITY`. Environment equivalents are `FOLIO_DOMAINCONNECT_PROVIDER`, `FOLIO_DOMAINCONNECT_SERVICE` and `FOLIO_DOMAINCONNECT_ALLOWED_HOSTS`. Only explicitly trusted HTTPS authorities are contacted. These values require a template registered with the DNS provider; they do not create universal login support. Without this configuration unsupported providers receive manual DNS instructions.

Public sharing does not open a firewall. Internet ingress is optional. [DNS and Cloudflare guide](docs/dns.md) covers manual records, token provisioning, official browser login and explicit tunnel start/stop. Install the official `cloudflared` executable in PATH for browser login and owned tunnel control. The default container includes the offline Folio core only; a derived image can add a trusted cloudflared binary and persist its normal login certificate location as well as Folio's `/data`. Existing login certificates are never overwritten by Folio.

<a id="docker"></a>

## Docker Compose

```sh
cp deploy/compose.env.example .env
# Review .env, then build from this checkout and start:
docker compose up --build -d
docker compose ps
# View only in a private terminal: initial logs contain the setup URL.
docker compose logs --tail 20 folio
```

Requires Docker Engine/Desktop with the Compose v2 plugin. The supplied [Compose file](compose.yaml) builds a local `folio:local` image, publishes loopback-only ports and persists `/data` in the `folio-data` named volume. It runs as UID/GID 10001, drops capabilities and uses a read-only image filesystem with writable `/data` and bounded `/tmp`. The healthcheck requests `/api/bootstrap` using the configured management Host, before and after initial setup. It is an HTTP process check, not an external storage connectivity or disaster-recovery check. `/tmp` is limited to 256 MiB; increase its allowance for larger temporary exports/uploads or install on a filesystem with sufficient temporary capacity. Never scale this service above one instance.

Set `FOLIO_APP_URL` to your reverse proxy's exact HTTPS origin. In path mode that one proxy route goes to `127.0.0.1:4310`; artifacts share the management origin and require trusted publishers. For isolated subdomains, also set `FOLIO_CONTENT_URL='https://{artifact}.artifacts.example.internal'`, configure wildcard DNS/TLS, and route artifact hosts to `127.0.0.1:4311`. Preserve the incoming Host header. Port 4311 is published but has no listener in path mode. A proxy in another container cannot reach the host's loopback mapping; attach it to the Compose network and use `folio:4310`/`folio:4311` as upstreams while preserving the public Host. For LAN direct binding, deliberately edit the port bindings and advertised URLs together.

The [release workflow](.github/workflows/release.yml) publishes verified binary archives and has no container publication step; these examples do not assume a GHCR/Docker Hub image. Use a reviewed source ref to build your own immutable tag, push it to a registry you operate and record its digest. Set `FOLIO_IMAGE=registry.example.internal/team/folio@sha256:YOUR_VERIFIED_DIGEST` and start with `docker compose up -d --no-build --pull never` after pulling that exact digest. `--no-build` prevents an accidental rebuild and `--pull never` supports a preloaded air-gap image. No provider credentials or bootstrap token are included in the example environment.

The named volume's actual name includes the Compose project name. Find it with `docker inspect "$(docker compose ps -aq folio)" --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Name}}{{end}}{{end}}'` and record the project name before an upgrade. Keep using the same project name and directory; changing them can create a new empty installation. Do not use `docker compose down -v` or delete this volume unless you intend to delete the installation.

For a host bind mount, precreate its directory with owner `10001:10001` and mode `0700` on Linux, then replace the named mount. Folio sets its data directory mode to `0700`, so group access alone does not suffice for an existing directory owned by another UID. On platforms with Docker Desktop, use a named volume unless you have verified bind-mount ownership behavior.

### Compose upgrade and volume backup

Stop the service and ensure no other writer exists before copying any data. The following creates a private, complete archive using the same locally available image, including hidden files and SQLite sidecars:

```sh
umask 077
mkdir -p backups
docker compose stop folio
docker compose run --rm --no-deps --entrypoint tar folio -C /data -czf - . > backups/folio-data.tar.gz
# Check the archive locally; treat its contents as credentials.
tar -tzf backups/folio-data.tar.gz > /dev/null
# Build a reviewed new source ref, or select your newly verified preloaded image:
docker compose build folio
docker compose up -d --no-build --pull never
docker compose ps
```

Keep the previous image and matching archive. To restore, stop Folio, attach an **empty replacement volume** at `/data`, and restore using the same image as the backup. With that replacement selected in the Compose file:

```sh
docker compose run --rm --no-deps -T --entrypoint tar folio -C /data -xzf - < backups/folio-data.tar.gz
docker compose up -d --no-build --pull never
```

The replacement volume must be owned by UID/GID 10001. Do not extract over a populated installation or roll back only the executable after a data format change. Restore external object stores from the matching snapshot and follow [restore verification](#backup-and-restore).

## Kubernetes

[Kustomize base and optional Ingress overlay](deploy/kubernetes/README.md) include a persistent volume claim, a ClusterIP Service, Secret wiring, UID/GID 10001, startup/readiness/liveness probes, and a single-replica `Recreate` Deployment. Start with port-forwarding on a private workstation; enable DNS/TLS ingress explicitly. Supply a locally loaded image or your own immutable registry digest. The manifests do not install a cluster, storage provisioner, ingress controller, certificate issuer or public DNS.

SQLite is the metadata store in every deployment mode. Keep exactly one writer and use storage with reliable SQLite file locks. A `ReadWriteOnce` claim alone does not enforce one Pod; do not add an HPA, multiple replicas or a second Deployment using the claim. This package provides persistent single-instance operation with downtime during updates, not active-active/high-availability SQLite.

## Linux systemd service

Install a verified native release at `/usr/local/bin/folio`. On a systemd Linux host:

```sh
sudo useradd --system --user-group --home-dir /var/lib/folio --shell /usr/sbin/nologin folio
sudo install -d -m 0750 /etc/folio
sudo install -m 0600 deploy/systemd/folio.env.example /etc/folio/folio.env
sudo install -m 0644 deploy/systemd/folio.service /etc/systemd/system/folio.service
# Review /etc/folio/folio.env and the unit before starting.
sudo systemctl daemon-reload
sudo systemctl enable --now folio
sudo systemctl status folio
```

Skip user creation if that dedicated account already exists; adapt the `nologin` path to your distribution. The unit creates `/var/lib/folio` owned by `folio`, keeps management on loopback and handles SIGTERM with a 30-second stop allowance. First-run setup appears in `sudo journalctl -u folio`; restrict journal access and do not forward the setup URL to a public log sink. `sudo systemctl stop folio` stops it for a full data backup; restore ownership with `sudo chown -R folio:folio /var/lib/folio` before restarting. Keep optional provider credentials outside the repository and grant only this service account access. For subdomain mode, set both content origin and listener in the private environment file. Native Windows releases can run interactively; this repository does not include a Windows Service wrapper.

## macOS background service

Install a verified native binary under `~/.local/bin/folio`. Copy [the launchd template](deploy/macos/cloud.melona.folio.plist) to `~/Library/LaunchAgents/cloud.melona.folio.plist`, replacing every `/Users/YOUR_USER` with your absolute home path. Plists do not expand `~` or `$HOME` inside these strings. Create private data and log directories first:

```sh
umask 077
mkdir -p "$HOME/Library/LaunchAgents" "$HOME/Library/Application Support/Folio" "$HOME/Library/Logs/Folio"
chmod 700 "$HOME/Library/Application Support/Folio" "$HOME/Library/Logs/Folio"
# After copying and editing the plist:
plutil -lint "$HOME/Library/LaunchAgents/cloud.melona.folio.plist"
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/cloud.melona.folio.plist"
launchctl print "gui/$(id -u)/cloud.melona.folio"
```

This user agent runs while you are logged in. Read the setup URL privately from `~/Library/Logs/Folio/stderr.log`. Stop it with `launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/cloud.melona.folio.plist"` before an upgrade or complete data backup; start again with `bootstrap`. Never run a manual `folio serve` concurrently against this agent's data directory. macOS may require normal downloaded-executable approval on first use.

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

## Optional Google sign-in

An installation administrator can configure a Google web OAuth client in **조직 설정 → Google 로그인**. It is off by default. Use the exact callback URI shown in settings and precreate active accounts; there is no Google-based open signup or automatic administrator promotion. [Google sign-in setup](docs/GOOGLE_LOGIN.md) explains client creation, Gmail/Workspace eligibility, domain restrictions and recovery administration. Keep a working password administrator while configuring and testing Google.
