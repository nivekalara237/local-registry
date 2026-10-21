---
title: Installation
description: Lancer local-registry depuis le binaire publié, l'image conteneur, systemd ou Kubernetes.
---

local-registry est un binaire statique unique, sans dépendance à l'exécution ni base de données. Choisissez une méthode d'installation ci-dessous. Chacune se termine par un serveur qui répond à `GET /healthz`.

| Méthode | À utiliser quand |
| --- | --- |
| [Binaire publié](#binaire-publié) | Vous exploitez une VM ou un serveur physique. |
| [Image conteneur](#image-conteneur) | Vous utilisez Docker ou Podman. |
| [systemd](#systemd) | Le binaire doit démarrer avec la machine et redémarrer en cas d'échec. |
| [Kubernetes](#kubernetes) | Vous exploitez un cluster avec des volumes persistants. |
| [Depuis les sources](#depuis-les-sources) | Vous avez besoin d'un commit non publié. |

Quelle que soit la méthode, prévoyez trois choses :

- **Un répertoire pour toutes les données**, l'asset path, par exemple `/srv/registry`. Il contient les versions publiées, la zone de staging et le journal d'audit. Voir [Stockage et sauvegarde](storage-and-backup.md).
- **Un token d'upload**, si des équipes publient sur ce serveur. Sans token, le registre est en lecture seule. Voir [Sécurité](security.md).
- **Un serveur par asset path.** Les uploads sont sérialisés par un verrou interne au processus. Deux serveurs qui écrivent dans le même répertoire peuvent abîmer le travail l'un de l'autre.

## Binaire publié

Chaque release GitHub fournit des archives pour Linux, macOS et Windows, en amd64 et arm64. Chaque archive contient le binaire `local-registry`, le README, la spécification `local-registry/v1`, le schéma JSON de `registry.yaml`, les deux scripts d'upload, et les fichiers `LICENSE` et `NOTICE`.

```sh
VERSION=1.0.0-rc.3                      # la release, sans le v initial
OS=linux ARCH=amd64                     # darwin ou windows ; arm64
BASE=https://github.com/nivekalara237/local-registry/releases/download/v$VERSION

curl -fsSLO "$BASE/local-registry_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSLO "$BASE/local-registry_${VERSION}_checksums.txt"
sha256sum --ignore-missing -c "local-registry_${VERSION}_checksums.txt"   # macOS : shasum -a 256 -c

tar xzf "local-registry_${VERSION}_${OS}_${ARCH}.tar.gz" local-registry
sudo install -m 0755 local-registry /usr/local/bin/local-registry
local-registry version
```

Les archives Windows sont des fichiers `.zip`.

Créez le répertoire de données et démarrez le serveur :

```sh
sudo install -d -o registry -g registry -m 0750 /srv/registry
local-registry serve --asset-path /srv/registry
curl -s http://localhost:8077/healthz     # {"status":"ok"}
```

Par défaut, le serveur écoute sur `localhost:8077`. Pour servir d'autres machines, réglez `--address 0.0.0.0` et lisez d'abord [Sécurité](security.md).

## Image conteneur

L'image est publiée sur GitHub Container Registry et sur Docker Hub, pour `linux/amd64` et `linux/arm64`. Elle repose sur Alpine et tourne sous l'utilisateur `registry` (uid et gid `10001`).

| Tag | Désigne |
| --- | --- |
| `1.0.0` | Une release exacte. Utilisez celui-ci en production. |
| `1.0` | Le dernier correctif d'une version mineure. Releases stables uniquement. |
| `latest` | La dernière release stable. Les pré-releases (`-rc.N`) ne le déplacent jamais. |

> [!NOTE]
> Jusqu'à la première release stable, seuls des tags exacts de pré-release comme `1.0.0-rc.3` existent.

L'image règle ces valeurs par des variables d'environnement :

| Variable | Valeur |
| --- | --- |
| `LOCAL_REGISTRY_ASSET_PATH` | `/data` (un volume) |
| `LOCAL_REGISTRY_ADDRESS` | `0.0.0.0` |
| `LOCAL_REGISTRY_PORT` | `8077` |

Les variables d'environnement priment sur un fichier de configuration. Ne mettez pas `asset-path`, `address` ni `port` dans un fichier monté dans le conteneur : les valeurs de l'image l'emportent. Voir [Configuration](configuration.md).

```sh
sudo install -d -o 10001 -g 10001 -m 0750 /srv/registry

export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"   # conservez-le dans votre coffre de secrets
docker run -d --name local-registry --restart unless-stopped \
  -p 8077:8077 \
  -v /srv/registry:/data \
  -e LOCAL_REGISTRY_UPLOAD_TOKEN \
  -e LOCAL_REGISTRY_PUBLIC_URL=https://registry.example.com \
  ghcr.io/nivekalara237/local-registry:1.0.0-rc.3
```

`-e LOCAL_REGISTRY_UPLOAD_TOKEN` sans valeur recopie la variable de votre shell : le token reste hors de la ligne de commande et de l'historique. Il reste visible pour quiconque peut lancer `docker inspect` : restreignez l'accès au démon Docker.

L'image définit un `HEALTHCHECK` sur `/healthz`. `docker ps` affiche le conteneur `healthy` dès que le serveur répond.

Pour passer d'autres réglages, montez un fichier de configuration et indiquez son chemin :

```sh
docker run -d --name local-registry \
  -p 8077:8077 -v /srv/registry:/data \
  -v /etc/local-registry/config.yaml:/etc/local-registry/config.yaml:ro \
  -e LOCAL_REGISTRY_CONFIG=/etc/local-registry/config.yaml \
  ghcr.io/nivekalara237/local-registry:1.0.0-rc.3
```

### Vérifier la signature de l'image

Chaque release signe ses images avec une signature Sigstore sans clé, émise pour le workflow de release du dépôt GitHub. Vérifiez une image avant de la déployer :

```sh
cosign verify ghcr.io/nivekalara237/local-registry:1.0.0-rc.3 \
  --certificate-identity-regexp 'https://github.com/nivekalara237/local-registry/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## systemd

Lancez le binaire publié comme service. Gardez tous les réglages, token compris, dans un fichier de configuration lisible par l'utilisateur du service seulement.

```yaml
# /etc/local-registry/config.yaml (propriétaire registry, mode 0600)
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
# Le serveur laisse 10 s aux requêtes en cours pour finir après SIGTERM.
TimeoutStopSec=15

[Install]
WantedBy=multi-user.target
```

```sh
sudo useradd --system --home /srv/registry --shell /usr/sbin/nologin registry
sudo install -d -o registry -g registry -m 0750 /srv/registry /var/log/local-registry
sudo chown registry:registry /etc/local-registry/config.yaml && sudo chmod 600 /etc/local-registry/config.yaml
sudo -u registry local-registry config --config /etc/local-registry/config.yaml   # vérifier avant de démarrer
sudo systemctl daemon-reload
sudo systemctl enable --now local-registry
journalctl -u local-registry -f
```

`local-registry config` résout la configuration exactement comme `serve` et s'arrête sur les mêmes erreurs, sans rien démarrer.

## Kubernetes

Le registre garde son état sur un seul volume et publie par des renommages atomiques sur ce système de fichiers. Lancez-le en **une seule réplique**, avec un volume `ReadWriteOnce` et la stratégie `Recreate` : l'ancien pod libère le volume avant que le nouveau le monte.

Créez le namespace et le token d'upload. Le token n'existe que dans le cluster :

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
  # asset-path, address et port viennent de l'image : ne les mettez pas ici.
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
        fsGroup: 10001          # rend le volume accessible en écriture à l'utilisateur registry
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: local-registry
          image: ghcr.io/nivekalara237/local-registry:1.0.0-rc.3
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
            readOnlyRootFilesystem: true   # le registre n'écrit que sous /data
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

Exposez le Service par votre contrôleur Ingress ou votre Gateway, et terminez TLS à cet endroit. Relevez les limites de taille de corps et de durée du proxy pour les uploads : un seul fichier peut atteindre `--max-upload-mb` (200 Mio par défaut) et un upload peut durer jusqu'à 30 minutes.

`optional: true` sur le secret laisse tourner le pod en lecture seule si le secret manque. Pour changer de token, mettez à jour le secret, puis lancez `kubectl -n local-registry rollout restart deployment/local-registry`.

## Depuis les sources

Il vous faut Go (la version indiquée dans `go.mod`).

```sh
git clone https://github.com/nivekalara237/local-registry.git && cd local-registry
go build -ldflags "-X main.version=$(git describe --tags)" -o local-registry ./cmd
./local-registry version
```

## Étapes suivantes

- [Configuration](configuration.md) : d'où vient chaque réglage, et ceux à fixer en production.
- [Sécurité](security.md) : token, HTTPS, ce qui est public.
- [Supervision](monitoring.md) : health check, logs, codes de sortie.
