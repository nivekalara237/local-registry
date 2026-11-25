---
title: Your first GoReleaser release
description: Tutorial — build a Go project with GoReleaser and publish it to the registry, with its product card and release notes.
sidebar_label: First GoReleaser release
---

This tutorial publishes a Go command-line tool, `bridge`, built by GoReleaser. At the end, version `v1.4.2` is on the registry with its archives, checksums, release notes and product card, and it is the product's latest version.

Follow this path when your project is written in Go and already uses GoReleaser, or can adopt it. For any other kind of deliverable, follow [Publish files without GoReleaser](./first-release-files.md).

## What you need

- The registry address and its upload token. If you do not run the registry yourself, ask the person or team responsible for it. To practise on your machine, the [Quick start](./quick-start.md) runs a trial registry.
- GoReleaser 2.x and git.
- [`localrectl`](../reference/localrectl.md), the command-line client of the registry.
- curl and jq, to check the result.

## 1. Configure GoReleaser

A minimal `.goreleaser.yaml` at the root of the project:

```yaml
version: 2
project_name: bridge
builds:
  - main: ./cmd/bridge
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
checksum:
  name_template: "{{ .ProjectName }}_{{ .Version }}_checksums.txt"
changelog:
  use: git
```

Nothing here is specific to the registry. The server reads what GoReleaser writes in `dist/`: the archives, the checksums file, `metadata.json`, `artifacts.json` and `CHANGELOG.md`.

## 2. Write the product card

Create `registry.yaml` next to `.goreleaser.yaml`. This file describes the product; the server requires it with every version.

```yaml
apiVersion: local-registry/v1
displayName: Bridge
description: "Sonar Bridge: bridge between SonarQube and internal tools."
license: Proprietary
owners:
  - name: Platform team
    email: platform@corp.example
links:
  - { name: source, url: "https://git.corp.example/platform/sonar-bridge" }
keywords: [sonar, cli]
```

Commit it with the code: each version carries the card it was published with. Every field is in the [registry.yaml reference](../reference/registry-yaml.md).

## 3. Build the release

Tag the commit and run GoReleaser. `--skip=publish` keeps GoReleaser from publishing anywhere else; the registry receives the files in the next step.

```sh
git tag v1.4.2
goreleaser release --clean --skip=publish,announce
```

Write your commit messages as [Conventional Commits](https://www.conventionalcommits.org) (`feat: …`, `fix: …`). The server turns GoReleaser's `CHANGELOG.md` into release notes: `feat` becomes *added*, `fix` becomes *fixed*, `fix(security)` becomes *security*. See [Write release notes](../guides/write-changelog.md).

## 4. Publish

```sh
export LOCAL_REGISTRY_URL=https://registry.corp.example
export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # from your secret store, never on the command line

localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml \
  --changelog-file dist/CHANGELOG.md --latest
```

`localrectl` reads the GoReleaser build in `dist/`, uploads its files, the product card and the release notes, then publishes the version given by `dist/metadata.json`, here `v1.4.2`. `--latest` makes it the product's latest version.

> [!TIP]
> Run the command with `--dry-run` first: it shows what would be sent, sends nothing and needs no token.

If the server refuses the publication, `localrectl` prints the error and exits with code `1`. The server gives the cause, for example `bridge_1.4.2_linux_amd64.tar.gz: sha256 is … but … says …`. The uploaded files stay in staging: fix the cause and run the command again. See [Troubleshooting](../troubleshooting.md).

## 5. Check the result

```sh
curl -s https://registry.corp.example/bridge/index | jq '{latest, versions: [.versions[].version]}'
curl -s https://registry.corp.example/bridge/releases/latest | jq '.artifacts[] | {name, os, arch, sha256}'
```

The index lists the product card and its versions. The release document lists each archive with its platform, size, sha256 and download URL:

```json
{ "name": "bridge_1.4.2_linux_amd64.tar.gz", "os": "linux", "arch": "amd64", "sha256": "3f1c…" }
```

These two documents are the stable way for clients to find and verify files. See [Consume releases](../guides/consume-releases.md).

## Next steps

- Automate it: [CI pipelines](../guides/ci-pipelines.md).
- Understand what happened on the server: [Staging and publish](../concepts/staging-and-publish.md) and [Latest version](../concepts/latest.md).
- Every publishing option: [localrectl reference](../reference/localrectl.md).
