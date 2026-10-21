---
title: HTTP API
description: Conventions shared by every route of the local-registry HTTP API, and the list of all routes.
sidebar_label: Overview
---

The local-registry HTTP API serves published releases and accepts new ones. This page gives the rules every route follows; each group of routes has its own page.

## Base URL

The server listens on `http://localhost:8077` by default (`--address`, `--port`), or on `https://` when TLS is enabled. Behind a reverse proxy, set `--public-url` (for example `https://registry.example.com`): the download URLs the server writes (`release.json`, the rewritten `artifacts.json`, `POST /releases`) then become absolute URLs under it. Without it they are relative paths such as `/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz`.

## Routes

| Method | Route | Auth | Returns | Page |
| --- | --- | --- | --- | --- |
| `GET` | `/products` | — | Every product, with its latest version | [Products](./products.md) |
| `GET` | `/{product}/index` | — | `index.json`: the product card and its versions | [Products](./products.md) |
| `GET` | `/{product}/releases/{version}` | — | `release.json` of a version (`latest` allowed) | [Products](./products.md) |
| `GET` | `/{product}/artifacts` | — | `artifacts.json` of the latest version | [Artifacts](./artifacts.md) |
| `GET` | `/{product}/artifacts/{version}` | — | `artifacts.json` of a version | [Artifacts](./artifacts.md) |
| `GET` | `/artifacts/metadatas/{product}` | — | `metadata.json` of a version (default: latest) | [Artifacts](./artifacts.md) |
| `GET` | `/artifacts/all` | — | Raw catalogue files of every directory | [Artifacts](./artifacts.md) |
| `GET` | `/healthz` | — | Liveness probe | [Artifacts](./artifacts.md) |
| `GET` | `/{product}/artifacts/{version}/files/{name}` | — | A file of a published version | [Downloads](./downloads.md) |
| `GET` | `/assets/binary/{filePath}` | — | A public file, by its path | [Downloads](./downloads.md) |
| `GET` | `/assets/checksum/{filename}` | — | Same as `/assets/binary` | [Downloads](./downloads.md) |
| `PUT` | `/{product}/artifacts/{version}/files/{name}` | token | Upload one file to staging | [Upload](./upload.md) |
| `POST` | `/{product}/artifacts/{version}/publish` | token | Check and publish a version | [Upload](./upload.md) |
| `POST` | `/releases/{product}/{version}` | — | Files of a version with their download URL | [Legacy](./legacy.md) |
| `POST` | `/releases/{version}` | — | Deprecated: `/releases/gitlab/{version}` | [Legacy](./legacy.md) |

Read routes need no authentication. Only `PUT` and `publish` need the upload token.

## Names

`{product}`, `{version}` and `{name}` must match `[A-Za-z0-9._-]+` and be neither `.` nor `..`.

- A refused name gives `404` on read routes and `400` on upload routes.
- `latest` is a valid **read** version: the latest version of the product (see [Latest](../../concepts/latest.md)). It is not an upload version.
- The product name `products` is reserved.
- The version names `artifacts.json`, `artifacts.goreleaser.json`, `release.json` and `index.json` are reserved: their directory would take the place of a product file.

Each path variable is **one segment**. In `/assets/binary/{filePath}`, encode `/` as `%2F`. Names are decoded once: `%252F` stays `%2F`. A literal `..` or `//` in the URL gets a `301` redirect to the cleaned path.

## Methods

Every `GET` route also answers `HEAD`. A wrong method answers `405` with an `Allow` header. An unknown path answers `404 page not found` as plain text.

## Responses and errors

JSON routes answer `Content-Type: application/json`. On error, the body is:

```json
{"error": "version sonarbridge/v1.4.2 is already published and cannot be changed"}
```

File routes (downloads) and routing errors answer plain text.

The `error` message is meant for humans and scripts. For a `500`, it is always `internal error`: the real cause is only written to the server log (and to the audit log for upload routes). Every message is listed in [Errors](../errors.md).

| Status | Meaning |
| --- | --- |
| `200` | OK |
| `201` | File uploaded to staging |
| `301` | Path cleaned (`..`, `//`) |
| `400` | Invalid name or parameter, reserved name |
| `401` | Missing or wrong token, with `WWW-Authenticate: Bearer` |
| `403` | Upload disabled: no token configured on the server |
| `404` | Not found, or name refused on a read route |
| `405` | Wrong method |
| `409` | Version already published |
| `413` | File or version too large |
| `422` | Content refused: `registry.yaml`, `CHANGELOG.md`, checksums, `artifacts.json`, `metadata.json`, or a quota on files |
| `500` | Internal error; the cause is only in the server log |
| `507` | Staging area full, or disk almost full |

## Authentication

`PUT` and `publish` require `Authorization: Bearer <token>`, compared in constant time before anything else. See [Upload](./upload.md).

Send the header to curl on its standard input, so the token never appears in the process list:

```sh
printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" |
  curl -fsS -H @- -X POST https://registry.example.com/sonarbridge/artifacts/v1.4.2/publish
```

## Freshness

The server reads the disk at each request and keeps no cache: a file copied by hand under the asset path is visible at once.

## Timeouts

| Timeout | Value | Scope |
| --- | --- | --- |
| Reading request headers | 5 s | all requests |
| Writing the response | **20 s** | every route except upload and publish, downloads included |
| Read and write | 30 min | `PUT …/files/{name}`, `POST …/publish` |
| Idle keep-alive connection | 60 s | all connections |

A download over a slow link that exceeds 20 s is cut: resume it with `Range` (`curl -C -`).
