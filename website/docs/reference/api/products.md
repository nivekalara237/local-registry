---
title: Products API
description: Read the products, their index and the release.json of each version, as defined by the local-registry/v1 standard.
sidebar_label: Products
---

These routes expose the documents of the [local-registry/v1 standard](../../concepts/standard-v1.md): the list of products and their search, each product's `index.json`, and each version's `release.json`. They are the recommended way to discover releases, whether they were built with goreleaser or not. They are read-only and need no token.

Only versions that have a `release.json` exist for these routes. Versions published before the standard stay available through the [artifacts](./artifacts.md) and [legacy](./legacy.md) routes.

## List products

`GET /products`

Lists every product that has at least one version with a `release.json`, sorted by name.

```sh
curl -s https://registry.example.com/products
```

```json
[
  {
    "product": "billing-db-migrations",
    "displayName": "Billing DB migrations",
    "description": "Liquibase changelogs of the billing database.",
    "latest": "v1.0.0",
    "updatedAt": "2026-10-18T09:12:44Z"
  },
  {
    "product": "sonarbridge",
    "displayName": "SonarBridge",
    "description": "Bridge between SonarQube and internal tools.",
    "latest": "v1.4.2",
    "updatedAt": "2026-10-12T08:00:00Z"
  }
]
```

| Field | Content |
| --- | --- |
| `product` | Product name: the `{product}` segment of every URL. |
| `displayName` | Display name, as in the product's [index](../index-json.md). |
| `description` | Description from the card (`registry.yaml`), `""` when none. |
| `latest` | Latest version, or `null` when no version is latest. |
| `updatedAt` | Most recent `publishedAt` of the product, UTC, RFC 3339. |

An empty registry answers `[]`. Errors: `500` only (unreadable disk).

## Search products

`GET /products/search`

Returns, one page at a time, the products that match every criterion given. Without a criterion, it returns every product. The rules are in the [standard](../../concepts/standard-v1.md), section 7.1 of the specification.

| Parameter | In | Description |
| --- | --- | --- |
| `q` | query | Text searched, regardless of case, in the name, display name, description and keywords. |
| `keyword` | query | Keyword the product must have, regardless of case. Repeatable: every keyword must match. |
| `label` | query | `key=value` label the product must have, exactly. Repeatable: every label must match. |
| `owner` | query | Name or email of an owner, regardless of case. |
| `license` | query | License of the product, regardless of case, such as `Apache-2.0`. |
| `deprecated` | query | `true` or `false`: whether the current version is deprecated. |
| `prerelease` | query | `true` or `false`: whether the current version is a pre-release. |
| `updatedSince` | query | `YYYY-MM-DD`: products published that day (UTC) or later. |
| `sort` | query | `product` (default), `displayName` or `updatedAt`. Prefix with `-` for descending order: `-updatedAt` lists the most recent first. |
| `limit` | query | Page size, from 1 to 100. Default: 25. |
| `cursor` | query | The `next` value of the previous page. |

The current version of a product is its latest version, else its highest version. A product with an old pre-release but a stable latest version is not a pre-release.

```sh
curl -s "https://registry.example.com/products/search?keyword=db&label=team=billing&sort=-updatedAt&limit=1"
```

```json
{
  "apiVersion": "local-registry/v1",
  "items": [
    {
      "product": "billing-db-migrations",
      "displayName": "Billing DB migrations",
      "description": "Liquibase changelogs of the billing database.",
      "latest": "v1.0.0",
      "updatedAt": "2026-10-18T09:12:44Z",
      "keywords": ["db", "liquibase"],
      "labels": { "team": "billing" },
      "license": "Proprietary",
      "deprecated": false
    }
  ],
  "total": 2,
  "limit": 1,
  "next": "eyJrIjoiMjAyNi0xMC0xOFQwOToxMjo0NFoiLCJwIjoiYmlsbGluZy1kYi1taWdyYXRpb25zIiwicSI6IjFhMmIzYzRkNWU2ZjcwODEifQ"
}
```

| Field | Content |
| --- | --- |
| `items` | The products of the page: the fields of [`GET /products`](#list-products), plus `keywords`, `labels`, `license` and `deprecated` (whether the current version is deprecated). |
| `total` | Number of products that match, all pages together. |
| `limit` | Page size used. |
| `next` | Cursor of the next page, `null` on the last page. |

**Pagination.** To get the next page, send the same request with `cursor=<next>`. The same URL is also in the `Link: <…>; rel="next"` header, which is absent on the last page. A page starts right after the last product of the previous one: a product published or removed in between is neither repeated nor skipped. Keep the same criteria and `sort` from page to page; `limit` may change. A loop that reads every page is in [Consume releases](../../guides/consume-releases.md#search-products).

| Status | Cause |
| --- | --- |
| `400` | Unknown parameter, invalid value (`limit`, `sort`, `deprecated`, `prerelease`, `label`, `updatedSince`), invalid cursor, or a cursor made for other criteria or another sort. The message lists every invalid parameter. |
| `500` | The disk cannot be read. |

## Get a product index

`GET /{product}/index`

Returns the product card and every version that has a `release.json`. The server serves `<product>/index.json` while it matches the disk, and rebuilds the document otherwise: a version removed or a `latest` changed by hand shows at once. Its format is in [index.json](../index-json.md).

| Parameter | In | Description |
| --- | --- | --- |
| `product` | path | Product name. |

```sh
curl -s https://registry.example.com/sonarbridge/index
```

```json
{
  "apiVersion": "local-registry/v1",
  "product": "sonarbridge",
  "displayName": "SonarBridge",
  "description": "Bridge between SonarQube and internal tools.",
  "license": "Proprietary",
  "owners": [{ "name": "Platform team", "email": "platform@corp.example" }],
  "links": [{ "name": "source", "url": "https://git.corp.example/platform/sonarbridge" }],
  "keywords": ["sonar", "cli"],
  "labels": { "domain": "quality" },
  "install": "curl -fsSL https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/… | tar xz\n",
  "latest": "v1.4.2",
  "updatedAt": "2026-10-12T08:00:00Z",
  "versions": [
    { "version": "v1.5.0-rc.1", "tag": "v1.5.0-rc.1", "publishedAt": "2026-10-12T08:00:00Z", "prerelease": true, "securityUpdates": false, "deprecated": false },
    { "version": "v1.4.2", "tag": "v1.4.2", "publishedAt": "2026-10-10T18:05:02Z", "prerelease": false, "securityUpdates": true, "deprecated": false }
  ]
}
```

| Status | Cause |
| --- | --- |
| `404` | Invalid product name, `products`, unknown product, or no version of it has a `release.json`. |
| `500` | A `release.json` on disk is not valid JSON, or the disk cannot be read. |

## Get a release

`GET /{product}/releases/{version}`

Returns the `release.json` of a version, as stored: its files with platform, size, sha256 and download URL, its release notes, and its checksums file. Its format is in [release.json](../release-json.md).

| Parameter | In | Description |
| --- | --- | --- |
| `product` | path | Product name. |
| `version` | path | Exact version, or `latest`. |

```sh
curl -s https://registry.example.com/sonarbridge/releases/latest
curl -s https://registry.example.com/billing-db-migrations/releases/v1.0.0
```

```json
{
  "apiVersion": "local-registry/v1",
  "product": "billing-db-migrations",
  "displayName": "Billing DB migrations",
  "version": "v1.0.0",
  "tag": null,
  "previousTag": null,
  "prerelease": false,
  "commit": null,
  "builtAt": null,
  "publishedAt": "2026-10-18T09:12:44Z",
  "publishedBy": "e2186dbdb1bb",
  "deprecated": false,
  "securityUpdates": false,
  "changes": [{ "kind": "added", "description": "orders table" }],
  "artifacts": [
    {
      "name": "billing-db-migrations_1.0.0.tar.gz",
      "kind": "archive",
      "format": "tar.gz",
      "mediaType": "application/gzip",
      "os": null,
      "arch": null,
      "variant": null,
      "size": 2311,
      "sha256": "6c08926d953d80ba627847e1b3eaa893e6040e239a197083f83cda85341baa00",
      "url": "https://registry.example.com/billing-db-migrations/artifacts/v1.0.0/files/billing-db-migrations_1.0.0.tar.gz",
      "sbom": null,
      "signature": null,
      "certificate": null
    }
  ],
  "checksums": {
    "name": "checksums.txt",
    "url": "https://registry.example.com/billing-db-migrations/artifacts/v1.0.0/files/checksums.txt",
    "signature": null,
    "certificate": null
  }
}
```

| Status | Cause |
| --- | --- |
| `404` | Invalid name, version not published, version without `release.json`, or `latest` with no latest version. |
| `500` | The stored `release.json` is not valid JSON (`<product>/<version>/release.json is not valid JSON`). |

> [!NOTE]
> `GET /{product}/releases/{version}` and the legacy `POST /releases/{product}/{version}` are distinct routes: the first returns the `release.json` document, the second lists the raw files of the version directory.
