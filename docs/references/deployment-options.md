# Deployment packaging references

Checked 2026-11-19 against the implementation and official references. These are configuration contracts, not proof of a rollout on an operator's cluster.

## Folio source contracts

- `cmd/folio/main.go`, `serve`: runtime flags/environment, path-versus-host layout, bind-all-listeners-before-ready, bootstrap logging, 10-second graceful shutdown.
- `internal/server/auth.go`, `/api/bootstrap`: unauthenticated GET returns 200 with `needsSetup`/`version` before and after setup. This existing endpoint is the process probe; no database write or provider check is implied.
- `internal/server/server.go`, `New` and `Handler`: data directory is forced to mode 0700, key/token files are private, SQLite uses WAL, and management rejects the wrong Host. Kubernetes uses an owned `/data/folio` child rather than trying to chmod the volume root.
- `internal/server/process_lock.go`: one live server owns an installation using a SQLite exclusive transaction; storage must support reliable SQLite file locking. Deployment policy still must prevent concurrent writers.
- `Dockerfile`: Alpine runtime includes BusyBox `sh`, `wget`, `tar`; image user UID/GID 10001, `/data` owned by that user, content listener environment bound to `0.0.0.0:4311` for explicit host mode.
- `.github/workflows/release.yml`: releases binary archives/checksums, with no image registry publication job. [v0.7.0 release assets](https://github.com/YangTaeyoung/folio/releases/tag/v0.7.0) were checked with `gh release view` on this date: six OS/CPU archives and `checksums.txt`. No first-party registry image is assumed; build/load or push to an operator registry and pin the verified digest.

## Official references and corresponding choices

- [Docker Compose service reference](https://docs.docker.com/reference/compose-file/services/): image/build selection, loopback port publishing, user, healthcheck, read-only filesystem, tmpfs, capability/security settings and stop grace period. [Compose up options](https://docs.docker.com/reference/cli/docker/compose/up/) define `--no-build` and `--pull never` for preloaded images.
- [Docker image save](https://docs.docker.com/reference/cli/docker/image/save/) and [load](https://docs.docker.com/reference/cli/docker/image/load/): transfer a built image archive to disconnected Docker hosts. This does not populate a Kubernetes node runtime.
- [Kubernetes Kustomize](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/): locally render base/overlay, patch origins/probes, and override images by digest.
- [Kubernetes Deployment strategy](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#recreate-deployment): `Recreate` terminates old Pods before creating new ones during updates. One replica remains an application requirement; manual extra Pods/node partitions still require operator fencing.
- [Kubernetes volumes and access modes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes): `ReadWriteOnce` is a single-node access mode, not proof of a single writer; claim deletion and reclaim policy affect underlying data retention.
- [Kubernetes security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod): UID/GID, fsGroup and volume ownership behavior. Storage-driver support must be verified in the target cluster.
- [Kubernetes probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#http-probes): set the virtual-host `Host` header via `httpHeaders`, not the HTTP probe connection host. Startup protects slow initialization; readiness/liveness have distinct traffic/restart effects.
- [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/): explicit optional Secret reference and operator-created values; apply RBAC/encryption controls instead of committing credentials.
- [Kubernetes Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/): an installed controller and TLS certificate are required for the optional overlay. Wildcard host routing is an explicit operator choice for isolated subdomains.
- [SQLite network filesystem cautions](https://www.sqlite.org/useovernet.html) and [WAL](https://www.sqlite.org/wal.html): choose a filesystem with reliable locking, preserve complete stopped installation data, and do not present shared network files as a multiwriter architecture.
- [systemd execution environment](https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html): dedicated user, state-directory ownership/mode, private temp and filesystem protections in the Linux service template.
- [Apple launchd agents](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html): per-user LaunchAgent, explicit ProgramArguments and private log paths for the macOS template; it runs in the logged-in user's session.

## Validation boundaries

`scripts/test_deployment.py` renders Compose and both Kubernetes configurations without contacting a cluster, then checks relevant storage, origin, routing and isolation policies. Its optional Docker smoke starts a fresh, uniquely named installation; it exercises setup, healthcheck, persistence, stopped-volume archive restore and explicit subdomain listener. It never uses existing Folio volumes or applies cluster resources. Cluster admission, storage-driver permissions/snapshots, network policy, ingress-controller behavior, TLS and site backup drills require validation on the operator's selected environment.

On 2026-11-19, both render checks and the optional smoke passed against a locally built working-tree image `folio:deployment-v08-test`. The smoke removed its uniquely named test containers and volumes afterward. `plutil -lint deploy/macos/cloud.melona.folio.plist` also passed. No Kubernetes cluster resources were applied, and the systemd/launchd services were not installed or started as part of this validation.

## CI tool provenance

Configuration tests download kubectl v1.36.1 for Linux/amd64 from `https://dl.k8s.io/release/v1.36.1/bin/linux/amd64/kubectl`, check the literal official SHA-256 `629d3f410e09bf49b64ae7079f7f0bda1191efed311f7d37fdbab0ad5b0ec2b7`, then render both Kustomize configurations. The same binary was exercised in an isolated Linux container. See [official binary/checksum installation guidance](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/#install-kubectl-binary-with-curl-on-linux). These checks do not apply resources to a cluster or validate a particular CSI driver/Ingress controller.
