# local-registry

Un registry HTTP local pour les releases produites par [goreleaser](https://goreleaser.com) : il sert le catalogue des versions, les binaires et leurs checksums, et accepte la publication d'une nouvelle version en deux temps (envoi des fichiers, puis publication atomique).

## Sommaire

- [Démarrage rapide](#démarrage-rapide)
- [Installation et lancement](#installation-et-lancement)
- [Arborescence des fichiers](#arborescence-des-fichiers)
- [Conventions communes à toutes les routes](#conventions-communes-à-toutes-les-routes)
- [Référence de l'API](#référence-de-lapi)
  - [Vue d'ensemble](#vue-densemble)
  - [`GET /{vcs}/artifacts`](#get-vcsartifacts)
  - [`GET /{vcs}/artifacts/{version}`](#get-vcsartifactsversion)
  - [`GET /artifacts/all`](#get-artifactsall)
  - [`GET /artifacts/metadatas`](#get-artifactsmetadatas)
  - [`GET /assets/binary/{filePath}`](#get-assetsbinaryfilepath)
  - [`GET /assets/checksum/{filename}`](#get-assetschecksumfilename)
  - [`POST /releases/{version}`](#post-releasesversion)
  - [Authentification de l'upload](#authentification-de-lupload)
  - [`PUT /{vcs}/artifacts/{version}/files/{name}`](#put-vcsartifactsversionfilesname)
  - [`POST /{vcs}/artifacts/{version}/publish`](#post-vcsartifactsversionpublish)
- [Publier une release pas à pas](#publier-une-release-pas-à-pas)
- [Le script `publish-to-local-registry.sh`](#le-script-publish-to-local-registrysh)
- [Quotas, espace disque et purge du staging](#quotas-espace-disque-et-purge-du-staging)
- [Journal d'audit](#journal-daudit)
- [Garanties d'atomicité et de cohérence](#garanties-datomicité-et-de-cohérence)
- [Opérations courantes](#opérations-courantes)
- [Utilisation comme bibliothèque Go](#utilisation-comme-bibliothèque-go)
- [Limites connues](#limites-connues)
- [Dépannage](#dépannage)

---

## Démarrage rapide

```sh
# 1. Compiler
go build -o local-registry ./cmd

# 2. Lancer, avec l'upload activé
export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"
./local-registry serve --asset-path /srv/registry    # ajoutez --tls-cert/--tls-key pour HTTPS

# 3. Dans le projet goreleaser : construire puis publier
goreleaser release --clean
LOCAL_REGISTRY_URL=http://localhost:8077 \
  /chemin/vers/scripts/publish-to-local-registry.sh --latest gitlab dist

# 4. Lire le catalogue
curl -s http://localhost:8077/gitlab/artifacts           # dernière version
curl -s http://localhost:8077/gitlab/artifacts/v0.1.0    # une version précise
```

---

## Installation et lancement

### Prérequis

| Outil | Version | Pour quoi |
| --- | --- | --- |
| Go | 1.25.1 ou plus (fixé par `go.mod`) | compiler le serveur |
| `sh` et `curl` | curl 7.76 ou plus (`--fail-with-body`) | le script de publication |
| goreleaser | testé avec 2.18.2 | produire les releases |

### Compiler

```sh
go build -o local-registry ./cmd

# Avec un numéro de version, affiché par `local-registry version`
go build -ldflags "-X main.version=v1.2.3" -o local-registry ./cmd
```

Le binaire n'a aucune dépendance à l'exécution.

### Commandes

```
local-registry <commande> [options]
```

| Commande | Rôle |
| --- | --- |
| `serve` | Lance le serveur. Voir [Configuration](#configuration). |
| `config` | Calcule la configuration exactement comme `serve`, la valide, et affiche chaque réglage avec sa provenance. Ne démarre rien. Voir [Vérifier la configuration](#vérifier-la-configuration). |
| `version` | Affiche la version, la version de Go, la plateforme et, si le binaire a été compilé depuis un dépôt git, le commit. Aussi `local-registry --version` (ou `-v`). |
| `completion bash\|zsh\|fish\|powershell` | Génère le script d'autocomplétion du shell. Ex. : `local-registry completion zsh > "${fpath[1]}/_local-registry"`. |
| `help [commande]` | Affiche l'aide. Aussi `--help` ou `-h` sur toute commande. |

`local-registry` sans commande affiche l'aide générale. L'aide de `serve` liste chaque option, sa variable d'environnement, sa valeur par défaut, un exemple de fichier de configuration et des exemples de lancement :

```sh
local-registry serve --help
```

```
local-registry version
# local-registry v1.2.3 (go1.25.1, darwin/arm64)
```

### Configuration

Chaque réglage peut venir de quatre sources. **La première qui le définit l'emporte** :

1. une option de la ligne de commande (`--port 9000`) ;
2. une variable d'environnement (`LOCAL_REGISTRY_PORT=9000`) ;
3. le fichier de configuration YAML (`port: 9000`) ;
4. la valeur par défaut.

Le nom de la variable d'environnement se déduit de l'option : `LOCAL_REGISTRY_` + nom en majuscules, `-` remplacé par `_`. La clé du fichier est le nom de l'option sans `--`.

| Option | Variable d'environnement | Défaut | Description |
| --- | --- | --- | --- |
| `--config` | `LOCAL_REGISTRY_CONFIG` | — | Fichier de configuration YAML. Facultatif. |
| `--asset-path` | `LOCAL_REGISTRY_ASSET_PATH` | **obligatoire** | Racine de toutes les données ; doit être un dossier existant (un chemin relatif est converti en absolu depuis le dossier courant). Le catalogue est lu dans `<asset-path>/bin.local/assets` ; les routes de fichiers ne servent que `bin.local/assets/` et `assets/` sous cette racine (voir [`/assets/binary`](#get-assetsbinaryfilepath)). |
| `--address` | `LOCAL_REGISTRY_ADDRESS` | `localhost` | Adresse d'écoute. `0.0.0.0` pour écouter sur toutes les interfaces. |
| `--port` | `LOCAL_REGISTRY_PORT` | `8077` | Port d'écoute (0 à 65535 ; `0` = port libre choisi par le système). |
| `--tls-cert` | `LOCAL_REGISTRY_TLS_CERT` | — | Fichier PEM du certificat (chaîne complète : certificat du serveur puis intermédiaires). Active **HTTPS uniquement**. Va avec `--tls-key`. Voir [HTTPS](#https). |
| `--tls-key` | `LOCAL_REGISTRY_TLS_KEY` | — | Fichier PEM de la clé privée du certificat. Va avec `--tls-cert`. |
| `--max-upload-mb` | `LOCAL_REGISTRY_MAX_UPLOAD_MB` | `200` | Taille maximale d'**un** fichier envoyé par `PUT`, en MiB (1 MiB = 1 048 576 octets). `0` ou une valeur négative revient à 200. |
| `--max-version-mb` | `LOCAL_REGISTRY_MAX_VERSION_MB` | `2048` | Taille totale en staging pour **une** version, en MiB. `0` : pas de limite. Voir [Quotas](#quotas-espace-disque-et-purge-du-staging). |
| `--max-version-files` | `LOCAL_REGISTRY_MAX_VERSION_FILES` | `100` | Nombre de fichiers en staging pour une version. `0` : pas de limite. |
| `--max-staging-mb` | `LOCAL_REGISTRY_MAX_STAGING_MB` | `10240` | Taille totale en staging pour **toutes** les versions, en MiB. `0` : pas de limite. |
| `--min-free-mb` | `LOCAL_REGISTRY_MIN_FREE_MB` | `1024` | Espace disque qu'un envoi doit laisser libre, en MiB. `0` : pas de contrôle. |
| `--staging-ttl` | `LOCAL_REGISTRY_STAGING_TTL` | `24h` | Durée d'inactivité après laquelle une version en staging est supprimée. Format Go (`90m`, `12h`, `168h`) ; au moins `1h`. `0` : jamais. |
| `--audit-log` | `LOCAL_REGISTRY_AUDIT_LOG` | `<asset-path>/bin.local/audit.log` | Fichier du [journal d'audit](#journal-daudit). Refusé s'il se trouve dans un arbre public (`bin.local/assets/` ou `assets/`). |
| `--disable-generate-token` | `LOCAL_REGISTRY_DISABLE_GENERATE_TOKEN` | `true` | `false` : générer un token d'upload aléatoire au démarrage et l'afficher sur la sortie standard seulement, si aucun token n'est configuré. Voir [Token généré au démarrage](#token-généré-au-démarrage). |
| *(aucune option)* | `LOCAL_REGISTRY_UPLOAD_TOKEN` | — | Token de l'upload, clé `upload-token` dans le fichier. Voir [Token d'upload](#token-dupload). |

Les options s'écrivent `--port 8077` ou `--port=8077`, avant ou après les autres. La forme à un seul tiret (`-port 8077`) n'est **pas** acceptée.

#### Variables dans les chemins

Les cinq réglages de chemin (`config`, `asset-path`, `tls-cert`, `tls-key`, `audit-log`) sont développés par le programme lui-même, quelle que soit leur source (option, variable d'environnement, fichier de configuration) :

| Écrit | Devient |
| --- | --- |
| `$HOME/.bridge/registry` ou `${HOME}/.bridge/registry` | `/Users/moi/.bridge/registry` |
| `~/.bridge/registry`, `~` | le dossier personnel de l'utilisateur qui lance le serveur |
| `$REGISTRY_ROOT/assets` | la valeur de `REGISTRY_ROOT` |

Cela compte surtout là où aucun shell ne passe : fichier de configuration, unité systemd, conteneur, argument entre guillemets simples (`'$HOME/…'`).

- Une variable **non définie** est une erreur, pas une chaîne vide : `Error: asset-path: environment variable $HOMEE is not set` (code 2). Une faute de frappe ne peut donc pas transformer `$HOMEE/x` en `/x`.
- Une variable définie mais vide est acceptée telle quelle.
- `~user/…` n'est pas développé : `Error: asset-path: ~user/registry: only ~ and ~/ are expanded, not ~user`.
- Le token et les autres réglages ne sont jamais développés (un `$` dans le token reste un `$`).
- `local-registry config` affiche les chemins une fois développés.

#### Fichier de configuration

Un fichier YAML qui reprend les noms des options, plus `upload-token` :

```yaml
# /etc/local-registry/config.yaml  (chmod 600 s'il contient le token)
asset-path: /srv/registry
address: 0.0.0.0
port: 8443
tls-cert: /etc/local-registry/fullchain.pem
tls-key: /etc/local-registry/privkey.pem
max-upload-mb: 1024
max-version-mb: 4096
staging-ttl: 48h
audit-log: /var/log/local-registry/audit.log
upload-token: change-moi
```

```sh
local-registry serve --config /etc/local-registry/config.yaml
# ou
LOCAL_REGISTRY_CONFIG=/etc/local-registry/config.yaml local-registry serve
```

- **Chemins :** les chemins relatifs du fichier sont résolus depuis le **dossier courant** du processus, pas depuis le dossier du fichier. Utilisez des chemins absolus, ou `$HOME/…` et `~/…` (voir [Variables dans les chemins](#variables-dans-les-chemins)).
- **Clés inconnues refusées :** une clé qui n'existe pas (faute de frappe comme `max-verion-mb`, ou clé imbriquée comme `tls: {cert: …}`) arrête le programme. Le fichier n'est donc jamais ignoré en silence.
- **Fichier facultatif :** sans `--config` ni `LOCAL_REGISTRY_CONFIG`, aucun fichier n'est lu, même s'il en existe un dans le dossier courant.
- **Surcharger le fichier :** une variable d'environnement ou une option remplace une valeur du fichier, par exemple `local-registry serve --config prod.yaml --port 9000`.

#### Token d'upload

Le token est lu dans `LOCAL_REGISTRY_UPLOAD_TOKEN` ou dans la clé `upload-token` du fichier de configuration (la variable l'emporte). Il n'existe **aucune option** `--upload-token` : une option de ligne de commande apparaît dans `ps` pour tous les utilisateurs de la machine.

- Absent ou vide : l'upload est désactivé (les routes d'upload répondent 403) et le serveur l'annonce au démarrage : `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled`.
- Dans un fichier lisible par d'autres utilisateurs (mode autre que `600`/`400`), le serveur démarre mais avertit : `warning: /etc/local-registry/config.yaml holds the upload token but is readable by other users (mode -rw-r--r--): chmod 600 /etc/local-registry/config.yaml`.
- Il n'est jamais affiché, ni par `config`, ni dans les logs, ni dans le journal d'audit : seulement son `token_id` (12 premiers caractères hexadécimaux de son sha256).

#### Token généré au démarrage

Pour un usage ponctuel (un test, une démonstration, une CI éphémère), le serveur peut créer lui-même un token :

```sh
local-registry serve --asset-path /srv/registry --disable-generate-token=false
```

```
Upload token: 9f2c4e…(64 caractères hexadécimaux)…
  generated for this run only and saved nowhere: it changes at each start (token_id 4ae3c36ce073)
##################################################
##          Local Registry Server               ##
…
```

| Situation | Comportement |
| --- | --- |
| `--disable-generate-token` absent ou `true` (défaut) | Rien n'est généré. Sans token configuré, l'upload est désactivé. |
| `false`, aucun token configuré | Un token aléatoire de 32 octets (64 caractères hexadécimaux, `crypto/rand`) est créé et affiché sur la **sortie standard**, avant la bannière. Il est valable jusqu'à l'arrêt du serveur ; un redémarrage en crée un autre. |
| `false`, un token configuré (`LOCAL_REGISTRY_UPLOAD_TOKEN` ou `upload-token` non vide) | Aucun token n'est généré : le token configuré reste en service, et un avertissement l'indique sur la sortie d'erreur (voir ci-dessous). |

L'avertissement nomme la source du token configuré, jamais sa valeur :

```
2026/10/09 23:18:54 WARN upload token generation skipped: a token is already configured (env LOCAL_REGISTRY_UPLOAD_TOKEN) and stays in use; unset LOCAL_REGISTRY_UPLOAD_TOKEN and upload-token to get a generated one, or drop --disable-generate-token=false
```

Le token généré n'est écrit **que** sur la sortie standard : ni dans les logs du serveur (sortie d'erreur), ni dans le journal d'audit (qui ne garde que son `token_id`), ni dans aucun fichier. `local-registry config` n'en génère pas : il indique seulement `generated at startup, printed on stdout only`.

Pour le récupérer dans un script :

```sh
local-registry serve --asset-path /srv/registry --disable-generate-token=false > registry.out 2> registry.log &
sleep 1
export LOCAL_REGISTRY_UPLOAD_TOKEN=$(awk '/^Upload token:/ {print $3}' registry.out)
```

> **Attention :** si la sortie standard est elle-même conservée, le token l'est aussi : redirection vers un fichier comme ci-dessus, journal systemd (`journalctl`), logs Docker ou Kubernetes, CI qui archive ses logs. Dans ces cas, préférez un token fixe dans `LOCAL_REGISTRY_UPLOAD_TOKEN` ou dans un fichier de configuration en `chmod 600`.

L'option est un booléen : écrivez `--disable-generate-token=false` avec le signe `=` (`--disable-generate-token false` serait lu comme l'option seule, donc `true`, suivie d'un argument en trop). Dans l'environnement ou le fichier, les valeurs acceptées sont `true`/`false`, `1`/`0`, `t`/`f` (toute casse) ; une autre valeur est refusée : `Error: disable-generate-token: "maybe" is not true or false` (code 2).

#### Vérifier la configuration

`local-registry config` prend les mêmes options que `serve`, applique les mêmes règles et la même validation, puis affiche le résultat sans rien démarrer :

```sh
LOCAL_REGISTRY_MAX_VERSION_FILES=50 local-registry config --config /etc/local-registry/config.yaml --staging-ttl 48h
```

```
SETTING            VALUE                                 SOURCE
config             /etc/local-registry/config.yaml       flag
asset-path         /srv/registry                         config file
address            localhost                             default
port               8443                                  config file
tls-cert           -                                     default
tls-key            -                                     default
max-upload-mb      200                                   default
max-version-mb     2048                                  default
max-version-files  50                                    env LOCAL_REGISTRY_MAX_VERSION_FILES
max-staging-mb     10240                                 default
min-free-mb        1024                                  default
staging-ttl        48h0m0s                               flag
audit-log          /srv/registry/bin.local/audit.log     default
disable-generate-token  true                            default
upload-token       set (token_id 1ec1c26b50d5)           config file
```

| Colonne | Contenu |
| --- | --- |
| `VALUE` | Valeur effective, après conversion (chemins absolus, audit-log par défaut calculé). `-` = vide. |
| `SOURCE` | `flag`, `env <VARIABLE>`, `config file` ou `default`. |

La ligne `upload-token` vaut `not set: upload disabled`, `set (token_id …)`, `generated at startup, printed on stdout only` (avec `--disable-generate-token=false` et aucun token), ou `set (token_id …); generation skipped` (avec `--disable-generate-token=false` et un token configuré).

C'est la façon la plus sûre de vérifier une configuration de production avant de redémarrer le service : une erreur sort avec le code `2` et le même message que `serve`.

#### Erreurs de configuration

Toute erreur arrête le programme avant le démarrage, avec le code de sortie `2`, un message `Error: …` et le rappel `Run 'local-registry serve --help' for usage.` :

| Message | Cause |
| --- | --- |
| `asset-path is required (--asset-path, LOCAL_REGISTRY_ASSET_PATH or asset-path in the config file)` | Réglage absent. Le serveur ne prend pas le dossier courant par défaut, pour ne pas exposer par erreur ce qu'il contient. |
| `asset-path /chemin is not an existing directory` | Dossier absent, ou chemin vers un fichier. |
| `asset-path: environment variable $X is not set` | Le chemin utilise une variable qui n'existe pas dans l'environnement du serveur (idem pour `config`, `tls-cert`, `tls-key`, `audit-log`). |
| `asset-path: ~user/…: only ~ and ~/ are expanded, not ~user` | `~user` n'est pas pris en charge : écrivez le chemin complet. |
| `port 70000 is out of range 0-65535` | Port invalide. |
| `tls-cert and tls-key go together` | Un seul des deux est donné. |
| `max-version-mb cannot be negative` (idem pour les autres quotas) | Valeur négative. |
| `staging-ttl must be 0 or at least 1h0m0s` | Durée trop courte : la purge pourrait supprimer un envoi encore en cours (un envoi peut durer 30 min). |
| `audit-log … would be downloadable: put it outside bin.local/assets and assets` | Le journal serait téléchargeable par la route de fichiers. |
| `port: "abc" is not a whole number` | Variable d'environnement ou valeur du fichier non numérique. |
| `staging-ttl: "demain" is not a duration such as 90m or 24h` | Durée illisible. |
| `invalid argument "abc" for "--port" flag: …` | Option non numérique. |
| `config file /chemin: …` | Fichier introuvable, illisible, ou YAML invalide. |
| `config file /chemin: unknown setting "max-verion-mb"` | Clé inconnue dans le fichier. |
| `unknown flag: --nope` | Option inconnue (y compris `--upload-token`). |
| `unexpected argument "x" for "local-registry serve"` | Argument en trop. |
| `unknown command "serv" for "local-registry"` | Commande inconnue. |

### HTTPS

Avec `--tls-cert` et `--tls-key`, le serveur ne parle **que** HTTPS (TLS 1.2 minimum, HTTP/2 activé). Une requête en HTTP simple sur ce port reçoit `400 Client sent an HTTP request to an HTTPS server.`

Le certificat et la clé sont chargés au démarrage : un fichier absent, illisible, ou une clé qui ne correspond pas au certificat arrête le programme avec le code `1` (`Error: loading TLS certificate: …`). Pour changer de certificat, remplacez les fichiers puis redémarrez.

```sh
# Certificat d'un vrai domaine (ex. Let's Encrypt)
./local-registry serve --asset-path /srv/registry --address 0.0.0.0 --port 8443 \
  --tls-cert /etc/letsencrypt/live/registry.example.com/fullchain.pem \
  --tls-key  /etc/letsencrypt/live/registry.example.com/privkey.pem

# Certificat auto-signé pour un usage local ou de test
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 365 \
  -keyout key.pem -out cert.pem -subj /CN=localhost \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
./local-registry serve --asset-path /srv/registry --tls-cert cert.pem --tls-key key.pem

# Côté client, faire confiance à ce certificat auto-signé
curl --cacert cert.pem https://localhost:8077/gitlab/artifacts
```

Le script de publication fonctionne tel quel en HTTPS : donnez une adresse `https://…` dans `LOCAL_REGISTRY_URL`. Pour un certificat auto-signé, ajoutez-le aux autorités de confiance du système ou exportez `CURL_CA_BUNDLE=/chemin/cert.pem` avant de lancer le script.

Sans `--tls-cert`, si l'upload est activé et que `--address` n'est pas une adresse locale (`localhost`, `127.0.0.1`, `::1`), le serveur avertit au démarrage, sur la sortie d'erreur :

```
warning: upload is enabled on 0.0.0.0 without TLS: the token travels in clear text (use --tls-cert/--tls-key or a TLS reverse proxy)
```

Le serveur démarre quand même : l'avertissement laisse la place à un reverse proxy qui fait le TLS devant lui.

### Exemples de lancement

```sh
# Lecture seule, en local
./local-registry serve --asset-path /srv/registry

# Accessible depuis le réseau, en HTTPS, upload activé, fichiers jusqu'à 1 GiB
LOCAL_REGISTRY_UPLOAD_TOKEN=change-moi \
  ./local-registry serve --asset-path /srv/registry --address 0.0.0.0 --port 9000 --max-upload-mb 1024 \
  --tls-cert fullchain.pem --tls-key privkey.pem

# Tout depuis l'environnement (conteneur, systemd)
export LOCAL_REGISTRY_ASSET_PATH=/srv/registry LOCAL_REGISTRY_ADDRESS=0.0.0.0 LOCAL_REGISTRY_UPLOAD_TOKEN=change-moi
./local-registry serve

# Fichier de configuration, avec une surcharge ponctuelle
./local-registry serve --config /etc/local-registry/config.yaml --port 9000
```

Exemple d'unité systemd :

```ini
# /etc/systemd/system/local-registry.service
[Unit]
Description=local-registry
After=network-online.target

[Service]
ExecStart=/usr/local/bin/local-registry serve --config /etc/local-registry/config.yaml
User=registry
Restart=on-failure
# SIGTERM par défaut : arrêt propre, 10 s pour finir les requêtes
TimeoutStopSec=15

[Install]
WantedBy=multi-user.target
```

### Ce qu'affiche le serveur

Au démarrage, une fois le port réservé, sur la sortie standard :

```
##################################################
##          Local Registry Server               ##
##################################################

Listen      : http://127.0.0.1:8077
Start At    : 2026-10-09 18:22:21.091998 +0200 CEST m=+0.003639167
Assets Path : /srv/registry
```

`Listen` affiche le protocole réellement servi (`http` ou `https`) et l'adresse réellement liée (ici `localhost` résolu en `127.0.0.1` ; `0.0.0.0` s'affiche `[::]`).

Puis une ligne de log par requête, sur la sortie d'erreur :

```
2026/10/09 18:22:21 INFO request method=GET path=/gitlab/artifacts status=200 bytes=15 duration=398.167µs remote=127.0.0.1
```

| Champ | Contenu |
| --- | --- |
| `method`, `path` | Méthode et chemin décodé (`%2F` apparaît comme `/`). |
| `status` | Code HTTP envoyé. |
| `bytes` | Taille du corps de réponse. |
| `duration` | Temps de traitement. |
| `remote` | Adresse IP de la connexion TCP (celle du reverse proxy s'il y en a un). |
| `forwarded_for` | Valeur de l'en-tête `X-Forwarded-For`, seulement s'il est présent. Recopiée telle quelle : un client peut l'inventer, elle n'est fiable que si un reverse proxy de confiance la remplace. |

Une requête en erreur produit en plus une ligne `WARN` (codes 4xx) ou `ERROR` (codes 5xx) avec la cause exacte, qui peut contenir des chemins disque. Ces détails ne sont jamais envoyés au client :

```
2026/10/09 18:42:13 WARN request failed method=PUT path=/gitlab/artifacts/v0.1.0/files/tool.tar.gz status=401 error="invalid or missing bearer token"
```

### Arrêt et codes de sortie

| Situation | Comportement | Code de sortie |
| --- | --- | --- |
| `Ctrl+C` (`SIGINT`) ou `SIGTERM` | Log `INFO shutting down`, plus aucune nouvelle connexion ; les requêtes en cours ont **10 secondes** pour se terminer. | `0` |
| `help`, `version`, `config` réussis | Affichage, rien ne démarre. | `0` |
| Les requêtes en cours dépassent 10 s à l'arrêt | Arrêt forcé : `Error: context deadline exceeded`. | `1` |
| Port déjà occupé, adresse invalide | Échec immédiat, avant la bannière : `Error: listen tcp 127.0.0.1:8077: bind: address already in use`. | `1` |
| Certificat ou clé TLS illisible ou incohérent | Échec immédiat : `Error: loading TLS certificate: …`. | `1` |
| Journal d'audit impossible à ouvrir (droits, dossier) | Échec immédiat : `Error: audit log: …`. | `1` |
| Erreur de commande ou de configuration (voir [Erreurs de configuration](#erreurs-de-configuration)) | Message d'erreur et rappel de l'aide, rien ne démarre. | `2` |

### Délais

| Délai | Valeur | Portée |
| --- | --- | --- |
| Lecture des en-têtes de requête | 5 s | toutes les requêtes |
| Écriture de la réponse | 20 s | toutes les routes sauf l'upload |
| Lecture et écriture | 30 min | `PUT .../files/{name}` et `POST .../publish` |
| Connexion inactive (keep-alive) | 60 s | toutes les connexions |

---

## Arborescence des fichiers

Tout vit sous `--asset-path` :

```
<asset-path>/
├── bin.local/
│   ├── assets/                          ← parcouru par GET /artifacts/all
│   │   └── binaries/                    ← public : servi par /assets/binary
│   │       └── <vcs>/                   ← ex. gitlab, github
│   │           ├── artifacts.json       ← version "latest" de <vcs>
│   │           └── <version>/           ← une version publiée, immuable
│   │               ├── artifacts.json
│   │               ├── metadata.json
│   │               ├── <projet>_<version>_checksums.txt
│   │               └── <projet>_<version>_<os>_<arch>.tar.gz …
│   ├── .uploads/<vcs>/<version>/        ← privé : staging, fichiers envoyés pas encore publiés
│   ├── .uploads-tmp/                    ← privé : fichiers en cours de réception
│   └── audit.log                        ← privé : journal d'audit (mode 0600)
├── assets/
│   └── binaries/gitlab/<version>/       ← public ; lu aussi par POST /releases/{version}
└── (tout autre fichier)                 ← privé : jamais servi
```

- `<vcs>` est un nom libre qui regroupe les versions d'un même produit (`gitlab`, `github`…).
- Le catalogue ne voit que `bin.local/assets/` : une version en staging n'y apparaît pas.
- `POST /releases/{version}` lit un autre arbre, `assets/binaries/gitlab/`, hors de `bin.local`. L'upload n'écrit jamais dans cet arbre.
- Vous pouvez aussi remplir `bin.local/assets/` à la main (copie de fichiers) : le serveur lit le disque à chaque requête, sans cache ni redémarrage.

---

## Conventions communes à toutes les routes

### Adresse de base

Les exemples utilisent `http://localhost:8077`. Les chemins sont sensibles à la casse.

### Noms autorisés

Partout où une route reçoit `{vcs}`, `{version}` ou `{name}`, la valeur doit :

- ne contenir que `A-Z`, `a-z`, `0-9`, `.`, `_` et `-` ;
- ne pas valoir `.` ni `..` ;
- ne pas être vide.

Une valeur refusée donne **404** sur les routes de lecture et **400** sur les routes d'upload. Sur les routes d'upload, `latest` est en plus interdit comme `{version}` (il est réservé à la lecture de la dernière version).

| Valeur | Acceptée ? |
| --- | --- |
| `v0.1.0`, `1.2.3-rc.1`, `tool_0.1.0_linux_amd64.tar.gz` | oui |
| `latest` | en lecture oui (= dernière version), en upload non |
| `..`, `a/b`, `a b`, `v1+build` | non |

### Encodage des chemins

Une variable de route occupe **un seul segment** d'URL. Pour désigner un fichier dans un sous-dossier (routes `/assets/...`), encodez `/` en `%2F` :

```sh
curl -O "http://localhost:8077/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Fv0.1.0%2Ftool_0.1.0_checksums.txt"
```

Un `..` écrit en clair dans l'URL (`/gitlab/artifacts/..`) est normalisé par le serveur, qui répond par une redirection `301` vers le chemin nettoyé ; la requête n'atteint jamais la route.

### Méthodes

- Chaque route `GET` répond aussi à `HEAD` (mêmes en-têtes, sans corps).
- Une méthode non prévue sur un chemin existant donne `405 Method Not Allowed` avec l'en-tête `Allow` (ex. `Allow: GET, HEAD`), corps texte.
- Un chemin inconnu donne `404` avec le corps texte `404 page not found`.

### Format des réponses

Les routes JSON répondent avec `Content-Type: application/json` et un corps terminé par un saut de ligne.

En cas d'erreur, le corps est toujours :

```json
{"error":"<message>"}
```

| Code | Signification | Message |
| --- | --- | --- |
| `400` | Requête invalide (nom, paramètre) | précis, ex. `invalid vcs or version name` |
| `401` | Token absent ou faux (+ en-tête `WWW-Authenticate: Bearer`) | `invalid or missing bearer token` |
| `403` | Upload désactivé | `upload is disabled: no token configured` |
| `404` | Ressource absente ou nom refusé en lecture | `not found`, ou précis pour l'upload |
| `409` | Version déjà publiée | `version gitlab/v0.1.0 is already published and cannot be changed` |
| `413` | Fichier trop gros, ou quota de la version dépassé | `file exceeds 209715200 bytes` |
| `422` | Contenu refusé (checksums, JSON, version) | précis, voir chaque route |
| `500` | Erreur interne (disque, JSON corrompu sur le disque) | `internal error` ; la cause est dans les logs |
| `507` | Stockage insuffisant : staging plein ou disque presque plein | `not enough free disk space on the registry` |

Exceptions : les routes de fichiers (`/assets/binary/`, `/assets/checksum/`) et les erreurs de routage (`404`, `405`, `301`) répondent en texte brut.

---

## Référence de l'API

### Vue d'ensemble

| Méthode | Route | Auth | Rôle |
| --- | --- | --- | --- |
| `GET` | [`/{vcs}/artifacts`](#get-vcsartifacts) | non | `artifacts.json` de la dernière version |
| `GET` | [`/{vcs}/artifacts/{version}`](#get-vcsartifactsversion) | non | `artifacts.json` d'une version |
| `GET` | [`/artifacts/all`](#get-artifactsall) | non | Tout le catalogue, fichiers bruts |
| `GET` | [`/artifacts/metadatas`](#get-artifactsmetadatas) | non | Non implémentée (404) |
| `GET` | [`/assets/binary/{filePath}`](#get-assetsbinaryfilepath) | non | Télécharger un fichier |
| `GET` | [`/assets/checksum/{filename}`](#get-assetschecksumfilename) | non | Télécharger un fichier (identique) |
| `POST` | [`/releases/{version}`](#post-releasesversion) | non | Lister une release de l'ancien arbre |
| `PUT` | [`/{vcs}/artifacts/{version}/files/{name}`](#put-vcsartifactsversionfilesname) | **oui** | Envoyer un fichier en staging |
| `POST` | [`/{vcs}/artifacts/{version}/publish`](#post-vcsartifactsversionpublish) | **oui** | Vérifier et publier une version |

---

### `GET /{vcs}/artifacts`

Renvoie le contenu de `bin.local/assets/binaries/<vcs>/artifacts.json`, c'est-à-dire la version marquée « latest » de `<vcs>`.

**Paramètres**

| Nom | Où | Description |
| --- | --- | --- |
| `vcs` | chemin | Nom du groupe de versions (voir [noms autorisés](#noms-autorisés)). |

**Exemple**

```sh
curl -s http://localhost:8077/gitlab/artifacts
```

```json
[{"name":"metadata.json","path":"dist/metadata.json","internal_type":36,"type":"Metadata"},{"name":"tool_0.1.0_linux_amd64.tar.gz","path":"dist/tool_0.1.0_linux_amd64.tar.gz","goos":"linux","goarch":"amd64","goamd64":"v1","internal_type":1,"type":"Archive","extra":{"Format":"tar.gz","ID":"default"}}]
```

Le fichier est relu et renvoyé tel quel, quel que soit son contenu JSON (tableau, objet…) ; la mise en forme peut différer de l'original (espaces), pas le contenu. Avec le flux d'upload, c'est le `dist/artifacts.json` de goreleaser (exemple ci-dessus, abrégé).

**Erreurs**

| Code | Cause |
| --- | --- |
| `404` | `vcs` refusé, ou `artifacts.json` absent (aucune version publiée avec `?latest=true`). |
| `500` | `artifacts.json` n'est pas du JSON valide. |

---

### `GET /{vcs}/artifacts/{version}`

Renvoie le contenu de `bin.local/assets/binaries/<vcs>/<version>/artifacts.json`.

**Paramètres**

| Nom | Où | Description |
| --- | --- | --- |
| `vcs` | chemin | Groupe de versions. |
| `version` | chemin | Version exacte (`v0.1.0`), ou `latest` : la route se comporte alors exactement comme [`GET /{vcs}/artifacts`](#get-vcsartifacts). |

**Exemples**

```sh
curl -s http://localhost:8077/gitlab/artifacts/v0.1.0
curl -s http://localhost:8077/gitlab/artifacts/latest     # = /gitlab/artifacts
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8077/gitlab/artifacts/v9.9.9   # 404
```

**Erreurs**

| Code | Cause |
| --- | --- |
| `404` | `vcs` ou `version` refusé, version non publiée (y compris en staging), `artifacts.json` absent. |
| `500` | `artifacts.json` n'est pas du JSON valide. |

---

### `GET /artifacts/all`

Parcourt tout `bin.local/assets/` et renvoie, pour chaque dossier qui contient au moins un fichier reconnu, le contenu **brut** (en chaînes de caractères) de ces fichiers.

**Fichiers reconnus dans un dossier**

| Champ de la réponse | Fichier |
| --- | --- |
| `artifacts_json` | `artifacts.json` |
| `metadata_json` | `metadata.json` |
| `checksums_text` | un fichier dont le nom finit par `_checksums.txt` (sans tenir compte de la casse) ; `…_checksums.txt.sig` est ignoré |
| `provider` | nom du dossier qui contient ces fichiers |

**Exemple**

Avec l'arbre :

```
bin.local/assets/binaries/gitlab/artifacts.json
bin.local/assets/binaries/gitlab/v0.1.0/{artifacts.json, metadata.json, tool_0.1.0_checksums.txt, *.tar.gz}
```

```sh
curl -s http://localhost:8077/artifacts/all
```

```json
[
  {
    "provider": "gitlab",
    "artifacts_json": "[{\"name\":\"tool_0.1.0_linux_amd64.tar.gz\"}]\n",
    "metadata_json": "",
    "checksums_text": ""
  },
  {
    "provider": "v0.1.0",
    "artifacts_json": "[{\"name\":\"tool_0.1.0_linux_amd64.tar.gz\"}]\n",
    "metadata_json": "{\"project_name\":\"tool\",\"tag\":\"v0.1.0\",\"version\":\"0.1.0\"}\n",
    "checksums_text": "8f6c80d2…  tool_0.1.0_linux_amd64.tar.gz\n"
  }
]
```

(réponse mise en forme pour la lecture ; le serveur la renvoie sur une ligne)

**À savoir**

- Les entrées sont triées par chemin de dossier : l'ordre est stable d'un appel à l'autre.
- `provider` est le nom du dossier parent des fichiers : `gitlab` pour la version latest, mais `v0.1.0` (et non `gitlab`) pour une version. Pour retrouver le `vcs` d'une version, lisez `metadata_json` ou interrogez [`/{vcs}/artifacts/{version}`](#get-vcsartifactsversion).
- Un fichier absent ou illisible donne une chaîne vide `""`.
- Les contenus sont des chaînes : il faut décoder `artifacts_json` et `metadata_json` une seconde fois côté client.
- Si `bin.local/assets/` existe mais ne contient aucun fichier reconnu, la réponse est `null` (et non `[]`).
- Le staging (`bin.local/.uploads/`) n'est pas parcouru.

**Erreurs**

| Code | Cause |
| --- | --- |
| `404` | `bin.local/assets/` n'existe pas. |
| `500` | Erreur de lecture du disque pendant le parcours. |

---

### `GET /artifacts/metadatas`

Route réservée, **non implémentée** : elle répond toujours `404 {"error":"not found"}`. Ce qu'elle doit renvoyer reste à définir.

```sh
curl -s http://localhost:8077/artifacts/metadatas
# {"error":"not found"}
```

---

### `GET /assets/binary/{filePath}`

Télécharge un fichier **public** situé sous `--asset-path`.

**Fichiers publics**

Seuls ces fichiers sont servis ; tout le reste répond 404, comme un fichier absent :

| Servi | Refusé |
| --- | --- |
| `bin.local/assets/…` : versions publiées, `artifacts.json` latest | le staging `bin.local/.uploads/…` et `bin.local/.uploads-tmp/…` |
| `assets/…` : l'ancien arbre lu par `/releases` | tout autre fichier sous `--asset-path` (sources, configuration…) |
| | tout chemin dont un élément commence par `.` (`.git`, `.env`, fichiers temporaires `.tmp-*`) |
| | un lien symbolique qui mène hors de `--asset-path` |

Un lien symbolique qui reste à l'intérieur de `--asset-path` fonctionne (par exemple `bin.local/assets/binaries/gitlab/current → v0.1.0`). Le contrôle des liens est fait par le système (`os.Root`) au moment de l'ouverture : un lien ne peut pas faire sortir de la racine, même s'il est créé après le démarrage.

**Paramètres**

| Nom | Où | Description |
| --- | --- | --- |
| `filePath` | chemin | Chemin du fichier **relatif à `--asset-path`**, en un seul segment : chaque `/` encodé en `%2F`. Il commence donc par `bin.local%2Fassets%2F` ou `assets%2F`. Un `/` initial est ignoré. |
| `Range` | en-tête, optionnel | Plage d'octets (`bytes=0-1023`) : réponse `206 Partial Content`. |
| `If-Modified-Since` | en-tête, optionnel | Réponse `304 Not Modified` si le fichier n'a pas changé. |

**Exemples**

```sh
# Télécharger une archive publiée, sous son nom d'origine (-J utilise Content-Disposition)
curl -fsS -OJ "http://localhost:8077/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Fv0.1.0%2Ftool_0.1.0_linux_amd64.tar.gz"

# Reprendre un téléchargement interrompu
curl -fsS -C - -o tool.tar.gz "http://localhost:8077/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Fv0.1.0%2Ftool_0.1.0_linux_amd64.tar.gz"

# Lire seulement les en-têtes (taille, date)
curl -sI "http://localhost:8077/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Fv0.1.0%2Ftool_0.1.0_linux_amd64.tar.gz"
```

**Réponse**

```
HTTP/1.1 200 OK
Accept-Ranges: bytes
Content-Disposition: attachment; filename=tool_0.1.0_linux_amd64.tar.gz
Content-Length: 1843202
Content-Type: application/octet-stream
Last-Modified: Fri, 09 Oct 2026 16:42:13 GMT
```

- Le fichier est envoyé en flux, sans être chargé en mémoire.
- `Content-Type` vaut toujours `application/octet-stream`.
- `filename` est le nom du fichier sans son dossier ; il est mis entre guillemets s'il contient des espaces ou des caractères spéciaux (`filename="my tool.bin"`).

**Encodage du nom** : `filePath` est décodé **une seule fois**, selon les règles standard des chemins d'URL. Encodez-le avec la fonction d'encodage de chemin de votre langage (`encodeURIComponent` en JavaScript, `url.PathEscape` en Go, `urllib.parse.quote(nom, safe="")` en Python) :

| Caractère dans le nom du fichier | À écrire dans l'URL |
| --- | --- |
| `/` (sous-dossier) | `%2F` |
| espace | `%20` |
| `+` | `+` ou `%2B` |
| `%` | `%25` |

Un nom encodé deux fois (`%252F`) n'est **pas** décodé une seconde fois : il désigne un fichier dont le nom contient littéralement `%2F`, en général absent (404).

**Erreurs** (corps texte `404 page not found`)

| Code | Cause |
| --- | --- |
| `404` | Fichier absent ; fichier non public (voir ci-dessus) ; chemin qui sort de `--asset-path` (`..%2F`, chemin absolu, lien symbolique) ; nom encodé deux fois (`%252F`) ; chemin qui désigne un dossier ; chemin non normalisé (`a%2F%2Fb`). |

Exemples refusés :

```sh
curl -s "http://localhost:8077/assets/binary/bin.local%2F.uploads%2Fgitlab%2Fv0.2.0%2Ftool.tar.gz"   # staging : 404
curl -s "http://localhost:8077/assets/binary/main.go"                                                # hors des arbres publics : 404
curl -s "http://localhost:8077/assets/binary/bin.local%2Fassets%2F.git%2Fconfig"                      # élément caché : 404
```

---

### `GET /assets/checksum/{filename}`

Strictement identique à [`GET /assets/binary/{filePath}`](#get-assetsbinaryfilepath) : même racine (`--asset-path`), mêmes règles, mêmes en-têtes, mêmes erreurs. Elle existe pour séparer les téléchargements de checksums dans les logs et les outils clients.

```sh
curl -fsS "http://localhost:8077/assets/checksum/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Fv0.1.0%2Ftool_0.1.0_checksums.txt"
```

```
8f6c80d2808e7cedf31d8215cf3c9a012c76fc6ae40ee250d0b6b85f7f54a5b2  tool_0.1.0_linux_amd64.tar.gz
…
```

---

### `POST /releases/{version}`

Liste les fichiers de `<asset-path>/assets/binaries/gitlab/<version>/` (l'ancien arbre, **pas** `bin.local`). La route ne modifie rien malgré la méthode `POST` et n'a pas de corps de requête.

**Paramètres**

| Nom | Où | Description |
| --- | --- | --- |
| `version` | chemin | Nom du dossier de version (voir [noms autorisés](#noms-autorisés)). |

**Exemple**

```sh
curl -s -X POST http://localhost:8077/releases/v1
```

```json
{
  "tag_name": "v1",
  "assets": [
    {
      "id": "e2fa445c-f2c7-4254-b46b-cff0d4d9af39",
      "name": "tool",
      "url": "/assets/binary/assets%2Fbinaries%2Fgitlab%2Fv1%2Ftool",
      "is_checksum": false
    },
    {
      "id": "50417fba-3f8e-46ae-870b-8e1373b9be26",
      "name": "tool.checksum.txt",
      "url": "/assets/binary/assets%2Fbinaries%2Fgitlab%2Fv1%2Ftool.checksum.txt",
      "is_checksum": true
    }
  ]
}
```

| Champ | Contenu |
| --- | --- |
| `tag_name` | La `version` demandée. |
| `assets` | Un élément par fichier du dossier (les sous-dossiers sont ignorés), trié par nom. `null` si le dossier est vide. |
| `assets[].id` | UUID **aléatoire, différent à chaque appel** : ne pas l'utiliser comme identifiant stable. |
| `assets[].name` | Nom du fichier. |
| `assets[].url` | Chemin HTTP qui **télécharge** ce fichier via [`GET /assets/binary/{filePath}`](#get-assetsbinaryfilepath), relatif à l'adresse du registry (pas de schéma ni d'hôte, pour fonctionner derrière un reverse proxy). Le nom est encodé (`%20` pour une espace…). Il ne révèle aucun chemin du disque du serveur. |
| `assets[].is_checksum` | `true` si le nom contient `.checksum.txt` (les fichiers goreleaser `_checksums.txt` donnent donc `false`). |

Télécharger tous les fichiers d'une release :

```sh
BASE=http://localhost:8077
curl -s -X POST $BASE/releases/v1 | jq -r '.assets[] | "\(.name) \(.url)"' |
  while read -r name url; do curl -fsS -o "$name" "$BASE$url"; done
```

Le dossier `gitlab` en dur, `is_checksum` et `id` sont conservés tels quels pour ne pas casser les clients existants.

**Erreurs**

| Code | Cause |
| --- | --- |
| `404` | `version` refusée ou dossier absent. |
| `405` | Requête en `GET` : la route n'accepte que `POST`. |

---

### Authentification de l'upload

Les deux routes d'upload exigent :

```
Authorization: Bearer <valeur de LOCAL_REGISTRY_UPLOAD_TOKEN>
```

| Situation | Réponse |
| --- | --- |
| Le serveur n'a pas de token | `403 {"error":"upload is disabled: no token configured"}`, quel que soit l'en-tête envoyé. |
| En-tête absent, schéma autre que `Bearer ` (ex. `Basic`), token faux | `401 {"error":"invalid or missing bearer token"}` + `WWW-Authenticate: Bearer`. |

La comparaison se fait à temps constant. L'authentification est vérifiée **avant** tout le reste : un client non authentifié n'apprend rien sur l'existence d'une version. Les routes de lecture ne demandent aucune authentification.

Pour ne pas exposer le token dans la liste des processus, passez-le à curl par l'entrée standard plutôt qu'en argument :

```sh
printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" | curl -H @- …
```

---

### `PUT /{vcs}/artifacts/{version}/files/{name}`

Envoie **un** fichier dans la zone de staging de la version `<vcs>/<version>`. Le fichier n'est visible nulle part dans le catalogue tant que la version n'est pas [publiée](#post-vcsartifactsversionpublish).

**Paramètres**

| Nom | Où | Obligatoire | Description |
| --- | --- | --- | --- |
| `vcs` | chemin | oui | Groupe de versions. |
| `version` | chemin | oui | Version à créer. `latest` est interdit. |
| `name` | chemin | oui | Nom sous lequel le fichier sera publié (voir [noms autorisés](#noms-autorisés)). |
| `Authorization` | en-tête | oui | `Bearer <token>`. |
| `X-Checksum-Sha256` | en-tête | non | sha256 attendu, en hexadécimal (majuscules ou minuscules). Si présent et différent du contenu reçu : 422 et rien n'est conservé. |
| corps | — | oui | Le contenu **brut** du fichier. `Content-Type` est ignoré. |

> N'utilisez pas `curl -F` (multipart) : le serveur enregistrerait l'enveloppe multipart avec le fichier. Utilisez `-T fichier` ou `--data-binary @fichier -X PUT`.

**Exemples**

```sh
TOKEN_HDR="Authorization: Bearer $LOCAL_REGISTRY_UPLOAD_TOKEN"

# Envoi simple
curl -fsS -H "$TOKEN_HDR" -T dist/tool_0.1.0_linux_amd64.tar.gz \
  http://localhost:8077/gitlab/artifacts/v0.1.0/files/tool_0.1.0_linux_amd64.tar.gz

# Avec vérification d'intégrité de bout en bout
SUM=$(shasum -a 256 dist/tool_0.1.0_linux_amd64.tar.gz | cut -d' ' -f1)
curl -fsS -H "$TOKEN_HDR" -H "X-Checksum-Sha256: $SUM" -T dist/tool_0.1.0_linux_amd64.tar.gz \
  http://localhost:8077/gitlab/artifacts/v0.1.0/files/tool_0.1.0_linux_amd64.tar.gz

# Depuis l'entrée standard (taille inconnue à l'avance : envoi en chunked)
tar czf - ./build | curl -fsS -H "$TOKEN_HDR" -T - \
  http://localhost:8077/gitlab/artifacts/v0.1.0/files/build.tar.gz
```

**Réponse `201 Created`**

```json
{"name":"tool_0.1.0_linux_amd64.tar.gz","size":1843202,"sha256":"8f6c80d2808e7cedf31d8215cf3c9a012c76fc6ae40ee250d0b6b85f7f54a5b2"}
```

| Champ | Contenu |
| --- | --- |
| `name` | Nom du fichier en staging. |
| `size` | Octets reçus. |
| `sha256` | sha256 calculé par le serveur pendant la réception. |

**Comportement**

- Le corps est lu en flux et écrit sur disque au fur et à mesure ; il n'est jamais chargé en mémoire.
- Le fichier arrive d'abord dans `bin.local/.uploads-tmp/`, puis n'est déplacé dans le staging (`bin.local/.uploads/<vcs>/<version>/<name>`) qu'une fois complet et vérifié. Un envoi interrompu ou refusé ne laisse aucun fichier.
- Renvoyer le même `name` avant publication **remplace** le fichier en staging (le dernier envoi gagne). C'est ainsi qu'on corrige un fichier refusé par la publication.
- Les envois de fichiers différents peuvent se faire en parallèle.
- Le délai de la requête est de 30 minutes.
- Les [quotas](#quotas-espace-disque-et-purge-du-staging) sont vérifiés avant la réception, puis une seconde fois, sous verrou, juste avant l'entrée en staging : deux envois parallèles ne peuvent pas dépasser ensemble un quota.
- Chaque envoi, accepté ou refusé, est inscrit au [journal d'audit](#journal-daudit).

**Erreurs** (dans l'ordre où elles sont vérifiées)

| Code | Message | Cause |
| --- | --- | --- |
| `403` | `upload is disabled: no token configured` | Serveur lancé sans `LOCAL_REGISTRY_UPLOAD_TOKEN`. |
| `401` | `invalid or missing bearer token` | Token absent ou faux. |
| `400` | `invalid vcs or version name` | `vcs` ou `version` refusé, ou `version` vaut `latest`. |
| `400` | `invalid file name: use [A-Za-z0-9._-] only` | `name` refusé. |
| `409` | `version gitlab/v0.1.0 is already published and cannot be changed` | La version est déjà publiée. |
| `422` | `version gitlab/v0.1.0 already has 100 staged files (limit 100)` | `--max-version-files` atteint. Remplacer un fichier déjà en staging reste possible. |
| `413` | `file exceeds 209715200 bytes` | Le fichier dépasse `--max-upload-mb`. |
| `413` | `version gitlab/v0.1.0 would exceed its staging quota of 2147483648 bytes` | Avec ce fichier, la version dépasserait `--max-version-mb`. |
| `507` | `upload staging area is full (limit 10737418240 bytes): publish or abandon pending versions` | Avec ce fichier, le staging de toutes les versions dépasserait `--max-staging-mb`. |
| `507` | `not enough free disk space on the registry` | L'envoi laisserait moins de `--min-free-mb` libres sur le disque. |
| `422` | `sha256 mismatch: got <reçu>, X-Checksum-Sha256 says <attendu>` | L'en-tête ne correspond pas au contenu reçu. |
| `500` | `internal error` | Erreur disque, ou connexion coupée par le client pendant l'envoi. |

---

### `POST /{vcs}/artifacts/{version}/publish`

Vérifie les fichiers en staging de `<vcs>/<version>` comme une release goreleaser complète, puis les rend visibles **d'un seul coup**.

**Paramètres**

| Nom | Où | Obligatoire | Description |
| --- | --- | --- | --- |
| `vcs` | chemin | oui | Groupe de versions. |
| `version` | chemin | oui | Version à publier. `latest` est interdit. |
| `latest` | requête | non | `true` : cette version devient aussi la version « latest » de `<vcs>`. Valeurs acceptées : `true`, `True`, `TRUE`, `t`, `T`, `1` et `false`, `False`, `FALSE`, `f`, `F`, `0`. Absent ou vide = `false`. |
| `Authorization` | en-tête | oui | `Bearer <token>`. |

Pas de corps de requête.

**Exemples**

```sh
TOKEN_HDR="Authorization: Bearer $LOCAL_REGISTRY_UPLOAD_TOKEN"

# Publier et marquer comme dernière version
curl -fsS -X POST -H "$TOKEN_HDR" "http://localhost:8077/gitlab/artifacts/v0.1.0/publish?latest=true"

# Publier une version corrective d'une ancienne branche, sans toucher à latest
curl -fsS -X POST -H "$TOKEN_HDR" "http://localhost:8077/gitlab/artifacts/v0.0.9/publish"
```

**Réponse `200 OK`**

```json
{"vcs":"gitlab","version":"v0.1.0","files":["artifacts.json","metadata.json","tool_0.1.0_checksums.txt","tool_0.1.0_darwin_amd64.tar.gz","tool_0.1.0_linux_amd64.tar.gz"],"latest":true}
```

| Champ | Contenu |
| --- | --- |
| `vcs`, `version` | La version publiée. |
| `files` | Fichiers publiés, triés par nom. |
| `latest` | `true` si `<vcs>/artifacts.json` a été remplacé. |

**Contrôles effectués avant publication**

Tous doivent passer, sinon rien n'est publié et le staging reste en place :

1. **Un et un seul fichier de checksums** : un fichier dont le nom, en minuscules, finit par `checksums.txt` (`tool_0.1.0_checksums.txt`, `checksums.txt`, `SHA_CHECKSUMS.TXT`…). Un `….txt.sig` ne compte pas.
2. **`artifacts.json` et `metadata.json` présents.**
3. **Fichier de checksums lisible** : chaque ligne non vide est exactement `<sha256 sur 64 caractères> <espaces> <nom>`, le format de goreleaser et de `sha256sum` (un `*` devant le nom, mode binaire de `sha256sum`, est accepté).
4. **Chaque fichier envoyé est listé et intact** : son nom figure dans le fichier de checksums et son sha256 correspond. Sont dispensés : le fichier de checksums lui-même, `artifacts.json`, `metadata.json`, et les fichiers finissant par `.sig` ou `.pem` (signatures). Les lignes du fichier de checksums qui désignent un fichier non envoyé sont tolérées.
5. **`artifacts.json` est du JSON valide.**
6. **`metadata.json` est un objet JSON** qui contient `tag` ou `version`.
7. **La version correspond** : `{version}` est égal au `tag`, à la `version`, ou à `v` + `version` de `metadata.json`.

| `{version}` dans l'URL | `metadata.json` | Résultat |
| --- | --- | --- |
| `v0.1.0` | `{"tag":"v0.1.0","version":"0.1.0"}` | accepté (tag) |
| `0.1.0` | `{"tag":"v0.1.0","version":"0.1.0"}` | accepté (version) |
| `v0.1.0` | `{"version":"0.1.0"}` | accepté (`v` + version) |
| `v0.1.0` | `{"tag":"v0.2.0","version":"0.2.0"}` | refusé 422 |

**Ce que fait la publication**

1. Le dossier de staging `bin.local/.uploads/<vcs>/<version>/` est **renommé** en `bin.local/assets/binaries/<vcs>/<version>/` : la version apparaît en une seule opération, complète.
2. Avec `latest=true`, `bin.local/assets/binaries/<vcs>/artifacts.json` est remplacé par une copie de l'`artifacts.json` de la version, via un fichier temporaire puis un renommage : un lecteur voit l'ancien ou le nouveau fichier, jamais un fichier à moitié écrit.

**Erreurs** (dans l'ordre où elles sont vérifiées)

| Code | Message | Cause |
| --- | --- | --- |
| `403` / `401` | voir [Authentification](#authentification-de-lupload) | |
| `400` | `invalid vcs or version name` | Nom refusé, ou `version` vaut `latest`. |
| `400` | `latest must be true or false` | Valeur de `latest` non reconnue (ex. `yes`). |
| `409` | `version gitlab/v0.1.0 is already published and cannot be changed` | Déjà publiée. |
| `404` | `nothing uploaded for gitlab/v0.1.0` | Aucun fichier en staging pour cette version. |
| `422` | `several checksums files: a_checksums.txt and b_checksums.txt` | Contrôle 1. |
| `422` | `no checksums file (*checksums.txt) uploaded` | Contrôle 1. |
| `422` | `artifacts.json not uploaded` / `metadata.json not uploaded` | Contrôle 2. |
| `422` | `tool_0.1.0_checksums.txt: line 3 is not "<sha256>  <name>"` | Contrôle 3. |
| `422` | `extra.bin is not listed in tool_0.1.0_checksums.txt` | Contrôle 4. |
| `422` | `tool_0.1.0_linux_amd64.tar.gz: sha256 is <réel> but tool_0.1.0_checksums.txt says <attendu>` | Contrôle 4. |
| `422` | `artifacts.json is not valid JSON` | Contrôle 5. |
| `422` | `metadata.json is not valid JSON` | Contrôle 6. |
| `422` | `metadata.json has neither tag nor version` | Contrôle 6. |
| `422` | `metadata.json describes v0.2.0 (version 0.2.0), not v0.1.0` | Contrôle 7. |
| `500` | `internal error` | Erreur disque. Si l'erreur survient en mettant à jour latest, **la version est publiée** mais latest n'a pas changé (le log le précise : `version published but latest not updated`). |

---

## Publier une release pas à pas

### Avec le script (recommandé)

Voir [Le script `publish-to-local-registry.sh`](#le-script-publish-to-local-registrysh).

```sh
goreleaser release --clean
LOCAL_REGISTRY_URL=http://localhost:8077 \
LOCAL_REGISTRY_UPLOAD_TOKEN=... \
  scripts/publish-to-local-registry.sh --latest gitlab dist
```

### À la main avec curl

Ce que fait le script, détaillé, pour une release `v0.1.0` du groupe `gitlab` dont la sortie goreleaser est dans `dist/` :

```sh
URL=http://localhost:8077
V=v0.1.0
auth() { printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN"; }

# 1. Chaque fichier listé dans les checksums, avec son sha256 attendu
while read -r sum name; do
  auth | curl -fsS -H @- -H "X-Checksum-Sha256: $sum" -T "dist/$name" "$URL/gitlab/artifacts/$V/files/$name"
done < dist/tool_0.1.0_checksums.txt

# 2. Le fichier de checksums et les métadonnées
for f in tool_0.1.0_checksums.txt metadata.json artifacts.json; do
  auth | curl -fsS -H @- -T "dist/$f" "$URL/gitlab/artifacts/$V/files/$f"
done

# 3. Rien n'est encore visible
curl -s -o /dev/null -w "%{http_code}\n" "$URL/gitlab/artifacts/$V"     # 404

# 4. Publication
auth | curl -fsS -H @- -X POST "$URL/gitlab/artifacts/$V/publish?latest=true"

# 5. Vérification
curl -s "$URL/gitlab/artifacts/$V"
curl -s "$URL/gitlab/artifacts"
```

### Pourquoi pas un bloc `publishers:` goreleaser

goreleaser (vérifié avec 2.18.2) lance ses `publishers:` **avant** d'écrire `dist/artifacts.json`, dans un ordre quelconque, et l'édition open source n'a pas de hook à la fin de la release. Un bloc `publishers:` seul ne peut donc ni envoyer `artifacts.json`, ni appeler `publish` au bon moment. Lancez le script après `goreleaser release`, par exemple dans la même étape de CI.

---

## Le script `publish-to-local-registry.sh`

`scripts/publish-to-local-registry.sh` envoie une sortie goreleaser complète au registry puis la publie.

### Utilisation

```
publish-to-local-registry.sh [--latest] <vcs> [dist-dir]
```

| Argument | Obligatoire | Description |
| --- | --- | --- |
| `--latest` | non | Publie avec `?latest=true` ; doit être le **premier** argument. Sans lui, `latest=false`. |
| `<vcs>` | oui | Groupe de versions (ex. `gitlab`). |
| `[dist-dir]` | non | Dossier de sortie goreleaser. Défaut : `dist`. |

| Variable | Obligatoire | Description |
| --- | --- | --- |
| `LOCAL_REGISTRY_URL` | oui | Adresse du registry, sans `/` final (ex. `http://localhost:8077`). |
| `LOCAL_REGISTRY_UPLOAD_TOKEN` | oui | Le token du serveur. Il est transmis à curl par l'entrée standard, jamais en argument. |

### Ce qu'il fait

1. Vérifie que `dist-dir` contient `metadata.json`, `artifacts.json` et **exactement un** `*checksums.txt`.
2. Lit la version dans le champ `tag` de `metadata.json`.
3. Envoie chaque fichier listé dans le fichier de checksums, avec `X-Checksum-Sha256`.
4. Envoie le fichier de checksums, puis les `*.sig` et `*.pem` présents à la racine de `dist-dir`, puis `metadata.json` et `artifacts.json`.
5. Appelle `publish` et affiche la réponse.

### Sortie

```
PUT tool_0.1.0_darwin_amd64.tar.gz
PUT tool_0.1.0_darwin_arm64.tar.gz
PUT tool_0.1.0_linux_amd64.tar.gz
PUT tool_0.1.0_linux_arm64.tar.gz
PUT tool_0.1.0_checksums.txt
PUT metadata.json
PUT artifacts.json
POST publish gitlab/v0.1.0 (latest=true)
{"vcs":"gitlab","version":"v0.1.0","files":[…],"latest":true}
```

### Codes de sortie

| Code | Situation |
| --- | --- |
| `0` | Version publiée. |
| `1` | Fichier absent dans `dist-dir`, ou refus du serveur. Le message d'erreur du serveur est affiché, ex. `{"error":"version gitlab/v0.1.0 is already published and cannot be changed"}`. |
| non nul (1 ou 2 selon le `sh`) | Argument `<vcs>` ou variable manquante : le message indique laquelle, ex. `LOCAL_REGISTRY_URL is not set`. |

En cas d'échec avant la publication, corrigez la cause et **relancez simplement le script** : les fichiers déjà en staging sont remplacés.

---

## Quotas, espace disque et purge du staging

Le staging reçoit des fichiers de toute personne qui a le token. Ces limites empêchent de remplir le disque, par erreur ou volontairement :

| Limite | Option | Défaut | Réponse au-delà |
| --- | --- | --- | --- |
| Taille d'un fichier | `--max-upload-mb` | 200 MiB | `413` |
| Taille en staging d'une version | `--max-version-mb` | 2 GiB | `413` |
| Fichiers en staging d'une version | `--max-version-files` | 100 | `422` |
| Taille en staging de toutes les versions | `--max-staging-mb` | 10 GiB | `507` |
| Espace disque laissé libre | `--min-free-mb` | 1 GiB | `507` |

- **Ce qui est compté :** seuls les fichiers en staging (`bin.local/.uploads/`). Les versions publiées ne comptent pas : une fois publiée, une version libère son quota.
- **Remplacement :** renvoyer un fichier sous le même nom ne compte que la différence de taille, et ne compte pas comme un fichier de plus.
- **Taille inconnue :** pour un envoi sans `Content-Length` (chunked, `curl -T -`), le contrôle d'espace disque suppose la plus grande taille que les quotas autoriseraient pour ce fichier.
- **Espace disque :** mesuré sur le disque de `bin.local/` (Linux, macOS et autres Unix). Sur les autres systèmes, ce contrôle est ignoré.
- **Désactiver une limite :** `0` sur la ligne de commande.

Exemple : une version pèse 2,5 GiB (gros binaires multi-plateformes) :

```sh
./local-registry serve --asset-path /srv/registry --max-upload-mb 1024 --max-version-mb 4096 --max-staging-mb 20480
```

### Purge automatique

Une version restée en staging sans aucun envoi pendant `--staging-ttl` (24 h par défaut) est supprimée : publication abandonnée, script interrompu, test. La purge :

- tourne au démarrage du serveur, puis toutes les heures (ou toutes les `--staging-ttl`/2 si c'est plus court) ;
- supprime le dossier `bin.local/.uploads/<vcs>/<version>/` dont le fichier le plus récent est plus vieux que `--staging-ttl`, puis le dossier `<vcs>` s'il est vide ;
- supprime aussi les fichiers de `bin.local/.uploads-tmp/` plus vieux que `--staging-ttl` (restes d'un arrêt brutal) ;
- ne touche **jamais** aux versions publiées ;
- inscrit chaque suppression au [journal d'audit](#journal-daudit) (`staging_purged`, `tmp_purged`).

Elle prend le même verrou que l'upload : elle ne peut pas supprimer une version pendant sa publication.

---

## Journal d'audit

Chaque événement de sécurité des routes d'upload est ajouté au fichier `--audit-log` (par défaut `<asset-path>/bin.local/audit.log`) : un objet JSON par ligne, écrit et synchronisé sur disque avant la réponse au client.

- Le fichier est créé avec le mode `0600` (lisible par le seul utilisateur du serveur), son dossier au besoin.
- Il n'est jamais servi par les routes de fichiers ; le serveur refuse de démarrer s'il est placé dans un arbre public.
- Le serveur ne fait qu'y ajouter des lignes : la rotation est à faire à l'extérieur (`logrotate` avec `copytruncate`, ou arrêt, déplacement, redémarrage).
- Si l'écriture échoue (disque plein, droits), la requête aboutit quand même et l'erreur est écrite dans les logs : `ERROR audit log write failed`.
- Le token n'y figure jamais. Pour savoir quel token a servi, chaque événement authentifié porte `token_id` : les 12 premiers caractères hexadécimaux du sha256 du token. Calculez-le avec `printf %s "$LOCAL_REGISTRY_UPLOAD_TOKEN" | shasum -a 256 | cut -c1-12`.

### Champs communs

| Champ | Contenu |
| --- | --- |
| `time` | Date UTC, RFC 3339 avec nanosecondes (`2026-10-09T17:21:23.888417Z`). |
| `event` | Type d'événement (tableau suivant). |
| `remote` | IP de la connexion TCP. Absent pour les événements de purge. |
| `forwarded_for` | En-tête `X-Forwarded-For` tel que reçu, s'il est présent (non fiable sans reverse proxy de confiance). |

### Événements

| `event` | Quand | Champs propres |
| --- | --- | --- |
| `auth_failed` | 401 ou 403 sur une route d'upload | `method`, `path`, `status`, `error`, `authorization_sent` (un en-tête `Authorization` était-il présent) |
| `upload` | `PUT` accepté | `vcs`, `version`, `name`, `size`, `sha256`, `replaced` (un fichier du même nom a été remplacé), `token_id` |
| `upload_refused` | `PUT` refusé après authentification | `vcs`, `version`, `name`, `status`, `error`, `token_id` |
| `publish` | `publish` réussi | `vcs`, `version`, `latest`, `files` (liste de `{name, size, sha256}` pour **chaque** fichier publié), `token_id` |
| `publish_refused` | `publish` refusé après authentification | `vcs`, `version`, `status`, `error`, `token_id` |
| `staging_purged` | Purge d'une version en staging | `vcs`, `version`, `files`, `bytes`, `idle` (durée d'inactivité, ex. `26h3m12s`) |
| `tmp_purged` | Purge de fichiers temporaires | `files` |

Pour un refus en `500`, `error` contient la cause réelle (chemin, erreur système), alors que le client ne reçoit que `internal error`.

### Exemples

```json
{
    "authorization_sent": true,
    "error": "invalid or missing bearer token",
    "event": "auth_failed",
    "method": "PUT",
    "path": "/gitlab/artifacts/v1/files/a",
    "remote": "127.0.0.1",
    "status": 401,
    "time": "2026-10-09T17:21:23.888417Z"
} {
    "event": "upload",
    "name": "tool_0.1.0_linux_amd64.tar.gz",
    "remote": "10.0.0.4",
    "replaced": false,
    "sha256": "8f6c80d2808e7cedf31d8215cf3c9a012c76fc6ae40ee250d0b6b85f7f54a5b2",
    "size": 1843202,
    "time": "2026-10-09T17:21:23.910397Z",
    "token_id": "1a7674eb4ee7",
    "vcs": "gitlab",
    "version": "v0.1.0"
} {
    "error": "version gitlab/v2 would exceed its staging quota of 1048576 bytes",
    "event": "upload_refused",
    "forwarded_for": "10.0.0.9",
    "name": "big",
    "remote": "127.0.0.1",
    "status": 413,
    "time": "2026-10-09T17:21:23.96298Z",
    "token_id": "1a7674eb4ee7",
    "vcs": "gitlab",
    "version": "v2"
} {
    "event": "publish",
    "files": [{
        "name": "artifacts.json",
        "size": 2051,
        "sha256": "636d…"
    }, {
        "name": "tool_0.1.0_linux_amd64.tar.gz",
        "size": 1843202,
        "sha256": "8f6c…"
    }],
    "latest": true,
    "remote": "10.0.0.4",
    "time": "2026-10-09T17:22:01.004Z",
    "token_id": "1a7674eb4ee7",
    "vcs": "gitlab",
    "version": "v0.1.0"
}
```

### Interroger le journal avec `jq`

```sh
LOG=/srv/registry/bin.local/audit.log

# Qui a publié quoi, et quand
jq -r 'select(.event=="publish") | "\(.time) \(.vcs)/\(.version) latest=\(.latest) depuis \(.remote)"' $LOG

# Le sha256 publié d'un fichier précis
jq -r 'select(.event=="publish" and .version=="v0.1.0") | .files[] | select(.name=="tool_0.1.0_linux_amd64.tar.gz") | .sha256' $LOG

# Échecs d'authentification par IP (tentatives de deviner le token ?)
jq -r 'select(.event=="auth_failed") | .remote' $LOG | sort | uniq -c | sort -rn

# Tous les refus des dernières 24 h
jq -c --arg since "$(date -u -v-1d +%Y-%m-%dT%H:%M:%S 2>/dev/null || date -u -d '1 day ago' +%Y-%m-%dT%H:%M:%S)" \
  'select((.event|endswith("_refused") or .=="auth_failed") and .time >= $since)' $LOG
```

Le sha256 inscrit à la publication permet de vérifier plus tard qu'un fichier publié n'a pas été modifié sur le disque :

```sh
shasum -a 256 /srv/registry/bin.local/assets/binaries/gitlab/v0.1.0/tool_0.1.0_linux_amd64.tar.gz
```

---

## Garanties d'atomicité et de cohérence

| Garantie                                                               | Comment                                                                                                                                             |
|------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|
| Un fichier en cours d'envoi n'est jamais visible, même en staging      | Il est reçu dans `bin.local/.uploads-tmp/` et n'est déplacé (renommage) dans le staging qu'une fois complet et vérifié.                             |
| Une version apparaît complète ou pas du tout                           | `publish` renomme le dossier de staging en une seule opération. Un lecteur ne voit jamais une version à moitié copiée.                              |
| Une version publiée est vérifiée                                       | Chaque fichier est contrôlé contre le fichier de checksums avant publication.                                                                       |
| `latest` change d'un bloc                                              | Le nouveau `artifacts.json` est écrit à côté puis renommé par-dessus l'ancien.                                                                      |
| Une version publiée ne change plus                                     | `PUT` et `publish` répondent 409. Les clients qui lisent une version et son sha256 dans ce registry obtiennent toujours les mêmes octets.           |
| Deux envois parallèles ne dépassent pas ensemble un quota              | Les quotas sont revérifiés sous le même verrou, juste avant l'entrée en staging.                                                                    |
| Aucun fichier n'entre dans une version pendant ou après sa publication | Les déplacements vers le staging et la publication sont sérialisés par un verrou ; la présence de la version publiée est revérifiée sous ce verrou. |

Ces renommages sont atomiques parce que `.uploads-tmp`, `.uploads` et `assets` sont sur le **même système de fichiers** (tous sous `bin.local/`). Ne montez pas l'un de ces dossiers sur un autre disque.

Le verrou est propre à un processus : ne lancez qu'**un seul** serveur avec upload sur un même `--asset-path`.

---

## Opérations courantes

### Corriger une publication refusée

Le staging est conservé après un refus. Renvoyez le fichier fautif sous le même nom (il est remplacé) ou envoyez le fichier manquant, puis rappelez `publish`. Ou relancez le script.

### Abandonner un envoi

Il n'y a pas de route de suppression. La [purge automatique](#purge-automatique) supprime le staging après `--staging-ttl` d'inactivité (24 h par défaut). Pour libérer la place tout de suite, effacez-le sur le serveur :

```sh
rm -rf <asset-path>/bin.local/.uploads/gitlab/v0.1.0
```

### Changer la version « latest »

Pour une nouvelle version, publiez-la avec `?latest=true`. Pour revenir à une version déjà publiée, copiez son `artifacts.json` sur le serveur :

```sh
cp <asset-path>/bin.local/assets/binaries/gitlab/v0.0.9/artifacts.json \
   <asset-path>/bin.local/assets/binaries/gitlab/artifacts.json
```

### Retirer ou remplacer une version publiée

L'API l'interdit volontairement. Sur le serveur, supprimez le dossier de la version (et corrigez `artifacts.json` latest s'il la désignait), puis republiez :

```sh
rm -rf <asset-path>/bin.local/assets/binaries/gitlab/v0.1.0
```

Préférez publier une nouvelle version (`v0.1.1`) : les clients qui ont déjà téléchargé `v0.1.0` gardent des octets cohérents.

### Changer le token

Arrêtez le serveur, changez `LOCAL_REGISTRY_UPLOAD_TOKEN`, relancez. Les requêtes en cours se terminent pendant l'arrêt.

### Nettoyer les restes d'un arrêt brutal

La [purge automatique](#purge-automatique) supprime les fichiers de `bin.local/.uploads-tmp/` plus vieux que `--staging-ttl`. Le reste se nettoie à la main :

Après un `kill -9` ou une panne pendant un envoi, des fichiers `upload-*` peuvent rester dans `bin.local/.uploads-tmp/`, et des fichiers `.tmp-*` dans `bin.local/assets/binaries/<vcs>/`. Ils sont ignorés par le serveur et peuvent être supprimés quand aucun envoi n'est en cours.

---

## Utilisation comme bibliothèque Go

Le paquet `sonarbridge-go/local-registry` (nom de paquet `local_registry`) peut être intégré à un autre programme.

```go
import localregistry "sonarbridge-go/local-registry"
```

### Serveur et routes

```go
s := localregistry.NewLocalRegistryServer(8077, "localhost", "/srv/registry")

// HTTPS (facultatif) : charge la paire maintenant, échoue si un fichier est invalide
if err := s.SetTLS("fullchain.pem", "privkey.pem"); err != nil {
	log.Fatal(err)
}

// Upload (PUT .../files/{name} et POST .../publish)
s.AddUploadRoutes(localregistry.UploadConfig{
	Token:       os.Getenv("LOCAL_REGISTRY_UPLOAD_TOKEN"), // vide = upload désactivé (403)
	MaxFileSize: 500 << 20,                                // octets ; 0 = DefaultMaxFileSize (200 MiB)
	MaxVersionSize:  4 << 30,        // 0 = DefaultMaxVersionSize (2 GiB) ; négatif = pas de limite
	MaxVersionFiles: 0,              // 0 = DefaultMaxVersionFiles (100) ; négatif = pas de limite
	MaxStagingSize:  -1,             // pas de limite
	MinFreeSpace:    0,              // 0 = DefaultMinFreeSpace (1 GiB) ; négatif = pas de contrôle
	StagingTTL:      48 * time.Hour, // 0 = DefaultStagingTTL (24 h) ; négatif = pas de purge
})

// Journal d'audit (sans lui, les événements vont dans slog)
audit, err := localregistry.OpenAuditLog("/var/log/local-registry/audit.log")
if err != nil {
	log.Fatal(err)
}
defer audit.Close()
s.SetAuditLog(audit)

// Téléchargement de fichiers sous la racine
s.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")

// Route JSON simple : reçoit les variables du chemin dans leur ordre d'apparition
s.AddRoute("/hello/{name}", "GET", func(pvs ...string) any {
	return map[string]string{"hello": pvs[0]}
})

// Route JSON qui peut échouer
s.AddRouteE("/items/{id}", "GET", func(pvs ...string) (any, error) {
	if !localregistry.ValidName(pvs[0]) {
		return nil, &localregistry.HTTPError{Status: http.StatusBadRequest, Msg: "invalid id"}
	}
	item, err := load(pvs[0])
	if errors.Is(err, os.ErrNotExist) {
		return nil, localregistry.ErrNotFound // 404 {"error":"not found"}
	}
	return item, err // autre erreur : 500 {"error":"internal error"}
})

s.Serve() // bloque jusqu'à SIGINT/SIGTERM ; sort du processus en code 1 si le port est pris
```

| Fonction                                        | Rôle                                                                                                                                                                                                                      |
|-------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `NewLocalRegistryServer(port, addr, assetPath)` | Crée le serveur. Aucune route n'est enregistrée.                                                                                                                                                                          |
| `AddRoute(uri, method, f)`                      | Route JSON. `method` est insensible à la casse ; le `/` initial de `uri` est facultatif. Les motifs suivent `net/http` (Go 1.22+) ; les jokers multi-segments `{x...}` ne sont pas pris en charge.                        |
| `AddRouteE(uri, method, f)`                     | Comme `AddRoute`, avec erreur : `*HTTPError` → son statut et son message ; `ErrNotFound` ou `fs.ErrNotExist` (même enveloppés) → 404 ; autre → 500 sans détail.                                                           |
| `AddFileRoute(uri, method, varName)`            | Sert le fichier désigné par la variable `varName`, relatif à la racine, avec les règles de [`/assets/binary`](#get-assetsbinaryfilepath) : fichiers publics seulement, ouverts via `os.Root`.                             |
| `SetTLS(certFile, keyFile) error`               | Passe le serveur en HTTPS seul (TLS 1.2 minimum). Charge la paire immédiatement ; en cas d'erreur, le serveur reste en HTTP.                                                                                              |
| `TLSEnabled() bool`                             | `true` après un `SetTLS` réussi.                                                                                                                                                                                          |
| `IsPublicFile(name) bool`                       | `true` si `name` (chemin relatif, séparé par `/`) est servi par les routes de fichiers : sous `bin.local/assets/` ou `assets/`, sans élément commençant par `.`.                                                          |
| `AddUploadRoutes(cfg)`                          | Enregistre les deux routes d'upload. Les champs de `UploadConfig` à `0` prennent leur valeur `Default*` ; un champ négatif désactive sa limite (sauf `MaxFileSize`, où négatif = défaut).                                 |
| `OpenAuditLog(path) (*AuditLog, error)`         | Ouvre (ou crée en `0600`) un journal d'audit en ajout. `Close()` le ferme.                                                                                                                                                |
| `SetAuditLog(a)`                                | Envoie les événements d'audit dans `a`. Sans appel, ils vont dans `slog` au niveau INFO (message `audit`).                                                                                                                |
| `Serve()`                                       | Lance le serveur, gère les signaux et l'arrêt propre, quitte le processus en code 1 en cas d'erreur.                                                                                                                      |
| `ServeContext(ctx) error`                       | Lance le serveur jusqu'à l'annulation de `ctx`, puis l'arrête proprement (10 s max). Renvoie l'erreur au lieu de quitter. Lance aussi la purge du staging si `AddUploadRoutes` a été appelé avec un `StagingTTL` positif. |
| `Handler() http.Handler`                        | Les routes, sans logs ni délais : pratique pour `httptest`.                                                                                                                                                               |

### Tester ses routes

```go
s := localregistry.NewLocalRegistryServer(0, "localhost", t.TempDir())
s.AddRoute("/hello/{name}", "GET", func(pvs ...string) any { return pvs[0] })

rec := httptest.NewRecorder()
s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/hello/bob", nil))
// rec.Code == 200, rec.Body.String() == "\"bob\"\n"
```

### Arrêt piloté par l'appelant

```go
ctx, cancel := context.WithCancel(context.Background())
go func() { <-stopRequested; cancel() }()
if err := s.ServeContext(ctx); err != nil {
	log.Fatal(err) // port occupé, ou arrêt plus long que 10 s
}
```

### Chemins et catalogue

```go
l := localregistry.NewLayout("/srv/registry")
l.AssetsDir()                          // /srv/registry/bin.local/assets
l.BinariesDir()                        // /srv/registry/bin.local/assets/binaries
l.VCSDir("gitlab")                     // .../binaries/gitlab, nil
l.VersionDir("gitlab", "v0.1.0")       // .../binaries/gitlab/v0.1.0, nil
l.ArtifactsFile("gitlab", "latest")    // .../binaries/gitlab/artifacts.json, nil
l.ArtifactsFile("gitlab", "v0.1.0")    // .../binaries/gitlab/v0.1.0/artifacts.json, nil
l.StagingDir("gitlab", "v0.1.0")       // /srv/registry/bin.local/.uploads/gitlab/v0.1.0, nil
l.UploadTmpDir()                       // /srv/registry/bin.local/.uploads-tmp
l.LegacyReleaseDir("v1")               // /srv/registry/assets/binaries/gitlab/v1, nil
l.VCSDir("../etc")                     // "", erreur qui enveloppe ErrNotFound

localregistry.ValidName("v0.1.0")      // true
localregistry.ValidName("..")          // false

localregistry.IsPublicFile("bin.local/assets/binaries/gitlab/v0.1.0/tool.tar.gz") // true
localregistry.IsPublicFile("bin.local/.uploads/gitlab/v0.1.0/tool.tar.gz")        // false

a := &localregistry.Asset{}
paths, err := a.FindAssetPaths(l.AssetsDir())       // []AssetPath : chemins, triés par dossier
contents, err := a.FindAssetContents(l.AssetsDir()) // []AssetContents : contenus (ce que renvoie /artifacts/all)
```

| Constante / variable     | Valeur                                    |
|--------------------------|-------------------------------------------|
| `DefaultMaxFileSize`     | `200 << 20` (200 MiB)                     |
| `ErrNotFound`            | Erreur sentinelle → 404 dans `AddRouteE`. |
| `DefaultMaxVersionSize`  | `2 << 30` (2 GiB)                         |
| `DefaultMaxVersionFiles` | `100`                                     |
| `DefaultMaxStagingSize`  | `10 << 30` (10 GiB)                       |
| `DefaultMinFreeSpace`    | `1 << 30` (1 GiB)                         |
| `DefaultStagingTTL`      | `24 * time.Hour`                          |

---

## Limites connues

- **Tout fichier de `bin.local/assets/` et `assets/` est public**, sans authentification en lecture. N'y placez rien de privé.
- **Sans `--tls-cert`, tout circule en clair**, token d'upload compris. Activez HTTPS ou placez le registry derrière un reverse proxy HTTPS dès qu'il quitte la machine.
- **Les liens symboliques ne sont contrôlés que sur les routes de fichiers.** Les routes de catalogue (`/{vcs}/artifacts…`, `/artifacts/all`) et l'upload suivent les liens présents sur le disque : ne laissez créer des liens sous `--asset-path` qu'à des personnes de confiance.
- **`/artifacts/metadatas`** renvoie toujours 404.
- **`POST /releases/{version}`** : dossier `gitlab` en dur, `is_checksum` ne reconnaît pas `_checksums.txt`, `id` change à chaque appel. Conservé tel quel pour la compatibilité.
- **`/artifacts/all`** : `provider` est le nom du dossier parent (le numéro de version pour une version) ; les fichiers illisibles donnent `""` sans erreur ; un catalogue vide renvoie `null`. Le catalogue ne reconnaît que les fichiers de checksums en `_checksums.txt`, alors que la publication accepte tout `*checksums.txt` : gardez le nom par défaut de goreleaser (`<projet>_<version>_checksums.txt`) pour que `checksums_text` soit rempli.
- **Pas de route de suppression**, ni pour le staging ni pour une version publiée (voir [Opérations courantes](#opérations-courantes)).
- **Quotas et espace disque :** pendant la réception, un fichier occupe déjà sa place dans `bin.local/.uploads-tmp/` ; plusieurs gros envois simultanés peuvent donc dépasser brièvement les quotas sur le disque, avant d'être refusés à l'entrée en staging. Le contrôle d'espace libre garde la marge `--min-free-mb`.
- **Audit :** une écriture ratée du journal ne bloque pas la requête (elle est signalée dans les logs). `X-Forwarded-For` est enregistré tel que reçu, sans être vérifié. Le journal n'est pas signé : quiconque peut écrire sur le serveur peut le modifier ; envoyez-le vers un système externe (syslog, SIEM) si vous en avez besoin comme preuve.
- **Pas d'`ETag`** sur les téléchargements ; `Last-Modified`, `If-Modified-Since` et `Range` fonctionnent.
- **Un seul serveur d'upload par `--asset-path`** (le verrou est interne au processus).

---

## Dépannage

| Symptôme                                                                                         | Cause et solution                                                                                                                                                                                                                                                      |
|--------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `WARN upload token generation skipped: a token is already configured (…)` | `--disable-generate-token=false` alors qu'un token est configuré : c'est lui qui sert. Retirez `LOCAL_REGISTRY_UPLOAD_TOKEN` et `upload-token` pour obtenir un token généré, ou retirez l'option. |
| `unexpected argument "false"` avec `--disable-generate-token false` | Option booléenne : écrivez `--disable-generate-token=false`. |
| `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled` au démarrage, puis 403              | Exportez la variable dans l'environnement du serveur, ou mettez `upload-token` dans le fichier de configuration. Vérifiez avec `local-registry config`.                                                                                                                |
| Une valeur n'est pas celle attendue                                                              | `local-registry config` (avec les mêmes options) montre la valeur effective et sa source : une variable `LOCAL_REGISTRY_*` oubliée dans l'environnement l'emporte sur le fichier.                                                                                      |
| `unknown flag: --port` (ou autre) alors que l'option existe                                      | L'option est donnée sans la commande : écrivez `local-registry serve --port …`. Les options appartiennent à `serve` et `config`.                                                                                                                                       |
| `unknown shorthand flag: 'p' in -port`                                                           | La forme à un tiret n'est plus acceptée : écrivez `--port`.                                                                                                                                                                                                            |
| `Error: listen tcp …: bind: address already in use`, code 1                                      | Un autre processus utilise le port : `lsof -i :8077`, ou changez `--port`.                                                                                                                                                                                             |
| 401 alors que le token est bon                                                                   | Vérifiez le schéma exact `Bearer ` (avec la majuscule et l'espace), l'absence d'espace ou de saut de ligne en trop dans le token, et que le serveur a été relancé après un changement de token.                                                                        |
| 404 sur `GET /gitlab/artifacts` après un upload                                                  | La version n'est pas publiée, ou a été publiée sans `?latest=true`. Interrogez `/gitlab/artifacts/<version>`.                                                                                                                                                          |
| 404 sur `/assets/binary/...`                                                                     | Le chemin est relatif à `--asset-path` (pas à `bin.local/assets`), doit commencer par `bin.local%2Fassets%2F` ou `assets%2F`, et chaque `/` doit être écrit `%2F`. Un fichier en staging, caché (`.xxx`) ou atteint par un lien symbolique sortant n'est jamais servi. |
| `asset-path is required …`, code 2                                                               | Réglage obligatoire : `--asset-path /srv/registry`, `LOCAL_REGISTRY_ASSET_PATH` ou `asset-path` dans le fichier.                                                                                                                                                       |
| `config file …: unknown setting "…"`, code 2                                                     | Faute de frappe dans le fichier de configuration : comparez avec la liste des réglages (`local-registry serve --help`).                                                                                                                                                |
| `Error: loading TLS certificate: …`, code 1                                                      | Fichier introuvable, ou clé qui ne correspond pas au certificat. Vérifiez la paire avec `openssl x509 -noout -pubkey -in cert.pem` et `openssl pkey -pubout -in key.pem` : les deux sorties doivent être identiques.                                                   |
| `curl: (60) SSL certificate problem`                                                             | Certificat auto-signé ou autorité inconnue : `curl --cacert cert.pem …`, ou `CURL_CA_BUNDLE=cert.pem` pour le script.                                                                                                                                                  |
| `Client sent an HTTP request to an HTTPS server.`                                                | Le serveur est en HTTPS : utilisez `https://`.                                                                                                                                                                                                                         |
| `warning: upload is enabled on … without TLS`                                                    | Le token passerait en clair sur le réseau : ajoutez `--tls-cert`/`--tls-key`, ou ignorez si un reverse proxy fait le TLS devant le serveur.                                                                                                                            |
| Un fichier téléchargé contient `--------------------------…` et `Content-Disposition: form-data` | Il a été envoyé avec `curl -F` (multipart). Renvoyez-le avec `curl -T`.                                                                                                                                                                                                |
| 409 au `PUT` ou au `publish`                                                                     | La version existe déjà : publiez un nouveau numéro, ou voir [Retirer ou remplacer une version publiée](#retirer-ou-remplacer-une-version-publiée).                                                                                                                     |
| 413 `file exceeds …`                                                                             | Fichier plus gros que `--max-upload-mb` (200 MiB par défaut) : relancez le serveur avec une limite plus haute.                                                                                                                                                         |
| 413 `… would exceed its staging quota …`                                                         | La version dépasse `--max-version-mb` : augmentez-le, ou vérifiez qu'il ne reste pas en staging des fichiers d'un essai précédent sous d'autres noms.                                                                                                                  |
| 422 `… already has N staged files …`                                                             | `--max-version-files` atteint : augmentez-le, ou abandonnez le staging de la version et recommencez.                                                                                                                                                                   |
| 507 `upload staging area is full …`                                                              | Trop de versions en attente : publiez-les, attendez la purge, ou [abandonnez-les](#abandonner-un-envoi).                                                                                                                                                               |
| 507 `not enough free disk space …`                                                               | Le disque du registry est presque plein : libérez de la place ou baissez `--min-free-mb`.                                                                                                                                                                              |
| 422 `… is not listed in …_checksums.txt`                                                         | Un fichier envoyé n'est pas dans le fichier de checksums (ex. un SBOM). Ne l'envoyez pas, ou faites-le inclure par goreleaser dans les checksums.                                                                                                                      |
| 422 `… sha256 is … but … says …`                                                                 | Le fichier a changé depuis la génération des checksums, ou un mauvais fichier a été envoyé sous ce nom. Renvoyez le bon fichier.                                                                                                                                       |
| 422 `metadata.json describes …, not …`                                                           | La version de l'URL ne correspond pas au tag goreleaser ; le script lit la version dans `metadata.json`, utilisez-le.                                                                                                                                                  |
| 301 inattendu                                                                                    | L'URL contient `..` ou `//` en clair ; le serveur redirige vers le chemin nettoyé.                                                                                                                                                                                     |
| Le client voit la connexion coupée sur un gros téléchargement                                    | Le délai d'écriture des routes de lecture est de 20 s : utilisez `Range` (`curl -C -`) pour reprendre, ou téléchargez depuis une connexion plus rapide.                                                                                                                |
