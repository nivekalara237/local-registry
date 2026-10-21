---
title: index.json
description: Normative reference of index.json, the document that describes a product and all its versions.
sidebar_label: index.json
---

`index.json` describes a product: its card (from `registry.yaml`), its latest version, and every version published with a `release.json`. It is what a catalogue or a portal shows for a product. The server rewrites `bin.local/assets/binaries/<product>/index.json` after each publish of the product, and `GET /{product}/index` rebuilds the same document from the disk at each request (see [Products API](./api/products.md)).

## Example

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
  "install": "curl -fsSL https://registry.example.com/… | tar xz\n",
  "latest": "v1.4.2",
  "updatedAt": "2026-10-12T08:00:00Z",
  "versions": [
    { "version": "v1.5.0-rc.1", "tag": "v1.5.0-rc.1", "publishedAt": "2026-10-12T08:00:00Z", "prerelease": true, "securityUpdates": false, "deprecated": false },
    { "version": "v1.4.2", "tag": "v1.4.2", "publishedAt": "2026-10-10T18:05:02Z", "prerelease": false, "securityUpdates": true, "deprecated": false },
    { "version": "v1.4.1", "tag": "v1.4.1", "publishedAt": "2026-09-30T14:22:10Z", "prerelease": false, "securityUpdates": false, "deprecated": "use v1.4.2: CSV export is broken" }
  ]
}
```

Every key is always present. Clients must ignore keys they do not know.

## Fields

| Field | Type | Content |
| --- | --- | --- |
| `apiVersion` | string | Always `local-registry/v1`. |
| `product` | string | Product name. |
| `displayName` | string | `displayName` of the `release.json` of the version that gives the card. |
| `description`, `license`, `install` | string | From the card; `""` without one. |
| `owners`, `links`, `keywords` | list | From the card; `[]` without one. |
| `labels` | map | From the card; `{}` without one. |
| `latest` | string or null | The latest version, `null` when none. |
| `updatedAt` | string | The most recent `publishedAt` among the versions, UTC, RFC 3339. |
| `versions` | list | Every version that has a `release.json` (below). |

The card is the `registry.yaml` of the `latest` version; without a latest version, it is the card of the version published last. The fields of the card are defined in [registry.yaml](./registry-yaml.md).

Each entry of `versions`, copied from that version's [`release.json`](./release-json.md):

| Field | Type | Content |
| --- | --- | --- |
| `version` | string | Version name. |
| `tag` | string or null | git tag, from `metadata.json`. |
| `publishedAt` | string | Publish time, UTC, RFC 3339. |
| `prerelease` | boolean | Semver pre-release. |
| `securityUpdates` | boolean | The version has a `security` release note. |
| `deprecated` | boolean or string | `false`, `true` or a message. |

## Rules

- **Which versions.** Only versions that have a `release.json`. Versions published before the standard are not converted: they stay downloadable through the other routes.
- **Order.** Semver versions first, newest first by semver precedence (`v1.10.0` before `v1.2.0`, `v1.0.0` before `v1.0.0-rc.1`). Other names follow, newest `publishedAt` first.
- **`latest`.** The `version` named by `<product>/release.json`, the copy that `publish?latest=true` writes, provided that version has a `release.json`; `null` otherwise. See [Latest](../concepts/latest.md).
- **Freshness.** The file on disk is rewritten atomically after each publish of the product. `GET /{product}/index` does not read it: it rebuilds the index at each request, so a `latest` changed by hand shows at once.
