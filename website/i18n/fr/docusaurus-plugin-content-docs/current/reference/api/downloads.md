---
title: API de téléchargement
description: Télécharger les fichiers d'une version publiée, ou tout fichier public par son chemin.
sidebar_label: Téléchargements
---

Ces routes téléchargent des fichiers. Utilisez la première pour toute version publiée : c'est l'URL écrite dans `release.json`, dans l'`artifacts.json` réécrit et dans les réponses de `POST /releases`. La seconde sert tout fichier public par son chemin, arborescence historique comprise. Les deux sont en lecture seule et ne demandent pas de jeton.

## Télécharger un fichier d'une version

`GET /{product}/artifacts/{version}/files/{name}`

Télécharge un fichier d'une version **publiée**. Les fichiers en staging ne sont jamais servis.

| Paramètre | Où | Description |
| --- | --- | --- |
| `product` | chemin | Nom du produit. |
| `version` | chemin | Version exacte. `latest` n'est **pas** accepté : résolvez-le d'abord avec [`GET /{product}/releases/latest`](./products.md). |
| `name` | chemin | Nom du fichier, encodé en pourcentage. |
| `Range` | en-tête, facultatif | Plage d'octets, pour reprendre un téléchargement. |
| `If-Modified-Since` | en-tête, facultatif | Téléchargement conditionnel. |

```sh
curl -fsSO https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz
curl -fsS -C - -O https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_windows_amd64.zip   # reprise
```

En-têtes de réponse :

| En-tête | Valeur |
| --- | --- |
| `Content-Type` | `application/octet-stream` |
| `Content-Disposition` | `attachment; filename=<name>` |
| `Content-Length`, `Last-Modified` | Taille et date de modification du fichier. |
| `Accept-Ranges` | `bytes` |

`Range` donne `206 Partial Content` ; `If-Modified-Since` donne `304 Not Modified` quand le fichier n'a pas changé.

| Statut | Cause (corps en texte brut) |
| --- | --- |
| `404` | Version non publiée, `version` vaut `latest`, nom invalide ou caché (`.x`), fichier absent, répertoire, ou lien symbolique qui sort du répertoire de la version. |

> [!TIP]
> Chaque fichier d'une version est listé avec son URL et son sha256 dans son `release.json` : lisez-le plutôt que de construire les URL à la main. Voir [Consommer les versions](../../guides/consume-releases.md).

## Télécharger un fichier public par son chemin

`GET /assets/binary/{filePath}`

Télécharge un fichier public par son chemin relatif au `--asset-path` du serveur, encodé en **un seul** segment d'URL. `GET /assets/checksum/{filename}` est la même route sous un autre nom.

| Paramètre | Où | Description |
| --- | --- | --- |
| `filePath` | chemin | Chemin relatif au répertoire de données, `/` encodé en `%2F`. |

```sh
curl -fsS -OJ "https://registry.example.com/assets/binary/bin.local%2Fassets%2Fbinaries%2Fsonarbridge%2Fv1.4.2%2Fsonarbridge_1.4.2_checksums.txt"
```

Encodez le chemin avec la fonction d'échappement de chemin de votre langage : `encodeURIComponent` en JavaScript, `url.PathEscape` en Go, `urllib.parse.quote(p, safe="")` en Python.

| Servi | Non servi (`404`, texte brut) |
| --- | --- |
| `bin.local/assets/…` (versions publiées) | le staging : `bin.local/.uploads/…`, `bin.local/.uploads-tmp/…` |
| `assets/…` (arborescence historique) | tout autre chemin sous le répertoire de données, comme `bin.local/audit.log` |
| | tout chemin dont un segment commence par `.` |
| | un lien symbolique qui sort du répertoire de données (vérifié à l'ouverture du fichier) |
| | un répertoire |
| | un chemin doublement encodé ou non normalisé (`a%2F%2Fb`) |

Les en-têtes de réponse sont les mêmes que ci-dessus, `Range` compris.
