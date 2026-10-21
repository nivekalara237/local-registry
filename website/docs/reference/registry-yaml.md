---
title: registry.yaml
description: Normative reference of registry.yaml, the product card uploaded with every version.
sidebar_label: registry.yaml
---

`registry.yaml` is the product card: who owns the product, what it is, where its sources are, how to install it, and whether this version is deprecated. It is the only file of the [local-registry/v1 standard](../concepts/standard-v1.md) written by hand, and it is **required**: every version is published with its own copy. For a commented walkthrough, see [Write registry.yaml](../guides/write-registry-yaml.md).

## Example

```yaml
apiVersion: local-registry/v1
displayName: SonarBridge
description: Bridge between SonarQube and internal tools.
license: Proprietary                  # SPDX identifier (MIT, Apache-2.0…) or "Proprietary"
owners:
  - name: Platform team
    email: platform@corp.example
links:
  - { name: source, url: https://git.corp.example/platform/sonarbridge }
  - { name: docs,   url: https://docs.corp.example/sonarbridge }
keywords: [sonar, cli]
labels:                               # free key/value pairs
  domain: quality
  cost-center: "4512"
install: |                            # Markdown, shown as is
  curl -fsSL https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz | tar xz
deprecated: false                     # or a message: "replaced by sonarbridge2"
```

The smallest valid card is one line:

```yaml
apiVersion: local-registry/v1
```

## Fields

| Field | Type | Required | Rule |
| --- | --- | --- | --- |
| `apiVersion` | string | **yes** | Exactly `local-registry/v1`. |
| `displayName` | string | no | At most 100 characters. Default: `project_name` of `metadata.json`, else the product name. |
| `description` | string | no | At most 500 characters. |
| `license` | string | no | Free text; an SPDX identifier or `Proprietary` is recommended. |
| `owners` | list of `{name, email}` | no | `name` is not empty; `email`, optional, contains `@`. |
| `links` | list of `{name, url}` | no | `name` is not empty; `url` is an `http://` or `https://` URL with a host. |
| `keywords` | list of strings | no | At most 20; none blank. |
| `labels` | map of strings | no | At most 50 entries. Numbers and booleans are read as strings (`4512` becomes `"4512"`). |
| `install` | string | no | At most 10 KiB. Markdown, shown as is by clients. |
| `deprecated` | boolean or string | no | `false` (default), `true`, or a message. An empty string means `false`. |

The whole file is at most 64 KiB and holds a single YAML document. Any other key is refused, so a typo cannot pass unnoticed. This includes `changes`: release notes come from [`CHANGELOG.md`](./changelog-format.md).

## Where the values go

| Field | Shown in |
| --- | --- |
| `displayName` | `release.json` of the version; `index.json` and `GET /products` when this version gives the card. |
| `deprecated` | `release.json` of the version, and its entry in `index.json`. |
| every other field | `index.json` and `GET /products`, from the card of the latest version (else of the version published last). |

See [release.json](./release-json.md) and [index.json](./index-json.md).

## Validation

The file is validated when it is uploaded (`PUT …/files/registry.yaml`), and again at publish. An invalid file answers `422` and is not kept.

| Message | Cause |
| --- | --- |
| `registry.yaml is larger than 64 KiB` | File too large. |
| `registry.yaml: empty` | No YAML document. |
| `registry.yaml: holds several YAML documents` | A `---` separator followed by a second document. |
| `registry.yaml: unmarshal errors: line <n>: field <key> not found in type endpoints.ProductFile` | Unknown key (typo, `changes`…). |
| `registry.yaml: line <n>: <reason>`, `registry.yaml: unmarshal errors: line <n>: cannot unmarshal …` | Invalid YAML, or a value of the wrong type. |
| `registry.yaml: apiVersion must be local-registry/v1` | Missing or other `apiVersion`. |
| `registry.yaml: displayName is longer than 100 characters` | |
| `registry.yaml: description is longer than 500 characters` | |
| `registry.yaml: more than 20 keywords` | |
| `registry.yaml: more than 50 labels` | |
| `registry.yaml: install is larger than 10 KiB` | |
| `registry.yaml: owners[<i>] has no name` | |
| `registry.yaml: owners[<i>].email "<email>" is not an email address` | No `@` in the email. |
| `registry.yaml: links[<i>] needs a name and an http(s) url` | |
| `registry.yaml: keywords[<i>] is empty` | |
| `registry.yaml: deprecated must be true, false or a message` | A list, a map or a number. |

At publish, a version without the file answers `422 registry.yaml not uploaded: every version needs its product card`.

## JSON Schema

[`schema/registry.schema.json`](https://github.com/nivekalara237/local-registry/blob/main/schema/registry.schema.json) (JSON Schema 2020-12) describes the same rules, so that CI can check the file before sending it. A test of the project checks that the schema and the server accept the same keys.

```sh
check-jsonschema --schemafile schema/registry.schema.json registry.yaml
```

The schema is also shipped in every release archive of local-registry.
