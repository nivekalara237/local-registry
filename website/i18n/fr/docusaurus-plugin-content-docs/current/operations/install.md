---
title: Installation
description: Installer et lancer Local Registry depuis le binaire publié, l'image conteneur, systemd ou Kubernetes.
---

Cette page s'adresse à qui installe et exploite le registre : vous sur votre propre serveur, ou l'équipe responsable de l'instance. Local Registry est un binaire statique autonome, sans dépendance à l'exécution ni base de données. Plusieurs méthodes d'installation sont disponibles. Quelle que soit celle que vous choisissez, le serveur doit répondre à `GET /healthz` une fois démarré.

| Méthode | À utiliser lorsque |
| --- | --- |
| [Binaire publié](#binaire-publié) | Vous utilisez une machine virtuelle ou un serveur physique. |
| [Image conteneur](#image-conteneur) | Vous utilisez Docker ou Podman. |
| [systemd](#systemd) | Vous souhaitez démarrer automatiquement le serveur avec la machine et le redémarrer en cas d'échec. |
| [Kubernetes](#kubernetes) | Vous exploitez un cluster disposant de volumes persistants. |
| [Depuis les sources](#depuis-les-sources) | Vous souhaitez compiler une version à partir des sources, notamment pour utiliser un commit non publié. |

Quelle que soit la méthode choisie, prévoyez les trois éléments suivants :

- **Un répertoire de données (`asset path`).** Il centralise les versions publiées, la zone de staging et le journal d'audit. Par exemple : `/srv/registry`. Consultez [Stockage et sauvegarde](storage-and-backup.md).
- **Un token d'upload.** Il est nécessaire si des fichiers doivent être publiés sur le serveur. En son absence, le registre fonctionne en lecture seule. Consultez [Sécurité](security.md).
- **Une seule instance du serveur par répertoire de données.** Les opérations d'upload sont sérialisées à l'aide d'un verrou interne au processus. Plusieurs instances écrivant simultanément dans le même répertoire risquent d'interférer entre elles et de corrompre les données.

## Binaire publié

Chaque release GitHub fournit des archives pour Linux, macOS et Windows, en architectures amd64 et arm64.

Chaque archive contient le binaire `local-registry`, le README, la spécification `local-registry/v1`, le schéma JSON de `registry.yaml`, les deux scripts d'upload, ainsi que les fichiers `LICENSE` et `NOTICE`.

```sh
VERSION=X.Y.Z                           # Version à installer, sans le préfixe v
OS=linux ARCH=amd64                     # linux, darwin ou windows ; amd64 ou arm64
BASE=https://github.com/nivekalara237/local-registry/releases/download/v$VERSION

curl -fsSLO "$BASE/local-registry_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSLO "$BASE/local-registry_${VERSION}_checksums.txt"
sha256sum --ignore-missing -c "local-registry_${VERSION}_checksums.txt"   # macOS : shasum -a 256 -c

tar xzf "local-registry_${VERSION}_${OS}_${ARCH}.tar.gz" local-registry
sudo install -m 0755 local-registry /usr/local/bin/local-registry
local-registry version
```

Les archives destinées à Windows sont fournies au format `.zip`.

Créez le répertoire de données, puis démarrez le serveur :

```sh
sudo install -d -o registry -g registry -m 0750 /srv/registry
local-registry serve --asset-path /srv/registry
curl -s http://localhost:8077/healthz     # {"status":"ok"}
```

Par défaut, le serveur écoute sur `localhost:8077`. Pour autoriser les connexions depuis d'autres machines, configurez `--address 0.0.0.0`. Consultez au préalable le guide [Sécurité](security.md).

## Image conteneur

L'image est publiée sur GitHub Container Registry et Docker Hub. Elle prend en charge les architectures `linux/amd64` et `linux/arm64`.

Basée sur Alpine, elle s'exécute avec l'utilisateur `registry` (UID et GID `10001`).

| Tag | Signification |
| --- | --- |
| `X.Y.Z` | Version exacte. À privilégier en production. |
| `X.Y` | Dernier correctif disponible d'une version mineure. Seules les releases stables sont prises en compte. |
| `latest` | Dernière release stable. Les préversions (`-rc.N`) ne modifient jamais ce tag. |

Dans les exemples ci-dessous, remplacez `X.Y.Z` par la version à déployer.

L'image configure les paramètres suivants à l'aide de variables d'environnement :

| Variable | Valeur par défaut |
| --- | --- |
| `LOCAL_REGISTRY_ASSET_PATH` | `/data` (volume de données) |
| `LOCAL_REGISTRY_ADDRESS` | `0.0.0.0` |
| `LOCAL_REGISTRY_PORT` | `8077` |

Les variables d'environnement sont prioritaires sur les valeurs définies dans un fichier de configuration. Ne renseignez donc pas `asset-path`, `address` ou `port` dans un fichier monté dans le conteneur : les valeurs définies par l'image prévalent.

Consultez [Configuration](configuration.md) pour plus de détails.

```sh
sudo install -d -o 10001 -g 10001 -m 0750 /srv/registry

export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"   # Conservez-le dans votre coffre de secrets

docker run -d --name local-registry --restart unless-stopped \
  -p 8077:8077 \
  -v /srv/registry:/data \
  -e LOCAL_REGISTRY_UPLOAD_TOKEN \
  -e LOCAL_REGISTRY_PUBLIC_URL=https://registry.corp.example \
  ghcr.io/nivekalara237/local-registry:X.Y.Z
```

L'option `-e LOCAL_REGISTRY_UPLOAD_TOKEN`, sans valeur explicite, transmet la variable déjà définie dans votre environnement shell. Le token n'apparaît ainsi ni dans la commande ni dans l'historique du terminal.

Attention : il reste accessible aux utilisateurs autorisés à exécuter `docker inspect`. Limitez donc l'accès au démon Docker.

L'image intègre un `HEALTHCHECK` qui vérifie la disponibilité de `/healthz`. La commande `docker ps` indique l'état `healthy` lorsque le serveur répond correctement.

Pour configurer d'autres paramètres, montez un fichier de configuration et indiquez son chemin :

```sh
docker run -d --name local-registry \
  -p 8077:8077 -v /srv/registry:/data \
  -v /etc/local-registry/config.yaml:/etc/local-registry/config.yaml:ro \
  -e LOCAL_REGISTRY_CONFIG=/etc/local-registry/config.yaml \
  ghcr.io/nivekalara237/local-registry:X.Y.Z
```

### Vérifier la signature de l'image

Chaque release signe ses images à l'aide de Sigstore, sans clé, avec une identité associée au workflow de publication du dépôt GitHub.

Avant de déployer une image, vérifiez sa signature :

```sh
cosign verify ghcr.io/nivekalara237/local-registry:X.Y.Z \
  --certificate-identity-regexp 'https://github.com/nivekalara237/local-registry/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## systemd

Vous pouvez exécuter le binaire publié en tant que service systemd. Regroupez les paramètres de configuration, y compris le token d'upload, dans un fichier accessible uniquement à l'utilisateur du service.

```yaml
# /etc/local-registry/config.yaml (propriétaire registry, permissions 0600)
asset-path: /srv/registry
address: 0.0.0.0
port: 8443
tls-cert: /etc/local-registry/fullchain.pem
tls-key: /etc/local-registry/privkey.pem
public-url: https://registry.corp.example:8443
audit-log: /var/log/local-registry/audit.log
upload-token: replace-with-the-output-of-openssl-rand-hex-32
```

Créez ensuite le fichier de service :

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
# Le serveur dispose de 10 secondes pour terminer les requêtes en cours après SIGTERM.
TimeoutStopSec=15

[Install]
WantedBy=multi-user.target
```

Créez l'utilisateur système et les répertoires nécessaires, puis configurez et démarrez le service :

```sh
sudo useradd --system --home /srv/registry --shell /usr/sbin/nologin registry
sudo install -d -o registry -g registry -m 0750 /srv/registry /var/log/local-registry
sudo chown registry:registry /etc/local-registry/config.yaml && sudo chmod 600 /etc/local-registry/config.yaml

# Vérifier la configuration avant de démarrer le service
sudo -u registry local-registry config --config /etc/local-registry/config.yaml

sudo systemctl daemon-reload
sudo systemctl enable --now local-registry
journalctl -u local-registry -f
```

La commande `local-registry config` applique les mêmes règles de résolution et de validation de configuration que `serve`, mais s'arrête sans démarrer le serveur.

## Kubernetes

Local Registry conserve son état sur un volume unique et utilise des renommages atomiques sur ce système de fichiers pour publier les versions.

Déployez-le avec **une seule réplique**, un volume `ReadWriteOnce` et une stratégie de déploiement `Recreate`. Cette stratégie permet à l'ancien pod de libérer le volume avant que le nouveau ne le monte.

Commencez par créer le namespace et le secret contenant le token d'upload. Le token est ainsi stocké dans le cluster :

```sh
kubectl create namespace local-registry

kubectl -n local-registry create secret generic local-registry-upload-token \
  --from-literal=upload-token="$(openssl rand -hex 32)"
```

Créez ensuite les ressources Kubernetes suivantes :

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
  # asset-path, address et port sont fournis par l'image : ne les renseignez pas ici.
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
        fsGroup: 10001          # Rend le volume accessible en écriture à l'utilisateur registry.
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
            readOnlyRootFilesystem: true   # Le registre n'écrit que sous /data.
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

Exposez le Service à l'aide de votre contrôleur Ingress ou de votre Gateway, et configurez la terminaison TLS à cet emplacement.

Pensez également à adapter les limites de taille des requêtes et les délais d'attente du proxy pour les opérations d'upload :

- Un fichier peut atteindre la limite définie par `--max-upload-mb`, fixée à 200 Mio par défaut.
- Un upload peut durer jusqu'à 30 minutes.

L'option `optional: true` appliquée au secret permet au pod de démarrer en lecture seule lorsque le secret est absent.

Pour modifier le token d'upload, mettez à jour le secret, puis redémarrez le déploiement :

```sh
kubectl -n local-registry rollout restart deployment/local-registry
```

## Depuis les sources

Pour compiler Local Registry à partir des sources, vous devez disposer de Go dans la version indiquée dans le fichier `go.mod`.

```sh
git clone https://github.com/nivekalara237/local-registry.git && cd local-registry

go build -ldflags "-X main.version=$(git describe --tags)" -o local-registry ./cmd

./local-registry version
```

## Étapes suivantes

- [Configuration](configuration.md) : d'où vient chaque réglage, et ceux à fixer en production.
- [Sécurité](security.md) : token, HTTPS, ce qui est public.
- [Supervision](monitoring.md) : health check, logs, codes de sortie.
