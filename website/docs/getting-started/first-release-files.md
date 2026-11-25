---
title: Publish files without GoReleaser
description: Tutorial — publish database migrations, or any set of files, as a versioned product with release notes.
sidebar_label: First release without GoReleaser
---

Not every deliverable is a Go binary. This tutorial publishes Liquibase database migrations as product `billing-db-migrations`, version `v1.0.0`. The same steps work for SQL scripts, Helm charts, configuration bundles, PDF documents or any other files.

All you need is the files and a product card. GoReleaser is not involved.

## What you need

- The registry address and its upload token. If you do not run the registry yourself, ask the person or team responsible for it. To practise on your machine, the [Quick start](./quick-start.md) runs a trial registry.
- [`localrectl`](../reference/localrectl.md), the command-line client of the registry.
- curl and jq, to check the result.

The starting point is a migrations directory:

```
migrations/
├── changelog-master.xml
└── changes/
    ├── 001-create-users-table.xml
    ├── 002-create-products-table.xml
    └── 003-create-orders-table.xml
```

## 1. Package the files

The registry stores the files of a version side by side, without subdirectories. Pack anything that has a tree into an archive, in an output directory of its own:

```sh
mkdir -p out
tar czf out/billing-db-migrations_1.0.0.tar.gz -C migrations changelog-master.xml changes
```

> [!IMPORTANT]
> `localrectl` sends every file of the directory you give it, and fails if that directory contains a subdirectory. Keep `out/` for the files to publish, and nothing else.

## 2. Write the release notes

Add `out/CHANGELOG.md` in the [Keep a Changelog](https://keepachangelog.com) format. The server reads the section of the version it publishes:

```markdown
# Changelog

## [1.0.0] - 2026-10-18
### Added
- Users, products and orders tables.
### Security
- No default password for the application account.
```

The notes appear in the version's `release.json`. A *Security* section also flags the version with `securityUpdates: true`, so consumers can spot versions they should not skip. See [Write release notes](../guides/write-changelog.md).

## 3. Write the product card

Create `registry.yaml`. Keep it with the migrations' sources, not in `out/`:

```yaml
apiVersion: local-registry/v1
displayName: Billing database migrations
description: Liquibase changelogs of the billing database.
license: Proprietary
owners:
  - name: Billing team
    email: billing@corp.example
keywords: [liquibase, database, billing]
install: |
  curl -fsSL https://registry.corp.example/billing-db-migrations/artifacts/v1.0.0/files/billing-db-migrations_1.0.0.tar.gz | tar xz
  liquibase update --changelog-file=changelog-master.xml
```

`install` is free Markdown that readers consult to know how to use or install the product.

## 4. Publish

```sh
export LOCAL_REGISTRY_URL=https://registry.corp.example
export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # from your secret store

localrectl publish billing-db-migrations --root out --version v1.0.0 --no-semver-standard \
  --product-file migrations/registry.yaml --latest
```

Without GoReleaser there is no `metadata.json` to read the version from, so you pass it with `--version`. `--no-semver-standard` allows the `v` prefix. `localrectl` uploads each file of `out/` and the product card, then publishes the version.

## 5. What the server did for you

Look at the published version:

```sh
curl -s https://registry.corp.example/billing-db-migrations/releases/v1.0.0 | jq '{changes, checksums: .checksums.name, artifacts: [.artifacts[] | {name, kind, format, os}]}'
```

```json
{
  "changes": [
    { "kind": "added", "description": "Users, products and orders tables." },
    { "kind": "security", "description": "No default password for the application account." }
  ],
  "checksums": "checksums.txt",
  "artifacts": [
    { "name": "billing-db-migrations_1.0.0.tar.gz", "kind": "archive", "format": "tar.gz", "os": null }
  ]
}
```

Three things happened without GoReleaser:

| What | How |
| --- | --- |
| **A checksums file** | You uploaded none, so the server wrote `checksums.txt` from the files it verified. Consumers check downloads against it as with any release. |
| **File kinds** | Inferred from the names: `.tar.gz` is an *archive*, `.deb` a *package*, anything else a *file*. A `.sig`, `.pem` or `.sbom.json` named after another file is attached to it. |
| **Release notes** | Read from your `CHANGELOG.md`, section `[1.0.0]`. |

`os` and `arch` are `null`: a migration does not depend on a platform. If your files do (one tool per OS, for instance), upload an `artifacts.json` that says so; it can be written by hand. See [Publish plain files](../guides/publish-plain-files.md).

## Next steps

- More cases and options: [Publish plain files](../guides/publish-plain-files.md).
- How the checks and the generated checksums protect your consumers: [Integrity](../concepts/integrity.md).
- Automate it: [CI pipelines](../guides/ci-pipelines.md).
