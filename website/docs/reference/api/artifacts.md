---
title: Artifacts API
description: Read the goreleaser files of a release (artifacts.json, metadata.json), the raw catalogue, and the liveness probe.
sidebar_label: Artifacts
---

These routes serve the goreleaser files of a release as they are stored: `artifacts.json` and `metadata.json`. They exist for clients built on goreleaser's formats. New clients should read [`release.json`](./products.md), which every version has, goreleaser or not. All routes here are read-only and need no token.

> [!NOTE]
> `artifacts.json` and `metadata.json` are optional since goreleaser is optional. A version published without them answers `404` on the routes that serve them.

## Get the latest artifacts.json

`GET /{product}/artifacts`

Returns `bin.local/assets/binaries/<product>/artifacts.json`: the copy of the latest version's `artifacts.json`, written by `publish?latest=true` (see [Latest](../../concepts/latest.md)).

| Parameter | In | Description |
| --- | --- | --- |
| `product` | path | Product name. |

```sh
curl -s https://registry.example.com/sonarbridge/artifacts
```

```json
[
  {"internal_type": 36, "name": "metadata.json", "path": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/metadata.json", "type": "Metadata"},
  {"extra": {"Checksum": "sha256:3f1c…", "Format": "tar.gz", "ID": "sonarbridge"}, "goarch": "amd64", "goos": "linux", "internal_type": 1,
   "name": "sonarbridge_1.4.2_linux_amd64.tar.gz", "path": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz", "type": "Archive"}
]
```

The JSON is returned as stored (decoded and re-encoded, same content). For versions published through the API, it is the rewritten file: each `path` is the file's download URL (see [Upload rules](../upload-rules.md)).

| Status | Cause |
| --- | --- |
| `404` | Invalid product name, no version published with `latest=true`, or a latest version without `artifacts.json`. |
| `500` | The stored file is not valid JSON. |

## Get the artifacts.json of a version

`GET /{product}/artifacts/{version}`

Returns `bin.local/assets/binaries/<product>/<version>/artifacts.json`. With `latest` as the version, it behaves like `GET /{product}/artifacts`.

| Parameter | In | Description |
| --- | --- | --- |
| `product` | path | Product name. |
| `version` | path | Exact version, or `latest`. |

```sh
curl -s https://registry.example.com/sonarbridge/artifacts/v1.4.2
```

| Status | Cause |
| --- | --- |
| `404` | Invalid name, version not published (staged versions included), or no `artifacts.json` in it. |
| `500` | The stored file is not valid JSON. |

## Get metadata.json

`GET /artifacts/metadatas/{product}`

Returns the `metadata.json` that goreleaser wrote for a published version.

| Parameter | In | Description |
| --- | --- | --- |
| `product` | path | Product name. |
| `version` | query, optional | Exact version. Absent, empty or `latest`: the latest version. |

```sh
curl -s https://registry.example.com/artifacts/metadatas/sonarbridge
curl -s "https://registry.example.com/artifacts/metadatas/sonarbridge?version=v1.4.2"
```

```json
{"project_name": "sonarbridge", "tag": "v1.4.2", "previous_tag": "v1.4.1", "version": "1.4.2", "commit": "9c9d7664ceef…", "date": "2026-10-10T20:01:27+02:00", "runtime": {"goos": "linux", "goarch": "amd64"}}
```

| Status | Cause |
| --- | --- |
| `404` | Invalid name, version not published, no `metadata.json` in it, or no latest version. |
| `500` | The stored file is not valid JSON (its content is not returned). |

## Get the raw catalogue

`GET /artifacts/all`

Scans `bin.local/assets/` and returns, for each directory holding at least one known file, the raw content of those files as strings. It is a low-level view, kept for existing clients.

```sh
curl -s https://registry.example.com/artifacts/all
```

```json
[
  {"provider": "sonarbridge", "artifacts_json": "[…]", "metadata_json": "", "checksums_text": ""},
  {"provider": "v1.4.2", "artifacts_json": "[…]", "metadata_json": "{\"tag\":\"v1.4.2\",…}", "checksums_text": "3f1c…  sonarbridge_1.4.2_linux_amd64.tar.gz\n…"}
]
```

| Field | Source |
| --- | --- |
| `provider` | Name of the directory: the product for its latest copy, **the version** for a version directory. |
| `artifacts_json` | Content of `artifacts.json`. |
| `metadata_json` | Content of `metadata.json`. |
| `checksums_text` | Content of the file whose name ends in `_checksums.txt`, any case. |

- Entries are sorted by directory path.
- A missing or unreadable file gives `""`. No known file at all gives `null` instead of a list.
- Staging is not scanned. A `checksums.txt` written by publish (no `_` before `checksums.txt`) is not recognised.

| Status | Cause |
| --- | --- |
| `404` | `bin.local/assets/` does not exist. |
| `500` | Read error while scanning. |

## Liveness probe

`GET /healthz`

Answers as soon as the server serves. Use it for Docker `HEALTHCHECK`, Kubernetes probes and load balancers.

```sh
curl -s http://localhost:8077/healthz
```

```json
{"status": "ok"}
```

It always answers `200`; it does not check the disk.
