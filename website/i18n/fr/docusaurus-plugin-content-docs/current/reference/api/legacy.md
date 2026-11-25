---
title: API historique
description: Les routes POST /releases qui listent les fichiers bruts d'une version avec leur URL de téléchargement.
sidebar_label: Historique
---

Ces routes listent les fichiers d'une version avec leur URL de téléchargement. Elles renvoient une liste de fichiers brute, sans les informations du [standard local-registry/v1](../../concepts/standard-v1.md). Préférez [`GET /{product}/releases/{version}`](./products.md), qui donne aussi la plateforme, la taille et le sha256 de chaque fichier.

## Lister les fichiers d'une version

`POST /releases/{product}/{version}`

Liste les fichiers d'une version, chacun avec l'URL qui le télécharge. La route est en lecture seule malgré `POST` ; elle ne prend pas de corps et ne demande pas de jeton.

| Paramètre | Où | Description |
| --- | --- | --- |
| `product` | chemin | Nom du produit. |
| `version` | chemin | Version exacte, ou `latest`. |

Les fichiers sont lus dans le premier répertoire qui existe :

| Ordre | Répertoire | `url` de chaque fichier |
| --- | --- | --- |
| 1 | Version publiée `bin.local/assets/binaries/<product>/<version>/` | `/{product}/artifacts/{version}/files/{name}` |
| 2 | Arborescence historique `assets/binaries/<product>/<version>/` | `/assets/binary/assets%2Fbinaries%2F<product>%2F<version>%2F<name>` |

```sh
curl -s -X POST https://registry.corp.example/releases/bridge/latest
```

```json
{
  "tag_name": "v1.4.2",
  "assets": [
    {"id": "4c229d4f-3bb0-5e36-8d08-ae51796b420a", "name": "artifacts.goreleaser.json", "url": "https://registry.corp.example/bridge/artifacts/v1.4.2/files/artifacts.goreleaser.json", "is_checksum": false},
    {"id": "04647fbf-38ae-503e-a543-104c5aad6870", "name": "bridge_1.4.2_checksums.txt", "url": "https://registry.corp.example/bridge/artifacts/v1.4.2/files/bridge_1.4.2_checksums.txt", "is_checksum": true},
    {"id": "dc3de688-c35d-54d9-8e27-23638e997a71", "name": "bridge_1.4.2_linux_amd64.tar.gz", "url": "https://registry.corp.example/bridge/artifacts/v1.4.2/files/bridge_1.4.2_linux_amd64.tar.gz", "is_checksum": false}
  ]
}
```

| Champ | Contenu |
| --- | --- |
| `tag_name` | La version listée : le vrai nom de version quand `latest` a été demandé. |
| `assets` | Une entrée par fichier ordinaire du répertoire, triée par nom, fichiers du serveur compris (`release.json`, `registry.yaml`…). Les répertoires, liens symboliques et fichiers cachés (`.x`) sont ignorés. `[]` quand le répertoire est vide. |
| `id` | UUID v5 de `product/version/name` : stable d'un appel et d'un redémarrage à l'autre. |
| `name` | Nom du fichier. |
| `url` | URL de téléchargement, nom encodé en pourcentage. Absolue sous `--public-url` s'il est défini, relative sinon. |
| `is_checksum` | `true` quand le nom finit par `checksums.txt` ou `checksum.txt`, casse ignorée. |

Télécharger tous les fichiers d'une version :

```sh
BASE=https://registry.corp.example
curl -s -X POST $BASE/releases/bridge/latest | jq -r '.assets[] | [.name, .url] | @tsv' |
  while IFS=$'\t' read -r name url; do
    case $url in http*) ;; *) url=$BASE$url ;; esac
    curl -fsS -o "$name" "$url"
  done
```

| Statut | Cause |
| --- | --- |
| `404` | Nom invalide, version absente des deux répertoires, ou `latest` sans dernière version. |
| `405` | `GET` utilisé au lieu de `POST`. |

## Lister les fichiers d'une version gitlab (dépréciée)

`POST /releases/{version}`

Même réponse que `POST /releases/gitlab/{version}`, pour les clients écrits quand le registre ne servait qu'un produit nommé `gitlab`. La réponse porte deux en-têtes de plus :

```
Deprecation: true
Link: </releases/gitlab/{version}>; rel="successor-version"
```

> [!WARNING]
> Cette route est dépréciée : ses réponses portent l'en-tête `Deprecation: true`. Appelez `POST /releases/gitlab/{version}`, ou mieux `GET /gitlab/releases/{version}`.
