---
title: localrectl
description: The command-line client of Local Registry. Settings, commands, publish, download and exit codes.
sidebar_label: localrectl
---

`localrectl` is the command-line client of the registry. It reads products, versions and releases, downloads files and checks their sha256, validates product cards, and publishes new versions. Reading needs no token; publishing needs the upload token.

This page sums up what the guides use. Every option and example is in the [localrectl README](https://github.com/nivekalara237/localrectl/blob/master/README.md).

## Install

- **Binaries** for Linux, macOS and Windows, with a checksums file, on the [Releases page](https://github.com/nivekalara237/localrectl/releases).
- **Docker image** [`nivekalara237/localrectl`](https://hub.docker.com/r/nivekalara237/localrectl):

  ```sh
  docker run --rm -e LOCAL_REGISTRY_URL=https://registry.corp.example nivekalara237/localrectl ping
  ```

## Usage

```
localrectl <command> [args] [flags]
```

Every command accepts `--help`. The result goes to stdout; progress bars, logs and errors go to stderr. Add `-o json` to get a result a script can parse.

## Configuration

Each setting comes, in order of priority, from a flag, an environment variable, the config file, then the default. The config file is the one given with `--config`, else `config`, `config.yaml` or `config.yml`, looked up in `/etc/localrectl/`, `~/.lr/`, then the current folder.

| Flag | Environment variable | Config file key | Description |
| --- | --- | --- | --- |
| `--server-url` | `LOCAL_REGISTRY_URL` | `server.url` | Registry address. Default: `http://localhost:8077`. |
| — | `LOCAL_REGISTRY_UPLOAD_TOKEN` | `server.upload-token` | Upload token. Needed by `publish` only. |
| `--server-ca-file` | `LOCAL_REGISTRY_CA_FILE` | `server.ca-file` | CA certificate of a registry whose HTTPS certificate comes from a private authority. |
| `-o, --output text\|json` | `LOCAL_REGISTRY_OUTPUT` | — | Output format. |

Other flags: `-v, --verbose`, `-q, --quiet`, `--no-color`, `--timeout` (seconds, default 300), `--retries` (default 3), `--exit-code` (default 1).

`localrectl config` prints the settings in use and where each one comes from.

> [!WARNING]
> Never pass the upload token on the command line: set `LOCAL_REGISTRY_UPLOAD_TOKEN`. `localrectl` never prints it.

## Commands

| Command | What it does |
| --- | --- |
| `ping` | Checks that the registry answers (`GET /healthz`). |
| `products` | Lists every product, all pages read. |
| `products search` | Searches the products, one page at a time: `-Q, --query`, `-k, --keyword` (repeatable), `-l, --label key=value` (repeatable), `--owner`, `--license`, `--deprecated[=false]`, `--prerelease[=false]`, `--updated-since DATE`, `--sort`, `--limit`, `-c, --cursor`, `--all`. |
| `versions <product>` | Lists the versions of a product. `--prerelease` adds the pre-releases. |
| `release <product> <version\|latest>` | Prints the `release.json` of a version: its artifacts and its checksums file. |
| `validate <registry.yaml>` | Checks a product card against the registry's schema. `--artifacts dist/artifacts.json` also checks a GoReleaser `artifacts.json`. |
| `download <product> [version]` | Downloads files of a version (default `latest`) and checks each sha256. See [download](#download). |
| `publish <product>` | Uploads the files of a new version, then publishes it. See [publish](#publish). |
| `config`, `version`, `completion bash\|zsh\|fish\|powershell` | Settings in use, client version, shell completion. |

## publish

Required: `--product-file registry.yaml`, and `--version X.Y.Z`, except for a GoReleaser build whose `metadata.json` gives the version. The upload token is required, except with `--dry-run`.

Where the files come from:

- **A GoReleaser build:** `--goreleaser-dir dist`; or `--goreleaser-artifacts-file` and `--goreleaser-metadata-file`; or a root folder that holds an `artifacts.json`.
- **A folder:** `-R, --root DIR` (default: the current folder). Every regular file in it is sent; hidden files never are. A subfolder fails the command, unless `--skip-subdirs`, or `--compress-subdirs` (`--compress-type tar.gz|zip`).

| Flag | Description |
| --- | --- |
| `--changelog-file F` | Sends `F` as `CHANGELOG.md`. |
| `--exclude NAME` (repeatable), `--exclude-match 'PATTERN'` | Files or subfolders of the root folder not to send. Not allowed with a GoReleaser build. |
| `--latest` | Makes the version the latest. |
| `--if-exists fail\|skip` | When the version is already published: fail (default), or succeed without sending anything. |
| `--no-semver-standard` | Accepts a version such as `v1.2.0`. |
| `--no-validate` | Skips the local checks. |
| `--dry-run` | Shows what would be sent, sends nothing. |

```sh
localrectl publish billing-agent --root ./out --version 1.2.0 --product-file registry.yaml --dry-run
localrectl publish billing-agent --goreleaser-dir dist --product-file registry.yaml --latest --if-exists skip
```

A published version never changes: publishing it again fails with `already published`, unless `--if-exists skip`.

## download

Which files, read from the `release.json` of the version:

| Selection | Files |
| --- | --- |
| `--file NAME` (repeatable) | These files, by name. |
| `--all` | Every file, the checksums file included. |
| `--os linux\|darwin\|macos\|windows`, `--arch amd64\|arm64\|…`, `--kind archive\|binary\|package\|file\|source` | Every matching file. |
| none | The archive for this machine: zip on Windows, tar.gz elsewhere. |

`--all`, `--file` and the filters cannot be combined.

| Flag | Description |
| --- | --- |
| `-d, --dir DIR` | Destination folder, created if missing. Default: the current folder. |
| `--force` | Replaces a file or an extracted entry that differs. Without it, a different existing file fails the command; a file with the same sha256 is skipped (`up-to-date`). |
| `--extract` | Extracts the downloaded tar.gz or zip archives into `--dir` and keeps the archives. Links and entries leading outside `--dir` are refused. |
| `--no-verify` | Skips the sha256 check. |
| `--dry-run` | Shows what would be downloaded, writes nothing. |
| `--verify-signature` | Verifies the Sigstore signature of the release's checksums file with `cosign` (which must be in `PATH`), then checks each file against that signed file. Needs `--key PUBLIC_KEY`, or `--certificate-identity ID` (or `--certificate-identity-regexp RE`) with `--certificate-oidc-issuer URL`. The signature must be in the Rekor transparency log. |

```sh
localrectl download billing-agent --extract -d /opt/billing-agent
localrectl download billing-agent v1.2.0 --os linux --arch arm64 -o json
localrectl download billing-agent --verify-signature --key cosign.pub
```

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success. |
| `1` | Failure, or the value of `--exit-code`. |
| `2` | Wrong use of the command: unknown flag, missing argument, wrong setting. |
