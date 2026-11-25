---
title: Install
description: Install and run Local Registry from the release binary, the container image, systemd or Kubernetes.
---

This page is for whoever installs and runs the registry: you on your own server, or the team responsible for the instance. Local Registry is one self-contained static binary, with no runtime dependency and no database. Several install methods are available. Whichever you choose, the server must answer `GET /healthz` once started.

| Method | Use it when |
| --- | --- |
| [Release binary](#release-binary) | You use a virtual machine or a physical server. |
| [Container image](#container-image) | You use Docker or Podman. |
| [systemd](#systemd) | You want the server to start with the machine and restart on failure. |
| [Kubernetes](#kubernetes) | You run a cluster with persistent volumes. |
| [From source](#from-source) | You want to build a version from the sources, for example to use an unreleased commit. |

Whatever the method, plan for these three things:

- **A data directory (the asset path).** It holds the published versions, the staging area and the audit log. For example: `/srv/registry`. See [Storage and backup](storage-and-backup.md).
- **An upload token.** It is needed if files are published to the server. Without it, the registry is read-only. See [Security](security.md).
- **A single server instance per data directory.** Uploads are serialised by a lock inside the process. Several instances writing to the same directory at once can interfere with each other and corrupt the data.

## Release binary

Every GitHub release ships archives for Linux, macOS and Windows, on amd64 and arm64.

Each archive holds the `local-registry` binary, the README, the `local-registry/v1` specification, the JSON schema of `registry.yaml`, the two upload scripts, and the `LICENSE` and `NOTICE` files.

```sh
VERSION=X.Y.Z                           # Version to install, without the leading v
OS=linux ARCH=amd64                     # linux, darwin or windows; amd64 or arm64
BASE=https://github.com/nivekalara237/local-registry/releases/download/v$VERSION

curl -fsSLO "$BASE/local-registry_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSLO "$BASE/local-registry_${VERSION}_checksums.txt"
sha256sum --ignore-missing -c "local-registry_${VERSION}_checksums.txt"   # macOS: shasum -a 256 -c

tar xzf "local-registry_${VERSION}_${OS}_${ARCH}.tar.gz" local-registry
sudo install -m 0755 local-registry /usr/local/bin/local-registry
local-registry version
```

Windows archives are `.zip` files.

Create the data directory, then start the server:

```sh
sudo install -d -o registry -g registry -m 0750 /srv/registry
local-registry serve --asset-path /srv/registry
curl -s http://localhost:8077/healthz     # {"status":"ok"}
```

By default, the server listens on `localhost:8077`. To accept connections from other machines, set `--address 0.0.0.0`. Read [Security](security.md) first.

## Container image

The image is published to GitHub Container Registry and Docker Hub, for `linux/amd64` and `linux/arm64`.

Based on Alpine, it runs as user `registry` (UID and GID `10001`).

| Tag | Meaning |
| --- | --- |
| `X.Y.Z` | One exact version. Use this one in production. |
| `X.Y` | The newest patch of a minor version. Stable releases only. |
| `latest` | The newest stable release. Pre-releases (`-rc.N`) never move this tag. |

In the examples below, replace `X.Y.Z` with the version to deploy.

The image sets these settings through environment variables:

| Variable | Default value |
| --- | --- |
| `LOCAL_REGISTRY_ASSET_PATH` | `/data` (data volume) |
| `LOCAL_REGISTRY_ADDRESS` | `0.0.0.0` |
| `LOCAL_REGISTRY_PORT` | `8077` |

Environment variables take precedence over the values of a configuration file. Do not set `asset-path`, `address` or `port` in a file mounted into the container: the image values win.

See [Configuration](configuration.md) for details.

```sh
sudo install -d -o 10001 -g 10001 -m 0750 /srv/registry

export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"   # Keep it in your secret store

docker run -d --name local-registry --restart unless-stopped \
  -p 8077:8077 \
  -v /srv/registry:/data \
  -e LOCAL_REGISTRY_UPLOAD_TOKEN \
  -e LOCAL_REGISTRY_PUBLIC_URL=https://registry.corp.example \
  ghcr.io/nivekalara237/local-registry:X.Y.Z
```

`-e LOCAL_REGISTRY_UPLOAD_TOKEN` with no explicit value passes the variable already set in your shell. The token thus appears neither in the command nor in the shell history.

Beware: it stays visible to users allowed to run `docker inspect`. Restrict access to the Docker daemon.

The image has a `HEALTHCHECK` on `/healthz`. `docker ps` shows the `healthy` state once the server answers.

To set other settings, mount a configuration file and give its path:

```sh
docker run -d --name local-registry \
  -p 8077:8077 -v /srv/registry:/data \
  -v /etc/local-registry/config.yaml:/etc/local-registry/config.yaml:ro \
  -e LOCAL_REGISTRY_CONFIG=/etc/local-registry/config.yaml \
  ghcr.io/nivekalara237/local-registry:X.Y.Z
```

### Verify the image signature

Each release signs its images with Sigstore, keyless, with an identity tied to the release workflow of the GitHub repository.

Before you deploy an image, verify its signature:

```sh
cosign verify ghcr.io/nivekalara237/local-registry:X.Y.Z \
  --certificate-identity-regexp 'https://github.com/nivekalara237/local-registry/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## systemd

You can run the release binary as a systemd service. Keep the settings, the upload token included, in a file readable by the service user only.

```yaml
# /etc/local-registry/config.yaml (owner registry, mode 0600)
asset-path: /srv/registry
address: 0.0.0.0
port: 8443
tls-cert: /etc/local-registry/fullchain.pem
tls-key: /etc/local-registry/privkey.pem
public-url: https://registry.corp.example:8443
audit-log: /var/log/local-registry/audit.log
upload-token: replace-with-the-output-of-openssl-rand-hex-32
```

Then create the service file:

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
# After SIGTERM, the server gives in-flight requests 10 seconds to finish.
TimeoutStopSec=15

[Install]
WantedBy=multi-user.target
```

Create the system user and the directories, then configure and start the service:

```sh
sudo useradd --system --home /srv/registry --shell /usr/sbin/nologin registry
sudo install -d -o registry -g registry -m 0750 /srv/registry /var/log/local-registry
sudo chown registry:registry /etc/local-registry/config.yaml && sudo chmod 600 /etc/local-registry/config.yaml

# Check the configuration before starting the service
sudo -u registry local-registry config --config /etc/local-registry/config.yaml

sudo systemctl daemon-reload
sudo systemctl enable --now local-registry
journalctl -u local-registry -f
```

`local-registry config` applies the same resolution and validation rules as `serve`, but stops without starting the server.

## Kubernetes

Local Registry keeps its state on a single volume and publishes versions with atomic renames on that file system.

Deploy it with **one replica**, a `ReadWriteOnce` volume and the `Recreate` deployment strategy. This strategy lets the old pod release the volume before the new one mounts it.

First create the namespace and the secret that holds the upload token. The token is thus stored in the cluster only:

```sh
kubectl create namespace local-registry

kubectl -n local-registry create secret generic local-registry-upload-token \
  --from-literal=upload-token="$(openssl rand -hex 32)"
```

Then create these Kubernetes resources:

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
    public-url: https://registry.corp.example
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
        fsGroup: 10001          # Makes the volume writable by the registry user.
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: local-registry
          image: ghcr.io/nivekalara237/local-registry:X.Y.Z
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
            readOnlyRootFilesystem: true   # The registry writes under /data only.
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

Expose the Service through your Ingress controller or Gateway, and terminate TLS there.

Also adjust the proxy's request size limits and timeouts for uploads:

- A file can reach the limit set by `--max-upload-mb`, 200 MiB by default.
- An upload may last up to 30 minutes.

`optional: true` on the secret lets the pod start read-only when the secret is missing.

To change the upload token, update the secret, then restart the deployment:

```sh
kubectl -n local-registry rollout restart deployment/local-registry
```

## From source

To build Local Registry from the sources, you need Go, in the version given in `go.mod`.

```sh
git clone https://github.com/nivekalara237/local-registry.git && cd local-registry

go build -ldflags "-X main.version=$(git describe --tags)" -o local-registry ./cmd

./local-registry version
```

## Next steps

- [Configuration](configuration.md): where each setting comes from, and the ones to set in production.
- [Security](security.md): token, HTTPS, what is public.
- [Monitoring](monitoring.md): health check, logs, exit codes.
