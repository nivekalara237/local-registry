---
title: Publish from CI
sidebar_label: Publish from CI
description: Publish every tagged release from GitHub Actions or GitLab CI, with the token kept as a masked secret.
---

This guide automates publishing: each tag pushed to your repository builds the release and publishes it to the registry. The examples use GitHub Actions and GitLab CI. The same two steps apply to any CI system: run goreleaser, then run `upload.sh`.

## Before you start

1. **Store the token as a secret.** Name it `LOCAL_REGISTRY_UPLOAD_TOKEN`, masked, and available only to tag pipelines or a protected environment. Never write it in the pipeline file, and never print it.
2. **Store the registry address** as a plain variable `LOCAL_REGISTRY_URL`, for example `https://registry.example.com`.
3. **Commit `upload.sh`** into your repository, for example as `ci/upload.sh`, taken from a local-registry release archive. A pinned copy keeps every build reproducible.
4. **Commit `registry.yaml`** at the root of your repository. See [Write the product card](write-registry-yaml.md).

> [!TIP]
> Mark only stable versions as latest. A pre-release tag such as `v1.5.0-rc.1` contains a `-`: publish it without `--latest`, so consumers of `latest` keep the stable version.

## GitHub Actions

```yaml
# .github/workflows/release.yml
name: release

on:
  push:
    tags: ["v*"]

permissions:
  contents: read

jobs:
  publish:
    runs-on: ubuntu-latest
    environment: registry # holds the LOCAL_REGISTRY_UPLOAD_TOKEN secret
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0 # goreleaser needs the tags for its changelog

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: goreleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          version: "~> v2"
          args: release --clean --skip=publish

      - name: publish to the registry
        env:
          LOCAL_REGISTRY_URL: ${{ vars.LOCAL_REGISTRY_URL }}
          LOCAL_REGISTRY_UPLOAD_TOKEN: ${{ secrets.LOCAL_REGISTRY_UPLOAD_TOKEN }}
        run: |
          latest=
          case "$GITHUB_REF_NAME" in *-*) ;; *) latest=--latest ;; esac
          bash ci/upload.sh $latest sonarbridge dist/artifacts.json
```

- `--skip=publish` keeps goreleaser from creating a GitHub release. Remove it if you also want one.
- `environment: registry` lets you restrict the secret to tags and require an approval. Pin the actions to a commit sha if your policy asks for it.

## GitLab CI

```yaml
# .gitlab-ci.yml
release:
  stage: deploy
  image:
    name: goreleaser/goreleaser:v2.12.0
    entrypoint: [""]
  rules:
    - if: $CI_COMMIT_TAG =~ /^v/
  variables:
    GIT_DEPTH: 0 # goreleaser needs the tags for its changelog
  before_script:
    - apk add --no-cache bash curl jq
  script:
    - goreleaser release --clean --skip=publish
    - |
      latest=
      case "$CI_COMMIT_TAG" in *-*) ;; *) latest=--latest ;; esac
      bash ci/upload.sh $latest sonarbridge dist/artifacts.json
```

Define `LOCAL_REGISTRY_UPLOAD_TOKEN` as a masked and protected variable, and protect your `v*` tags, so that only protected tags can publish. Choose the goreleaser image tag your project uses.

## Plain files

For a product built without goreleaser, replace the goreleaser step with the commands that produce your files, and pass a directory and a version. See [Publish files without goreleaser](publish-plain-files.md).

```sh
tar czf out/billing-db-migrations_${CI_COMMIT_TAG#v}.tar.gz -C migrations changelog-master.xml changes
bash ci/upload.sh --latest --version "$CI_COMMIT_TAG" --product-file migrations/registry.yaml billing-db-migrations out
```

## Validate before you publish

Add these checks to merge requests, so a release never fails at the last step:

```sh
# The product card matches the schema shipped with local-registry
check-jsonschema --schemafile schema/registry.schema.json registry.yaml

# The release can be read and every file exists; sends nothing, needs no token
goreleaser release --snapshot --clean --skip=publish
bash ci/upload.sh --dry-run --version v0.0.0-dryrun sonarbridge dist/artifacts.json
```

## Behaviour to rely on

- **A failed job can be run again.** A refused publish leaves the files in staging, and the next run replaces them. See [Staging and publish](../concepts/staging-and-publish.md).
- **A version is immutable.** Running the job again after a successful publish answers `409 … already published`. Treat it as "already done", or publish a new tag.
- **Exit codes** of `upload.sh`: `0` published, `1` a local check failed, a variable is missing or the server refused (its message is printed), `2` wrong usage. See [Scripts](../reference/scripts.md).
- **Self-signed HTTPS.** Point curl at your CA with `CURL_CA_BUNDLE=/path/to/ca.pem`.
