---
title: Configuration
description: D'où vient chaque réglage, le fichier de configuration, et les réglages à revoir avant la production.
---

Chaque réglage de `local-registry serve` peut venir d'un flag, d'une variable d'environnement ou d'un fichier YAML. Cette page explique comment ils se combinent, comment vérifier le résultat, et quels réglages comptent en production. La liste complète des flags est dans la [référence de la ligne de commande](../reference/cli.md).

## Priorité

Pour chaque réglage, le serveur prend la première source qui le définit :

1. **Flag de la ligne de commande** : `--port 9000`.
2. **Variable d'environnement** : `LOCAL_REGISTRY_` suivi du nom du flag en majuscules, `-` remplacé par `_` : `LOCAL_REGISTRY_PORT=9000`.
3. **Fichier de configuration**, donné par `--config` ou `LOCAL_REGISTRY_CONFIG` : `port: 9000`.
4. **Valeur par défaut.**

```mermaid
flowchart LR
  F[Flag] -->|absent| E[Variable LOCAL_REGISTRY_*] -->|absente| C[Fichier de configuration] -->|absent| D[Défaut]
```

> [!IMPORTANT]
> Une variable `LOCAL_REGISTRY_*` oubliée l'emporte sans bruit sur le fichier. L'image conteneur règle `LOCAL_REGISTRY_ASSET_PATH`, `LOCAL_REGISTRY_ADDRESS` et `LOCAL_REGISTRY_PORT` : dans un conteneur, réglez ces trois-là par l'environnement, pas par le fichier.

Le token d'upload est le seul réglage sans flag, car un flag afficherait le secret dans `ps`. Il vient de `LOCAL_REGISTRY_UPLOAD_TOKEN`, sinon de la clé `upload-token` du fichier.

## Fichier de configuration

Le fichier est en YAML, avec les mêmes clés que les flags, plus `upload-token`. Le serveur ne lit aucun fichier sans `--config` ou `LOCAL_REGISTRY_CONFIG`.

```yaml
# /etc/local-registry/config.yaml (mode 0600 : il contient le token)
asset-path: /srv/registry
address: 0.0.0.0
port: 8443
tls-cert: /etc/local-registry/fullchain.pem
tls-key: /etc/local-registry/privkey.pem
public-url: https://registry.example.com:8443
max-version-mb: 4096
staging-ttl: 48h
audit-log: /var/log/local-registry/audit.log
upload-token: replace-with-the-output-of-openssl-rand-hex-32
```

Le fichier est contrôlé strictement :

- **Les clés inconnues sont refusées**, avec le code de sortie `2` : une faute de frappe comme `max-verion-mb` arrête le serveur au lieu d'être ignorée. Les clés imbriquées aussi.
- **Un token dans un fichier lisible par d'autres utilisateurs** déclenche un avertissement au démarrage : `warning: <file> holds the upload token but is readable by other users (mode -rw-r--r--): chmod 600 <file>`.
- **Les valeurs sont typées.** `port: abc` ou `staging-ttl: 2 days` arrêtent le serveur avec un message clair, au lieu de devenir `0`.

### Chemins

`config`, `asset-path`, `tls-cert`, `tls-key` et `audit-log` développent `$VAR`, `${VAR}`, `~` et `~/`, quelle que soit leur source. C'est utile là où aucun shell ne tourne : un fichier de configuration, une unité systemd, un argument entre apostrophes.

- Une variable non définie est une erreur, pas une chaîne vide : `$HOMEE/registry` ne devient jamais `/registry`.
- `~user` est refusé.
- Les chemins relatifs partent du répertoire de travail du processus, pas du répertoire du fichier. Utilisez des chemins absolus.

## Vérifier la configuration effective

`local-registry config` résout la configuration exactement comme `serve`, la valide, et affiche chaque réglage avec sa source. Elle ne démarre rien : lancez-la avant chaque redémarrage.

```sh
local-registry config --config /etc/local-registry/config.yaml
```

```
SETTING                 VALUE                               SOURCE
config                  /etc/local-registry/config.yaml     flag
asset-path              /srv/registry                       config file
address                 0.0.0.0                             config file
port                    8443                                config file
tls-cert                /etc/local-registry/fullchain.pem   config file
tls-key                 /etc/local-registry/privkey.pem     config file
max-upload-mb           200                                 default
max-version-mb          4096                                config file
max-version-files       50                                  env LOCAL_REGISTRY_MAX_VERSION_FILES
max-staging-mb          10240                               default
min-free-mb             1024                                default
staging-ttl             48h0m0s                             config file
audit-log               /var/log/local-registry/audit.log   config file
public-url              https://registry.example.com:8443   config file
disable-generate-token  true                                default
upload-token            set (token_id e2186dbdb1bb)         config file
```

- `-` dans la colonne `VALUE` signifie vide.
- Le token n'est jamais affiché. Vous voyez s'il est défini et son `token_id`, l'identifiant qu'utilisent les logs. Voir [Sécurité](security.md).

## Réglages à revoir avant la production

| Réglage | Défaut | Pourquoi c'est important |
| --- | --- | --- |
| `asset-path` | aucun, **obligatoire** | Toutes les données y vivent. Le serveur ne se rabat jamais sur le répertoire de travail, ce qui publierait son contenu. |
| `address` | `localhost` | Par défaut, seuls les clients locaux se connectent. Réglez `0.0.0.0` (ou une interface) pour servir le réseau. |
| `port` | `8077` | `0` choisit un port libre : utile en test seulement. |
| `tls-cert`, `tls-key` | aucun | Les deux ou aucun. Avec eux, le serveur ne parle que HTTPS. Sans eux, placez un reverse proxy TLS devant. Voir [Sécurité](security.md). |
| `public-url` | aucun | L'adresse qu'utilisent les clients, par exemple `https://registry.example.com`. Elle rend les URL de téléchargement absolues. Réglez-la dès que le serveur est derrière un proxy ou un autre nom d'hôte. `artifacts.json` et `release.json` sont écrits à la publication : ils ne changent que pour les versions publiées ensuite ; `POST /releases/…` l'utilise tout de suite. |
| `upload-token` | aucun | Sans lui, le registre est en lecture seule : les routes d'upload répondent `403`. |
| `audit-log` | `<asset-path>/bin.local/audit.log` | Déplacez-le vers votre volume de logs si vous les collectez là. Il ne peut pas être dans une arborescence publique. Voir [Journal d'audit](audit-log.md). |
| Quotas et `staging-ttl` | voir [Quotas et staging](quotas-and-staging.md) | Protègent le disque des uploads trop gros ou abandonnés. |

### Token généré pour un essai

`--disable-generate-token=false` fait générer à `serve` un token aléatoire au démarrage quand aucun n'est configuré, et l'affiche sur la sortie standard seulement. Le token change à chaque démarrage. Utilisez-le pour essayer le registre, pas en production : la sortie standard est souvent conservée par systemd, Docker, Kubernetes ou la CI.

Les flags booléens prennent `=` : écrivez `--disable-generate-token=false`, pas `--disable-generate-token false`.

## Erreurs de configuration

Toute erreur de configuration arrête le programme avant qu'il écoute, avec `Error: <message>` et le code de sortie `2`. Les plus courantes :

| Message | Correction |
| --- | --- |
| `asset-path is required (…)` | Réglez `--asset-path`, `LOCAL_REGISTRY_ASSET_PATH` ou `asset-path`. |
| `asset-path <dir> is not an existing directory` | Créez d'abord le répertoire. |
| `tls-cert and tls-key go together` | Réglez les deux, ou aucun. |
| `staging-ttl must be 0 or at least 1h0m0s` | Un upload peut durer 30 minutes : un TTL plus court pourrait le purger en cours. |
| `audit-log … would be downloadable: put it outside bin.local/assets and assets` | Sortez le journal d'audit des arborescences publiques. |
| `public-url "…" must be an http or https URL with no user, query or fragment, …` | Utilisez une adresse simple comme `https://registry.example.com` (un préfixe de chemin est accepté). |
| `config file <file>: unknown setting "…"` | Corrigez le nom de la clé. |

La liste complète est dans la [référence de la ligne de commande](../reference/cli.md).

## Appliquer un changement

Le serveur lit sa configuration et son certificat TLS une fois, au démarrage. Pour appliquer un changement, redémarrez-le. Un redémarrage laisse jusqu'à 10 secondes aux requêtes en cours ; les uploads en staging survivent au redémarrage.
