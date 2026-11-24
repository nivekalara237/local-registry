---
title: Write the product card
sidebar_label: Write registry.yaml
description: Write registry.yaml, the product card every version must carry, and check it before you publish.
---

`registry.yaml` is the card of your product. It holds its name, owners, links and install instructions. Every version must come with this card: the registry refuses any publication without it. This guide explains how to write a valid card and catch mistakes before you publish.

## Where to put the file

You can create `registry.yaml` in the directory of your choice and commit it with the code. When you publish, give its path to `localrectl` with the `--product-file` option.

The card is sent with each published version. Each version thus keeps the product information as it was when it was published. The product page (`index.json`) shows the card of the latest version.

## A complete example

```yaml
# Required. The only accepted value.
apiVersion: local-registry/v1

# The name people see. Up to 100 characters.
displayName: Bridge

# One or two sentences: what it does, for whom. Up to 500 characters.
description: "Sonar Bridge: bridge between SonarQube and internal tools."

# An SPDX identifier (MIT, Apache-2.0…) or "Proprietary".
license: Proprietary

# Who to contact. "email" is optional.
owners:
  - name: Platform team
    email: platform@corp.example

# Useful pages. URLs must start with http:// or https://.
links:
  - { name: source, url: https://git.corp.example/platform/sonar-bridge }
  - { name: docs, url: https://docs.corp.example/bridge }

# Up to 20 keywords to search and filter the product.
keywords: [sonar, cli]

# Free key/value pairs for your inventory. Up to 50.
labels:
  domain: quality
  cost-center: "4512"

# Markdown, shown as is. Up to 10 KiB.
install: |
  Download the archive of your platform and unpack it:

      curl -fsSL https://registry.corp.example/bridge/artifacts/v1.4.2/files/bridge_1.4.2_linux_amd64.tar.gz | tar xz

# false, true, or a message.
deprecated: false
```

Only `apiVersion` is required. Each field and its limits are described in the [registry.yaml reference](../reference/registry-yaml.md).

## Check the card before you publish

The server validates the card when it receives it, and answers `422 registry.yaml: <reason>` if its content is wrong.

You can also run this check locally, before publishing, with the JSON Schema shipped in the Local Registry release archives (`schema/registry.schema.json`).

First install the `check-jsonschema` tool:

```sh
pip install check-jsonschema
```

Then give the schema and the file to check:

```sh
check-jsonschema --schemafile schema/registry.schema.json registry.yaml
```

Replace `registry.yaml` with the path of your file if it sits in another directory.

If you can reach the registry, `localrectl validate registry.yaml` also checks the card against the server's schema. See [localrectl](../reference/localrectl.md).

## Common mistakes

| Mistake | Effect | Fix |
| --- | --- | --- |
| Card copied from another product, `displayName` unchanged | Your product shows under another product's name | Check `displayName`, `description` and `links` when you reuse an existing card |
| A typo in a key, such as `displayname` | `422 registry.yaml: …`: unknown keys are refused | Use the exact documented key names |
| A `changes:` section | `422`: release notes do not belong in the card | Use a `CHANGELOG.md` file: see [Write release notes](write-changelog.md) |
| `license: Prioritary` | Accepted, but wrong in the inventories | Use an SPDX identifier or `Proprietary` |
| A link without a scheme, such as `git.corp.example/…` | `422` | Write a full URL starting with `https://` |
| `install` pointing at another product or version | Users install the wrong product or version | Check the product names, versions and file URLs |
| Several YAML documents (`---`) | `422` | Keep the file to a single YAML document |

> [!TIP]
> Numbers and booleans in `labels` are read as text: `cost-center: 4512` becomes `"4512"`. Quoting them is still recommended, to make that intent explicit.

## Deprecate a product

To flag a product as deprecated, set the `deprecated` field in the card sent with a new version. Add a message that tells users what to use instead:

```yaml
deprecated: replaced by bridge2, see https://docs.corp.example/bridge2
```

This value is stored in the `release.json` of that version. Versions already published keep the product card they were published with.
