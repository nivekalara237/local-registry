---
title: What is Local Registry?
description: An open source release registry that stores versioned files, checks them and serves them over a stable HTTP API.
sidebar_label: Introduction
---

Local Registry is an open source release registry. You publish your versioned deliverables to it (Go binaries built by GoReleaser, database migrations, configuration bundles, any file), and servers, scripts and pipelines download them from it.

You can install it on your own server, or use an instance run by the person or team responsible for it. In this documentation, the registry is reachable at `https://registry.corp.example`: replace it with the address of your instance. Publishing needs the upload token set in the server configuration; reading needs no token.

On the server side, Local Registry is one static binary with no database: it stores everything in a plain directory and serves it over HTTP.

## Why use it

Releases you distribute yourself need a place that is neither a public release page nor a full artifact manager. Local Registry fills that need with three guarantees:

| Guarantee | What it means for you |
| --- | --- |
| **A version appears whole, or not at all** | Files are uploaded to a private area (staging), checked, then published in one atomic operation. A client never sees half a release. |
| **What you download is what was built** | The SHA-256 of every file is checked before publication. The checksums travel with the version. |
| **A published version never changes** | Once published, a version is immutable. Uploading to it again answers `409 Conflict`. |

On top of the files, the server writes a normalised description of each version, `release.json`, and of each product, `index.json`. Clients read these documents instead of guessing file names. This is the [`local-registry/v1` standard](./concepts/standard-v1.md).

## How it works

![localrectl uploads the files to the private staging area with the token and asks for publication; after the checks, the version joins the immutable published versions, which servers, pipelines and scripts download without a token](/img/fonctionnement-lr.gif)

1. A publisher uploads the files of a version with the upload token, usually with [`localrectl`](./reference/localrectl.md).
2. It asks for publication. The server checks the files, writes `release.json` and makes the version visible in one step.
3. Any machine on the network reads the catalogue and downloads the files. Reading needs no token.

The details are in [Staging and publish](./concepts/staging-and-publish.md).

## Who this documentation is for

| You are… | Start with |
| --- | --- |
| **A developer or release engineer** who publishes versions | [Quick start](./getting-started/quick-start.md), then [Your first GoReleaser release](./getting-started/first-release-goreleaser.md) or [Publish files without GoReleaser](./getting-started/first-release-files.md) |
| **An administrator** who installs and runs the server | [Install](./operations/install.md), [Configuration](./operations/configuration.md), [Security](./operations/security.md) |
| **An integrator** who consumes releases | [Consume releases](./guides/consume-releases.md) and the [HTTP API reference](./reference/api/index.md) |

## What it is not

Knowing the scope of Local Registry avoids surprises:

- **Not a container or package registry.** It speaks neither Docker, npm, Maven nor APT. It serves files over plain HTTP.
- **No read access control.** Everything published is readable by anyone who can reach the server. The registry must therefore stay on a private network, or behind a reverse proxy that authenticates. See [Security](./operations/security.md).
- **Not multi-node.** One server owns one storage directory. See [Limitations](./limitations.md).

> [!TIP]
> New to the vocabulary (product, version, staging, latest)? The [Glossary](./glossary.md) defines each term in one line.
