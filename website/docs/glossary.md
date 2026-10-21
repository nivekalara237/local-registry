---
title: Glossary
description: Every term of local-registry, defined in one or two lines.
---

The vocabulary used across this documentation, in alphabetical order.

| Term | Definition |
| --- | --- |
| **Artifact** | A downloadable file of a version: an archive, a binary, a package, a SQL script… Checksums files, signatures, SBOMs and the documents of the standard are not artifacts; they describe or cover them. |
| **artifacts.json** | The list of files goreleaser writes in `dist/`. Optional. When you upload it, the server checks it and rewrites its paths into download URLs. |
| **Attachment** | A signature, certificate or SBOM linked to the file it covers by its name (`<file>.sig`, `<file>.sbom.json`…). See [Integrity](./concepts/integrity.md). |
| **Checksums file** | A text file of `<sha256>  <name>` lines, one per artifact. Uploaded (goreleaser's `*checksums.txt`) or written by the server as `checksums.txt`. |
| **index.json** | The document that describes a product: its card and its versions. Written by the server. See [index.json](./reference/index-json.md). |
| **Latest version** | The version a product designates as current, chosen explicitly at publication. See [Latest version](./concepts/latest.md). |
| **Legacy tree** | `assets/binaries/<product>/<version>/` under the storage directory: files placed by hand before the upload API existed. Read as a fallback by `POST /releases/{product}/{version}` and served by `/assets/binary`. See [Legacy routes](./reference/api/legacy.md). |
| **metadata.json** | goreleaser's description of a build (tag, commit, date). Optional. Its values fill `release.json`. |
| **Product** | What a team delivers and versions, such as `sonarbridge`. The first segment of every URL. See [Products and versions](./concepts/products-and-versions.md). |
| **Product card** | `registry.yaml`: the description of a product (name, owners, links, license…), uploaded with each version. Required. See [registry.yaml](./reference/registry-yaml.md). |
| **Public URL** | The address clients use to reach the server (`--public-url`), for instance behind a reverse proxy. When set, download URLs in the documents are absolute. |
| **Publish** | The step that checks a staged version and makes it visible, atomically. See [Staging and publish](./concepts/staging-and-publish.md). |
| **Release document** | `release.json`: the normalised description of one version, written by the server at publication. See [release.json](./reference/release-json.md). |
| **Release notes** | The typed list of changes of a version (*added*, *fixed*, *security*…), extracted from `CHANGELOG.md`. |
| **Staging** | The private area where the files of a version wait until it is published. Purged after `--staging-ttl` of inactivity. |
| **Storage directory** | The directory given by `--asset-path`. It holds every published version, staging and the audit log. See [Storage and backup](./operations/storage-and-backup.md). |
| **token_id** | The first 12 hexadecimal digits of the sha256 of the upload token. Identifies the token in logs and in `publishedBy` without revealing it. |
| **Upload token** | The shared secret publishers send as `Authorization: Bearer <token>`. Reads need no token. See [Security](./operations/security.md). |
| **Version** | One immutable, published release of a product, usually named after a git tag (`v1.4.2`). |
