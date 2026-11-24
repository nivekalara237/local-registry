---
title: Publish a GoReleaser release
sidebar_label: Publish with GoReleaser
description: Send the output of a GoReleaser release to the registry and publish it as one version.
---

This guide publishes the output of `goreleaser release` as one version of a product. Use it when your project already builds with GoReleaser: [`localrectl`](../reference/localrectl.md) reads the GoReleaser build in `dist/`, whatever your archive layout.

![GoReleaser builds dist/, localrectl uploads each file to staging, then publishes the version](/img/goreleaser-release-flow.svg)

## Before you start

- The registry address, for example `https://registry.corp.example`, and its upload token. If you do not run the registry yourself, ask the person or team responsible for it.
- `localrectl` on the machine that publishes. See [localrectl](../reference/localrectl.md#install).
- A `registry.yaml` product card in your project. The server refuses a version without it. See [Write the product card](write-registry-yaml.md).

> [!IMPORTANT]
> Publish **after** `goreleaser release` has finished. GoReleaser writes `dist/artifacts.json` after its own `publishers:`, so a GoReleaser publisher cannot send it.

## Steps

1. **Build the release.** From the root of your project, on a tag:

   ```sh
   git tag v1.4.2
   goreleaser release --clean
   ```

   If GoReleaser must not create a GitHub or GitLab release, add `--skip=publish`. `dist/artifacts.json`, `dist/metadata.json` and `dist/CHANGELOG.md` are still written.

2. **Check what would be sent.** A dry run shows what would be sent and sends nothing. It needs no token.

   ```sh
   localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml \
     --changelog-file dist/CHANGELOG.md --dry-run
   ```

   The version comes from GoReleaser's `metadata.json`. Pass `--version` to choose another one.

3. **Publish.** Set the registry address and the token, then run the command again without `--dry-run`. Add `--latest` to make this version the product's latest one.

   ```sh
   export LOCAL_REGISTRY_URL=https://registry.corp.example
   export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # from your secret store, never on the command line

   localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml \
     --changelog-file dist/CHANGELOG.md --latest
   ```

   `localrectl` reads the token from `LOCAL_REGISTRY_UPLOAD_TOKEN` and never prints it. With `-o json`, it prints the result as `{"total", "total_size", "status", "version", "product"}`, where `status` is `PUBLISHED`.

## Check the result

```sh
curl -fsS https://registry.corp.example/bridge/releases/v1.4.2 | jq '{version, tag, changes, files: [.artifacts[].name]}'
curl -fsS https://registry.corp.example/bridge/index | jq '{latest, versions: [.versions[].version]}'
```

`release.json` lists every file with its platform, size and SHA-256. If you used `--latest`, `latest` in the index is now `v1.4.2`.

## What is published

- Every file that GoReleaser marks as part of the release: archives, packages, checksums, signatures, certificates, SBOMs, `metadata.json`, and binaries published as is (`formats: [binary]`).
- Raw build outputs (`Binary` entries without a format) are not published: they are the files your archives are made from.
- The `registry.yaml` product card, the `CHANGELOG.md` release notes and `artifacts.json`. The server keeps the uploaded `artifacts.json` as `artifacts.goreleaser.json` and writes a version of it where each `path` is the file's download URL.

The full rules are in [Upload rules](../reference/upload-rules.md).

## If something goes wrong

- **`localrectl` stops before sending anything.** A local check failed. Fix the cause it gives and run it again.
- **The server refuses the publish (`422`).** Nothing is published and the uploaded files stay in staging. Fix the cause and run the command again: the staged files are replaced. See [Staging and publish](../concepts/staging-and-publish.md).
- **`already published`.** A version is immutable. Publish a new version, or add `--if-exists skip` when a previous run already published this version.
- **`metadata.json describes …, not …`.** The requested version is not the one in GoReleaser's `metadata.json`. Drop `--version`, or pass the tag.

Every message is explained in [Troubleshooting](../troubleshooting.md) and [Errors](../reference/errors.md).

## Next steps

- Automate it: [Publish from CI](ci-pipelines.md).
- Change which version is the latest one: [Change the latest version](change-latest.md).
