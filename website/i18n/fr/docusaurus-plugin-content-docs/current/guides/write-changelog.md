---
title: Écrire les notes de version
sidebar_label: Écrire les notes de version
description: Donner à chaque version ses notes, à partir du changelog de goreleaser ou d'un fichier Keep a Changelog.
---

Les notes d'une version viennent d'un `CHANGELOG.md` envoyé avec elle. Le registre transforme ce fichier en liste `changes` dans `release.json`, et signale les versions qui contiennent des correctifs de sécurité. Ce guide couvre les deux formats lus : le changelog de goreleaser, et un fichier [Keep a Changelog](https://keepachangelog.com) écrit à la main.

Les notes sont optionnelles. Sans `CHANGELOG.md`, `changes` est une liste vide.

## Quel format choisir

| Vous construisez avec | Utilisez | Écrit par |
| --- | --- | --- |
| goreleaser, avec des [Conventional Commits](https://www.conventionalcommits.org/) | le `dist/CHANGELOG.md` de goreleaser | goreleaser, à partir de vos messages de commit |
| autre chose, ou des notes rédigées | Keep a Changelog | votre équipe, à la main |

Les deux formats peuvent cohabiter dans un même fichier.

## Avec goreleaser

goreleaser écrit `dist/CHANGELOG.md` à chaque release. `upload.sh` l'envoie automatiquement quand il se trouve à côté d'`artifacts.json`.

1. **Écrivez des messages Conventional Commits**, comme `feat(api): CSV export` ou `fix(security): bump golang.org/x/net`.
2. **Laissez goreleaser garder les préfixes.** Dans `.goreleaser.yaml`, lisez le changelog depuis git ou GitHub, et ne filtrez pas les commits `feat:` et `fix:` :

   ```yaml
   changelog:
     use: github # ou git
     sort: asc
     filters:
       exclude:
         - "^docs:"
         - "^test:"
         - "^chore:"
   ```

3. **Faites la release comme d'habitude.** Chaque commit devient une note :

   | Message de commit | Note |
   | --- | --- |
   | `feat(api): CSV export` | `added` : CSV export |
   | `fix: projects beyond 500` | `fixed` : projects beyond 500 |
   | `fix(security): bump golang.org/x/net` | `security` : bump golang.org/x/net |
   | `perf: faster hashing` | `changed` : faster hashing |
   | `docs: typo` | pas une note |

La correspondance complète est dans la [référence de CHANGELOG.md](../reference/changelog-format.md).

> [!WARNING]
> Si une règle `filters` ou un titre de `groups` de goreleaser réécrit les messages sans leur préfixe `feat:` / `fix:`, le registre ne trouve rien à lire, et `changes` reste vide.

## À la main, avec Keep a Changelog

Tenez un seul `CHANGELOG.md` pour tout le produit, la version la plus récente en premier. Sous chaque version, listez les changements sous les titres standard : `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`.

```markdown
# Changelog

## [Unreleased]
### Added
- invoices table

## [1.1.0] - 2026-10-18
### Added
- orders table
### Security
- remove the default password

## [1.0.0] - 2026-09-01
### Added
- users and products tables
```

Quand le fichier a des titres de version, le registre lit **seulement la section de la version publiée**. Publier `v1.1.0` donne deux notes : « orders table » (`added`) et « remove the default password » (`security`). Le titre peut s'écrire avec ou sans `v` et crochets : `## [1.1.0]`, `## 1.1.0` et `## v1.1.0` correspondent tous à `v1.1.0`. La section `Unreleased` n'est jamais publiée.

> [!IMPORTANT]
> Ajoutez la section **avant** de publier. Une version sans section dans le fichier n'a pas de notes, et une version publiée ne peut plus être modifiée.

Un fichier sans aucun titre de version est lu en entier : utilisez cette forme si vous écrivez un petit `CHANGELOG.md` par release.

Envoyez le fichier avec la release :

- dans un dossier publié avec `upload.sh`, placez `CHANGELOG.md` dans le dossier ;
- avec goreleaser, c'est `dist/CHANGELOG.md`, écrit par goreleaser.

## Vérifier le résultat

```sh
curl -fsS https://registry.example.com/sonarbridge/releases/v1.1.0 | jq '{securityUpdates, changes}'
```

```json
{
  "securityUpdates": true,
  "changes": [
    { "kind": "added", "description": "orders table" },
    { "kind": "security", "description": "remove the default password" }
  ]
}
```

`securityUpdates` vaut `true` quand au moins une note est de type `security`. L'indicateur apparaît aussi pour chaque version dans l'`index.json` du produit : les utilisateurs repèrent ainsi les versions à ne pas sauter.

## Limites

- 1 Mio au plus, texte UTF-8. Les fins de ligne Windows sont acceptées.
- Le serveur vérifie la taille et l'encodage à l'upload (`422` sinon), et lit les notes à la publication.
