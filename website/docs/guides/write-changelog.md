---
title: Write release notes
sidebar_label: Write release notes
description: Give each version its release notes, from goreleaser's changelog or from a Keep a Changelog file.
---

A version's release notes come from a `CHANGELOG.md` uploaded with it. The registry turns that file into the `changes` list of `release.json`, and flags versions that contain security fixes. This guide covers the two formats it reads: goreleaser's changelog, and a hand-written [Keep a Changelog](https://keepachangelog.com) file.

The notes are optional. Without `CHANGELOG.md`, `changes` is an empty list.

## Which format to use

| You build with | Use | Written by |
| --- | --- | --- |
| goreleaser, with [Conventional Commits](https://www.conventionalcommits.org/) | goreleaser's `dist/CHANGELOG.md` | goreleaser, from your commit messages |
| anything else, or curated notes | Keep a Changelog | your team, by hand |

Both formats can mix in one file.

## From goreleaser

goreleaser writes `dist/CHANGELOG.md` at each release. `upload.sh` sends it automatically when it sits beside `artifacts.json`.

1. **Write Conventional Commit messages**, such as `feat(api): CSV export` or `fix(security): bump golang.org/x/net`.
2. **Let goreleaser keep the prefixes.** In `.goreleaser.yaml`, read the changelog from git or GitHub, and do not filter out the `feat:` and `fix:` commits:

   ```yaml
   changelog:
     use: github # or git
     sort: asc
     filters:
       exclude:
         - "^docs:"
         - "^test:"
         - "^chore:"
   ```

3. **Release as usual.** Each commit becomes one note:

   | Commit message | Note |
   | --- | --- |
   | `feat(api): CSV export` | `added`: CSV export |
   | `fix: projects beyond 500` | `fixed`: projects beyond 500 |
   | `fix(security): bump golang.org/x/net` | `security`: bump golang.org/x/net |
   | `perf: faster hashing` | `changed`: faster hashing |
   | `docs: typo` | not a note |

The full mapping is in the [CHANGELOG.md reference](../reference/changelog-format.md).

> [!WARNING]
> If a goreleaser `filters` rule or a `groups` title rewrites the messages without their `feat:` / `fix:` prefix, the registry finds nothing to read, and `changes` is empty.

## By hand, with Keep a Changelog

Keep one `CHANGELOG.md` for the whole product, newest version first. Under each version, list the changes under the standard headings: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`.

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

When the file has version headings, the registry reads **only the section of the version being published**. Publishing `v1.1.0` gives two notes: "orders table" (`added`) and "remove the default password" (`security`). The heading may be written with or without `v` and brackets: `## [1.1.0]`, `## 1.1.0` and `## v1.1.0` all match `v1.1.0`. The `Unreleased` section is never published.

> [!IMPORTANT]
> Add the section **before** you publish. A version without a section in the file gets no notes, and a published version cannot be changed.

A file without any version heading is read whole: use that form when you write one small `CHANGELOG.md` per release.

Send the file with the release:

- in a directory published with `upload.sh`, put `CHANGELOG.md` in the directory;
- with goreleaser, it is `dist/CHANGELOG.md`, written by goreleaser.

## Check the result

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

`securityUpdates` is `true` when at least one note has the `security` kind. It also shows for each version in the product's `index.json`, so consumers can spot versions they should not skip.

## Limits

- At most 1 MiB, UTF-8 text. Windows line endings are accepted.
- The server checks size and encoding at upload (`422` otherwise), and reads the notes at publish.
