---
title: CHANGELOG.md
description: How the server turns an uploaded CHANGELOG.md into the release notes of a version.
sidebar_label: CHANGELOG.md
---

`CHANGELOG.md` is the optional source of a version's release notes: the `changes` of its [`release.json`](./release-json.md). It is either the file goreleaser writes (`dist/CHANGELOG.md`) or one the team writes in the [Keep a Changelog](https://keepachangelog.com) format. Both forms are read, and may mix in one file. For advice on writing one, see [Write a changelog](../guides/write-changelog.md).

## Upload rules

- Name: `CHANGELOG.md`, uploaded with the version like any file.
- At most 1 MiB, UTF-8 text. Otherwise the upload answers `422 CHANGELOG.md is larger than 1 MiB` or `422 CHANGELOG.md is not UTF-8 text`.
- It needs no line in the checksums file and no entry in `artifacts.json`.
- It is published with the version and stays downloadable.
- Without it, `changes` is `[]` and `securityUpdates` is `false`.

## Keep a Changelog

Each list item (`- …` or `* …`) under one of these level-3 headings (any case) is a note of that kind, its text kept as is:

| Heading | `kind` |
| --- | --- |
| `### Added` | `added` |
| `### Changed` | `changed` |
| `### Deprecated` | `deprecated` |
| `### Removed` | `removed` |
| `### Fixed` | `fixed` |
| `### Security` | `security` |

A list item under any other heading falls back to the goreleaser rule below.

**Version sections.** When the file has version headings at level 2, only the section of the published version is read:

- A version heading is `## [1.2.0] - 2026-10-18`, `## [1.2.0]`, `## 1.2.0`, `## v1.2.0` or `## [Unreleased]`.
- The section of version `v1.2.0` is the one whose heading names `1.2.0` or `v1.2.0`; it runs to the next level-1 or level-2 heading.
- A version without a section has no notes.
- A file without any version heading is read whole, as the notes of the version it is uploaded with.

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

Published as `v1.0.0`, this file gives:

```json
"changes": [
  { "kind": "added", "description": "orders table" },
  { "kind": "added", "description": "index on users.email" },
  { "kind": "security", "description": "no default password" }
]
```

## goreleaser

Each other list item whose message is a [Conventional Commit](https://www.conventionalcommits.org/) (`type(scope)!: description`) becomes one note. Before reading it, goreleaser's decorations are removed: the leading commit sha (`9c9d766 ` or `9c9d766: `), and a trailing author such as ` (@user)`, ` (Name <mail>)` or ` by @user in <url>`.

| Commit type | `kind` |
| --- | --- |
| `feat`, `add` | `added` |
| `fix`, `bugfix` | `fixed` |
| `perf`, `refactor`, `change` | `changed` |
| `deprecate`, `deprecated` | `deprecated` |
| `remove`, `removed` | `removed` |
| `sec`, `security`, or any type with the scope `security` or `sec` (`fix(security): …`) | `security` |
| any other type marked breaking (`chore!: …`) | `changed` |
| `docs`, `test`, `chore`, `ci`, `build`, `style`… | not a note |

Everything else is ignored: headings, `**Full Changelog**` lines, and messages without a known type.

```markdown
## Changelog
### Features
* 9c9d7664ceefa1b2: feat(api): CSV export (@alice)
* 1a2b3c4: fix: projects beyond 500 (Bob <bob@corp.example>)
* 1a2b3c5: fix(security): bump golang.org/x/net
* 1a2b3c6: docs: typo
```

gives `added` "CSV export", `fixed` "projects beyond 500" and `security` "bump golang.org/x/net".

> [!TIP]
> In `.goreleaser.yaml`, use `changelog.use: git` or `github`, and keep the `feat:`, `fix:`… prefixes in the messages: when goreleaser's `filters` drop them, nothing is left to read.
