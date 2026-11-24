---
title: Legacy API
description: The POST /releases routes that list the raw files of a version with their download URL.
sidebar_label: Legacy
---

These routes list the files of a version with their download URL. They return a raw file list, without the information of the [local-registry/v1 standard](../../concepts/standard-v1.md). Prefer [`GET /{product}/releases/{version}`](./products.md), which also gives each file's platform, size and sha256.

## List the files of a version

`POST /releases/{product}/{version}`

Lists the files of a version, each with the URL that downloads it. The route is read-only despite `POST`; it takes no body and needs no token.

| Parameter | In | Description |
| --- | --- | --- |
| `product` | path | Product name. |
| `version` | path | Exact version, or `latest`. |

Files are read from the first directory that exists:

| Order | Directory | `url` of each file |
| --- | --- | --- |
| 1 | Published version `bin.local/assets/binaries/<product>/<version>/` | `/{product}/artifacts/{version}/files/{name}` |
| 2 | Legacy tree `assets/binaries/<product>/<version>/` | `/assets/binary/assets%2Fbinaries%2F<product>%2F<version>%2F<name>` |

```sh
curl -s -X POST https://registry.corp.example/releases/bridge/latest
```

```json
{
  "tag_name": "v1.4.2",
  "assets": [
    {"id": "4c229d4f-3bb0-5e36-8d08-ae51796b420a", "name": "artifacts.goreleaser.json", "url": "https://registry.corp.example/bridge/artifacts/v1.4.2/files/artifacts.goreleaser.json", "is_checksum": false},
    {"id": "04647fbf-38ae-503e-a543-104c5aad6870", "name": "bridge_1.4.2_checksums.txt", "url": "https://registry.corp.example/bridge/artifacts/v1.4.2/files/bridge_1.4.2_checksums.txt", "is_checksum": true},
    {"id": "dc3de688-c35d-54d9-8e27-23638e997a71", "name": "bridge_1.4.2_linux_amd64.tar.gz", "url": "https://registry.corp.example/bridge/artifacts/v1.4.2/files/bridge_1.4.2_linux_amd64.tar.gz", "is_checksum": false}
  ]
}
```

| Field | Content |
| --- | --- |
| `tag_name` | The version listed: the real version name when `latest` was asked. |
| `assets` | One entry per regular file of the directory, sorted by name, the server's own files included (`release.json`, `registry.yaml`…). Directories, symbolic links and hidden files (`.x`) are skipped. `[]` when the directory is empty. |
| `id` | UUID v5 of `product/version/name`: stable across calls and restarts. |
| `name` | File name. |
| `url` | Download URL, name percent-encoded. Absolute under `--public-url` when set, else relative. |
| `is_checksum` | `true` when the name ends with `checksums.txt` or `checksum.txt`, any case. |

Download every file of a release:

```sh
BASE=https://registry.corp.example
curl -s -X POST $BASE/releases/bridge/latest | jq -r '.assets[] | [.name, .url] | @tsv' |
  while IFS=$'\t' read -r name url; do
    case $url in http*) ;; *) url=$BASE$url ;; esac
    curl -fsS -o "$name" "$url"
  done
```

| Status | Cause |
| --- | --- |
| `404` | Invalid name, version in neither directory, or `latest` with no latest version. |
| `405` | `GET` used instead of `POST`. |

## List the files of a gitlab version (deprecated)

`POST /releases/{version}`

Same answer as `POST /releases/gitlab/{version}`, for clients written when the registry served a single product named `gitlab`. The response carries two extra headers:

```
Deprecation: true
Link: </releases/gitlab/{version}>; rel="successor-version"
```

> [!WARNING]
> This route is deprecated: its answers carry the `Deprecation: true` header. Call `POST /releases/gitlab/{version}`, or better `GET /gitlab/releases/{version}`.
