---
title: index.json
description: Référence normative d'index.json, le document qui décrit un produit et toutes ses versions.
sidebar_label: index.json
---

`index.json` décrit un produit : sa fiche (tirée de `registry.yaml`), sa dernière version, et toutes les versions publiées avec un `release.json`. C'est ce qu'un catalogue ou un portail affiche pour un produit. Le serveur réécrit `bin.local/assets/binaries/<product>/index.json` après chaque publication du produit, et `GET /{product}/index` reconstruit le même document depuis le disque à chaque requête (voir [API des produits](./api/products.md)).

## Exemple

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
  "install": "curl -fsSL https://registry.example.com/… | tar xz\n",
  "latest": "v1.4.2",
  "updatedAt": "2026-10-12T08:00:00Z",
  "versions": [
    { "version": "v1.5.0-rc.1", "tag": "v1.5.0-rc.1", "publishedAt": "2026-10-12T08:00:00Z", "prerelease": true, "securityUpdates": false, "deprecated": false },
    { "version": "v1.4.2", "tag": "v1.4.2", "publishedAt": "2026-10-10T18:05:02Z", "prerelease": false, "securityUpdates": true, "deprecated": false },
    { "version": "v1.4.1", "tag": "v1.4.1", "publishedAt": "2026-09-30T14:22:10Z", "prerelease": false, "securityUpdates": false, "deprecated": "use v1.4.2: CSV export is broken" }
  ]
}
```

Toutes les clés sont toujours présentes. Les clients doivent ignorer les clés qu'ils ne connaissent pas.

## Champs

| Champ | Type | Contenu |
| --- | --- | --- |
| `apiVersion` | chaîne | Toujours `local-registry/v1`. |
| `product` | chaîne | Nom du produit. |
| `displayName` | chaîne | `displayName` du `release.json` de la version qui fournit la fiche. |
| `description`, `license`, `install` | chaîne | Tirés de la fiche ; `""` sans fiche. |
| `owners`, `links`, `keywords` | liste | Tirés de la fiche ; `[]` sans fiche. |
| `labels` | table | Tiré de la fiche ; `{}` sans fiche. |
| `latest` | chaîne ou null | La dernière version, `null` s'il n'y en a pas. |
| `updatedAt` | chaîne | Le `publishedAt` le plus récent parmi les versions, UTC, RFC 3339. |
| `versions` | liste | Toutes les versions qui ont un `release.json` (ci-dessous). |

La fiche est le `registry.yaml` de la version `latest` ; sans dernière version, c'est la fiche de la version publiée en dernier. Les champs de la fiche sont définis dans [registry.yaml](./registry-yaml.md).

Chaque entrée de `versions`, copiée du [`release.json`](./release-json.md) de la version :

| Champ | Type | Contenu |
| --- | --- | --- |
| `version` | chaîne | Nom de la version. |
| `tag` | chaîne ou null | Tag git, tiré de `metadata.json`. |
| `publishedAt` | chaîne | Date de publication, UTC, RFC 3339. |
| `prerelease` | booléen | Pré-version semver. |
| `securityUpdates` | booléen | La version a une note de version `security`. |
| `deprecated` | booléen ou chaîne | `false`, `true` ou un message. |

## Règles

- **Quelles versions.** Seulement celles qui ont un `release.json`. Les versions publiées avant le standard ne sont pas converties : elles restent téléchargeables par les autres routes.
- **Ordre.** Les versions semver d'abord, de la plus récente à la plus ancienne selon la précédence semver (`v1.10.0` avant `v1.2.0`, `v1.0.0` avant `v1.0.0-rc.1`). Les autres noms suivent, du `publishedAt` le plus récent au plus ancien.
- **`latest`.** La `version` désignée par `<product>/release.json`, la copie qu'écrit `publish?latest=true`, à condition que cette version ait un `release.json` ; `null` sinon. Voir [Dernière version](../concepts/latest.md).
- **Fraîcheur.** Le fichier sur disque est réécrit de façon atomique après chaque publication du produit. `GET /{product}/index` ne le lit pas : il reconstruit l'index à chaque requête, donc un `latest` changé à la main apparaît immédiatement.
