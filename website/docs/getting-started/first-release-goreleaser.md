---
title: Your first goreleaser release
description: Tutorial — build a Go project with goreleaser and publish it to local-registry, with its product card and release notes.
sidebar_label: First goreleaser release
---

This tutorial publishes a Go command-line tool, `sonarbridge`, built by goreleaser. At the end, version `v1.4.2` is on the registry with its archives, checksums, release notes and product card, and it is the product's latest version.

Use this path when your project is in Go and already uses goreleaser, or can. For any other kind of deliverable, follow [Publish files without goreleaser](./first-release-files.md).

## What you need

- A registry you can reach, and its upload token. Locally, the [Quick start](./quick-start.md) gives you one.
- goreleaser 2.x, git, bash, curl 7.76+ and jq.
- `scripts/upload.sh` from the local-registry repository. Copy it into your project, or call it by path.

## 1. Configure goreleaser

A minimal `.goreleaser.yaml` at the root of the project:

```yaml
version: 2
project_name: sonarbridge
builds:
  - main: ./cmd/sonarbridge
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

Nothing here is specific to local-registry. The server reads what goreleaser writes in `dist/`: the archives, the checksums file, `metadata.json`, `artifacts.json` and `CHANGELOG.md`.

## 2. Write the product card

Create `registry.yaml` next to `.goreleaser.yaml`. It describes the product: the server requires it with every version.

```yaml
apiVersion: local-registry/v1
displayName: SonarBridge
description: Bridge between SonarQube and internal tools.
license: Proprietary
owners:
  - name: Platform team
    email: platform@corp.example
links:
  - { name: source, url: "https://git.corp.example/platform/sonarbridge" }
keywords: [sonar, cli]
```

Commit it with the code: each version carries the card it was published with. Every field is in the [registry.yaml reference](../reference/registry-yaml.md).

## 3. Build the release

Tag the commit and run goreleaser. `--skip=publish` keeps goreleaser from pushing anywhere else; local-registry receives the files in the next step.

```sh
git tag v1.4.2
goreleaser release --clean --skip=publish,announce
```

> [!NOTE]
> goreleaser needs a git remote, even with `--skip=publish`. Without one it stops with `couldn't get remote URL`.

Write your commit messages as [Conventional Commits](https://www.conventionalcommits.org) (`feat: …`, `fix: …`). The server turns goreleaser's `CHANGELOG.md` into release notes: `feat` becomes *added*, `fix` becomes *fixed*, `fix(security)` becomes *security*. See [Write release notes](../guides/write-changelog.md).

## 4. Publish

```sh
export LOCAL_REGISTRY_URL=https://registry.example.com
export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # from your secret store, never on the command line

scripts/upload.sh --latest sonarbridge dist/artifacts.json
```

`upload.sh` reads `dist/artifacts.json` and, in this order:

1. Checks locally that every listed file exists and that a `registry.yaml` is found. Nothing is sent if a check fails.
2. Uploads each published file (archives, checksums file, `metadata.json`), with its sha256 when goreleaser recorded one, so the server verifies it on arrival.
3. Uploads `registry.yaml`, `dist/CHANGELOG.md`, then `artifacts.json` last.
4. Publishes `v1.4.2` (the `tag` of `metadata.json`) with `latest=true`.

> [!TIP]
> Run it once with `--dry-run` first. It prints every file it would send, sends nothing, and needs no token.

If the server refuses the publication, it prints why, for example `sonarbridge_1.4.2_linux_amd64.tar.gz: sha256 is … but … says …`. The uploaded files stay in staging: fix the cause and run the script again. See [Troubleshooting](../troubleshooting.md).

## 5. Check the result

```sh
curl -s https://registry.example.com/sonarbridge/index | jq '{latest, versions: [.versions[].version]}'
curl -s https://registry.example.com/sonarbridge/releases/latest | jq '.artifacts[] | {name, os, arch, sha256}'
```

The index lists the product card and its versions. The release document lists each archive with its platform, size, sha256 and download URL:

```json
{ "name": "sonarbridge_1.4.2_linux_amd64.tar.gz", "os": "linux", "arch": "amd64", "sha256": "3f1c…" }
```

These two documents are the stable way for clients to find and verify files. See [Consume releases](../guides/consume-releases.md).

## Next steps

- Automate it: [CI pipelines](../guides/ci-pipelines.md).
- Understand what happened on the server: [Staging and publish](../concepts/staging-and-publish.md) and [Latest version](../concepts/latest.md).
- Every option of the upload script: [Scripts reference](../reference/scripts.md).
