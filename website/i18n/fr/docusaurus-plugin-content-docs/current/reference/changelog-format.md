---
title: CHANGELOG.md
description: Comment le serveur transforme un CHANGELOG.md déposé en notes de version.
sidebar_label: CHANGELOG.md
---

`CHANGELOG.md` est la source facultative des notes d'une version : les `changes` de son [`release.json`](./release-json.md). C'est soit le fichier qu'écrit goreleaser (`dist/CHANGELOG.md`), soit un fichier que l'équipe écrit au format [Keep a Changelog](https://keepachangelog.com). Les deux formes sont lues, et peuvent se mélanger dans un même fichier. Pour des conseils de rédaction, voir [Écrire un changelog](../guides/write-changelog.md).

## Règles de dépôt

- Nom : `CHANGELOG.md`, déposé avec la version comme n'importe quel fichier.
- 1 Mio au plus, texte UTF-8. Sinon, le dépôt répond `422 CHANGELOG.md is larger than 1 MiB` ou `422 CHANGELOG.md is not UTF-8 text`.
- Il n'a besoin ni d'une ligne dans le fichier de checksums ni d'une entrée dans `artifacts.json`.
- Il est publié avec la version et reste téléchargeable.
- Sans lui, `changes` vaut `[]` et `securityUpdates` vaut `false`.

## Keep a Changelog

Chaque élément de liste (`- …` ou `* …`) placé sous l'un de ces titres de niveau 3 (casse ignorée) est une note de ce type, son texte conservé tel quel :

| Titre | `kind` |
| --- | --- |
| `### Added` | `added` |
| `### Changed` | `changed` |
| `### Deprecated` | `deprecated` |
| `### Removed` | `removed` |
| `### Fixed` | `fixed` |
| `### Security` | `security` |

Un élément de liste placé sous un autre titre relève de la règle goreleaser ci-dessous.

**Sections de version.** Quand le fichier a des titres de version de niveau 2, seule la section de la version publiée est lue :

- Un titre de version s'écrit `## [1.2.0] - 2026-10-18`, `## [1.2.0]`, `## 1.2.0`, `## v1.2.0` ou `## [Unreleased]`.
- La section de la version `v1.2.0` est celle dont le titre nomme `1.2.0` ou `v1.2.0` ; elle court jusqu'au titre suivant de niveau 1 ou 2.
- Une version sans section n'a pas de notes.
- Un fichier sans aucun titre de version est lu en entier, comme les notes de la version avec laquelle il est déposé.

```markdown
# Changelog

## [Unreleased]
### Added
- not shipped yet

## [1.0.0] - 2026-10-18
### Added
- orders table
- index on users.email
### Security
- no default password

## [0.9.0] - 2026-09-01
### Added
- users table
```

Publié en `v1.0.0`, ce fichier donne :

```json
"changes": [
  { "kind": "added", "description": "orders table" },
  { "kind": "added", "description": "index on users.email" },
  { "kind": "security", "description": "no default password" }
]
```

## goreleaser

Chaque autre élément de liste dont le message est un [Conventional Commit](https://www.conventionalcommits.org/) (`type(scope)!: description`) devient une note. Avant la lecture, les décorations de goreleaser sont retirées : le sha du commit en tête (`9c9d766 ` ou `9c9d766: `), et un auteur en fin de ligne comme ` (@user)`, ` (Name <mail>)` ou ` by @user in <url>`.

| Type de commit | `kind` |
| --- | --- |
| `feat`, `add` | `added` |
| `fix`, `bugfix` | `fixed` |
| `perf`, `refactor`, `change` | `changed` |
| `deprecate`, `deprecated` | `deprecated` |
| `remove`, `removed` | `removed` |
| `sec`, `security`, ou tout type de portée `security` ou `sec` (`fix(security): …`) | `security` |
| tout autre type marqué cassant (`chore!: …`) | `changed` |
| `docs`, `test`, `chore`, `ci`, `build`, `style`… | pas une note |

Tout le reste est ignoré : les titres, les lignes `**Full Changelog**` et les messages sans type connu.

```markdown
## Changelog
### Features
* 9c9d7664ceefa1b2: feat(api): CSV export (@alice)
* 1a2b3c4: fix: projects beyond 500 (Bob <bob@corp.example>)
* 1a2b3c5: fix(security): bump golang.org/x/net
* 1a2b3c6: docs: typo
```

donne `added` « CSV export », `fixed` « projects beyond 500 » et `security` « bump golang.org/x/net ».

> [!TIP]
> Dans `.goreleaser.yaml`, utilisez `changelog.use: git` ou `github`, et gardez les préfixes `feat:`, `fix:`… dans les messages : quand les `filters` de goreleaser les retirent, il ne reste rien à lire.
