<p align="center">
  <img alt="GoReleaser Logo" src="./assets/logo-fl.svg?v=3&s=300" height="300" />
  <p align="center" style="color: #43f9f9">
    <img height="30" src="https://cdn.simpleicons.org/go/555555/ffffff" alt="Go" />
  </p>
</p>

---
[![ci](https://github.com/nivekalara237/local-registry/actions/workflows/ci.yml/badge.svg)](https://github.com/nivekalara237/local-registry/actions/workflows/ci.yml)
[![release](https://github.com/nivekalara237/local-registry/actions/workflows/release.yml/badge.svg)](https://github.com/nivekalara237/local-registry/actions/workflows/release.yml)
---

[English](README.md) | **Français**

Un registry HTTP local pour les releases [goreleaser](https://goreleaser.com), ou tout fichier versionné. Il sert le catalogue des versions, les binaires et leurs checksums, et accepte de nouvelles versions en deux temps : envoi des fichiers, puis publication atomique.

## Sommaire

- [Démarrage rapide](#démarrage-rapide)
- [Lancer le serveur](#lancer-le-serveur)
- [Arborescence](#arborescence)
- [Conventions de l'API](#conventions-de-lapi)
- [Référence de l'API](#référence-de-lapi)
- [Produits et standard local-registry/v1](#produits-et-standard-local-registryv1)
- [Publier une release](#publier-une-release)
- [Quotas et purge du staging](#quotas-et-purge-du-staging)
- [Journal d'audit](#journal-daudit)
- [Garanties](#garanties)
- [Opérations](#opérations)
- [Bibliothèque Go](#bibliothèque-go)
- [Limites connues](#limites-connues)
- [Dépannage](#dépannage)

## Démarrage rapide

```sh
go build -o local-registry ./cmd

export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"
./local-registry serve --asset-path /srv/registry

# Dans le projet goreleaser
goreleaser release --clean
LOCAL_REGISTRY_URL=http://localhost:8077 \
  scripts/upload.sh --latest github dist/artifacts.json

curl -s http://localhost:8077/github/artifacts                 # artifacts.json de la version latest
curl -s -X POST http://localhost:8077/releases/github/latest   # ses fichiers et leurs URL
```

---

## Lancer le serveur

### Prérequis

| Outil | Version | Pour |
| --- | --- | --- |
| Go | 1.25.1+ (`go.mod`) | compiler |
| bash, curl, jq | curl 7.76+ (`--fail-with-body`) | `scripts/upload.sh` |
| goreleaser | testé avec 2.18.2 | produire les releases |

### Compiler

```sh
go build -o local-registry ./cmd
go build -ldflags "-X main.version=v1.2.3" -o local-registry ./cmd   # avec un numéro de version
```

Le binaire n'a aucune dépendance à l'exécution.

### Commandes

| Commande | Effet |
| --- | --- |
| `serve` | Lance le serveur. |
| `config` | Calcule et valide la configuration comme `serve`, affiche chaque réglage et sa source, ne démarre rien. |
| `version` (ou `--version`, `-v`) | Affiche `local-registry v1.2.3 (go1.25.1, darwin/arm64, commit 9c9d7664ceef)`. |
| `completion bash\|zsh\|fish\|powershell` | Affiche un script d'autocomplétion. |
| `help [commande]` (ou `-h`, `--help`) | Aide. `serve --help` liste chaque option avec sa variable d'environnement et son défaut. |

### Configuration

Chaque réglage vient de la première source qui le définit :

1. option de ligne de commande (`--port 9000`) ;
2. variable d'environnement : `LOCAL_REGISTRY_` + nom de l'option en majuscules, `-` → `_` (`LOCAL_REGISTRY_PORT=9000`) ;
3. fichier YAML donné par `--config` (`port: 9000`) ;
4. valeur par défaut.

| Option | Défaut | Description |
| --- | --- | --- |
| `--config` | — | Fichier de configuration YAML. |
| `--asset-path` | **obligatoire** | Racine de toutes les données ; dossier existant. |
| `--address` | `localhost` | Adresse d'écoute (`0.0.0.0` : toutes les interfaces). |
| `--port` | `8077` | Port d'écoute, 0–65535 (`0` : port libre quelconque). |
| `--tls-cert`, `--tls-key` | — | Chaîne de certificats et clé PEM. Les deux ou aucun. Active [HTTPS](#https) seul. |
| `--max-upload-mb` | `200` | Plus gros fichier accepté, en MiB. `≤ 0` vaut 200. |
| `--max-version-mb` | `2048` | Octets en staging par version, en MiB. `0` : sans limite. |
| `--max-version-files` | `100` | Fichiers en staging par version. `0` : sans limite. |
| `--max-staging-mb` | `10240` | Octets en staging pour toutes les versions, en MiB. `0` : sans limite. |
| `--min-free-mb` | `1024` | Espace disque qu'un envoi doit laisser libre, en MiB. `0` : pas de contrôle. |
| `--staging-ttl` | `24h` | Purge une version en staging inactive depuis cette durée. Durée Go, `0` (jamais) ou ≥ `1h`. |
| `--audit-log` | `<asset-path>/bin.local/audit.log` | Fichier du [journal d'audit](#journal-daudit). Refusé dans `bin.local/assets/` ou `assets/` (téléchargeable). |
| `--public-url` | — | Adresse à laquelle les clients joignent le serveur (`https://registry.example.com`, préfixe de chemin permis). Rend absolues les URL écrites dans `artifacts.json` et renvoyées par `/releases`. Vide : URL relatives. |
| `--disable-generate-token` | `true` | `=false` : générer un token d'upload au démarrage. Voir [Token généré](#token-généré). |
| *(pas d'option)* `LOCAL_REGISTRY_UPLOAD_TOKEN` | — | Token d'upload ; clé `upload-token` du fichier. Voir [Token d'upload](#token-dupload). |

Les options s'écrivent `--port 8077` ou `--port=8077` ; la forme `-port` est refusée. Un booléen s'écrit avec `=` : `--disable-generate-token=false`.

**Chemins.** `config`, `asset-path`, `tls-cert`, `tls-key` et `audit-log` développent `$VAR`, `${VAR}`, `~` et `~/`, quelle que soit leur source (utile là où aucun shell ne passe : fichier, systemd, guillemets simples). Une variable non définie est une erreur, pas une chaîne vide ; `~user` est refusé. Un chemin relatif part du dossier courant, pas du dossier du fichier de configuration.

**Fichier de configuration.** Mêmes clés que les options, plus `upload-token`. Les clés inconnues (fautes de frappe, clés imbriquées) sont refusées. Aucun fichier n'est lu sans `--config`/`LOCAL_REGISTRY_CONFIG`.

```yaml
# /etc/local-registry/config.yaml — chmod 600 s'il contient le token
asset-path: /srv/registry
address: 0.0.0.0
port: 8443
tls-cert: /etc/local-registry/fullchain.pem
tls-key: /etc/local-registry/privkey.pem
public-url: https://registry.example.com:8443
max-version-mb: 4096
staging-ttl: 48h
audit-log: /var/log/local-registry/audit.log
upload-token: change-moi
```

#### Token d'upload

Lu dans `LOCAL_REGISTRY_UPLOAD_TOKEN`, sinon dans la clé `upload-token`. Il n'y a pas d'option `--upload-token` (elle apparaîtrait dans `ps`).

- Vide : les routes d'upload répondent 403 ; le démarrage affiche `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled`.
- Dans un fichier lisible par d'autres utilisateurs : le démarrage avertit `warning: <fichier> holds the upload token but is readable by other users (mode -rw-r--r--): chmod 600 <fichier>`.
- Jamais affiché ni journalisé. Logs et journal d'audit montrent son `token_id` : les 12 premiers caractères hexadécimaux de son sha256 (`printf %s "$TOKEN" | shasum -a 256 | cut -c1-12`).

#### Token généré

Avec `--disable-generate-token=false` et aucun token configuré, `serve` génère un token aléatoire (32 octets, 64 caractères hexadécimaux) et l'affiche sur la **sortie standard seulement**, avant la bannière. Il change à chaque démarrage.

```
Upload token: 9f2c4e…
  generated for this run only and saved nowhere: it changes at each start (token_id 4ae3c36ce073)
```

Avec un token configuré, rien n'est généré, le token configuré reste en service, et la sortie d'erreur affiche :

```
WARN upload token generation skipped: a token is already configured (env LOCAL_REGISTRY_UPLOAD_TOKEN) and stays in use; unset LOCAL_REGISTRY_UPLOAD_TOKEN and upload-token to get a generated one, or drop --disable-generate-token=false
```

Le serveur n'écrit le token dans aucun fichier, mais la sortie standard peut être conservée par systemd (`journalctl`), Docker, Kubernetes ou une CI. Dans ces cas, utilisez un token fixe.

#### Vérifier la configuration

```sh
LOCAL_REGISTRY_MAX_VERSION_FILES=50 local-registry config --config /etc/local-registry/config.yaml --staging-ttl 48h
```

```
SETTING                 VALUE                               SOURCE
config                  /etc/local-registry/config.yaml     flag
asset-path              /srv/registry                       config file
address                 localhost                           default
port                    8443                                config file
tls-cert                -                                   default
tls-key                 -                                   default
max-upload-mb           200                                 default
max-version-mb          2048                                default
max-version-files       50                                  env LOCAL_REGISTRY_MAX_VERSION_FILES
max-staging-mb          10240                               default
min-free-mb             1024                                default
staging-ttl             48h0m0s                             flag
audit-log               /srv/registry/bin.local/audit.log   default
public-url              https://registry.example.com        config file
disable-generate-token  true                                default
upload-token            set (token_id e2186dbdb1bb)         config file
```

`-` signifie vide. `upload-token` vaut `not set: upload disabled`, `set (token_id …)`, `generated at startup, printed on stdout only` ou `set (token_id …); generation skipped`.

#### Erreurs de configuration

Toute erreur arrête le programme avant le démarrage, avec le code `2` et `Error: <message>` :

| Message | Cause |
| --- | --- |
| `asset-path is required (…)` | Pas de racine. Le dossier courant n'est jamais pris par défaut. |
| `asset-path <dossier> is not an existing directory` | Dossier absent, ou fichier. |
| `<réglage>: environment variable $X is not set` | Un chemin utilise une variable non définie. |
| `<réglage>: ~user/…: only ~ and ~/ are expanded, not ~user` | `~user` n'est pas pris en charge. |
| `port 70000 is out of range 0-65535` | Port invalide. |
| `tls-cert and tls-key go together` | Un seul des deux est donné. |
| `max-version-mb cannot be negative` (idem autres quotas) | Valeur négative. |
| `staging-ttl must be 0 or at least 1h0m0s` | La purge pourrait supprimer un envoi en cours (un envoi peut durer 30 min). |
| `audit-log … would be downloadable: put it outside bin.local/assets and assets` | Journal d'audit dans un arbre public. |
| `public-url "…" must be an http or https URL with no user, query or fragment, …` | `--public-url` invalide. |
| `port: "abc" is not a whole number`, `staging-ttl: "x" is not a duration such as 90m or 24h`, `disable-generate-token: "x" is not true or false` | Valeur illisible dans l'environnement ou le fichier. |
| `config file <fichier>: unknown setting "max-verion-mb"` | Clé inconnue dans le fichier. |
| `config file <fichier>: …` | Fichier absent, illisible ou YAML invalide. |
| `unknown flag: --x`, `unexpected argument "x" for "local-registry serve"`, `unknown command "x"` | Erreur de ligne de commande. |

### HTTPS

Avec `--tls-cert` et `--tls-key`, le serveur ne parle que HTTPS (TLS 1.2+, HTTP/2). Une requête HTTP simple reçoit `400 Client sent an HTTP request to an HTTPS server.` Les certificats sont chargés au démarrage ; une paire invalide sort avec le code `1` (`Error: loading TLS certificate: …`). Redémarrez pour les changer.

```sh
# Certificat auto-signé pour un usage local
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 365 \
  -keyout key.pem -out cert.pem -subj /CN=localhost -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
./local-registry serve --asset-path /srv/registry --tls-cert cert.pem --tls-key key.pem
curl --cacert cert.pem https://localhost:8077/github/artifacts     # scripts : export CURL_CA_BUNDLE=cert.pem
```

Sans TLS, avec l'upload activé sur une adresse non locale, le démarrage avertit `warning: upload is enabled on 0.0.0.0 without TLS: the token travels in clear text (…)` et démarre quand même (pour un reverse proxy TLS).

### systemd

```ini
# /etc/systemd/system/local-registry.service
[Unit]
Description=local-registry
After=network-online.target

[Service]
ExecStart=/usr/local/bin/local-registry serve --config /etc/local-registry/config.yaml
User=registry
Restart=on-failure
TimeoutStopSec=15

[Install]
WantedBy=multi-user.target
```

### Sorties

Au démarrage (sortie standard) :

```
##################################################
##          Local Registry Server               ##
##################################################

Listen      : http://127.0.0.1:8077
Start At    : 2026-10-10 10:56:12.091998 +0200 CEST m=+0.003639167
Assets Path : /srv/registry
```

`Listen` indique le protocole et l'adresse réellement liés. Puis une ligne par requête sur la sortie d'erreur :

```
2026/10/10 10:56:15 INFO request method=PUT path=/github/artifacts/v1.2.3/files/metadata.json status=201 bytes=112 duration=6.7ms remote=127.0.0.1
```

`remote` est l'adresse TCP ; `forwarded_for` s'ajoute si `X-Forwarded-For` est présent, tel quel (non vérifié). Une requête en échec ajoute une ligne `WARN` (4xx) ou `ERROR` (5xx) avec la cause exacte ; le client ne reçoit que le message listé dans les erreurs de chaque route.

### Codes de sortie et délais

| Situation | Code |
| --- | --- |
| `SIGINT`/`SIGTERM` : plus de nouvelle connexion, 10 s pour les requêtes en cours | `0` |
| Les requêtes en cours dépassent 10 s à l'arrêt (`Error: context deadline exceeded`) | `1` |
| Port occupé, adresse invalide, paire TLS invalide, journal d'audit impossible à ouvrir | `1` |
| Erreur de commande ou de configuration | `2` |

| Délai | Valeur | Portée |
| --- | --- | --- |
| Lecture des en-têtes | 5 s | toutes les requêtes |
| Écriture de la réponse | **20 s** | toutes les routes sauf upload et publish (téléchargements compris) |
| Lecture et écriture | 30 min | `PUT …/files/{name}`, `POST …/publish` |
| Connexion keep-alive inactive | 60 s | toutes les connexions |

---

## Arborescence

```
<asset-path>/
├── bin.local/
│   ├── assets/                              public, parcouru par GET /artifacts/all
│   │   └── binaries/<product>/
│   │       ├── release.json                 copie du release.json de la version latest : la marque de latest
│   │       ├── artifacts.json               copie de l'artifacts.json de la version latest, si elle en a un
│   │       ├── index.json                   le produit et ses versions (local-registry/v1)
│   │       └── <version>/                   version publiée, immuable
│   │           ├── registry.yaml            fiche produit, obligatoire
│   │           ├── release.json             la version (local-registry/v1)
│   │           ├── artifacts.json           s'il a été envoyé ; réécrit : chaque path est une URL de téléchargement
│   │           ├── artifacts.goreleaser.json   artifacts.json tel qu'envoyé
│   │           ├── metadata.json, CHANGELOG.md   s'ils ont été envoyés
│   │           ├── *checksums.txt           envoyé, ou checksums.txt écrit par la publication
│   │           └── archives, binaires, fichiers…
│   ├── .uploads/<product>/<version>/        privé : staging (envoyé, pas publié)
│   ├── .uploads-tmp/                        privé : fichiers en cours de réception
│   └── audit.log                            privé, mode 0600
├── assets/binaries/<product>/<version>/     public : ancien arbre, lu par POST /releases
└── tout le reste                            privé, jamais servi
```

- `<product>` est ce qu'une équipe livre et versionne (`sonarbridge`, `db-migrations`, `github`…) : un nom libre choisi à l'upload, sauf `products`. Voir [Produits](#produits-et-standard-local-registryv1).
- Le serveur lit le disque à chaque requête (pas de cache) : un fichier copié à la main est visible aussitôt.
- L'upload n'écrit jamais dans l'ancien arbre `assets/`.

---

## Conventions de l'API

**Noms.** `{product}`, `{version}` et `{name}` doivent respecter `[A-Za-z0-9._-]+` et ne pas valoir `.` ni `..`. Un nom refusé donne `404` en lecture et `400` en upload. `latest` est une version valide en lecture (la dernière) mais pas en upload, pas plus qu'`artifacts.json`, `artifacts.goreleaser.json`, `release.json` et `index.json`.

**Un segment par variable.** Dans `/assets/binary/{filePath}`, encodez `/` en `%2F`. Les noms sont décodés une seule fois : `%252F` reste `%2F`. Un `..` ou `//` en clair dans l'URL reçoit un `301` vers le chemin nettoyé.

**Méthodes.** Toute route `GET` répond aussi à `HEAD`. Mauvaise méthode : `405` avec `Allow`. Chemin inconnu : `404 page not found` (texte).

**Erreurs.** Les routes JSON répondent `Content-Type: application/json` et, en erreur, `{"error":"<message>"}`. Les routes de fichiers et les erreurs de routage répondent en texte.

| Code | Sens |
| --- | --- |
| `400` | Nom ou paramètre invalide |
| `401` | Token absent ou faux (avec `WWW-Authenticate: Bearer`) |
| `403` | Upload désactivé (aucun token configuré) |
| `404` | Introuvable, ou nom refusé en lecture |
| `409` | Version déjà publiée |
| `413` | Fichier ou version trop gros |
| `422` | Contenu refusé (`registry.yaml`, checksums, `artifacts.json`, `metadata.json`) |
| `500` | Erreur interne ; la cause n'est que dans les logs du serveur |
| `507` | Staging plein ou disque presque plein |

---

## Référence de l'API

| Méthode | Route | Auth | Renvoie |
| --- | --- | --- | --- |
| `GET` | [`/{product}/artifacts`](#get-productartifacts) | — | `artifacts.json` de la version latest |
| `GET` | [`/{product}/artifacts/{version}`](#get-productartifactsversion) | — | `artifacts.json` d'une version |
| `GET` | [`/artifacts/metadatas/{product}`](#get-artifactsmetadatasproduct) | — | `metadata.json` d'une version (défaut : latest) |
| `POST` | [`/releases/{product}/{version}`](#post-releasesproductversion) | — | Fichiers d'une version avec leur URL de téléchargement |
| `POST` | [`/releases/{version}`](#post-releasesversion-dépréciée) | — | Dépréciée : `/releases/gitlab/{version}` |
| `GET` | [`/{product}/artifacts/{version}/files/{name}`](#get-productartifactsversionfilesname) | — | Un fichier d'une version publiée |
| `GET` | [`/assets/binary/{filePath}`](#get-assetsbinaryfilepath) | — | Un fichier public par son chemin |
| `GET` | [`/assets/checksum/{filename}`](#get-assetsbinaryfilepath) | — | Identique à `/assets/binary` |
| `GET` | [`/artifacts/all`](#get-artifactsall) | — | Fichiers bruts du catalogue, par dossier |
| `GET` | [`/products`](#produits-et-standard-local-registryv1) | — | Tous les produits, avec leur version latest |
| `GET` | [`/{product}/index`](#produits-et-standard-local-registryv1) | — | `index.json` : un produit et ses versions |
| `GET` | [`/{product}/releases/{version}`](#produits-et-standard-local-registryv1) | — | `release.json` d'une version (`latest` accepté) |
| `PUT` | [`/{product}/artifacts/{version}/files/{name}`](#put-productartifactsversionfilesname) | token | Envoyer un fichier en staging |
| `POST` | [`/{product}/artifacts/{version}/publish`](#post-productartifactsversionpublish) | token | Vérifier et publier une version |

### Version latest

`publish?latest=true` copie le `release.json` de la version dans `<product>/release.json`, et son `artifacts.json` dans `<product>/artifacts.json` (supprimé si la version n'en a pas). La version latest est la `version` de `<product>/release.json`, si `<product>/<version>/release.json` existe ; sinon il n'y en a pas : `404`. Une version publiée avant l'existence de `release.json` ne peut pas être latest.

### `GET /{product}/artifacts`

`bin.local/assets/binaries/<product>/artifacts.json` : l'`artifacts.json` de la version latest. Une version latest publiée sans `artifacts.json` donne `404` : lisez plutôt [`release.json`](#produits-et-standard-local-registryv1).

```sh
curl -s http://localhost:8077/github/artifacts
```

```json
[{"internal_type":36,"name":"metadata.json","path":"/github/artifacts/v1.2.3/files/metadata.json","type":"Metadata"},{"extra":{"Checksum":"sha256:2b51…","Format":"binary"},"goarch":"arm64","goos":"linux","name":"ci-bridge-vcs-github_Linux_arm64","path":"/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_Linux_arm64","type":"Binary"}]
```

Le JSON est renvoyé tel que stocké (réencodé, même contenu). Pour une release publiée par l'API, c'est le fichier [réécrit](#réécriture-dartifactsjson).

| Code | Cause |
| --- | --- |
| `404` | `product` invalide, aucune version publiée avec `latest=true`, ou version latest sans `artifacts.json`. |
| `500` | Fichier stocké en JSON invalide. |

### `GET /{product}/artifacts/{version}`

`bin.local/assets/binaries/<product>/<version>/artifacts.json`. `latest` équivaut à [`GET /{product}/artifacts`](#get-productartifacts).

```sh
curl -s http://localhost:8077/github/artifacts/v1.2.3
```

| Code | Cause |
| --- | --- |
| `404` | Nom invalide, version non publiée (staging compris), fichier absent. |
| `500` | Fichier stocké en JSON invalide. |

### `GET /artifacts/metadatas/{product}`

`metadata.json` d'une version publiée de `<product>`.

| Paramètre | Où | Description |
| --- | --- | --- |
| `product` | chemin | Produit. |
| `version` | requête | Version exacte. Absent, vide ou `latest` : la [version latest](#version-latest). |

```sh
curl -s http://localhost:8077/artifacts/metadatas/github
curl -s "http://localhost:8077/artifacts/metadatas/github?version=v1.2.3"
```

```json
{"project_name":"sonarbridge-go","tag":"v1.2.3","previous_tag":"v1.2.2","version":"1.2.3","commit":"5c8e4a1…","date":"2026-10-10T10:56:12Z","runtime":{"goos":"darwin","goarch":"arm64"}}
```

| Code | Cause |
| --- | --- |
| `404` | Nom invalide, version non publiée, pas de `metadata.json`, ou pas de version latest. |
| `500` | Fichier stocké en JSON invalide (son contenu n'est pas renvoyé). |

### `POST /releases/{product}/{version}`

Liste les fichiers d'une version, chacun avec l'URL qui le télécharge. Lecture seule malgré `POST` ; pas de corps de requête.

| Paramètre | Où | Description |
| --- | --- | --- |
| `product` | chemin | Produit. |
| `version` | chemin | Version exacte, ou `latest` (voir [Version latest](#version-latest)). |

Les fichiers sont lus dans le premier dossier qui existe :

| Ordre | Dossier | `url` |
| --- | --- | --- |
| 1 | Version publiée `bin.local/assets/binaries/<product>/<version>/` | `/{product}/artifacts/{version}/files/{name}` |
| 2 | Ancien arbre `assets/binaries/<product>/<version>/` | `/assets/binary/assets%2Fbinaries%2F<product>%2F<version>%2F<name>` |

```sh
curl -s -X POST http://localhost:8077/releases/github/latest
```

```json
{
  "tag_name": "v1.2.3",
  "assets": [
    {"id": "d7417cd3-…", "name": "ci-bridge-vcs-github_Linux_arm64", "url": "/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_Linux_arm64", "is_checksum": false},
    {"id": "8b1e0c52-…", "name": "ci-bridge-vcs-github_checksums.txt", "url": "/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_checksums.txt", "is_checksum": true}
  ]
}
```

| Champ | Contenu |
| --- | --- |
| `tag_name` | La version listée (le vrai numéro pour `latest`). |
| `assets` | Un élément par fichier régulier, trié par nom ; dossiers, liens symboliques et fichiers cachés (`.x`) ignorés. `[]` si vide. |
| `id` | UUID v5 de `product/version/name` : stable d'un appel et d'un redémarrage à l'autre. |
| `url` | URL de téléchargement, nom encodé. Absolue sous `--public-url` s'il est défini, sinon relative. |
| `is_checksum` | Le nom finit par `checksums.txt` ou `checksum.txt`, sans tenir compte de la casse. |

Télécharger tous les fichiers d'une release :

```sh
BASE=http://localhost:8077
curl -s -X POST $BASE/releases/github/latest | jq -r '.assets[] | [.name, .url] | @tsv' |
  while IFS=$'\t' read -r name url; do
    case $url in http*) ;; *) url=$BASE$url ;; esac
    curl -fsS -o "$name" "$url"
  done
```

| Code | Cause |
| --- | --- |
| `404` | Nom invalide, version absente des deux arbres, ou pas de version latest. |
| `405` | Appel en `GET`. |

### `POST /releases/{version}` (dépréciée)

Même réponse que `POST /releases/gitlab/{version}`, plus `Deprecation: true` et `Link: </releases/gitlab/{version}>; rel="successor-version"`. Sera retirée.

### `GET /{product}/artifacts/{version}/files/{name}`

Télécharge un fichier d'une version **publiée**. C'est l'URL écrite dans l'`artifacts.json` réécrit et renvoyée par `/releases`.

```sh
curl -fsSO http://localhost:8077/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_Linux_arm64
curl -fsS -C - -O http://localhost:8077/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_Windows_x86_64.zip   # reprise
```

En-têtes : `Content-Type: application/octet-stream`, `Content-Disposition: attachment; filename=<name>`, `Content-Length`, `Last-Modified`, `Accept-Ranges: bytes`. `Range` et `If-Modified-Since` sont respectés.

`404` (texte) : version non publiée (un fichier en staging n'est jamais servi), `version` vaut `latest`, nom invalide ou caché, fichier absent, dossier, ou lien symbolique qui sort du dossier de la version.

### `GET /assets/binary/{filePath}`

Télécharge un fichier public par son chemin relatif à `--asset-path`, encodé en un seul segment. `GET /assets/checksum/{filename}` est identique.

| Servi | Non servi (`404`, texte) |
| --- | --- |
| `bin.local/assets/…` | staging `bin.local/.uploads*/…` |
| `assets/…` (ancien arbre) | tout autre chemin sous `--asset-path` |
| | tout chemin dont un élément commence par `.` |
| | un lien symbolique qui sort de `--asset-path` (contrôlé par `os.Root` à l'ouverture) |

```sh
curl -fsS -OJ "http://localhost:8077/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgithub%2Fv1.2.3%2Fci-bridge-vcs-github_checksums.txt"
```

Encodez le chemin avec la fonction d'encodage de chemin de votre langage (`encodeURIComponent`, `url.PathEscape`, `urllib.parse.quote(p, safe="")`). Mêmes en-têtes que ci-dessus. Aussi `404` : chemin encodé deux fois ou non normalisé (`a%2F%2Fb`), dossier.

### `GET /artifacts/all`

Parcourt `bin.local/assets/` et renvoie, pour chaque dossier contenant au moins un fichier reconnu, le contenu brut de ces fichiers, en chaînes.

| Champ | Source |
| --- | --- |
| `provider` | Nom du dossier (le produit pour la copie latest, **la version** pour un dossier de version). |
| `artifacts_json` | `artifacts.json` |
| `metadata_json` | `metadata.json` |
| `checksums_text` | fichier finissant par `_checksums.txt` (toute casse) |

```json
[{"provider":"github","artifacts_json":"[…]","metadata_json":"","checksums_text":""},
 {"provider":"v1.2.3","artifacts_json":"[…]","metadata_json":"{\"tag\":\"v1.2.3\",…}","checksums_text":"2b51…  ci-bridge-vcs-github_Linux_arm64\n…"}]
```

Trié par chemin de dossier. Fichier absent ou illisible : `""`. Aucun fichier reconnu : `null`. Le staging n'est pas parcouru. `404` si `bin.local/assets/` n'existe pas ; `500` sur une erreur de lecture pendant le parcours.

### Authentification de l'upload

`PUT` et `publish` exigent `Authorization: Bearer <token>`, comparé à temps constant avant tout autre contrôle.

| Situation | Réponse |
| --- | --- |
| Le serveur n'a pas de token | `403 upload is disabled: no token configured` |
| En-tête absent, autre schéma, token faux | `401 invalid or missing bearer token` + `WWW-Authenticate: Bearer` |

Passez le token à curl par l'entrée standard pour qu'il n'apparaisse pas dans `ps` : `printf 'Authorization: Bearer %s\n' "$TOKEN" | curl -H @- …`.

### `PUT /{product}/artifacts/{version}/files/{name}`

Envoie un fichier dans le staging de `<product>/<version>`. Rien n'est visible avant la [publication](#post-productartifactsversionpublish).

| Paramètre | Où | Description |
| --- | --- | --- |
| `product`, `version` | chemin | `version` ne peut pas valoir `latest`. |
| `name` | chemin | Nom du fichier publié. `artifacts.goreleaser.json`, `release.json` et `index.json` sont réservés. |
| `X-Checksum-Sha256` | en-tête, facultatif | sha256 attendu (hexadécimal, toute casse). Différent : `422`, rien n'est gardé. |
| corps | — | Contenu brut du fichier (`curl -T fichier`). Pas de multipart : `curl -F` enregistrerait l'enveloppe. |

```sh
printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" |
  curl -fsS -H @- -H "X-Checksum-Sha256: $(shasum -a 256 tool.tar.gz | cut -d' ' -f1)" \
    -T tool.tar.gz http://localhost:8077/github/artifacts/v1.2.3/files/tool_1.2.3_linux_amd64.tar.gz
```

`201 Created` : `{"name":"tool_1.2.3_linux_amd64.tar.gz","size":1843202,"sha256":"8f6c…"}`.

- Le corps est écrit en flux dans `bin.local/.uploads-tmp/`, puis déplacé dans le staging une fois complet et vérifié. Un envoi interrompu ou refusé ne laisse rien.
- Renvoyer le même nom remplace le fichier en staging.
- **`artifacts.json` s'envoie en dernier.** À sa réception, il est [contrôlé](#contrôles-dartifactsjson) : chaque fichier qu'il publie doit déjà être en staging, avec le sha256 de son `extra.Checksum`. Le renvoyer le recontrôle.

Erreurs, dans l'ordre des contrôles :

| Code | Message |
| --- | --- |
| `403`, `401` | voir [Authentification de l'upload](#authentification-de-lupload) |
| `400` | `invalid product or version name`, `product name products is reserved`, `version name release.json is reserved`, `invalid file name: use [A-Za-z0-9._-] only`, `artifacts.goreleaser.json is reserved: publishing writes it` |
| `409` | `version github/v1.2.3 is already published and cannot be changed` |
| `422` | `version github/v1.2.3 already has 100 staged files (limit 100)` (remplacer un fichier reste possible) |
| `413` | `file exceeds 209715200 bytes`, `version github/v1.2.3 would exceed its staging quota of 2147483648 bytes` |
| `507` | `upload staging area is full (limit … bytes): publish or abandon pending versions`, `not enough free disk space on the registry` |
| `422` | `sha256 mismatch: got <réel>, X-Checksum-Sha256 says <attendu>` |
| `422` | `artifacts.json …` : voir [Contrôles d'artifacts.json](#contrôles-dartifactsjson) |
| `500` | `internal error` (erreur disque, client déconnecté) |

### `POST /{product}/artifacts/{version}/publish`

Vérifie les fichiers en staging, puis les rend visibles d'un coup. Pas de corps. Une release goreleaser (`artifacts.json`, `metadata.json`, checksums) est vérifiée en entier ; sans goreleaser, `registry.yaml` et les fichiers suffisent.

| Paramètre | Où | Description |
| --- | --- | --- |
| `product`, `version` | chemin | `version` ne peut pas valoir `latest`. |
| `latest` | requête | `true` : en faire aussi la version latest. `true/false`, `1/0`, `t/f` (toute casse). Défaut `false`. |

```sh
printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" |
  curl -fsS -H @- -X POST "http://localhost:8077/github/artifacts/v1.2.3/publish?latest=true"
```

`200 OK` : `{"product":"github","version":"v1.2.3","files":["artifacts.goreleaser.json","artifacts.json",…],"latest":true}` (`files` trié).

**Contrôles.** Tous doivent passer, sinon rien n'est publié et le staging est gardé :

1. Au plus un fichier de checksums (nom finissant par `checksums.txt`, toute casse). Facultatif.
2. `registry.yaml` est en staging. `artifacts.json` et `metadata.json` sont facultatifs.
3. Avec un fichier de checksums : chaque ligne est `<sha256 sur 64 caractères>  <nom>` (`*nom` accepté).
4. Avec un fichier de checksums : chaque fichier en staging y figure avec un sha256 identique, sauf le fichier de checksums, `artifacts.json`, `metadata.json`, `registry.yaml`, `CHANGELOG.md`, `*.sig`, `*.pem`, `*.sigstore.json`. Les lignes désignant un fichier non envoyé sont tolérées.
5. Avec `metadata.json` : un objet JSON avec `tag` ou `version`, et `{version}` vaut `tag`, `version`, ou `v` + `version`.
6. Avec `artifacts.json` : il repasse ses [contrôles](#contrôles-dartifactsjson) (détecte un fichier retiré ou remplacé après lui), et chaque fichier en staging en est une entrée publiée, sauf `artifacts.json`, `metadata.json`, `registry.yaml` et `CHANGELOG.md`.
7. Au moins un artefact : un fichier d'`artifacts.json` ou, sans lui, un fichier en staging autre que `registry.yaml`, `CHANGELOG.md`, `metadata.json`, le fichier de checksums et les pièces jointes (`*.sig`, SBOM…).

**Ensuite**, dans l'ordre :

1. Dans le staging : l'`artifacts.json` envoyé, s'il y en a un, est copié en `artifacts.goreleaser.json` puis remplacé par sa [réécriture](#réécriture-dartifactsjson).
2. Sans fichier de checksums, `checksums.txt` est calculé (`<sha256>  <nom>` par nom, les fichiers du contrôle 4).
3. `release.json` est écrit (voir [SPEC.fr.md](SPEC.fr.md)), puis `checksums.txt`.
4. Le dossier de staging est renommé en `bin.local/assets/binaries/<product>/<version>/` : la version apparaît complète en une opération.
5. Avec `latest=true`, l'`artifacts.json` de la version est copié de façon atomique (fichier temporaire + renommage) dans `<product>/artifacts.json`, ou ce fichier est supprimé si la version n'en a pas ; puis `release.json` dans `<product>/release.json`.
6. `<product>/index.json` est réécrit.

| Code | Message |
| --- | --- |
| `400` | `invalid product or version name`, `latest must be true or false` |
| `409` | `version … is already published and cannot be changed` |
| `404` | `nothing uploaded for github/v1.2.3` |
| `422` | contrôle 1 : `several checksums files: a and b` |
| `422` | contrôle 2 : `registry.yaml not uploaded: every version needs its product card` |
| `422` | contrôles 3–4 : `<fichier>: line 3 is not "<sha256>  <name>"`, `<nom> is not listed in <checksums>`, `<nom>: sha256 is <réel> but <checksums> says <attendu>` |
| `422` | contrôle 5 : `metadata.json is not valid JSON`, `metadata.json has neither tag nor version`, `metadata.json describes v0.2.0 (version 0.2.0), not v1.2.3` |
| `422` | contrôle 6 : voir [contrôles d'artifacts.json](#contrôles-dartifactsjson), `<nom> is uploaded but artifacts.json does not list it` |
| `422` | contrôle 7 : `no artifact to publish: upload at least one file besides registry.yaml` |
| `500` | `internal error`. Si l'erreur survient en mettant à jour latest, la version **est** publiée mais latest n'a pas changé (log : `version published but latest not updated`). |

#### Contrôles d'artifacts.json

Une entrée est un **fichier publié** si son `type` vaut `Archive`, `File`, `Source`, `Linux Package`, `Checksum`, `Signature`, `Certificate`, `SBOM` ou `Metadata`, ou `Binary` **avec** `extra.Format` (`formats: [binary]` de goreleaser). Les autres entrées sont ignorées, notamment `Binary` sans `extra.Format` : la sortie brute du build dont l'archive est faite, absente du fichier de checksums, souvent en plusieurs exemplaires de même nom.

| Règle | Message `422` |
| --- | --- |
| Tableau JSON d'objets, 32 MiB au plus | `artifacts.json is not a JSON array of objects`, `artifacts.json: entry 3 is null`, `artifacts.json is larger than 32 MiB` |
| `name`, `path`, `type` sont des chaînes | `artifacts.json: name, path and type must be strings: …` |
| Chaque fichier publié a un nom unique et [valide](#conventions-de-lapi), autre que `artifacts.json` et `artifacts.goreleaser.json` | `artifacts.json: a Archive entry has no name`, `artifacts.json lists <nom> twice`, `artifacts.json lists "<nom>", a name that cannot be uploaded` |
| Chaque fichier publié est en staging | `artifacts.json references <nom>, which is not uploaded: upload every file first, artifacts.json last` |
| Le sha256 correspond à `extra.Checksum` quand il commence par `sha256:` | `<nom>: sha256 is <réel> but artifacts.json says <attendu>` |

#### Réécriture d'artifacts.json

L'`artifacts.json` publié ne garde que les fichiers publiés, dans l'ordre d'origine. Dans chaque entrée, seul `path` change : il devient l'URL de téléchargement `/{product}/artifacts/{version}/files/{name}`, préfixée par `--public-url` s'il est défini. Les autres champs sont gardés tels qu'envoyés ; les clés sont triées par ordre alphabétique, indentation de 2 espaces.

Entrée envoyée :

```json
{"name": "ci-bridge-vcs-github_Linux_arm64", "path": "bin.local/assets/binaries/github/github_linux_arm64_v8.0/ci-bridge-vcs-github",
 "goos": "linux", "goarch": "arm64", "internal_type": 2, "type": "Binary",
 "extra": {"Binary": "ci-bridge-vcs-github", "Checksum": "sha256:2b51…", "Format": "binary", "ID": "vcs-github-plugin-archive"}}
```

Entrée publiée, avec `--public-url https://registry.example.com` :

```json
{"extra": {"Binary": "ci-bridge-vcs-github", "Checksum": "sha256:2b51…", "Format": "binary", "ID": "vcs-github-plugin-archive"},
 "goarch": "arm64", "goos": "linux", "internal_type": 2, "name": "ci-bridge-vcs-github_Linux_arm64",
 "path": "https://registry.example.com/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_Linux_arm64", "type": "Binary"}
```

Un `path` relatif se résout contre l'adresse où `artifacts.json` a été lu. Définissez `--public-url` derrière un reverse proxy qui change l'hôte ou ajoute un préfixe : le serveur ne se fie jamais à l'en-tête `Host`. Les versions publiées avant l'existence de cette réécriture ne sont pas modifiées.

---

## Produits et standard local-registry/v1

Chaque version porte la fiche `registry.yaml` de son produit (obligatoire), et ses notes de version viennent d'un `CHANGELOG.md` facultatif : celui de goreleaser, ou un fichier au format [Keep a Changelog](https://keepachangelog.com). À la publication, le serveur écrit `release.json` dans la version (fichiers, plateformes, sha256, SBOM et signatures, notes de version) et réécrit `<product>/index.json` (fiche et versions). Le format complet est décrit dans [SPEC.fr.md](SPEC.fr.md), avec le JSON Schema [`schema/registry.schema.json`](schema/registry.schema.json).

```yaml
# registry.yaml, à la racine du projet : envoyé par upload.sh
apiVersion: local-registry/v1
displayName: SonarBridge
description: Pont entre SonarQube et les outils internes.
owners: [{ name: Équipe Plateforme, email: platform@corp.example }]
```

```sh
curl -s http://localhost:8077/products                         # tous les produits
curl -s http://localhost:8077/sonarbridge/index                # fiche et versions
curl -s http://localhost:8077/sonarbridge/releases/latest      # release.json de la version latest
```

- `registry.yaml` est validé à l'upload (`422 registry.yaml: <raison>`) et obligatoire à la publication.
- goreleaser est facultatif. Sans `artifacts.json`, le type de chaque fichier est déduit de son nom (archive, paquet, fichier ; `.sig`, `.pem`, SBOM rattachés au fichier qu'ils couvrent) et `os`/`arch` valent `null` : pour les donner, envoyez un `artifacts.json`, même écrit à la main. Sans `metadata.json`, `tag`, `commit` et `builtAt` valent `null`. Sans fichier de checksums, la publication écrit `checksums.txt`.
- Une version sans `artifacts.json` ou sans `metadata.json` répond `404` sur les routes qui les servent (`/{product}/artifacts…`, `/artifacts/metadatas/…`) : lisez `release.json`.
- `release.json`, `index.json` et `artifacts.goreleaser.json` sont écrits par le serveur : les envoyer donne `400`. Le nom de produit `products` est réservé.
- Les versions publiées avant le standard n'ont pas de `release.json` : elles ne figurent pas dans l'index, et restent disponibles par les autres routes.

## Publier une release

goreleaser lance ses `publishers:` avant d'écrire `artifacts.json` : la publication doit donc s'exécuter **après** `goreleaser release`, par exemple dans la même étape de CI. Sans goreleaser, `upload.sh` publie un dossier de fichiers.

### `scripts/upload.sh` (recommandé)

Envoie chaque fichier publié d'un `artifacts.json` (sous son `name`, lu depuis son `path`), `artifacts.json` en dernier, puis publie. Fonctionne quelle que soit l'organisation de la sortie goreleaser. Avec un dossier sans `artifacts.json`, envoie plutôt chacun de ses fichiers. Requiert bash (3.2+), curl, jq ; `sha256sum` ou `shasum` pour un dossier.

```
upload.sh [--latest] [--dry-run] [--root DIR] [--version V] [--product-file F] <product> [artifacts.json | DIR]
```

| Argument | Description |
| --- | --- |
| `--latest` | Publie avec `latest=true`. |
| `--dry-run` | Affiche ce qui serait envoyé, n'envoie rien ; ni URL ni token requis. Les fichiers sont tout de même contrôlés. |
| `--root DIR` | Dossier auquel les `path` sont relatifs (dossier de travail de goreleaser). Défaut : déduit de l'emplacement d'`artifacts.json` (retrait du dossier du `path` de l'entrée `Metadata` ou `Checksum`), sinon le dossier courant. |
| `--version V` | Version à publier. Défaut : `tag`, sinon `version`, du `metadata.json` listé (de `DIR/metadata.json` pour un dossier). |
| `--product-file F` | Fiche produit envoyée sous le nom `registry.yaml`, obligatoire. Défaut : `registry.yaml` sous la racine (ou `DIR`), sinon dans le dossier courant. Le `CHANGELOG.md` voisin d'`artifacts.json` est aussi envoyé s'il existe. |
| `<product>` | Produit. |
| `[artifacts.json \| DIR]` | Défaut `dist/artifacts.json`. Un `DIR` contenant un `artifacts.json` désigne ce fichier. |

Environnement : `LOCAL_REGISTRY_URL` (`/` final retiré) et `LOCAL_REGISTRY_UPLOAD_TOKEN` (transmis par l'entrée standard de curl).

Il retient les fichiers publiés avec la [règle du serveur](#contrôles-dartifactsjson) et, avant tout envoi, vérifie que les noms sont uniques, que chaque fichier existe et qu'une fiche produit est trouvée. Chaque fichier part avec `X-Checksum-Sha256` quand son entrée a un checksum `sha256:`.

**Dossier sans goreleaser.** Chaque fichier ordinaire de `DIR` part avec `X-Checksum-Sha256`, puis la fiche produit. `DIR` ne doit contenir ni sous-dossier (archivez-le d'abord) ni fichier écrit par le serveur (`release.json`, `index.json`, `artifacts.goreleaser.json`). Le serveur déduit les types et écrit `checksums.txt`.

```sh
tar czf out/gong-db-migrations_1.0.0.tar.gz -C migrations changelog-master.xml changes
scripts/upload.sh --latest --version v1.0.0 --product-file migrations/registry.yaml gong-db-migrations out
```

```
$ scripts/upload.sh --latest github bin.local/assets/binaries/github/artifacts.json
== github v1.2.3 from bin.local/assets/binaries/github/artifacts.json (root /home/moi/sonarbridge-go)
PUT metadata.json <- /home/moi/sonarbridge-go/bin.local/assets/binaries/github/metadata.json
PUT ci-bridge-vcs-github_Linux_arm64 <- /home/moi/sonarbridge-go/bin.local/assets/binaries/github/github_linux_arm64_v8.0/ci-bridge-vcs-github (sha256 2b51…)
…
PUT artifacts.json <- bin.local/assets/binaries/github/artifacts.json
POST publish github/v1.2.3 (latest=true)
{"product":"github","version":"v1.2.3","files":[…],"latest":true}
```

Codes de sortie : `0` publié (ou simulation réussie) ; `1` contrôle local en échec, variable manquante ou refus du serveur (son message est affiché) ; `2` erreur d'usage. Après un échec, corrigez la cause et relancez : les fichiers en staging sont remplacés.

### `scripts/publish-to-local-registry.sh`

```
publish-to-local-registry.sh [--latest] <product> [dist-dir]
```

Lit la version dans le `tag` de `metadata.json`, envoie chaque fichier listé dans le fichier de checksums depuis `dist-dir` (avec `X-Checksum-Sha256`), puis le fichier de checksums, les `*.sig`/`*.pem`/`*.sigstore.json`, `metadata.json`, `registry.yaml` (dossier courant, obligatoire), `dist-dir/CHANGELOG.md` s'il existe, `artifacts.json`, et publie. Sortie goreleaser uniquement, et seulement si chaque fichier du fichier de checksums est à la racine de `dist-dir` sous son nom publié (archives) ; sinon utilisez `upload.sh`. Mêmes variables d'environnement. `--latest` doit venir en premier.

### À la main

```sh
URL=http://localhost:8077 V=v1.2.3
auth() { printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN"; }
while read -r sum name; do
  auth | curl -fsS -H @- -H "X-Checksum-Sha256: $sum" -T "dist/$name" "$URL/github/artifacts/$V/files/$name"
done < dist/tool_checksums.txt
for f in tool_checksums.txt metadata.json registry.yaml artifacts.json; do   # artifacts.json en dernier
  auth | curl -fsS -H @- -T "dist/$f" "$URL/github/artifacts/$V/files/$f"
done
auth | curl -fsS -H @- -X POST "$URL/github/artifacts/$V/publish?latest=true"
```

---

## Quotas et purge du staging

| Limite | Option | Défaut | Au-delà |
| --- | --- | --- | --- |
| Un fichier | `--max-upload-mb` | 200 MiB | `413` |
| Octets en staging d'une version | `--max-version-mb` | 2 GiB | `413` |
| Fichiers en staging d'une version | `--max-version-files` | 100 | `422` |
| Octets en staging de toutes les versions | `--max-staging-mb` | 10 GiB | `507` |
| Espace disque laissé libre | `--min-free-mb` | 1 GiB | `507` |

- Seul le staging compte ; les versions publiées non.
- Remplacer un fichier ne compte que la différence de taille, pas un fichier de plus.
- Sans `Content-Length` (chunked), le contrôle disque suppose la plus grande taille permise par les quotas.
- Les quotas sont vérifiés avant la réception, puis à nouveau sous le verrou d'upload avant l'entrée en staging.
- Le contrôle d'espace libre fonctionne sur les systèmes Unix (Linux, macOS…) et est ignoré ailleurs.

**Purge.** Avec `--staging-ttl` > 0, au démarrage puis toutes les `min(ttl/2, 1h)`, le serveur supprime chaque version en staging dont le fichier le plus récent est plus vieux que le TTL, et les fichiers de `bin.local/.uploads-tmp/` plus vieux que le TTL. Les versions publiées ne sont jamais touchées. Chaque suppression est auditée (`staging_purged`, `tmp_purged`). La purge prend le verrou d'upload.

---

## Journal d'audit

Un objet JSON par ligne, ajouté et synchronisé sur disque avant la réponse au client. Fichier en mode `0600` ; jamais servi ; rotation externe (`logrotate` avec `copytruncate`). Si l'écriture échoue, la requête aboutit quand même et la sortie d'erreur affiche `ERROR audit log write failed`. Le token n'y figure jamais, seulement son `token_id`.

| Champ | Contenu |
| --- | --- |
| `time` | UTC, RFC 3339 avec nanosecondes. |
| `event` | Voir ci-dessous. |
| `remote`, `forwarded_for` | Adresse TCP ; `X-Forwarded-For` tel que reçu. Absents pour la purge. |

| `event` | Quand | Champs |
| --- | --- | --- |
| `auth_failed` | 401/403 sur une route d'upload | `method`, `path`, `status`, `error`, `authorization_sent` |
| `upload` | `PUT` accepté | `product`, `version`, `name`, `size`, `sha256`, `replaced`, `token_id` |
| `upload_refused` | `PUT` refusé après authentification | `product`, `version`, `name`, `status`, `error`, `token_id` |
| `publish` | publication réussie | `product`, `version`, `latest`, `files` (`{name, size, sha256}` de chaque fichier publié : `artifacts.json` réécrit, `artifacts.goreleaser.json` envoyé), `token_id` |
| `publish_refused` | publication refusée après authentification | `product`, `version`, `status`, `error`, `token_id` |
| `staging_purged` | version en staging purgée | `product`, `version`, `files`, `bytes`, `idle` |
| `tmp_purged` | fichiers temporaires purgés | `files` |

Pour un `500`, `error` contient la vraie cause ; le client n'a reçu que `internal error`.

```sh
LOG=/srv/registry/bin.local/audit.log
jq -r 'select(.event=="publish") | "\(.time) \(.product)/\(.version) latest=\(.latest) depuis \(.remote)"' $LOG
jq -r 'select(.event=="auth_failed") | .remote' $LOG | sort | uniq -c | sort -rn
```

---

## Garanties

| Garantie | Mécanisme |
| --- | --- |
| Un fichier en cours de réception n'est jamais visible | Reçu dans `.uploads-tmp/`, renommé dans le staging une fois complet et vérifié. |
| Une version apparaît complète ou pas du tout | La publication renomme le dossier de staging en une opération. |
| Les fichiers publiés correspondent aux checksums | Chaque fichier est vérifié avant publication avec le fichier de checksums envoyé ; sans lui, la publication l'écrit à partir des fichiers en staging. |
| Les `path` d'`artifacts.json` désignent les fichiers de la version | Réécrit dans le staging avant le renommage, après contrôle de chaque fichier listé. |
| Latest change d'un bloc | Nouveau `<product>/release.json` écrit à côté de l'ancien, puis renommé par-dessus, après `<product>/artifacts.json`. |
| Une version publiée ne change plus | `PUT` et publish répondent `409`. |
| Des envois parallèles ne dépassent pas un quota ensemble | Quotas revérifiés sous le verrou d'upload. |

Les renommages sont atomiques parce que `.uploads-tmp`, `.uploads` et `assets` sont sur le même système de fichiers sous `bin.local/` : ne les montez pas séparément. Le verrou est propre au processus : un **seul** serveur d'upload par `--asset-path`.

---

## Opérations

| Tâche | Comment |
| --- | --- |
| Corriger une publication refusée | Le staging est gardé : renvoyez le fichier manquant ou corrigé sous le même nom (puis `artifacts.json` si besoin), relancez la publication ou le script. |
| Abandonner un envoi | Attendre la purge, ou `rm -rf <asset-path>/bin.local/.uploads/<product>/<version>`. |
| Remettre latest sur une ancienne version | Sous `bin.local/assets/binaries/` : `cp <product>/<ancienne>/release.json <product>/release.json`, et `cp <product>/<ancienne>/artifacts.json <product>/artifacts.json` (ou supprimez-le si `<ancienne>` n'en a pas). |
| Retirer une version publiée | Pas d'API. `rm -rf bin.local/assets/binaries/<product>/<version>` (et corriger `<product>/release.json` et `<product>/artifacts.json` si c'était latest). Préférez publier une nouvelle version. |
| Changer le token | Arrêter, changer `LOCAL_REGISTRY_UPLOAD_TOKEN`, relancer. |
| Après un arrêt brutal | Les fichiers `upload-*` de `bin.local/.uploads-tmp/` (purgés après le TTL) et `.tmp-*` de `bin.local/assets/binaries/<product>/` sont ignorés et supprimables quand aucun envoi n'est en cours. |

---

## Bibliothèque Go

```go
import localregistry "sonarbridge-go/local-registry" // paquet local_registry

s := localregistry.NewLocalRegistryServer(8077, "localhost", "/srv/registry")
if err := s.SetTLS("fullchain.pem", "privkey.pem"); err != nil { // facultatif
	log.Fatal(err)
}
s.AddUploadRoutes(localregistry.UploadConfig{
	Token:           os.Getenv("LOCAL_REGISTRY_UPLOAD_TOKEN"), // vide : upload désactivé (403)
	MaxFileSize:     500 << 20,      // octets ; ≤ 0 : DefaultMaxFileSize
	MaxVersionSize:  4 << 30,        // 0 : défaut ; < 0 : sans limite (idem champs suivants)
	MaxVersionFiles: 0,
	MaxStagingSize:  -1,
	MinFreeSpace:    0,
	StagingTTL:      48 * time.Hour, // < 0 : pas de purge
	PublicURL:       "https://registry.example.com",
})
s.AddReleaseRoutes("https://registry.example.com")
s.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")
audit, err := localregistry.OpenAuditLog("/var/log/local-registry/audit.log")
if err != nil {
	log.Fatal(err)
}
defer audit.Close()
s.SetAuditLog(audit)
if err := s.ServeContext(ctx); err != nil { // nil après un arrêt propre
	log.Fatal(err)
}
```

| Fonction | Effet |
| --- | --- |
| `NewLocalRegistryServer(port, addr, assetPath)` | Serveur sans route. |
| `AddRoute(uri, method, f)` / `AddRouteE(uri, method, f)` | Route JSON ; `f` reçoit les variables du chemin dans l'ordre. Erreurs de `AddRouteE` : `*HTTPError` → son code et son message ; `ErrNotFound` ou `fs.ErrNotExist` → `404` ; autre → `500 internal error`. |
| `AddFileRoute(uri, method, varName)` | Sert le fichier public désigné par `varName`, comme `/assets/binary`. |
| `AddUploadRoutes(cfg)` | `PUT …/files/{name}`, `POST …/publish`, `GET …/files/{name}`. |
| `AddReleaseRoutes(publicURL)` | `POST /releases/{product}/{version}`, `POST /releases/{version}`, `GET /artifacts/metadatas/{product}`. |
| `AddProductRoutes()` | `GET /products`, `GET /{product}/index`, `GET /{product}/releases/{version}`. |
| `SetTLS(cert, key) error`, `TLSEnabled() bool` | HTTPS seul, TLS 1.2+. |
| `OpenAuditLog(path)`, `SetAuditLog(a)` | Journal d'audit ; sans lui, les événements vont dans `slog` (message `audit`). |
| `ServeContext(ctx) error` | Sert jusqu'à la fin de `ctx`, puis arrête proprement (10 s) ; lance la purge. |
| `Serve()` | `ServeContext` avec SIGINT/SIGTERM ; quitte le processus en code 1 en cas d'erreur. |
| `Handler() http.Handler` | Les routes sans logs ni délais, pour `httptest`. |
| `NewLayout(root)` | Chemins : `AssetsDir`, `BinariesDir`, `ProductDir`, `VersionDir`, `ArtifactsFile`, `StagingDir`, `UploadsDir`, `UploadTmpDir`, `Root`. Lectures : `LatestVersion(layout, product)`, `ListRelease(layout, product, version, publicURL)`, `ReadMetadata(layout, product, version)`. |
| `ValidName(s)`, `IsPublicFile(name)`, `TokenID(token)` | Règle des noms, règle des routes de fichiers, `token_id` d'audit. |
| `Asset.FindAssetPaths(dir)`, `Asset.FindAssetContents(dir)` | Parcours du catalogue (`/artifacts/all`). |

Constantes : `DefaultMaxFileSize` (200 MiB), `DefaultMaxVersionSize` (2 GiB), `DefaultMaxVersionFiles` (100), `DefaultMaxStagingSize` (10 GiB), `DefaultMinFreeSpace` (1 GiB), `DefaultStagingTTL` (24 h).

---

## Limites connues

- Tout ce qui est sous `bin.local/assets/` et `assets/` est public ; la lecture ne demande aucune authentification.
- Sans TLS, tout circule en clair, token compris.
- Les réponses des routes de lecture, téléchargements compris, doivent se terminer en **20 s** : un gros fichier sur un lien lent est coupé (reprise avec `Range`, `curl -C -`).
- Les liens symboliques ne sont contrôlés que sur les routes de fichiers ; le catalogue et l'upload suivent les liens présents sur le disque.
- Pas de route de suppression, ni pour le staging ni pour une version publiée.
- `/artifacts/all` : `provider` est le nom du dossier (la version pour un dossier de version) ; un fichier illisible donne `""` ; un catalogue vide donne `null` ; seuls les noms en `_checksums.txt` sont reconnus.
- `POST /releases/{version}` est dépréciée (gitlab seulement) et sera retirée.
- Un `<product>/release.json` qui ne nomme aucune version publiée n'en désigne aucune : les recherches de `latest` répondent `404`. Une version publiée avant l'existence de `release.json` ne peut pas être latest.
- Le journal d'audit n'est pas signé ; un échec d'écriture ne fait pas échouer la requête ; `X-Forwarded-For` n'est pas vérifié.
- Un seul serveur d'upload par `--asset-path` (verrou interne au processus).

---

## Dépannage

| Symptôme | Cause et solution |
| --- | --- |
| `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled`, puis `403` | Définissez la variable ou `upload-token` dans l'environnement du serveur ; vérifiez avec `local-registry config`. |
| `401` avec le bon token | Préfixe exact `Bearer ` ; pas d'espace ni de saut de ligne en trop ; serveur relancé après un changement de token. |
| `WARN upload token generation skipped …` | Un token est configuré et reste en service. Retirez-le ou retirez `--disable-generate-token=false`. |
| `unexpected argument "false"` | Écrivez `--disable-generate-token=false`. |
| `unknown flag: --port` | Les options appartiennent à une commande : `local-registry serve --port …`. |
| Un réglage n'a pas la valeur attendue | `local-registry config` montre chaque valeur et sa source ; une variable `LOCAL_REGISTRY_*` oubliée l'emporte sur le fichier. |
| `bind: address already in use`, code 1 | Un autre processus utilise le port : `lsof -i :8077`. |
| `Error: loading TLS certificate`, code 1 | Fichier absent, ou clé qui ne correspond pas au certificat. |
| `curl: (60) SSL certificate problem` | Auto-signé : `curl --cacert cert.pem`, ou `CURL_CA_BUNDLE` pour les scripts. |
| `Client sent an HTTP request to an HTTPS server.` | Utilisez `https://`. |
| `422 artifacts.json references …, which is not uploaded` | `artifacts.json` envoyé avant un fichier qu'il liste : envoyez ce fichier, puis `artifacts.json` à nouveau. `upload.sh` respecte l'ordre. |
| `422 … is uploaded but artifacts.json does not list it` | Fichier en trop en staging (essai précédent, binaire brut du build) : abandonnez le staging et renvoyez, ou utilisez `upload.sh`. |
| `422 … is not listed in …checksums.txt` | Fichier envoyé absent du fichier de checksums : ne l'envoyez pas, faites-le inclure par goreleaser, ou n'envoyez pas de fichier de checksums (la publication en écrit un). |
| `422 registry.yaml not uploaded` | Chaque version a besoin de sa fiche produit : ajoutez `registry.yaml` ([Produits](#produits-et-standard-local-registryv1)), ou `upload.sh --product-file`. |
| `422 no artifact to publish` | Seuls `registry.yaml` (et metadata, changelog, checksums) sont en staging : envoyez les fichiers eux-mêmes. |
| `422 … sha256 is … but … says …` | Mauvais fichier ou fichier modifié : renvoyez le bon. |
| `422 metadata.json describes …, not …` | La version de l'URL diffère du tag goreleaser ; les scripts la lisent dans `metadata.json`. |
| `409 … already published` | Publiez une nouvelle version, ou voir [Opérations](#opérations). |
| `413` / `422 … staged files` / `507` | Relevez le [quota](#quotas-et-purge-du-staging) concerné, ou videz l'ancien staging. |
| `404` sur `/releases/<product>/latest` ou `/artifacts/metadatas/<product>` | Aucune version publiée avec `latest=true`, `<product>/release.json` modifié à la main, ou (metadatas) version latest sans `metadata.json`. |
| `404` sur `/<product>/artifacts` | La version latest a été publiée sans `artifacts.json` : lisez `/<product>/releases/latest`. |
| `404` sur `POST /releases/v1.2.3` | La route dépréciée ne lit que `gitlab` : utilisez `/releases/<product>/v1.2.3`. |
| `404` sur `/assets/binary/…` | Chemin relatif à `--asset-path`, commençant par `bin.local%2Fassets%2F` ou `assets%2F`, chaque `/` en `%2F` ; staging, fichiers cachés et liens sortants ne sont jamais servis. |
| `path` relatifs dans `artifacts.json` | Comportement par défaut. Préfixez par l'adresse du registry, ou définissez `--public-url` (vaut pour les versions publiées ensuite). |
| Fichier téléchargé commençant par `--------------------------` | Envoyé avec `curl -F` : renvoyez-le avec `curl -T`. |
| Téléchargement coupé après 20 s | Voir [Limites connues](#limites-connues) : reprise avec `curl -C -`. |
| `301` inattendu | `..` ou `//` en clair dans l'URL. |
