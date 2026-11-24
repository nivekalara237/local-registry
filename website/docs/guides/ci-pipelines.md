---
title: Publish from CI
sidebar_label: Publish from CI
description: Publish every tagged release from GitHub Actions or GitLab CI, with the token kept as a masked secret.
---

This guide automates publishing: each tag pushed to your repository triggers the build of the release, then its publication to the registry.

The examples use GitHub Actions and GitLab CI. Whatever your CI platform, the process rests on two steps: run GoReleaser, then run `localrectl publish`.

## Before you start

1. **Store the token as a secret.** Name it `LOCAL_REGISTRY_UPLOAD_TOKEN`. Make it a masked variable and restrict it to pipelines triggered by tags or run in a protected environment. Never write it in the pipeline file, and never print it in the job logs.
2. **Set the registry address** in an environment variable named `LOCAL_REGISTRY_URL`, for example `https://registry.corp.example`.
3. **Make `localrectl` available to the job.** Add to your CI image a binary from the [localrectl Releases page](https://github.com/nivekalara237/localrectl/releases), checked against its checksums file, or use its Docker image. See [localrectl](../reference/localrectl.md#install).
4. **Commit `registry.yaml` to your repository.** See [Write the product card](write-registry-yaml.md).

> [!TIP]
> Mark only stable versions as latest. Pre-release tags, such as `v1.5.0-rc.1`, contain a hyphen (`-`). Publish them without `--latest`, so that consumers of `latest` keep getting the latest stable version.

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
          fetch-depth: 0 # GoReleaser needs the tags to generate the changelog

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: goreleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          version: "~> v2"
          args: release --clean --skip=publish

      # localrectl must be installed on the runner: see "Before you start".
      - name: publish to the registry
        env:
          LOCAL_REGISTRY_URL: ${{ vars.LOCAL_REGISTRY_URL }}
          LOCAL_REGISTRY_UPLOAD_TOKEN: ${{ secrets.LOCAL_REGISTRY_UPLOAD_TOKEN }}
        run: |
          latest=
          case "$GITHUB_REF_NAME" in *-*) ;; *) latest=--latest ;; esac
          localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml \
            --changelog-file dist/CHANGELOG.md $latest
```

- `--skip=publish` keeps GoReleaser from publishing the release on GitHub. Remove it if you also want a GitHub release.
- `environment: registry` ties the job to a GitHub environment, to control access to the secret and, if needed, require an approval before the job runs. Pin the actions to commit SHAs if your security policy asks for it.

## GitLab CI

```yaml
# .gitlab-ci.yml
release:
  stage: deploy
  image:
    name: goreleaser/goreleaser:v2.12.0 # localrectl must be present in the image: see "Before you start"
    entrypoint: [""]
  rules:
    - if: $CI_COMMIT_TAG =~ /^v/
  variables:
    GIT_DEPTH: 0 # GoReleaser needs the tags to generate the changelog
  script:
    - goreleaser release --clean --skip=publish
    - |
      latest=
      case "$CI_COMMIT_TAG" in *-*) ;; *) latest=--latest ;; esac
      localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml \
        --changelog-file dist/CHANGELOG.md $latest
```

Define `LOCAL_REGISTRY_UPLOAD_TOKEN` as a masked and protected variable, then protect your tags matching `v*`. Only the pipelines allowed to use that variable can then publish releases.

Also choose the GoReleaser image tag that matches the version your project uses.

## Files without GoReleaser

If your product is built without GoReleaser, replace the GoReleaser step with the commands that produce your files. Then pass a directory holding the artifacts to publish, and the target version.

See [Publish files without GoReleaser](publish-plain-files.md).

```sh
tar czf out/billing-db-migrations_${CI_COMMIT_TAG#v}.tar.gz -C migrations changelog-master.xml changes

localrectl publish billing-db-migrations --root out --version "$CI_COMMIT_TAG" --no-semver-standard \
  --product-file migrations/registry.yaml --latest
```

## Validate before you publish

Add these checks to your merge request pipelines, to catch errors before a publication runs:

```sh
# Checks that the product card matches the schema shipped with Local Registry
check-jsonschema --schemafile schema/registry.schema.json registry.yaml

# Checks the release configuration and the files to send, without publishing or using a token
goreleaser release --snapshot --clean --skip=publish

localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml --dry-run
```

## Behaviour to know

- **A failed job can be run again.** A refused publish leaves the files in staging. The next run replaces the files sent before. See [Staging and publish](../concepts/staging-and-publish.md).
- **A published version is immutable.** Running the job again after a successful publish fails with `already published`. Add `--if-exists skip` so that this case succeeds without sending anything, or create a new tag.
- **Exit codes of `localrectl`:**
    - `0`: success.
    - `1`: failure, for example a local check or a refusal from the server (the error is printed on stderr).
    - `2`: wrong use of the command.

  See [localrectl](../reference/localrectl.md) for details.
- **HTTPS with a certificate from a private authority.** Give `localrectl` the certificate of that certificate authority with the `LOCAL_REGISTRY_CA_FILE` variable, for example `LOCAL_REGISTRY_CA_FILE=/path/to/ca.pem`.
