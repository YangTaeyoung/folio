# Kubernetes installation

Run all commands from the repository root, against a deliberately selected Kubernetes context. Review `kubectl config current-context` first. The manifests never install a cluster or contact a cloud provider. Requirements: a Kubernetes cluster supporting `apps/v1` Deployments, `networking.k8s.io/v1` Ingress (only if used), a configured default StorageClass or manually bound PV, `kubectl` with Kustomize support, and an image available to the target nodes.

## Prepare an image and storage

Build from a reviewed source checkout, then either load `folio:local` on every eligible node with your cluster distribution's import command or push to your own registry:

```sh
docker build -t folio:local .
docker tag folio:local registry.example.internal/team/folio:YOUR_REVIEWED_REF
docker push registry.example.internal/team/folio:YOUR_REVIEWED_REF
```

The release workflow publishes binary archives and has no container publication step; these manifests do not assume a GHCR/Docker Hub image. Record your pushed image's digest and add an `images` entry to `base/kustomization.yaml` in a private deployment copy:

```yaml
images:
  - name: folio
    newName: registry.example.internal/team/folio
    digest: sha256:YOUR_VERIFIED_DIGEST
```

Use your actual digest, not this placeholder. Add `imagePullSecrets` under the Pod spec if your registry requires them; create that secret through your normal registry-credential workflow, never in Git. For disconnected clusters, mirror the image to an internal registry or import it into **each node's container runtime**. `docker load` on a workstation does not populate a cluster. `IfNotPresent` permits node-local images; immutable digests avoid stale mutable tags. Build for the node CPU architecture and transfer [verified offline archives](../../INSTALL.md#offline-and-air-gap-installation).

The PVC requests 10 GiB using the default StorageClass. Choose a suitable class and capacity in `base/pvc.yaml` before applying. Use block/local filesystem storage with reliable SQLite locks; do not treat an arbitrary NFS/RWX share as supported SQLite storage. A claim's `ReadWriteOnce` access mode permits multiple Pods on one node, so the Deployment also uses **one replica and `Recreate`**. Do not scale it, add an HPA, or start another writer on the claim. During node partitions, fence/stop the old node before forcing a replacement writer; do not force-delete a Pod merely to evade storage attachment or process-lock errors.

The Pod runs UID/GID 10001 and asks the CSI volume driver/kubelet for `fsGroup: 10001`. The app creates **`/data/folio` inside the mounted PVC**, so it owns that private directory and can set mode `0700`. Mounting the PVC root directly as Folio's data directory can fail because the application enforces owner-only permissions. Your storage driver must honor the group/ownership contract; if it does not, preprovision the application directory with UID/GID 10001 through your storage-admin process. Restored files must have that ownership too. No privileged/root init container is included.

## Private first start

The base advertises `http://folio.localhost:4310`, using path mode under `/c/{artifact}`. Artifacts share the management origin: only trusted publishers should use this mode. The Service is ClusterIP, with no Ingress or LoadBalancer.

```sh
kubectl kustomize deploy/kubernetes/base > /tmp/folio-rendered.yaml
# Review the rendered image, origins, namespace and volume before applying.
kubectl apply -k deploy/kubernetes/base
kubectl -n folio rollout status deployment/folio --timeout=5m
kubectl -n folio port-forward --address 127.0.0.1 service/folio 4310:4310
```

Open `http://folio.localhost:4310` from that workstation (its resolver must map `.localhost` to loopback). In another private terminal, use `kubectl -n folio logs deployment/folio --tail=20` for the one-time setup URL. Log access can reveal the setup credential; restrict Pod log/exec permissions and initial log forwarding. The generated token persists privately in `/data/folio/bootstrap.token` until setup completes. Keep port-forward running for the browser session.

Optionally supply your own randomly generated setup token through an existing Secret. Do this **before starting the Deployment**. This command uses a private temporary file, never a token literal in a manifest or CLI argument:

```sh
kubectl apply -f deploy/kubernetes/base/namespace.yaml
umask 077
token_file=$(mktemp)
openssl rand -hex 32 | tr -d '\n' > "$token_file"
kubectl -n folio create secret generic folio-bootstrap --from-file=token="$token_file"
rm -f "$token_file"
```

Use a trap or your secret manager to clean the temporary file if interrupted. Secret lookup is optional; without it Folio generates a token. An existing token is read at process startup. After setup, delete the optional Secret and restart the Pod during a maintenance window to remove it from the environment. Kubernetes Secrets require your cluster's RBAC and encryption-at-rest controls; their base64 representation is not encryption. Never commit a generated Secret, data directory, archive or token.

All three probes use unauthenticated `GET /api/bootstrap` with `Host: folio.localhost:4310`. This endpoint returns 200 before and after organization setup. Startup permits up to five minutes; readiness gates Service traffic, and liveness checks HTTP responsiveness. These are not per-request database writes or external-bucket health checks. **When changing `FOLIO_APP_URL`, change the Host header on all three probes to its exact authority, including a non-default port.** Kubernetes sends probes to the Pod IP; Folio rejects an unrelated Host. Increase startup/probe budgets for your storage and workload rather than hiding recurring startup failures.

## Optional HTTPS Ingress

Make a private copy of the overlay, then edit all occurrences of `folio.example.internal` in `origins.yaml` and `ingress.yaml`, choose your installed `ingressClassName`, and create DNS and a certificate through your approved process. Create the TLS Secret in the `folio` namespace from those files:

```sh
kubectl -n folio create secret tls folio-tls --cert=/private/server.crt --key=/private/server.key
kubectl kustomize deploy/kubernetes/overlays/ingress
kubectl apply -k deploy/kubernetes/overlays/ingress
kubectl -n folio rollout status deployment/folio --timeout=5m
```

Apply either the base or your overlay consistently; later applying the base overwrites the overlay origins. This path-mode overlay routes one hostname to management port 4310 and updates the probe Host values. The ingress controller must preserve the original Host and support your chosen upload sizes, buffering and timeouts. No issuer/controller, public DNS change, wildcard certificate or external exposure is created automatically. Validate private access denial, login, publishing and asset loads through actual HTTPS before using the installation.

For isolated **subdomain mode**, set `FOLIO_CONTENT_URL` to `https://{artifact}.artifacts.example.internal`, arrange wildcard DNS/TLS, and add a distinct Ingress host rule `"*.artifacts.example.internal"` with backend Service port **`content` (4311)**. Add that wildcard to TLS hosts with a matching certificate. Keep the management rule on **`http` (4310)** and the probe Host on the management authority. Never route artifact hosts to the authenticated management origin or rewrite all artifact Hosts to one upstream Host. The image already binds its content listener to `0.0.0.0:4311`; it is unused in path mode. Preserve your existing explicit content template on upgrades.

## Upgrade, backup and restore

There is planned downtime for updates. Keep a private copy of the rendered manifests, the exact previous image/digest and origin configuration. Stop ingress/traffic for a backup, scale to zero, and wait for the application Pod to terminate before accessing installation files:

```sh
kubectl -n folio scale deployment/folio --replicas=0
kubectl -n folio wait --for=delete pod -l app.kubernetes.io/name=folio --timeout=120s
# Edit this maintenance Pod's image to your exact available Folio image first.
kubectl apply -f deploy/kubernetes/maintenance-pod.yaml
kubectl -n folio wait --for=condition=Ready pod/folio-maintenance --timeout=120s
umask 077
mkdir -p backups
kubectl -n folio exec folio-maintenance -- tar -C /data/folio -czf - . > backups/folio-data.tar.gz
tar -tzf backups/folio-data.tar.gz > /dev/null
kubectl -n folio delete pod folio-maintenance --wait=true
```

This archive includes all application data, hidden lock files, SQLite sidecars, installation key and local objects. Back up external stores separately at the same stopped point. A CSI volume snapshot is an alternative only with the application stopped and matching external-store backups. Keep backups encrypted and access controlled.

After backup, update the image digest in your private Kustomization and apply the same base/overlay. Its replica count is one, so it starts Folio again. Do not manually increase replicas for availability. Restore drills must target a separate PVC/namespace and use no live DNS.

For recovery, keep the application at zero, provision an **empty replacement PVC** and point both Deployment and maintenance Pod at that claim. Start the maintenance Pod using the backed-up release image; it creates `/data/folio` as UID 10001. Restore the complete archive:

```sh
kubectl -n folio exec -i folio-maintenance -- tar -C /data/folio -xzf - < backups/folio-data.tar.gz
kubectl -n folio delete pod folio-maintenance --wait=true
# Restore the backed-up image/origins and matching external objects, then:
kubectl apply -k deploy/kubernetes/base # or your reviewed overlay
kubectl -n folio rollout status deployment/folio --timeout=5m
```

Verify owner-only permissions, login, one private artifact, historical revisions, external storage and `folio doctor` before reopening ingress. Do not copy a running SQLite database alone, restore over live files or roll back only the image after a data-format migration. Never delete the namespace/PVC as a normal upgrade; deleting a claim can delete its underlying volume depending on its reclaim policy. A PodDisruptionBudget would not make this one-writer installation highly available.

## Verification

Run `python3 scripts/test_deployment.py` with Docker Compose, `kubectl` and PyYAML installed. It renders configurations locally and checks storage, probe origins, replicas, secret references and ingress routing without contacting a cluster. `python3 scripts/test_deployment.py --docker-image folio:local` additionally exercises a fresh isolated Docker installation and restart, including setup and both content modes. It creates and removes only a uniquely named test project/volume. An actual cluster rollout, storage-driver ownership, ingress/TLS and a restore drill remain environment-specific acceptance checks; config rendering alone does not prove them.

Design references and source contracts: [deployment references](../../docs/references/deployment-options.md).
