---
title: API des produits
description: Lire les produits, leur index et le release.json de chaque version, tels que les définit le standard local-registry/v1.
sidebar_label: Produits
---

Ces routes exposent les documents du [standard local-registry/v1](../../concepts/standard-v1.md) : la liste des produits et leur recherche, l'`index.json` de chaque produit et le `release.json` de chaque version. C'est le moyen recommandé de découvrir les versions, qu'elles aient été construites avec goreleaser ou non. Elles sont en lecture seule et ne demandent pas de jeton.

Seules les versions qui ont un `release.json` existent pour ces routes. Les versions publiées avant le standard restent disponibles par les routes [artifacts](./artifacts.md) et [historiques](./legacy.md).

## Lister les produits

`GET /products`

Liste tous les produits qui ont au moins une version avec un `release.json`, triés par nom.

```sh
curl -s https://registry.example.com/products
```

```json
[
  {
    "product": "billing-db-migrations",
    "displayName": "Billing DB migrations",
    "description": "Liquibase changelogs of the billing database.",
    "latest": "v1.0.0",
    "updatedAt": "2026-10-18T09:12:44Z"
  },
  {
    "product": "sonarbridge",
    "displayName": "SonarBridge",
    "description": "Bridge between SonarQube and internal tools.",
    "latest": "v1.4.2",
    "updatedAt": "2026-10-12T08:00:00Z"
  }
]
```

| Champ | Contenu |
| --- | --- |
| `product` | Nom du produit : le segment `{product}` de toutes les URL. |
| `displayName` | Nom affiché, comme dans l'[index](../index-json.md) du produit. |
| `description` | Description de la fiche (`registry.yaml`), `""` sans fiche. |
| `latest` | Dernière version, ou `null` quand aucune version n'est la dernière. |
| `updatedAt` | `publishedAt` le plus récent du produit, UTC, RFC 3339. |

Un registre vide répond `[]`. Erreurs : `500` seulement (disque illisible).

## Rechercher des produits

`GET /products/search`

Renvoie, une page à la fois, les produits qui répondent à tous les critères donnés. Sans critère, il renvoie tous les produits. Les règles sont dans le [standard](../../concepts/standard-v1.md), section 7.1 de la spécification.

| Paramètre | Dans | Description |
| --- | --- | --- |
| `q` | query | Texte cherché, sans tenir compte de la casse, dans le nom, le nom affiché, la description et les mots-clés. |
| `keyword` | query | Mot-clé que le produit doit avoir, sans tenir compte de la casse. Répétable : tous les mots-clés doivent correspondre. |
| `label` | query | Label `clé=valeur` que le produit doit avoir, à l'identique. Répétable : tous les labels doivent correspondre. |
| `owner` | query | Nom ou e-mail d'un owner, sans tenir compte de la casse. |
| `license` | query | Licence du produit, sans tenir compte de la casse, comme `Apache-2.0`. |
| `deprecated` | query | `true` ou `false` : la version courante est-elle dépréciée. |
| `prerelease` | query | `true` ou `false` : la version courante est-elle une pré-version. |
| `updatedSince` | query | `YYYY-MM-DD` : produits publiés ce jour-là (UTC) ou après. |
| `sort` | query | `product` (par défaut), `displayName` ou `updatedAt`. Préfixez par `-` pour l'ordre décroissant : `-updatedAt` liste les plus récents d'abord. |
| `limit` | query | Taille de la page, de 1 à 100. Par défaut : 25. |
| `cursor` | query | La valeur `next` de la page précédente. |

La version courante d'un produit est sa dernière version, sinon sa version la plus haute. Un produit qui a une ancienne pré-version mais une dernière version stable n'est pas une pré-version.

```sh
curl -s "https://registry.example.com/products/search?keyword=db&label=team=billing&sort=-updatedAt&limit=1"
```

```json
{
  "apiVersion": "local-registry/v1",
  "items": [
    {
      "product": "billing-db-migrations",
      "displayName": "Billing DB migrations",
      "description": "Liquibase changelogs of the billing database.",
      "latest": "v1.0.0",
      "updatedAt": "2026-10-18T09:12:44Z",
      "keywords": ["db", "liquibase"],
      "labels": { "team": "billing" },
      "license": "Proprietary",
      "deprecated": false
    }
  ],
  "total": 2,
  "limit": 1,
  "next": "eyJrIjoiMjAyNi0xMC0xOFQwOToxMjo0NFoiLCJwIjoiYmlsbGluZy1kYi1taWdyYXRpb25zIiwicSI6IjFhMmIzYzRkNWU2ZjcwODEifQ"
}
```

| Champ | Contenu |
| --- | --- |
| `items` | Les produits de la page : les champs de [`GET /products`](#lister-les-produits), plus `keywords`, `labels`, `license` et `deprecated` (la version courante est-elle dépréciée). |
| `total` | Nombre de produits qui correspondent, toutes pages confondues. |
| `limit` | Taille de page utilisée. |
| `next` | Curseur de la page suivante, `null` sur la dernière page. |

**Pagination.** Pour obtenir la page suivante, renvoyez la même requête avec `cursor=<next>`. La même URL est aussi dans l'en-tête `Link: <…>; rel="next"`, absent sur la dernière page. Une page commence juste après le dernier produit de la précédente : un produit publié ou retiré entre-temps n'est ni répété ni sauté. Gardez les mêmes critères et le même `sort` d'une page à l'autre ; `limit` peut changer. Une boucle qui lit toutes les pages est dans [Consommer les versions](../../guides/consume-releases.md#rechercher-des-produits).

| Statut | Cause |
| --- | --- |
| `400` | Paramètre inconnu, valeur invalide (`limit`, `sort`, `deprecated`, `prerelease`, `label`, `updatedSince`), curseur invalide, ou curseur créé pour d'autres critères ou un autre tri. Le message liste tous les paramètres invalides. |
| `500` | Le disque est illisible. |

## Lire l'index d'un produit

`GET /{product}/index`

Renvoie la fiche du produit et toutes ses versions qui ont un `release.json`. Le serveur sert `<product>/index.json` tant qu'il correspond au disque, et reconstruit le document sinon : une version retirée ou un `latest` changé à la main apparaît immédiatement. Son format est décrit dans [index.json](../index-json.md).

| Paramètre | Où | Description |
| --- | --- | --- |
| `product` | chemin | Nom du produit. |

```sh
curl -s https://registry.example.com/sonarbridge/index
```

```json
{
  "apiVersion": "local-registry/v1",
  "product": "sonarbridge",
  "displayName": "SonarBridge",
  "description": "Bridge between SonarQube and internal tools.",
  "license": "Proprietary",
  "owners": [{ "name": "Platform team", "email": "platform@corp.example" }],
  "links": [{ "name": "source", "url": "https://git.corp.example/platform/sonarbridge" }],
  "keywords": ["sonar", "cli"],
  "labels": { "domain": "quality" },
  "install": "curl -fsSL https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/… | tar xz\n",
  "latest": "v1.4.2",
  "updatedAt": "2026-10-12T08:00:00Z",
  "versions": [
    { "version": "v1.5.0-rc.1", "tag": "v1.5.0-rc.1", "publishedAt": "2026-10-12T08:00:00Z", "prerelease": true, "securityUpdates": false, "deprecated": false },
    { "version": "v1.4.2", "tag": "v1.4.2", "publishedAt": "2026-10-10T18:05:02Z", "prerelease": false, "securityUpdates": true, "deprecated": false }
  ]
}
```

| Statut | Cause |
| --- | --- |
| `404` | Nom de produit invalide, `products`, produit inconnu, ou aucune de ses versions n'a de `release.json`. |
| `500` | Un `release.json` sur le disque n'est pas du JSON valide, ou le disque est illisible. |

## Lire une version

`GET /{product}/releases/{version}`

Renvoie le `release.json` d'une version, tel qu'il est stocké : ses fichiers avec plateforme, taille, sha256 et URL de téléchargement, ses notes de version et son fichier de checksums. Son format est décrit dans [release.json](../release-json.md).

| Paramètre | Où | Description |
| --- | --- | --- |
| `product` | chemin | Nom du produit. |
| `version` | chemin | Version exacte, ou `latest`. |

```sh
curl -s https://registry.example.com/sonarbridge/releases/latest
curl -s https://registry.example.com/billing-db-migrations/releases/v1.0.0
```

```json
{
  "apiVersion": "local-registry/v1",
  "product": "billing-db-migrations",
  "displayName": "Billing DB migrations",
  "version": "v1.0.0",
  "tag": null,
  "previousTag": null,
  "prerelease": false,
  "commit": null,
  "builtAt": null,
  "publishedAt": "2026-10-18T09:12:44Z",
  "publishedBy": "e2186dbdb1bb",
  "deprecated": false,
  "securityUpdates": false,
  "changes": [{ "kind": "added", "description": "orders table" }],
  "artifacts": [
    {
      "name": "billing-db-migrations_1.0.0.tar.gz",
      "kind": "archive",
      "format": "tar.gz",
      "mediaType": "application/gzip",
      "os": null,
      "arch": null,
      "variant": null,
      "size": 2311,
      "sha256": "6c08926d953d80ba627847e1b3eaa893e6040e239a197083f83cda85341baa00",
      "url": "https://registry.example.com/billing-db-migrations/artifacts/v1.0.0/files/billing-db-migrations_1.0.0.tar.gz",
      "sbom": null,
      "signature": null,
      "certificate": null
    }
  ],
  "checksums": {
    "name": "checksums.txt",
    "url": "https://registry.example.com/billing-db-migrations/artifacts/v1.0.0/files/checksums.txt",
    "signature": null,
    "certificate": null
  }
}
```

| Statut | Cause |
| --- | --- |
| `404` | Nom invalide, version non publiée, version sans `release.json`, ou `latest` sans dernière version. |
| `500` | Le `release.json` stocké n'est pas du JSON valide (`<product>/<version>/release.json is not valid JSON`). |

> [!NOTE]
> `GET /{product}/releases/{version}` et l'historique `POST /releases/{product}/{version}` sont deux routes distinctes : la première renvoie le document `release.json`, la seconde liste les fichiers bruts du répertoire de la version.
