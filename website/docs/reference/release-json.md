---
title: release.json
description: Normative reference of release.json, the document the server writes for each published version.
sidebar_label: release.json
---

`release.json` describes one published version: its files with platform, size, sha256 and download URL, its signatures and SBOMs, its checksums file and its release notes. The server writes it at publish, inside the version, so it appears with the version and never changes afterwards. It is the document clients should read; it exists for every version, built with goreleaser or not.

It is served by `GET /{product}/releases/{version}` (`latest` allowed, see [Products API](./api/products.md)) and as a file, `GET /{product}/artifacts/{version}/files/release.json`.

## Example

```json
{
  "apiVersion": "local-registry/v1",
  "product": "sonarbridge",
  "displayName": "SonarBridge",
  "version": "v1.4.2",
  "tag": "v1.4.2",
  "previousTag": "v1.4.1",
  "prerelease": false,
  "commit": "9c9d7664ceefa1b2…",
  "builtAt": "2026-10-10T20:01:27+02:00",
  "publishedAt": "2026-10-10T18:05:02Z",
  "publishedBy": "e2186dbdb1bb",
  "deprecated": false,
  "securityUpdates": true,
  "changes": [
    { "kind": "added", "description": "CSV export" },
    { "kind": "security", "description": "bump golang.org/x/net" }
  ],
  "artifacts": [
    {
      "name": "sonarbridge_1.4.2_linux_amd64.tar.gz",
      "kind": "archive",
      "format": "tar.gz",
      "mediaType": "application/gzip",
      "os": "linux",
      "arch": "amd64",
      "variant": "v1",
      "size": 4821337,
      "sha256": "3f1c…",
      "url": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz",
      "sbom": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz.sbom.json",
      "signature": null,
      "certificate": null
    },
    {
      "name": "migrations.sql",
      "kind": "file",
      "format": "sql",
      "mediaType": "application/sql",
      "os": null,
      "arch": null,
      "variant": null,
      "size": 18213,
      "sha256": "9ab0…",
      "url": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/migrations.sql",
      "sbom": null,
      "signature": null,
      "certificate": null
    }
  ],
  "checksums": {
    "name": "sonarbridge_1.4.2_checksums.txt",
    "url": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_checksums.txt",
    "signature": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_checksums.txt.sigstore.json",
    "certificate": null
  }
}
```

**Every key is always present.** An unknown value is `null`, never a missing key. Clients must ignore keys they do not know: new ones may be added within `local-registry/v1`.

## Version fields

| Field | Type | Source |
| --- | --- | --- |
| `apiVersion` | string | Always `local-registry/v1`. |
| `product` | string | The `{product}` segment of the publish URL. |
| `displayName` | string | `displayName` of `registry.yaml`, else `project_name` of `metadata.json`, else the product. |
| `version` | string | The `{version}` segment of the publish URL (`v1.4.2`): use it in URLs as is. |
| `tag` | string or null | `tag` of `metadata.json`; `null` without it. |
| `previousTag` | string or null | `previous_tag` of `metadata.json`. |
| `prerelease` | boolean | `true` when `version` is a semver pre-release, such as `v1.5.0-rc.1`. |
| `commit` | string or null | `commit` of `metadata.json`. |
| `builtAt` | string or null | `date` of `metadata.json`, as goreleaser wrote it. |
| `publishedAt` | string | Server clock at publish, UTC, RFC 3339. |
| `publishedBy` | string | `token_id` of the upload token: the first 12 hex digits of its sha256, never the token. |
| `deprecated` | boolean or string | `deprecated` of `registry.yaml`: `false`, `true` or a message. |
| `securityUpdates` | boolean | `true` when one of `changes` has kind `security`. |
| `changes` | list | Release notes from `CHANGELOG.md`, in order; `[]` without it. See [CHANGELOG.md](./changelog-format.md). |
| `artifacts` | list | The downloadable files of the version (below). |
| `checksums` | object or null | The checksums file (below). |

Each entry of `changes`:

| Field | Content |
| --- | --- |
| `kind` | `added`, `changed`, `deprecated`, `removed`, `fixed` or `security`. |
| `description` | The note, as one line of text. |

## Artifacts

One entry per file of its own: archives, binaries, packages, other files, source archives. SBOMs, signatures and certificates are not entries: they are attached to the file they cover.

| Field | Type | Content |
| --- | --- | --- |
| `name` | string | File name. |
| `kind` | string | `archive`, `binary`, `package`, `file` or `source` (see Kinds). |
| `format` | string or null | `extra.Format` of `artifacts.json` when set, else the extension without the dot (`tar.gz`, `tar.xz`, `tar.zst` and `tar.bz2` count as one), else `null`. |
| `mediaType` | string | From the end of the name (see Media types). |
| `os` | string or null | `goos` of `artifacts.json`. |
| `arch` | string or null | `goarch` of `artifacts.json`. |
| `variant` | string or null | The first set of `goamd64`, `goarm`, `goarm64`, `go386`, `gomips`, `goppc64`, `goriscv64`. |
| `size` | number | Bytes, measured by the server. |
| `sha256` | string | Computed by the server: the same as in the checksums file. |
| `url` | string | `/{product}/artifacts/{version}/files/{name}`, absolute under `--public-url` when it is set. |
| `sbom` | string or null | URL of the SBOM that covers this file. |
| `signature` | string or null | URL of its signature. |
| `certificate` | string or null | URL of its certificate. |

`os`, `arch` and `variant` are `null` for a file that does not depend on a platform, and for every file of a version published without `artifacts.json`.

### Checksums

| Field | Type | Content |
| --- | --- | --- |
| `name` | string | Name of the checksums file. |
| `url` | string | Its download URL. |
| `signature` | string or null | URL of its signature (for example `<name>.sigstore.json`). |
| `certificate` | string or null | URL of its certificate. |

The checksums file is the `Checksum` entry of `artifacts.json`, else the uploaded `*checksums.txt`, else the `checksums.txt` written by publish. Every version has one.

## Kinds

With an `artifacts.json`, each published entry's goreleaser `type` gives its role:

| goreleaser type | `kind` | Note |
| --- | --- | --- |
| `Archive` | `archive` | `format`: `tar.gz`, `zip`… |
| `Binary` with `extra.Format` | `binary` | A binary published without an archive. |
| `Linux Package` | `package` | `format`: `deb`, `rpm`, `apk`, `archlinux`. |
| `File` | `file` | Any other file (`release.extra_files`): configuration, SQL, PDF, Helm chart… |
| `Source` | `source` | Source archive. |
| `Checksum` | — | Fills `checksums`. |
| `SBOM` | — | Attached as `sbom` to `<name>` when it is named `<name>.sbom.json`, `<name>.spdx.json`, `<name>.cdx.json` or `<name>.sbom`. |
| `Signature` | — | Attached as `signature` to `<name>`, or to the checksums file, when named `<name>.sigstore.json`, `<name>.bundle` or `<name>.sig`. |
| `Certificate` | — | Attached as `certificate` the same way, for `<name>.pem`, `<name>.crt` or `<name>.cert`. |
| `Metadata` | — | `metadata.json`: its values fill the version fields. |
| `Binary` without `extra.Format` | ignored | A build output an archive was made from. |

An attachment that covers no listed file stays downloadable but appears in no entry. When a file has two attachments of the same kind, the first one in `artifacts.json` wins.

### Without artifacts.json

Each staged file is typed from its name, then follows the table above. `registry.yaml`, `CHANGELOG.md`, `metadata.json` and the files the server writes are not entries.

| Name | Type |
| --- | --- |
| The checksums file | `Checksum` |
| Another staged file's name + an SBOM, signature or certificate suffix (above) | `SBOM`, `Signature` or `Certificate` |
| Ends with `.tar.gz`, `.tgz`, `.tar.xz`, `.tar.zst`, `.tar.bz2`, `.tar` or `.zip` | `Archive` |
| Ends with `.deb`, `.rpm` or `.apk` | `Linux Package` |
| Anything else | `File` |

A `.sig` or `.pem` file that covers no other staged file is a plain `File`. To give `os` and `arch`, upload an `artifacts.json`, written by hand if need be, with `name`, `path`, `type`, `goos` and `goarch` for each file.

## Media types

Matched on the end of the name, case ignored, in this order:

| Suffix | `mediaType` |
| --- | --- |
| `.tar.gz`, `.tgz`, `.gz` | `application/gzip` |
| `.tar.xz`, `.xz` | `application/x-xz` |
| `.tar.zst`, `.zst` | `application/zstd` |
| `.tar` | `application/x-tar` |
| `.zip` | `application/zip` |
| `.deb` | `application/vnd.debian.binary-package` |
| `.rpm` | `application/x-rpm` |
| `.exe` | `application/vnd.microsoft.portable-executable` |
| `.msi` | `application/x-msi` |
| `.dmg` | `application/x-apple-diskimage` |
| `.pdf` | `application/pdf` |
| `.sql` | `application/sql` |
| `.yaml`, `.yml` | `application/yaml` |
| `.json` | `application/json` |
| `.sh` | `application/x-sh` |
| `.md` | `text/markdown` |
| `.txt` | `text/plain` |
| anything else | `application/octet-stream` |
