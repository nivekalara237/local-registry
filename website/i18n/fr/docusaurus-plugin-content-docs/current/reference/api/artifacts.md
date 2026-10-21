---
title: API des artifacts
description: Lire les fichiers goreleaser d'une version (artifacts.json, metadata.json), le catalogue brut et la sonde de vivacité.
sidebar_label: Artifacts
---

Ces routes servent les fichiers goreleaser d'une version tels qu'ils sont stockés : `artifacts.json` et `metadata.json`. Elles existent pour les clients construits sur les formats de goreleaser. Les nouveaux clients devraient lire [`release.json`](./products.md), que toute version possède, avec ou sans goreleaser. Toutes les routes de cette page sont en lecture seule et ne demandent pas de jeton.

> [!NOTE]
> `artifacts.json` et `metadata.json` sont facultatifs, puisque goreleaser l'est. Une version publiée sans eux répond `404` sur les routes qui les servent.

## Lire l'artifacts.json de la dernière version

`GET /{product}/artifacts`

Renvoie `bin.local/assets/binaries/<product>/artifacts.json` : la copie de l'`artifacts.json` de la dernière version, écrite par `publish?latest=true` (voir [Dernière version](../../concepts/latest.md)).

| Paramètre | Où | Description |
| --- | --- | --- |
| `product` | chemin | Nom du produit. |

```sh
curl -s https://registry.example.com/sonarbridge/artifacts
```

```json
[
  {"internal_type": 36, "name": "metadata.json", "path": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/metadata.json", "type": "Metadata"},
  {"extra": {"Checksum": "sha256:3f1c…", "Format": "tar.gz", "ID": "sonarbridge"}, "goarch": "amd64", "goos": "linux", "internal_type": 1,
   "name": "sonarbridge_1.4.2_linux_amd64.tar.gz", "path": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz", "type": "Archive"}
]
```

Le JSON est renvoyé tel qu'il est stocké (décodé puis réencodé, même contenu). Pour les versions publiées par l'API, c'est le fichier réécrit : chaque `path` est l'URL de téléchargement du fichier (voir [Règles de dépôt](../upload-rules.md)).

| Statut | Cause |
| --- | --- |
| `404` | Nom de produit invalide, aucune version publiée avec `latest=true`, ou dernière version sans `artifacts.json`. |
| `500` | Le fichier stocké n'est pas du JSON valide. |

## Lire l'artifacts.json d'une version

`GET /{product}/artifacts/{version}`

Renvoie `bin.local/assets/binaries/<product>/<version>/artifacts.json`. Avec `latest` comme version, se comporte comme `GET /{product}/artifacts`.

| Paramètre | Où | Description |
| --- | --- | --- |
| `product` | chemin | Nom du produit. |
| `version` | chemin | Version exacte, ou `latest`. |

```sh
curl -s https://registry.example.com/sonarbridge/artifacts/v1.4.2
```

| Statut | Cause |
| --- | --- |
| `404` | Nom invalide, version non publiée (les versions en staging comprises), ou pas d'`artifacts.json` dedans. |
| `500` | Le fichier stocké n'est pas du JSON valide. |

## Lire metadata.json

`GET /artifacts/metadatas/{product}`

Renvoie le `metadata.json` que goreleaser a écrit pour une version publiée.

| Paramètre | Où | Description |
| --- | --- | --- |
| `product` | chemin | Nom du produit. |
| `version` | requête, facultatif | Version exacte. Absent, vide ou `latest` : la dernière version. |

```sh
curl -s https://registry.example.com/artifacts/metadatas/sonarbridge
curl -s "https://registry.example.com/artifacts/metadatas/sonarbridge?version=v1.4.2"
```

```json
{"project_name": "sonarbridge", "tag": "v1.4.2", "previous_tag": "v1.4.1", "version": "1.4.2", "commit": "9c9d7664ceef…", "date": "2026-10-10T20:01:27+02:00", "runtime": {"goos": "linux", "goarch": "amd64"}}
```

| Statut | Cause |
| --- | --- |
| `404` | Nom invalide, version non publiée, pas de `metadata.json` dedans, ou pas de dernière version. |
| `500` | Le fichier stocké n'est pas du JSON valide (son contenu n'est pas renvoyé). |

## Lire le catalogue brut

`GET /artifacts/all`

Parcourt `bin.local/assets/` et renvoie, pour chaque répertoire qui contient au moins un fichier connu, le contenu brut de ces fichiers sous forme de chaînes. C'est une vue bas niveau, conservée pour les clients existants.

```sh
curl -s https://registry.example.com/artifacts/all
```

```json
[
  {"provider": "sonarbridge", "artifacts_json": "[…]", "metadata_json": "", "checksums_text": ""},
  {"provider": "v1.4.2", "artifacts_json": "[…]", "metadata_json": "{\"tag\":\"v1.4.2\",…}", "checksums_text": "3f1c…  sonarbridge_1.4.2_linux_amd64.tar.gz\n…"}
]
```

| Champ | Source |
| --- | --- |
| `provider` | Nom du répertoire : le produit pour sa copie de la dernière version, **la version** pour un répertoire de version. |
| `artifacts_json` | Contenu de `artifacts.json`. |
| `metadata_json` | Contenu de `metadata.json`. |
| `checksums_text` | Contenu du fichier dont le nom finit par `_checksums.txt`, casse ignorée. |

- Les entrées sont triées par chemin de répertoire.
- Un fichier absent ou illisible donne `""`. Aucun fichier connu donne `null` au lieu d'une liste.
- Le staging n'est pas parcouru. Un `checksums.txt` écrit par la publication (sans `_` avant `checksums.txt`) n'est pas reconnu.

| Statut | Cause |
| --- | --- |
| `404` | `bin.local/assets/` n'existe pas. |
| `500` | Erreur de lecture pendant le parcours. |

## Sonde de vivacité

`GET /healthz`

Répond dès que le serveur sert. À utiliser pour le `HEALTHCHECK` Docker, les sondes Kubernetes et les répartiteurs de charge.

```sh
curl -s http://localhost:8077/healthz
```

```json
{"status": "ok"}
```

Elle répond toujours `200` ; elle ne vérifie pas le disque.
