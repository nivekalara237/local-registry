---
title: Write the product card
sidebar_label: Write registry.yaml
description: Write registry.yaml, the product card every version must carry, and check it before you publish.
---

`registry.yaml` is the card of your product: its name, owners, links and how to install it. Every version must carry one: the registry refuses to publish without it. This guide writes a good card and catches mistakes before publishing.

## Where it lives

Keep `registry.yaml` at the root of your repository and commit it with the code. `upload.sh` finds it there, or takes the file you pass with `--product-file`. The card is uploaded with each version, so each version keeps the card it was published with. The product page (`index.json`) shows the card of the latest version.

## A complete example

```yaml
# Required. The only accepted value.
apiVersion: local-registry/v1

# The name people see. Up to 100 characters.
displayName: SonarBridge

# One or two sentences: what it does, for whom. Up to 500 characters.
description: Bridge between SonarQube and internal tools.

# An SPDX identifier (MIT, Apache-2.0…) or "Proprietary".
license: Proprietary

# Who to contact. "email" is optional.
owners:
  - name: Platform team
    email: platform@corp.example

# Useful pages. URLs must start with http:// or https://.
links:
  - { name: source, url: https://git.corp.example/platform/sonarbridge }
  - { name: docs, url: https://docs.corp.example/sonarbridge }

# Up to 20 words to search and filter on.
keywords: [sonar, cli]

# Free key/value pairs for your inventory. Up to 50.
labels:
  domain: quality
  cost-center: "4512"

# Markdown, shown as is. Up to 10 KiB.
install: |
  Download the archive of your platform and unpack it:

      curl -fsSL https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz | tar xz

# false, true, or a message.
deprecated: false
```

Only `apiVersion` is required. Every field and limit is in the [registry.yaml reference](../reference/registry-yaml.md).

## Check it before you publish

The server checks the card when it receives it and answers `422 registry.yaml: <reason>` if it is wrong. Check it earlier, in your merge requests, with the JSON Schema shipped in every local-registry release (`schema/registry.schema.json`):

```sh
pip install check-jsonschema
check-jsonschema --schemafile schema/registry.schema.json registry.yaml
```

## Common mistakes

| Mistake | Effect | Fix |
| --- | --- | --- |
| Card copied from another product, `displayName` unchanged | Your product shows under another product's name | Review `displayName`, `description` and `links` when you copy a card |
| A typo in a key, such as `displayname` | `422 registry.yaml: …`: unknown keys are refused | Use the exact key names above |
| A `changes:` section | `422`: release notes do not belong in the card | Write a `CHANGELOG.md`: see [Write release notes](write-changelog.md) |
| `license: Prioritary` | Accepted, but wrong in every inventory | Use an SPDX identifier or `Proprietary` |
| A link without a scheme, such as `git.corp.example/…` | `422` | Write the full `https://…` URL |
| `install` pointing at another product or version | Users install the wrong thing | Keep the product and file names of this product |
| Several YAML documents (`---`) | `422` | Keep one document per file |

> [!TIP]
> Numbers and booleans in `labels` are read as text: `cost-center: 4512` becomes `"4512"`. Quote them anyway, to make it explicit.

## Deprecate a product

Set `deprecated` in the card of a new version, with a message that tells users what to do:

```yaml
deprecated: replaced by sonarbridge2, see https://docs.corp.example/sonarbridge2
```

The value is stored in that version's `release.json`. Versions already published keep the card they had.
