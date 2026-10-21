---
title: Publish files without goreleaser
sidebar_label: Publish plain files
description: Publish database migrations, configuration bundles or any set of files as a versioned product.
---

goreleaser is optional. This guide publishes a directory of files, such as database migrations, a configuration bundle or documents, as one version of a product. The registry checks and serves these files exactly like a goreleaser release.

## Before you start

- A registry you can reach and its upload token.
- `bash`, `curl`, `jq`, and `sha256sum` or `shasum` on the machine that publishes.
- `scripts/upload.sh`, from a local-registry release archive.

## Steps

The example publishes Liquibase changelogs as product `billing-db-migrations`, version `v1.0.0`.

1. **Prepare one directory with the files to publish.** The registry stores files, not folders: a directory to publish may hold no subdirectory. Archive each folder first.

   ```sh
   mkdir -p out
   tar czf out/billing-db-migrations_1.0.0.tar.gz -C migrations changelog-master.xml changes
   ```

   Name the files the way your consumers will look for them. A name may only use `A-Z a-z 0-9 . _ -`.

2. **Add the product card.** Write a `registry.yaml` in the directory, or keep it elsewhere and pass `--product-file`. See [Write the product card](write-registry-yaml.md).

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
   ci/upload.sh --dry-run --version v1.0.0 --product-file migrations/registry.yaml billing-db-migrations out
   ```

   ```
   == billing-db-migrations v1.0.0 from /home/me/billing/out
   PUT CHANGELOG.md <- /home/me/billing/out/CHANGELOG.md (sha256 d58c…)
   PUT billing-db-migrations_1.0.0.tar.gz <- /home/me/billing/out/billing-db-migrations_1.0.0.tar.gz (sha256 613f…)
   PUT registry.yaml <- /home/me/billing/migrations/registry.yaml (sha256 56b5…)
   POST publish billing-db-migrations/v1.0.0 (latest=true)
   ```

   `--version` is required unless the directory holds a `metadata.json` with a `tag`.

5. **Publish.**

   ```sh
   export LOCAL_REGISTRY_URL=https://registry.example.com
   export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # from your secret store
   ci/upload.sh --latest --version v1.0.0 --product-file migrations/registry.yaml billing-db-migrations out
   ```

## Check the result

```sh
curl -fsS https://registry.example.com/billing-db-migrations/releases/v1.0.0 \
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

- **It writes `checksums.txt`** when you send no checksums file. The file lists the sha256 of every published file, in the format of goreleaser and `sha256sum`. Each upload is also checked on arrival against the sha256 that `upload.sh` sends.
- **It infers each file's kind from its name.** `.tar.gz`, `.tgz`, `.zip` and other archives are `archive`; `.deb`, `.rpm` and `.apk` are `package`; anything else is `file`. A file named after another file plus `.sig`, `.pem` or `.sbom.json` is attached to it as its signature, certificate or SBOM.
- **`tag`, `commit` and `builtAt` are `null`** in `release.json`, because there is no goreleaser `metadata.json`.

The full rules are in [Upload rules](../reference/upload-rules.md).

## Give a platform to each file

Without goreleaser, `os` and `arch` are `null`: the registry does not guess a platform from a file name. When consumers need them, for example to pick the right binary, write an `artifacts.json` by hand and publish it instead of the directory.

```json
[
  { "name": "agent_1.0.0_linux_amd64.tar.gz", "path": "agent_1.0.0_linux_amd64.tar.gz", "type": "Archive", "goos": "linux", "goarch": "amd64" },
  { "name": "agent_1.0.0_windows_amd64.zip", "path": "agent_1.0.0_windows_amd64.zip", "type": "Archive", "goos": "windows", "goarch": "amd64" }
]
```

```sh
ci/upload.sh --latest --version v1.0.0 --product-file registry.yaml agent out/artifacts.json
```

- `name` is the published name, `path` is where `upload.sh` reads the file. Here `path` is relative to the directory of `artifacts.json`. Use `--root` to choose another base directory.
- `type` is one of the goreleaser types, for example `Archive`, `File`, `Linux Package`, `Checksum` or `Signature`.
- Every file in staging must be listed, apart from `registry.yaml`, `CHANGELOG.md` and `metadata.json`.

## If something goes wrong

| Message | What to do |
| --- | --- |
| `… is a directory: archive it first` | Archive the folder with `tar czf`, as in step 1. |
| `no registry.yaml in … nor in the current directory` | Add `registry.yaml` to the directory, or pass `--product-file`. |
| `no metadata.json in …: pass --version` | Pass `--version v1.0.0`. |
| `422 no artifact to publish` | Only the card, the notes or checksums were sent: add the files themselves. |
| `release.json is reserved` | Rename or remove that file: the server writes it. |

More in [Troubleshooting](../troubleshooting.md).
