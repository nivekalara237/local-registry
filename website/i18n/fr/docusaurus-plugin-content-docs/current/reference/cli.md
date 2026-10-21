---
title: Ligne de commande
description: Toutes les commandes, options, variables d'environnement et clés de configuration du binaire local-registry, avec ses codes de sortie.
sidebar_label: Ligne de commande
---

`local-registry` est un binaire unique. `serve` lance le serveur ; `config` affiche la configuration que `serve` utiliserait. Cette page liste tous les réglages ; [Configuration](../operations/configuration.md) explique comment les choisir.

## Commandes

| Commande | Effet |
| --- | --- |
| `serve` | Lance le serveur jusqu'à `SIGINT` ou `SIGTERM`. |
| `config` | Résout et valide la configuration exactement comme `serve`, affiche chaque réglage avec sa source, ne démarre rien. |
| `version` (ou `--version`, `-v`) | Affiche `local-registry v1.4.2 (go1.27.2, linux/amd64, commit 9c9d7664ceef)`. |
| `completion bash\|zsh\|fish\|powershell` | Affiche un script de complétion pour le shell. |
| `help [commande]` (ou `-h`, `--help`) | Aide. `serve --help` liste chaque option avec sa variable d'environnement et sa valeur par défaut. |

`local-registry` seul affiche l'aide.

```sh
local-registry serve --asset-path /srv/registry
local-registry config --config /etc/local-registry/config.yaml
```

## D'où viennent les réglages

Chaque réglage est pris dans la première source qui le définit :

1. l'option de la ligne de commande : `--port 9000` ;
2. la variable d'environnement : `LOCAL_REGISTRY_` + le nom de l'option en majuscules, `-` remplacé par `_` : `LOCAL_REGISTRY_PORT=9000` ;
3. le fichier YAML donné par `--config` (ou `LOCAL_REGISTRY_CONFIG`) : `port: 9000` ;
4. la valeur par défaut.

Les options s'écrivent `--port 8077` ou `--port=8077` ; un seul tiret (`-port`) est refusé. Un booléen prend `=` : `--disable-generate-token=false`.

## Réglages

| Option | Variable d'environnement | Défaut | Description |
| --- | --- | --- | --- |
| `--config` | `LOCAL_REGISTRY_CONFIG` | — | Fichier de configuration YAML. Aucun fichier n'est lu sans elle. |
| `--asset-path` | `LOCAL_REGISTRY_ASSET_PATH` | **obligatoire** | Racine de toutes les données ; doit être un répertoire existant. Le répertoire courant n'est jamais pris par défaut. |
| `--address` | `LOCAL_REGISTRY_ADDRESS` | `localhost` | Adresse d'écoute ; `0.0.0.0` pour toutes les interfaces. |
| `--port` | `LOCAL_REGISTRY_PORT` | `8077` | Port d'écoute, de 0 à 65535 ; `0` choisit un port libre. |
| `--tls-cert` | `LOCAL_REGISTRY_TLS_CERT` | — | Certificat PEM (chaîne). Avec `--tls-key`, le serveur ne parle plus que HTTPS. |
| `--tls-key` | `LOCAL_REGISTRY_TLS_KEY` | — | Clé privée PEM. Les deux réglages TLS ou aucun. |
| `--max-upload-mb` | `LOCAL_REGISTRY_MAX_UPLOAD_MB` | `200` | Plus gros fichier déposé, en Mio. `0` ou moins vaut 200. |
| `--max-version-mb` | `LOCAL_REGISTRY_MAX_VERSION_MB` | `2048` | Octets en staging par version, en Mio. `0` : pas de limite. |
| `--max-version-files` | `LOCAL_REGISTRY_MAX_VERSION_FILES` | `100` | Fichiers en staging par version. `0` : pas de limite. |
| `--max-staging-mb` | `LOCAL_REGISTRY_MAX_STAGING_MB` | `10240` | Octets en staging pour toutes les versions ensemble, en Mio. `0` : pas de limite. |
| `--min-free-mb` | `LOCAL_REGISTRY_MIN_FREE_MB` | `1024` | Espace disque qu'un dépôt doit laisser libre, en Mio. `0` : pas de contrôle. |
| `--staging-ttl` | `LOCAL_REGISTRY_STAGING_TTL` | `24h` | Purge les versions en staging inactives depuis cette durée. Une durée Go : `0` (jamais) ou au moins `1h`. |
| `--audit-log` | `LOCAL_REGISTRY_AUDIT_LOG` | `<asset-path>/bin.local/audit.log` | Fichier du journal d'audit. Refusé dans `bin.local/assets/` ou `assets/`, qui sont téléchargeables. |
| `--public-url` | `LOCAL_REGISTRY_PUBLIC_URL` | — | Adresse par laquelle les clients joignent le serveur, comme `https://registry.example.com` (un préfixe de chemin est permis). Rend absolues les URL de téléchargement écrites par le serveur. Vide : URL relatives. |
| `--disable-generate-token` | `LOCAL_REGISTRY_DISABLE_GENERATE_TOKEN` | `true` | `=false` : génère un jeton de dépôt aléatoire au démarrage quand aucun n'est configuré. |
| *(pas d'option)* | `LOCAL_REGISTRY_UPLOAD_TOKEN` | — | Jeton de dépôt ; clé de configuration `upload-token`. Vide : dépôt désactivé (`403`). |

Les quotas sont expliqués dans [Quotas et staging](../operations/quotas-and-staging.md).

**Chemins.** `config`, `asset-path`, `tls-cert`, `tls-key` et `audit-log` développent `$VAR`, `${VAR}`, `~` et `~/`, quelle que soit leur source : utile là où aucun shell ne tourne (fichier de configuration, unité systemd, apostrophes). Une variable non définie est une erreur, pas une chaîne vide ; `~user` est refusé. Un chemin relatif est résolu depuis le répertoire courant, pas depuis le répertoire du fichier de configuration.

## Fichier de configuration

Les mêmes clés que les options, plus `upload-token`. Une clé inconnue, faute de frappe ou clé imbriquée, est refusée.

```yaml
# /etc/local-registry/config.yaml (chmod 600 : il contient le jeton)
asset-path: /srv/registry
address: 0.0.0.0
port: 8443
tls-cert: /etc/local-registry/fullchain.pem
tls-key: /etc/local-registry/privkey.pem
public-url: https://registry.example.com:8443
max-version-mb: 4096
staging-ttl: 48h
audit-log: /var/log/local-registry/audit.log
upload-token: change-me
```

## Jeton de dépôt

Le jeton est lu dans `LOCAL_REGISTRY_UPLOAD_TOKEN`, sinon dans la clé `upload-token`. Il n'y a pas d'option `--upload-token` : elle apparaîtrait dans la liste des processus.

| Situation | Effet |
| --- | --- |
| Pas de jeton | Les routes de dépôt répondent `403`. Le démarrage affiche `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled` sur stderr. |
| Jeton dans un fichier lisible par d'autres utilisateurs | Le démarrage avertit `warning: <file> holds the upload token but is readable by other users (mode <mode>): chmod 600 <file>`. |
| Jeton défini, serveur à l'écoute sur une adresse autre que loopback, sans TLS | Le démarrage avertit `warning: upload is enabled on <address> without TLS: the token travels in clear text (use --tls-cert/--tls-key or a TLS reverse proxy)` et démarre quand même, pour un reverse proxy TLS. |

Le jeton n'est jamais affiché ni journalisé. Les journaux et le journal d'audit montrent son `token_id` : les 12 premiers chiffres hexadécimaux de son sha256.

```sh
printf %s "$LOCAL_REGISTRY_UPLOAD_TOKEN" | shasum -a 256 | cut -c1-12
```

### Jeton généré

Avec `--disable-generate-token=false` et aucun jeton configuré, `serve` tire un jeton aléatoire (32 octets, 64 chiffres hexadécimaux) et l'affiche sur **stdout uniquement**, avant la bannière. Il n'est enregistré nulle part et change à chaque démarrage.

```
Upload token: 9f2c4e…
  generated for this run only and saved nowhere: it changes at each start (token_id 4ae3c36ce073)
```

Avec un jeton configuré, rien n'est généré, le jeton configuré reste en usage, et stderr affiche :

```
WARN upload token generation skipped: a token is already configured (env LOCAL_REGISTRY_UPLOAD_TOKEN) and stays in use; unset LOCAL_REGISTRY_UPLOAD_TOKEN and upload-token to get a generated one, or drop --disable-generate-token=false
```

> [!WARNING]
> systemd (`journalctl`), Docker, Kubernetes et les exécuteurs de CI peuvent conserver stdout. Dans ces cas, configurez plutôt un jeton fixe.

## Sortie de `config`

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

`-` signifie vide. `upload-token` vaut `not set: upload disabled`, `set (token_id …)`, `generated at startup, printed on stdout only` ou `set (token_id …); generation skipped`. Le jeton lui-même n'est jamais affiché.

## Erreurs de configuration

Toute erreur arrête le programme avant son démarrage, avec le code de sortie `2`, `Error: <message>` et `Run '<command> --help' for usage.` sur stderr.

| Message | Cause |
| --- | --- |
| `asset-path is required (--asset-path, LOCAL_REGISTRY_ASSET_PATH or asset-path in the config file)` | Pas de répertoire de données. |
| `asset-path <dir> is not an existing directory` | Répertoire absent, ou c'est un fichier. |
| `<setting>: environment variable <X> is not set` | Un chemin utilise une variable non définie. |
| `<setting>: <path>: only ~ and ~/ are expanded, not ~user` | `~user` n'est pas pris en charge. |
| `port <n> is out of range 0-65535` | Port invalide. |
| `tls-cert and tls-key go together` | Un seul des deux est défini. |
| `<setting> cannot be negative` | Un quota négatif. |
| `staging-ttl must be 0 or at least 1h0m0s` | La purge pourrait supprimer un dépôt en cours : un dépôt peut durer 30 minutes. |
| `audit-log <file> would be downloadable: put it outside bin.local/assets and assets` | Journal d'audit dans une arborescence publique. |
| `public-url "<url>" must be an http or https URL with no user, query or fragment, such as https://registry.example.com` | `--public-url` invalide. |
| `<setting>: "<value>" is not a whole number` | Nombre illisible dans l'environnement ou le fichier. |
| `staging-ttl: "<value>" is not a duration such as 90m or 24h` | Durée illisible. |
| `disable-generate-token: "<value>" is not true or false` | Booléen illisible. |
| `config file <file>: unknown setting "<key>"` | Clé inconnue dans le fichier. |
| `config file <file>: <reason>` | Fichier absent, illisible ou YAML invalide. |
| `unknown flag: --<x>`, `unexpected argument "<x>" for "local-registry serve"`, `unknown command "<x>" for "local-registry"` | Erreur de ligne de commande. |

## HTTPS

Avec `--tls-cert` et `--tls-key`, le serveur ne parle que HTTPS : TLS 1.2 ou plus, HTTP/2. Une requête HTTP en clair reçoit `400 Client sent an HTTP request to an HTTPS server.` Le certificat est chargé au démarrage ; une paire invalide fait sortir avec le code `1` (`Error: loading TLS certificate: …`). Redémarrez le serveur pour le changer. Voir [Sécurité](../operations/security.md).

## Sortie

Au démarrage, sur stdout :

```
##################################################
##          Local Registry Server               ##
##################################################

Listen      : https://0.0.0.0:8443
Start At    : 2026-10-10 10:56:12.091998 +0200 CEST m=+0.003639167
Assets Path : /srv/registry
```

`Listen` montre le schéma et l'adresse réellement utilisés. Ensuite, une ligne par requête sur stderr :

```
2026/10/10 10:56:15 INFO request method=PUT path=/sonarbridge/artifacts/v1.4.2/files/metadata.json status=201 bytes=112 duration=6.7ms remote=10.0.4.12
```

`remote` est le pair TCP. `forwarded_for` est ajouté quand la requête porte `X-Forwarded-For`, tel qu'envoyé : il n'est pas considéré comme fiable. Une requête en échec ajoute une ligne `WARN` (4xx) ou `ERROR` (5xx) avec la cause exacte ; le client ne reçoit que le message documenté pour la route.

## Codes de sortie

| Situation | Code de sortie |
| --- | --- |
| `SIGINT` ou `SIGTERM` : n'accepte plus de connexions, laisse 10 s aux requêtes en cours, s'arrête | `0` |
| Requêtes encore en cours après 10 s à l'arrêt (`Error: context deadline exceeded`) | `1` |
| Port occupé, mauvaise adresse, paire TLS invalide, journal d'audit impossible à ouvrir | `1` |
| Erreur de commande ou de configuration | `2` |
