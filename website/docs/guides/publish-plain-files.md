---
title: Publish files without GoReleaser
sidebar_label: Publish plain files
description: Publish database migrations, configuration bundles or any set of files as a versioned product.
---

GoReleaser is optional. This guide publishes a directory of files, such as database migrations, a configuration bundle or documents, as one version of a product. The registry checks and serves these files exactly like a GoReleaser release.

## Before you start

- The registry address and its upload token. If you do not run the registry yourself, ask the person or team responsible for it.
- [`localrectl`](../reference/localrectl.md) on the machine that publishes.

## Steps

The example publishes Liquibase changelogs as product `billing-db-migrations`, version `v1.0.0`.

1. **Prepare one directory with the files to publish.** The registry stores files, not folders: a directory to publish may hold no subdirectory. Archive each subdirectory first, or let `localrectl` do it with `--compress-subdirs`.

   ```sh
   mkdir -p out
   tar czf out/billing-db-migrations_1.0.0.tar.gz -C migrations changelog-master.xml changes
   ```

   Name the files the way your consumers will look for them. A name may only use `A-Z a-z 0-9 . _ -`.

2. **Write the product card.** Keep `registry.yaml` outside the directory to publish, with your sources, and pass it with `--product-file`. See [Write the product card](write-registry-yaml.md).

   ```yaml
   apiVersion: local-registry/v1
   displayName: Billing database migrations
   description: Liquibase changelogs of the billing database.
   license: Proprietary
   owners:
     - name: Billing team
       email: billing@corp.example
   ```

3. **Add release notes (optional).** A `CHANGELOG.md` in the directory becomes the version's notes. See [Write release notes](write-changelog.md).

   ```markdown
   ## [1.0.0] - 2026-10-18
   ### Added
   - users, products and orders tables
   ```

4. **Check what would be sent.**

   ```sh
   localrectl publish billing-db-migrations --root out --version v1.0.0 --no-semver-standard \
     --product-file migrations/registry.yaml --dry-run
   ```

   The dry run shows what would be sent, sends nothing and needs no token. `--version` is required outside a GoReleaser build; `--no-semver-standard` allows the `v` prefix.

5. **Publish.**

   ```sh
   export LOCAL_REGISTRY_URL=https://registry.corp.example
   export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # from your secret store

   localrectl publish billing-db-migrations --root out --version v1.0.0 --no-semver-standard \
     --product-file migrations/registry.yaml --latest
   ```

## Check the result

```sh
curl -fsS https://registry.corp.example/billing-db-migrations/releases/v1.0.0 \
  | jq '{version, changes, checksums: .checksums.name, artifacts: [.artifacts[] | {name, kind, format}]}'
```

```json
{
  "version": "v1.0.0",
  "changes": [{ "kind": "added", "description": "users, products and orders tables" }],
  "checksums": "checksums.txt",
  "artifacts": [{ "name": "billing-db-migrations_1.0.0.tar.gz", "kind": "archive", "format": "tar.gz" }]
}
```

## What the server does for you

- **It writes `checksums.txt`** when you send no checksums file. The file lists the sha256 of every published file, in the format of GoReleaser and `sha256sum`. Each upload that carries a SHA-256 is also checked on arrival.
- **It infers each file's kind from its name.** `.tar.gz`, `.tgz`, `.zip` and other archives are `archive`; `.deb`, `.rpm` and `.apk` are `package`; anything else is `file`. A file named after another file plus `.sig`, `.pem` or `.sbom.json` is attached to it as its signature, certificate or SBOM.
- **`tag`, `commit` and `builtAt` are `null`** in `release.json`, because there is no GoReleaser `metadata.json`.

The full rules are in [Upload rules](../reference/upload-rules.md).

## Give a platform to each file

Without GoReleaser, `os` and `arch` are `null`: the registry does not guess a platform from a file name. When consumers need them, for example to pick the right binary, write an `artifacts.json` by hand, put it in the directory to publish, and publish that directory: `localrectl` then handles it as a GoReleaser build.

```json
[
  { "name": "agent_1.0.0_linux_amd64.tar.gz", "path": "agent_1.0.0_linux_amd64.tar.gz", "type": "Archive", "goos": "linux", "goarch": "amd64" },
  { "name": "agent_1.0.0_windows_amd64.zip", "path": "agent_1.0.0_windows_amd64.zip", "type": "Archive", "goos": "windows", "goarch": "amd64" }
]
```

```sh
localrectl publish agent --root out --version v1.0.0 --no-semver-standard \
  --product-file registry.yaml --latest
```

- `name` is the published name, `path` the location of the file. How `localrectl` resolves `path` is described in its [README](https://github.com/nivekalara237/localrectl/blob/master/README.md).
- `type` is one of the GoReleaser types, for example `Archive`, `File`, `Linux Package`, `Checksum` or `Signature`.
- Every file in staging must be listed, apart from `registry.yaml`, `CHANGELOG.md` and `metadata.json`.

## If something goes wrong

| Situation | What to do |
| --- | --- |
| The directory holds a subdirectory | Archive it with `tar czf`, as in step 1, or add `--compress-subdirs` (or `--skip-subdirs` to ignore it). |
| `--product-file` or `--version` is missing | `localrectl` refuses to publish: add the missing option. |
| `422 no artifact to publish` | Only the card, the notes or checksums were sent: add the files themselves. |
| `release.json is reserved` | Rename or remove that file, or exclude it with `--exclude`: the server writes it. |

More in [Troubleshooting](../troubleshooting.md).
