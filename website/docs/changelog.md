---
title: Changelog
description: Changes of local-registry that affect operators, publishers and clients.
---

This page lists the changes of local-registry that matter to the people who run it, publish to it, or read from it. Versions follow [semantic versioning](https://semver.org). Before `1.0.0`, pre-releases may still change behaviour.

## 1.0.0-rc.3

### Added

- **The `local-registry/v1` standard.** Every published version now has a `release.json` that describes it: product, version, tag, commit, release notes, and each file with its kind, format, platform, size, sha256 and download URL. Each product has an `index.json` with its card and all its versions. See [The local-registry/v1 standard](concepts/standard-v1.md).
- **Product routes:** `GET /products`, `GET /{product}/index` and `GET /{product}/releases/{version}` (`latest` allowed). See [Product routes](reference/api/products.md).
- **The product card, `registry.yaml`**, uploaded with each version: display name, description, owners, links, keywords, labels, install instructions, deprecation. A JSON schema ships in `schema/registry.schema.json`. See [Write registry.yaml](guides/write-registry-yaml.md).
- **Release notes from `CHANGELOG.md`**: goreleaser's changelog (Conventional Commits) or a Keep a Changelog file. With version headings, only the section of the published version is read. See [Write a changelog](guides/write-changelog.md).
- **Publishing without goreleaser.** `metadata.json`, `artifacts.json` and the checksums file are now optional. Without a checksums file, publish writes `checksums.txt`. Without `artifacts.json`, the kind of each file is inferred from its name. See [Publish plain files](guides/publish-plain-files.md).
- **`upload.sh` publishes a directory**: `upload.sh --version v1.0.0 <product> <dir>` sends every file of the directory, with its sha256, and the product card.
- **Signatures**: `*.sigstore.json` files no longer need a line in the checksums file.

### Changed

- **`registry.yaml` is required** for every publish. Without it, publish answers `422 registry.yaml not uploaded`.
- **"Version group" is now "product"**, everywhere. The publish response field `vcs` is now `product`, and so is the field of the audit log. **Update clients that read the `vcs` field.** URLs do not change.
- **The latest version is designated by `<product>/release.json`**, a copy of the latest version's `release.json`. `<product>/artifacts.json` is still written when the latest version has one, and removed otherwise. The `409 latest … is ambiguous` error no longer exists. Versions published before this change cannot be the latest: publish again. See [Latest](concepts/latest.md).
- **Reserved names**: the product `products`, the versions `release.json`, `index.json`, `artifacts.json` and `artifacts.goreleaser.json`, and uploads named `release.json` or `index.json` are refused with `400`.
- **`upload.sh` on Windows**: the CRs that a native `jq.exe` adds are removed, which fixes `400 Bad Request` on Git Bash, MSYS2 and Cygwin.
- **`upload.sh --root DIR` without a file** reads `DIR/dist/artifacts.json`, else publishes `DIR` itself, instead of the `dist/artifacts.json` of the current directory.

### Removed

- The `--require-product-file` flag, its `LOCAL_REGISTRY_REQUIRE_PRODUCT_FILE` variable and its `require-product-file` key: the product card is always required now. Remove the setting from your configuration file, or the server refuses to start (`unknown setting`).

### Deprecated

- `POST /releases/{version}`: it reads the `gitlab` product only. Use `POST /releases/{product}/{version}`.

### Security

- **Go 1.27.3.** Binaries and images are built with Go 1.27.3, which fixes [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809).
- **The checksums file of each release is signed** (Sigstore keyless). Verify it with the `cosign verify-blob` command of the release notes.

## 1.0.0-rc.2

- Fixes the name of an environment variable of the release pipeline. No change to the server.

## 1.0.0-rc.1

First pre-release.

- Read routes: the catalogue, `artifacts.json` and `metadata.json` of a version, the files of a version with their download URLs, and file downloads.
- Two-step upload: send the files, then publish the version atomically, with checks of the checksums, `artifacts.json` and `metadata.json`.
- `artifacts.json` rewritten at publish: each `path` becomes the download URL of the file.
- Upload token, quotas, staging purge, audit log, HTTPS, configuration file and `config` command.
- Release archives for Linux, macOS and Windows (amd64, arm64), and container images on GitHub Container Registry and Docker Hub.
