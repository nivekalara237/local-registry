---
title: Install
description: Run local-registry from the release binary, the container image, systemd or Kubernetes.
---

local-registry is one static binary with no runtime dependency and no database. Choose an install method below. Each one ends with a running server that answers `GET /healthz`.

| Method | Use it when |
| --- | --- |
| [Release binary](#release-binary) | You run a VM or a bare-metal host. |
| [Container image](#container-image) | You run Docker or Podman. |
| [systemd](#systemd) | The binary must start with the host and restart on failure. |
| [Kubernetes](#kubernetes) | You run a cluster with persistent volumes. |
| [From source](#from-source) | You need an unreleased commit. |

Whatever the method, plan for three things:

- **One directory for all data**, the asset path, such as `/srv/registry`. It holds the published versions, the staging area and the audit log. See [Storage and backup](storage-and-backup.md).
- **One upload token**, if teams publish to this server. Without a token the registry is read-only. See [Security](security.md).
- **One server per asset path.** Uploads are serialised by a lock inside the process. Two servers that write to the same directory can corrupt each other's work.

## Release binary

Every release on GitHub ships archives for Linux, macOS and Windows, on amd64 and arm64. Each archive holds the `local-registry` binary, the README, the `local-registry/v1` specification, the JSON schema of `registry.yaml`, the two upload scripts, and the `LICENSE` and `NOTICE` files.

```sh
VERSION=1.0.0-rc.2                      # the release, without the leading v
OS=linux ARCH=amd64                     # darwin or windows; arm64
BASE=https://github.com/nivekalara237/local-registry/releases/download/v$VERSION

curl -fsSLO "$BASE/local-registry_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSLO "$BASE/local-registry_${VERSION}_checksums.txt"
sha256sum --ignore-missing -c "local-registry_${VERSION}_checksums.txt"   # macOS: shasum -a 256 -c

tar xzf "local-registry_${VERSION}_${OS}_${ARCH}.tar.gz" local-registry
sudo install -m 0755 local-registry /usr/local/bin/local-registry
local-registry version
```

Windows archives are `.zip` files.

Create the data directory and start the server:

```sh
sudo install -d -o registry -g registry -m 0750 /srv/registry
local-registry serve --asset-path /srv/registry
curl -s http://localhost:8077/healthz     # {"status":"ok"}
```

The server listens on `localhost:8077` by default. To serve other hosts, set `--address 0.0.0.0` and read [Security](security.md) first.

## Container image

The image is published to the GitHub Container Registry and to Docker Hub for `linux/amd64` and `linux/arm64`. It is based on Alpine and runs as user `registry` (uid and gid `10001`).

| Tag | Points to |
| --- | --- |
| `1.0.0` | One exact release. Use this one in production. |
| `1.0` | The newest patch of a minor version. Stable releases only. |
| `latest` | The newest stable release. Pre-releases (`-rc.N`) never move it. |

> [!NOTE]
> Until the first stable release, only exact pre-release tags such as `1.0.0-rc.2` exist.

The image sets these defaults through environment variables:

| Variable | Value |
| --- | --- |
| `LOCAL_REGISTRY_ASSET_PATH` | `/data` (a volume) |
| `LOCAL_REGISTRY_ADDRESS` | `0.0.0.0` |
| `LOCAL_REGISTRY_PORT` | `8077` |

Environment variables take precedence over a configuration file. Do not set `asset-path`, `address` or `port` in a file mounted into the container: the image values win. See [Configuration](configuration.md).

```sh
sudo install -d -o 10001 -g 10001 -m 0750 /srv/registry

export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"   # keep it in your secret store
docker run -d --name local-registry --restart unless-stopped \
  -p 8077:8077 \
  -v /srv/registry:/data \
  -e LOCAL_REGISTRY_UPLOAD_TOKEN \
  -e LOCAL_REGISTRY_PUBLIC_URL=https://registry.example.com \
  ghcr.io/nivekalara237/local-registry:1.0.0-rc.2
```

`-e LOCAL_REGISTRY_UPLOAD_TOKEN` with no value copies the variable from your shell, so the token stays out of the command line and your shell history. It is still visible to anyone who can run `docker inspect`: restrict access to the Docker daemon.

The image has a `HEALTHCHECK` on `/healthz`. `docker ps` shows the container as `healthy` once the server answers.

To pass more settings, mount a configuration file and point to it:

```sh
docker run -d --name local-registry \
  -p 8077:8077 -v /srv/registry:/data \
  -v /etc/local-registry/config.yaml:/etc/local-registry/config.yaml:ro \
  -e LOCAL_REGISTRY_CONFIG=/etc/local-registry/config.yaml \
  ghcr.io/nivekalara237/local-registry:1.0.0-rc.2
```

### Verify the image signature

Each release signs its images with a keyless Sigstore signature, issued to the release workflow of the GitHub repository. Verify an image before you deploy it:

```sh
cosign verify ghcr.io/nivekalara237/local-registry:1.0.0-rc.2 \
  --certificate-identity-regexp 'https://github.com/nivekalara237/local-registry/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## systemd

Run the release binary as a service. Keep every setting, the token included, in a configuration file readable by the service user only.

```yaml
# /etc/local-registry/config.yaml (owner registry, mode 0600)
asset-path: /srv/registry
address: 0.0.0.0
port: 8443
tls-cert: /etc/local-registry/fullchain.pem
tls-key: /etc/local-registry/privkey.pem
public-url: https://registry.example.com:8443
audit-log: /var/log/local-registry/audit.log
upload-token: replace-with-the-output-of-openssl-rand-hex-32
```

```ini
# /etc/systemd/system/local-registry.service
[Unit]
Description=local-registry
After=network-online.target
Wants=network-online.target

[Service]
User=registry
Group=registry
ExecStart=/usr/local/bin/local-registry serve --config /etc/local-registry/config.yaml
Restart=on-failure
# The server gives running requests 10 s to finish on SIGTERM.
TimeoutStopSec=15

[Install]
WantedBy=multi-user.target
```

```sh
sudo useradd --system --home /srv/registry --shell /usr/sbin/nologin registry
sudo install -d -o registry -g registry -m 0750 /srv/registry /var/log/local-registry
sudo chown registry:registry /etc/local-registry/config.yaml && sudo chmod 600 /etc/local-registry/config.yaml
sudo -u registry local-registry config --config /etc/local-registry/config.yaml   # check before starting
sudo systemctl daemon-reload
sudo systemctl enable --now local-registry
journalctl -u local-registry -f
```

`local-registry config` resolves the configuration exactly like `serve` and stops on the same errors, without starting anything.

## Kubernetes

The registry keeps its state on one volume and publishes with atomic renames on that file system. Run it as **one replica** with a `ReadWriteOnce` volume and the `Recreate` strategy, so the old pod releases the volume before the new one mounts it.

Create the namespace and the upload token. The token lives only in the cluster:

```sh
kubectl create namespace local-registry
kubectl -n local-registry create secret generic local-registry-upload-token \
  --from-literal=upload-token="$(openssl rand -hex 32)"
```

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata: {name: local-registry-data, namespace: local-registry}
spec:
  accessModes: [ReadWriteOnce]
  resources: {requests: {storage: 20Gi}}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: local-registry-config, namespace: local-registry}
data:
  # asset-path, address and port come from the image: do not set them here.
  config.yaml: |
    public-url: https://registry.example.com
    staging-ttl: 24h
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: local-registry, namespace: local-registry}
spec:
  replicas: 1
  strategy: {type: Recreate}
  selector: {matchLabels: {app.kubernetes.io/name: local-registry}}
  template:
    metadata: {labels: {app.kubernetes.io/name: local-registry}}
    spec:
      automountServiceAccountToken: false
      terminationGracePeriodSeconds: 30
      securityContext:
        runAsNonRoot: true
        runAsUser: 10001
        runAsGroup: 10001
        fsGroup: 10001          # makes the volume writable by the registry user
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: local-registry
          image: ghcr.io/nivekalara237/local-registry:1.0.0-rc.2
          args: [serve]
          env:
            - name: LOCAL_REGISTRY_CONFIG
              value: /etc/local-registry/config.yaml
            - name: LOCAL_REGISTRY_UPLOAD_TOKEN
              valueFrom:
                secretKeyRef: {name: local-registry-upload-token, key: upload-token, optional: true}
          ports: [{name: http, containerPort: 8077}]
          startupProbe: {httpGet: {path: /healthz, port: http}, periodSeconds: 2, failureThreshold: 15}
          livenessProbe: {httpGet: {path: /healthz, port: http}, periodSeconds: 20, timeoutSeconds: 3}
          readinessProbe: {httpGet: {path: /healthz, port: http}, periodSeconds: 10, timeoutSeconds: 3}
          resources:
            requests: {cpu: 50m, memory: 64Mi}
            limits: {memory: 256Mi}
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true   # the registry writes under /data only
            capabilities: {drop: [ALL]}
          volumeMounts:
            - {name: data, mountPath: /data}
            - {name: config, mountPath: /etc/local-registry, readOnly: true}
      volumes:
        - {name: data, persistentVolumeClaim: {claimName: local-registry-data}}
        - {name: config, configMap: {name: local-registry-config}}
---
apiVersion: v1
kind: Service
metadata: {name: local-registry, namespace: local-registry}
spec:
  selector: {app.kubernetes.io/name: local-registry}
  ports: [{name: http, port: 80, targetPort: http}]
```

Expose the Service through your Ingress controller or Gateway, and terminate TLS there. Raise the proxy's body size and timeout limits for uploads: a single file can reach `--max-upload-mb` (200 MiB by default) and an upload may last up to 30 minutes.

`optional: true` on the secret keeps the pod running read-only when the secret is missing. To rotate the token, update the secret, then `kubectl -n local-registry rollout restart deployment/local-registry`.

## From source

You need Go (the version in `go.mod`).

```sh
git clone https://github.com/nivekalara237/local-registry.git && cd local-registry
go build -ldflags "-X main.version=$(git describe --tags)" -o local-registry ./cmd
./local-registry version
```

## Next steps

- [Configuration](configuration.md): where each setting comes from, and the ones to set in production.
- [Security](security.md): token, HTTPS, what is public.
- [Monitoring](monitoring.md): health check, logs, exit codes.
