<p align="center">
  <img alt="local-registry" src="./assets/logo-fl.svg?v=3&s=300" height="300" />
</p>

<p align="center">
  <a href="https://github.com/nivekalara237/local-registry/actions/workflows/ci.yml"><img alt="ci" src="https://github.com/nivekalara237/local-registry/actions/workflows/ci.yml/badge.svg" /></a>
  <a href="https://github.com/nivekalara237/local-registry/actions/workflows/tests.yml"><img alt="tests" src="https://github.com/nivekalara237/local-registry/actions/workflows/tests.yml/badge.svg" /></a>
  <a href="https://github.com/nivekalara237/local-registry/actions/workflows/security.yml"><img alt="security" src="https://github.com/nivekalara237/local-registry/actions/workflows/security.yml/badge.svg" /></a>
  <a href="https://codecov.io/gh/nivekalara237/local-registry"><img alt="coverage" src="https://codecov.io/gh/nivekalara237/local-registry/graph/badge.svg?token=3UEZCSODUL" /></a>
  <a href="https://github.com/nivekalara237/local-registry/actions/workflows/release.yml"><img alt="release" src="https://github.com/nivekalara237/local-registry/actions/workflows/release.yml/badge.svg" /></a>
</p>


<p align="center">
    <a href="https://github.com/nivekalara237/local-registry/releases"><img alt="latest version" src="https://img.shields.io/github/v/release/nivekalara237/local-registry?include_prereleases&sort=semver&display_name=tag&label=release" /></a>
    <a href="https://github.com/nivekalara237/local-registry/actions/workflows/docs.yml"><img alt="docs" src="https://github.com/nivekalara237/local-registry/actions/workflows/docs.yml/badge.svg" /></a>
    <a href="LICENSE"><img alt="licence" src="https://img.shields.io/badge/license-Apache--2.0-blue.svg" /></a>
</p>

<p align="center"><a href="https://local-registry.nivekaa.com"><b>Documentation</b></a> | <a href="README.fr.md">Français</a></p>

# local-registry

**A private release registry for your internal deliverables.** Teams publish versioned files to it (Go binaries built by [goreleaser](https://goreleaser.com), database migrations, configuration bundles, any file), and servers, scripts and pipelines download them from it over plain HTTP.

One static binary, no database: everything lives in a directory you can back up like any other.

## Why local-registry

- **A version appears whole, or not at all.** Files go to a private staging area, are checked, then published in one atomic step. A client never sees half a release.
- **What you download is what was built.** Every file is checked against its sha256 before publication; checksums, SBOMs and signatures travel with the version.
- **Published means immutable.** A version cannot change once published.
- **A stable contract for clients.** Each version gets a normalised `release.json`, each product an `index.json`: clients stop guessing file names. See the [`local-registry/v1` standard](SPEC.md).
- **goreleaser-native, not goreleaser-only.** Send a goreleaser `dist/` as is, or any directory of files with a short product card.
- **Built for operations.** Upload token, audit log, staging quotas and purge, HTTPS, signed multi-arch images with SBOM.

## Quick start

Run the server:

```sh
export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"
docker run -d --name local-registry -p 8077:8077 \
  -v local-registry-data:/data -e LOCAL_REGISTRY_UPLOAD_TOKEN \
  ghcr.io/nivekalara237/local-registry:1.0.0-rc.3
```

Publish a version, from a clone of this repository: a product card and any files.

```sh
mkdir -p out && echo "hello" > out/hello.txt
printf 'apiVersion: local-registry/v1\ndisplayName: Hello\n' > registry.yaml

LOCAL_REGISTRY_URL=http://localhost:8077 \
  scripts/upload.sh --latest --version v1.0.0 hello out
```

Consume it:

```sh
curl -s http://localhost:8077/hello/releases/latest     # the version, its files, their sha256
curl -sO http://localhost:8077/hello/artifacts/v1.0.0/files/hello.txt
```

With goreleaser, run `goreleaser release`, then `scripts/upload.sh --latest <product>`: the script reads `dist/artifacts.json`.

## Documentation

The full documentation, in English and French, lives in [`website/`](website/README.md). It is a Docusaurus site, and every page also reads on GitHub.

| I want to… | Read |
| --- | --- |
| Understand the model | [Introduction](website/docs/intro.md), [Staging and publish](website/docs/concepts/staging-and-publish.md) |
| Try it in 5 minutes | [Quick start](website/docs/getting-started/quick-start.md) |
| Publish from goreleaser or from CI | [Publish with goreleaser](website/docs/guides/publish-with-goreleaser.md), [CI pipelines](website/docs/guides/ci-pipelines.md) |
| Publish files without goreleaser | [Publish plain files](website/docs/guides/publish-plain-files.md) |
| Install and run the server | [Install](website/docs/operations/install.md), [Configuration](website/docs/operations/configuration.md), [Security](website/docs/operations/security.md) |
| Integrate with the API | [Consume releases](website/docs/guides/consume-releases.md), [HTTP API reference](website/docs/reference/api/index.md) |
| Fix a problem | [Troubleshooting](website/docs/troubleshooting.md) |

The [`local-registry/v1` standard](SPEC.md) is the normative definition of `registry.yaml`, `release.json`, `index.json` and the publication rules, for those who build clients or tools.

## Install

| Channel | Get it |
| --- | --- |
| Container image (linux/amd64, linux/arm64, signed) | `ghcr.io/nivekalara237/local-registry`, also on Docker Hub |
| Binaries (Linux, macOS, Windows) | [GitHub releases](https://github.com/nivekalara237/local-registry/releases) |
| From source (Go 1.27+) | `go build -o local-registry ./cmd` |

[Install](website/docs/operations/install.md) covers systemd, Docker and Kubernetes.

## Contributing

Issues and pull requests are welcome. Before opening a pull request:

```sh
gofmt -l . && go vet ./... && go test -race ./...
cd website && npm ci && npm run build    # when you change the documentation
```

## License

Apache License 2.0: see [LICENSE](LICENSE) and [NOTICE](NOTICE).
