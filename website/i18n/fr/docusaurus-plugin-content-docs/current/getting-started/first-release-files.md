---
title: Publier des fichiers sans GoReleaser
description: Tutoriel — publier des migrations de base de données, ou tout ensemble de fichiers, comme un produit versionné avec ses notes de version.
sidebar_label: Première release sans GoReleaser
---

Tous les livrables ne sont pas des binaires Go. Ce tutoriel publie des migrations Liquibase sous le produit `billing-db-migrations`, version `v1.0.0`. Les mêmes étapes valent pour des scripts SQL, des charts Helm, des paquets de configuration, des documents PDF ou tout autre fichier.

Il suffit des fichiers et d'une fiche produit. GoReleaser n'intervient pas.

## Prérequis

- L'adresse du registre et son token d'upload. Si vous n'exploitez pas le registre vous-même, demandez-les à la personne ou à l'équipe qui en est responsable. Pour vous entraîner sur votre machine, le [Démarrage rapide](./quick-start.md) lance un registre d'essai.
- [`localrectl`](../reference/localrectl.md), le client en ligne de commande du registre.
- curl et jq, pour vérifier le résultat.

Le point de départ est un dossier de migrations :

```
migrations/
├── changelog-master.xml
└── changes/
    ├── 001-create-users-table.xml
    ├── 002-create-products-table.xml
    └── 003-create-orders-table.xml
```

## 1. Empaqueter les fichiers

Le registre range les fichiers d'une version côte à côte, sans sous-dossier. Empaquetez tout ce qui a une arborescence dans une archive, dans un dossier de sortie dédié :

```sh
mkdir -p out
tar czf out/billing-db-migrations_1.0.0.tar.gz -C migrations changelog-master.xml changes
```

> [!IMPORTANT]
> `localrectl` envoie tous les fichiers du dossier que vous lui indiquez, et échoue si ce dossier contient un sous-dossier. Réservez `out/` aux fichiers à publier, et à rien d'autre.

## 2. Écrire les notes de version

Ajoutez `out/CHANGELOG.md` au format [Keep a Changelog](https://keepachangelog.com). Le serveur lit la section de la version qu'il publie :

```markdown
# Changelog

## [1.0.0] - 2026-10-18
### Added
- Tables users, products et orders.
### Security
- Plus de mot de passe par défaut pour le compte applicatif.
```

Les notes apparaissent dans le `release.json` de la version. Une section *Security* marque aussi la version `securityUpdates: true`, pour que les consommateurs repèrent les versions à ne pas sauter. Consultez [Écrire les notes de version](../guides/write-changelog.md).

> [!NOTE]
> Les titres de section restent en anglais (`### Added`, `### Fixed`…) : ce sont eux que le serveur reconnaît. Le texte des notes peut être en français.

## 3. Écrire la fiche produit

Créez `registry.yaml`. Gardez-le avec les sources des migrations, pas dans `out/` :

```yaml
apiVersion: local-registry/v1
displayName: Migrations de la base billing
description: Changelogs Liquibase de la base de facturation.
license: Proprietary
owners:
  - name: Équipe facturation
    email: billing@corp.example
keywords: [liquibase, database, billing]
install: |
  curl -fsSL https://registry.corp.example/billing-db-migrations/artifacts/v1.0.0/files/billing-db-migrations_1.0.0.tar.gz | tar xz
  liquibase update --changelog-file=changelog-master.xml
```

`install` est du Markdown libre que les lecteurs consultent pour savoir comment utiliser ou installer le produit.

## 4. Publier

```sh
export LOCAL_REGISTRY_URL=https://registry.corp.example
export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # depuis votre coffre de secrets

localrectl publish billing-db-migrations --root out --version v1.0.0 --no-semver-standard \
  --product-file migrations/registry.yaml --latest
```

Sans GoReleaser, il n'y a pas de `metadata.json` où lire la version : vous la passez avec `--version`. `--no-semver-standard` autorise le préfixe `v`. `localrectl` envoie chaque fichier de `out/` et la fiche produit, puis publie la version.

## 5. Ce que le serveur a fait pour vous

Regardez la version publiée :

```sh
curl -s https://registry.corp.example/billing-db-migrations/releases/v1.0.0 | jq '{changes, checksums: .checksums.name, artifacts: [.artifacts[] | {name, kind, format, os}]}'
```

```json
{
  "changes": [
    { "kind": "added", "description": "Tables users, products et orders." },
    { "kind": "security", "description": "Plus de mot de passe par défaut pour le compte applicatif." }
  ],
  "checksums": "checksums.txt",
  "artifacts": [
    { "name": "billing-db-migrations_1.0.0.tar.gz", "kind": "archive", "format": "tar.gz", "os": null }
  ]
}
```

Trois choses se sont faites sans GoReleaser :

| Quoi | Comment |
| --- | --- |
| **Un fichier de checksums** | Vous n'en avez pas envoyé : le serveur a écrit `checksums.txt` à partir des fichiers qu'il a vérifiés. Les consommateurs contrôlent leurs téléchargements avec, comme pour toute version. |
| **Le type des fichiers** | Déduit des noms : `.tar.gz` est une *archive*, `.deb` un *package*, le reste un *file*. Un `.sig`, `.pem` ou `.sbom.json` qui porte le nom d'un autre fichier lui est rattaché. |
| **Les notes de version** | Lues dans votre `CHANGELOG.md`, section `[1.0.0]`. |

`os` et `arch` valent `null` : une migration ne dépend pas d'une plateforme. Si vos fichiers en dépendent (un outil par OS, par exemple), envoyez un `artifacts.json` qui le précise ; il peut être écrit à la main. Consultez [Publier des fichiers simples](../guides/publish-plain-files.md).

## Pour aller plus loin

- Plus de cas et d'options : [Publier des fichiers simples](../guides/publish-plain-files.md).
- Comment les contrôles et les checksums générés protègent vos consommateurs : [Intégrité](../concepts/integrity.md).
- L'automatiser : [Pipelines CI](../guides/ci-pipelines.md).
