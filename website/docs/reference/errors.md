---
title: Errors
description: Every error message the server sends, grouped by phase, with its cause and how to fix it.
sidebar_label: Errors
---

Every error of a JSON route is a status code and a body `{"error": "<message>"}`. The messages below are the server's exact strings; `<…>` marks a value filled in. A `500` always says `internal error`: the real cause is in the server log, and in the audit log for upload routes. For symptoms rather than messages, see [Troubleshooting](../troubleshooting.md).

## Authentication

Upload routes only (`PUT …/files/{name}`, `POST …/publish`).

| Status | Message | Cause | Fix |
| --- | --- | --- | --- |
| `403` | `upload is disabled: no token configured` | The server runs without `LOCAL_REGISTRY_UPLOAD_TOKEN` nor `upload-token`. | Configure a token on the server and restart it. See [Security](../operations/security.md). |
| `401` | `invalid or missing bearer token` | No `Authorization` header, another scheme than `Bearer`, or a wrong token. | Send `Authorization: Bearer <token>` with the server's token. Compare `token_id`s with `local-registry config`. |

## Names

| Status | Message | Cause | Fix |
| --- | --- | --- | --- |
| `400` | `invalid product or version name` | A name outside `[A-Za-z0-9._-]`, `.`, `..`, or the version `latest` on an upload route. | Use a valid name: a git tag such as `v1.4.2` is. |
| `400` | `product name products is reserved` | `products` would collide with `GET /products`. | Pick another product name. |
| `400` | `version name <version> is reserved` | `artifacts.json`, `artifacts.goreleaser.json`, `release.json` or `index.json` as a version. | Pick another version name. |
| `400` | `invalid file name: use [A-Za-z0-9._-] only` | A file name with spaces, `/`, or other characters. | Rename the file. |
| `400` | `<name> is reserved: publishing writes it` | Upload of `artifacts.goreleaser.json`, `release.json` or `index.json`. | Do not send it: the server writes it. |
| `400` | `latest must be true or false` | `publish?latest=` with another value. | Use `true` or `false`. |
| `404` | `not found` | On read routes, an invalid name gives `404`. | Check the name. |

## State of the version

| Status | Message | Cause | Fix |
| --- | --- | --- | --- |
| `409` | `version <product>/<version> is already published and cannot be changed` | A published version is immutable. | Publish a new version. |
| `404` | `nothing uploaded for <product>/<version>` | Publish called before any upload, or after the staging purge. | Upload the files, then publish. |

## Quotas and disk

Raised while uploading. See [Quotas and staging](../operations/quotas-and-staging.md).

| Status | Message | Cause | Fix |
| --- | --- | --- | --- |
| `413` | `file exceeds <bytes> bytes` | A file over `--max-upload-mb`. | Raise the limit, or split the file. |
| `413` | `version <product>/<version> would exceed its staging quota of <bytes> bytes` | The version's staged files would exceed `--max-version-mb`. | Raise the limit, or publish fewer files. |
| `422` | `version <product>/<version> already has <n> staged files (limit <n>)` | More files than `--max-version-files`. Replacing a staged file is still allowed. | Raise the limit, or archive files together. |
| `507` | `upload staging area is full (limit <bytes> bytes): publish or abandon pending versions` | All staged versions together exceed `--max-staging-mb`. | Publish pending versions, delete abandoned ones, or raise the limit. |
| `507` | `not enough free disk space on the registry` | The upload would leave less than `--min-free-mb` free. | Free disk space or grow the volume. |

## File content

Raised when the file is uploaded.

| Status | Message | Cause | Fix |
| --- | --- | --- | --- |
| `422` | `sha256 mismatch: got <actual>, X-Checksum-Sha256 says <expected>` | The bytes received are not the ones announced: corrupted transfer, or wrong file. | Send the file again; check the local file. |
| `422` | `registry.yaml: <reason>`, `registry.yaml is larger than 64 KiB` | Invalid product card. | Fix the file: every reason is listed in [registry.yaml](./registry-yaml.md). |
| `422` | `CHANGELOG.md is larger than 1 MiB` | Release notes too large. | Keep only recent versions, or the section of this version. |
| `422` | `CHANGELOG.md is not UTF-8 text` | A binary file, or another encoding. | Save it as UTF-8. |

### artifacts.json

Raised when `artifacts.json` is uploaded, and again at publish. See [Upload rules](./upload-rules.md).

| Status | Message | Cause | Fix |
| --- | --- | --- | --- |
| `422` | `artifacts.json is larger than 32 MiB` | Too large. | — |
| `422` | `artifacts.json is not a JSON array of objects` | Not goreleaser's format. | Send `dist/artifacts.json` as goreleaser wrote it. |
| `422` | `artifacts.json: entry <i> is null` | A `null` in the array. | Remove it. |
| `422` | `artifacts.json: name, path and type must be strings: <reason>` | A field of the wrong type. | Fix the entry. |
| `422` | `artifacts.json: a <type> entry has no name` | A published entry without `name`. | Fix the entry. |
| `422` | `artifacts.json lists <name> twice` | Two published files with one name. | Rename them in `.goreleaser.yaml`. |
| `422` | `artifacts.json lists "<name>", a name that cannot be uploaded` | An invalid or reserved name. | Rename the file. |
| `422` | `artifacts.json references <name>, which is not uploaded: upload every file first, artifacts.json last` | `artifacts.json` sent before a file it lists, or that file is missing. | Upload the file, then `artifacts.json` again. |
| `422` | `<name>: sha256 is <actual> but artifacts.json says <expected>` | The staged file is not the one goreleaser built. | Upload the right file. |

## Publish

Raised by `POST …/publish`, in this order. Nothing is published; staging is kept.

| Status | Message | Cause | Fix |
| --- | --- | --- | --- |
| `422` | `several checksums files: <a> and <b>` | Two files ending in `checksums.txt`. | Keep one. |
| `422` | `registry.yaml not uploaded: every version needs its product card` | No product card. | Upload `registry.yaml`, then publish again. |
| `422` | `<checksums>: line <n> is not "<sha256>  <name>"` | A malformed line in the checksums file. | Send the file goreleaser or `sha256sum` wrote. |
| `422` | `<name> is not listed in <checksums>` | A staged file that the checksums file does not cover. | Add it to the checksums, or do not upload it; or send no checksums file and let publish write one. |
| `422` | `<name>: sha256 is <actual> but <checksums> says <expected>` | The staged file differs from the checksums. | Upload the right file. |
| `422` | `metadata.json is not valid JSON` | Corrupted `metadata.json`. | Send goreleaser's file, or none. |
| `422` | `metadata.json has neither tag nor version` | Not goreleaser's `metadata.json`. | Send goreleaser's file, or none. |
| `422` | `metadata.json describes <tag> (version <v>), not <version>` | `metadata.json` of another build. | Publish under the version goreleaser built, or send the right file. |
| `422` | `<name> is uploaded but artifacts.json does not list it` | With `artifacts.json`, a staged file it does not publish. | Remove the cause: list the file (`release.extra_files`) or do not upload it. |
| `422` | `no artifact to publish: upload at least one file besides registry.yaml` | Only the card, notes, metadata, checksums or attachments were staged. | Upload the files of the release. |
| `500` | `internal error` | Disk or permission error. When the server log says `version published but latest not updated` or `version published but index.json not updated`, the version **is** published. | Read the server log; see [Upload rules](./upload-rules.md). |

## Read routes

| Status | Message | Cause |
| --- | --- | --- |
| `404` | `not found` | Unknown product or version, no latest version, a version without the requested file (`artifacts.json`, `metadata.json`, `release.json`), or an invalid name. |
| `404` | `404 page not found` (plain text) | Unknown path. |
| `404` | `404 page not found` (plain text) | On download routes: file missing, staged, hidden, outside the public trees, or `latest` used as the version. |
| `405` | `Method Not Allowed` (plain text) | Wrong method, such as `GET /releases/…`. The `Allow` header gives the right one. |
| `500` | `<product>/<version>/release.json is not valid JSON` | A `release.json` damaged on disk. |
| `500` | `internal error` | Unreadable disk, or a stored `artifacts.json` or `metadata.json` that is not valid JSON. |

## Server startup

Startup errors are written to stderr as `Error: <message>`, and the process exits with code `2` (configuration) or `1` (runtime). They are listed in [Command line](./cli.md).
