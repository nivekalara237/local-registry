---
title: Downloads API
description: Download the files of a published version, or any public file by its path.
sidebar_label: Downloads
---

These routes download files. Use the first one for every published version: it is the URL written in `release.json`, in the rewritten `artifacts.json` and in `POST /releases` answers. The second one serves any public file by its path, including the legacy tree. Both are read-only and need no token.

## Download a file of a version

`GET /{product}/artifacts/{version}/files/{name}`

Downloads one file of a **published** version. Staged files are never served.

| Parameter | In | Description |
| --- | --- | --- |
| `product` | path | Product name. |
| `version` | path | Exact version. `latest` is **not** accepted: resolve it first with [`GET /{product}/releases/latest`](./products.md). |
| `name` | path | File name, percent-encoded. |
| `Range` | header, optional | Byte range, to resume a download. |
| `If-Modified-Since` | header, optional | Conditional download. |

```sh
curl -fsSO https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz
curl -fsS -C - -O https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_windows_amd64.zip   # resume
```

Response headers:

| Header | Value |
| --- | --- |
| `Content-Type` | `application/octet-stream` |
| `Content-Disposition` | `attachment; filename=<name>` |
| `Content-Length`, `Last-Modified` | Size and modification time of the file. |
| `Accept-Ranges` | `bytes` |

`Range` gives `206 Partial Content`; `If-Modified-Since` gives `304 Not Modified` when the file has not changed.

| Status | Cause (plain text body) |
| --- | --- |
| `404` | Version not published, `version` is `latest`, invalid or hidden name (`.x`), missing file, directory, or symbolic link leaving the version directory. |

> [!TIP]
> Every file of a version is listed with its URL and sha256 in its `release.json`: read it rather than building URLs by hand. See [Consume releases](../../guides/consume-releases.md).

## Download a public file by path

`GET /assets/binary/{filePath}`

Downloads a public file by its path relative to the server's `--asset-path`, encoded as **one** URL segment. `GET /assets/checksum/{filename}` is the same route under another name.

| Parameter | In | Description |
| --- | --- | --- |
| `filePath` | path | Path relative to the asset path, `/` encoded as `%2F`. |

```sh
curl -fsS -OJ "https://registry.example.com/assets/binary/bin.local%2Fassets%2Fbinaries%2Fsonarbridge%2Fv1.4.2%2Fsonarbridge_1.4.2_checksums.txt"
```

Encode the path with your language's path escaping: `encodeURIComponent` in JavaScript, `url.PathEscape` in Go, `urllib.parse.quote(p, safe="")` in Python.

| Served | Not served (`404`, plain text) |
| --- | --- |
| `bin.local/assets/…` (published releases) | staging: `bin.local/.uploads/…`, `bin.local/.uploads-tmp/…` |
| `assets/…` (legacy tree) | any other path under the asset path, such as `bin.local/audit.log` |
| | any path with a segment starting with `.` |
| | a symbolic link leading outside the asset path (checked when the file is opened) |
| | a directory |
| | a double-encoded or non-normalised path (`a%2F%2Fb`) |

The response headers are the same as above, `Range` included.
