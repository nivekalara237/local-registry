---
title: Publish files without goreleaser
description: Tutorial — publish database migrations, or any set of files, as a versioned product with release notes.
sidebar_label: First release without goreleaser
---

Not every deliverable is a Go binary. This tutorial publishes Liquibase database migrations as product `billing-db-migrations`, version `v1.0.0`. The same steps work for SQL scripts, Helm charts, configuration bundles, PDF documents or any other files.

All you need is the files and a product card. goreleaser is not involved.

## What you need

- A registry you can reach, and its upload token. Locally, the [Quick start](./quick-start.md) gives you one.
- bash, curl 7.76+, jq, and `sha256sum` or `shasum`.
- `scripts/upload.sh` from the local-registry repository.

Our starting point is a migrations directory:

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
> `upload.sh` sends every file of the directory you give it, and refuses a directory that contains subdirectories. Keep `out/` for what you publish, and nothing else.

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
  curl -fsSL https://registry.example.com/billing-db-migrations/artifacts/v1.0.0/files/billing-db-migrations_1.0.0.tar.gz | tar xz
  liquibase update --changelog-file=changelog-master.xml
```

`install` is free Markdown that portals and people read to know how to use the product.

## 4. Publish

```sh
export LOCAL_REGISTRY_URL=https://registry.example.com
export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # from your secret store

scripts/upload.sh --latest --version v1.0.0 --product-file migrations/registry.yaml billing-db-migrations out
```

Without goreleaser there is no `metadata.json` to read the version from, so you pass it with `--version`. The script uploads each file of `out/` with its sha256, then the product card, then publishes.

## 5. What the server did for you

Look at the published version:

```sh
curl -s https://registry.example.com/billing-db-migrations/releases/v1.0.0 | jq '{changes, checksums: .checksums.name, artifacts: [.artifacts[] | {name, kind, format, os}]}'
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

Three things happened without goreleaser:

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
