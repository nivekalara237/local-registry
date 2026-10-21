---
title: Publishing scripts
description: Options, modes, environment and exit codes of scripts/upload.sh and scripts/publish-to-local-registry.sh.
sidebar_label: Scripts
---

The repository ships two shell scripts that upload a version and publish it, so that CI needs no client of its own. `upload.sh` is the recommended one: it handles goreleaser output of any layout, and plain directories of files. `publish-to-local-registry.sh` is a smaller POSIX `sh` script for simple goreleaser outputs. For step-by-step use, see [Publish with goreleaser](../guides/publish-with-goreleaser.md) and [Publish plain files](../guides/publish-plain-files.md).

Both scripts read the same environment:

| Variable | Description |
| --- | --- |
| `LOCAL_REGISTRY_URL` | Server address, such as `https://registry.example.com`. A trailing `/` is removed (`upload.sh`). |
| `LOCAL_REGISTRY_UPLOAD_TOKEN` | Upload token. Sent to curl on its standard input, never on a command line, never printed. |

## upload.sh

```
upload.sh [--latest] [--dry-run] [--root DIR] [--version V] [--product-file F] <product> [artifacts.json | DIR]
```

Requires bash 3.2 or later, curl 7.76 or later (`--fail-with-body`) and jq; for a directory, `sha256sum` or `shasum` too.

| Argument | Description |
| --- | --- |
| `--latest` | Publish with `latest=true`: the version becomes the product's latest. |
| `--dry-run` | Print what would be sent and send nothing. No URL nor token needed. The local checks still run. |
| `--root DIR` | Directory the `path` fields of `artifacts.json` are relative to: goreleaser's working directory. Default: deduced from where `artifacts.json` sits (the directory of its `Metadata` or `Checksum` entry is stripped from it), else the working directory. |
| `--version V` | Version to publish. Default: `tag`, else `version`, of `metadata.json`: the one `artifacts.json` lists, or `DIR/metadata.json` for a directory. |
| `--product-file F` | Product card, sent as `registry.yaml`. Default: `registry.yaml` in the root (or in `DIR`), else in the working directory. Required: the script stops if none is found. |
| `<product>` | Product name. |
| `artifacts.json` or `DIR` | What to publish. Default: `dist/artifacts.json`; with `--root`, `ROOT/dist/artifacts.json` when it exists, else `ROOT` as a directory. A directory that holds an `artifacts.json` means that file. |

Options are written `--root DIR` or `--root=DIR`. `-h` or `--help` prints the usage.

### With artifacts.json (goreleaser)

1. Selects the published files of `artifacts.json` with the server's rule: types `Archive`, `File`, `Source`, `Linux Package`, `Checksum`, `Signature`, `Certificate`, `SBOM`, `Metadata`, and `Binary` with `extra.Format`.
2. Before sending anything, checks that names are unique, that no entry is named `artifacts.json`, that every file exists under the root, and that a product card is found.
3. Sends each file under its `name`, read from its `path`, with `X-Checksum-Sha256` when its entry has a `sha256:` checksum.
4. Sends `registry.yaml`, then the `CHANGELOG.md` that sits beside `artifacts.json` when it exists, then `artifacts.json` itself, last.
5. Calls publish.

```sh
export LOCAL_REGISTRY_URL=https://registry.example.com
scripts/upload.sh --latest sonarbridge dist/artifacts.json
```

```
== sonarbridge v1.4.2 from dist/artifacts.json (root /builds/platform/sonarbridge)
PUT metadata.json <- /builds/platform/sonarbridge/dist/metadata.json
PUT sonarbridge_1.4.2_linux_amd64.tar.gz <- /builds/platform/sonarbridge/dist/sonarbridge_1.4.2_linux_amd64.tar.gz (sha256 3f1c…)
…
PUT registry.yaml <- /builds/platform/sonarbridge/registry.yaml
PUT CHANGELOG.md <- /builds/platform/sonarbridge/dist/CHANGELOG.md
PUT artifacts.json <- dist/artifacts.json
POST publish sonarbridge/v1.4.2 (latest=true)
{"product":"sonarbridge","version":"v1.4.2","files":[…],"latest":true}
```

### With a directory (no goreleaser)

1. Lists every regular file of `DIR`. A subdirectory stops the script: archive it first. A file named `release.json`, `index.json` or `artifacts.goreleaser.json` stops it too: the server writes those.
2. Sends each file with `X-Checksum-Sha256`, computed locally.
3. Sends the product card as `registry.yaml` (a `registry.yaml` inside `DIR` is that card, unless `--product-file` names another).
4. Calls publish. The server infers each file's kind from its name and writes `checksums.txt`.

```sh
mkdir -p out
tar czf out/billing-db-migrations_1.0.0.tar.gz -C migrations changelog-master.xml changes
cp migrations/CHANGELOG.md out/
scripts/upload.sh --latest --version v1.0.0 --product-file migrations/registry.yaml billing-db-migrations out
```

### Errors and exit codes

| Exit code | When |
| --- | --- |
| `0` | Published, or dry run OK. |
| `1` | A local check failed, a variable is missing, or the server refused a request: its error message is printed. |
| `2` | Usage error (unknown option, wrong number of arguments). |

Messages, all prefixed with `upload.sh: `:

| Message | Fix |
| --- | --- |
| `jq is required`, `curl is required`, `sha256sum or shasum is required` | Install the tool. |
| `LOCAL_REGISTRY_URL is not set`, `LOCAL_REGISTRY_UPLOAD_TOKEN is not set` | Export the variable, or use `--dry-run`. |
| `<file> not found: run goreleaser first, or pass a directory of files` | Wrong path, or goreleaser did not run. |
| `<file> is not a JSON array` | Not a goreleaser `artifacts.json`. |
| `--root <dir> is not a directory` | Wrong `--root`. |
| `no uploadable file in <file>` | `artifacts.json` lists only build outputs. |
| `several entries of <file> share a name: <names>` | Two published files with one name: rename them in `.goreleaser.yaml`. |
| `<file> lists a file named artifacts.json` | Rename that file. |
| `<name>: <path> not found (wrong --root?)` | Pass the goreleaser working directory as `--root`. |
| `--product-file <file> not found` | Wrong path. |
| `no registry.yaml in <dir> nor in the current directory: pass --product-file` | Write a `registry.yaml` (see [registry.yaml](./registry-yaml.md)) or point to it. |
| `<dir>/<sub> is a directory: archive it first, e.g. tar czf …` | Archive the subdirectory. |
| `<path> is not a regular file` | Remove the special file from `DIR`. |
| `<path>: <name> is reserved, the server writes it` | Remove the file from `DIR`. |
| `no file to publish in <dir>` | `DIR` is empty. |
| `<file> lists no metadata.json: pass --version`, `no metadata.json in <dir>: pass --version`, `no tag nor version in <file>: pass --version` | Pass `--version`. |

After a failure, fix the cause and run the script again: staged files are replaced.

> [!NOTE]
> On Windows (Git Bash, MSYS2, Cygwin), the native `jq.exe` ends its lines with CRLF; the script removes the carriage returns. See [Windows](../guides/windows.md).

## publish-to-local-registry.sh

```
publish-to-local-registry.sh [--latest] <product> [dist-dir]
```

A POSIX `sh` script for goreleaser output only, when every file of the checksums file sits at the root of `dist-dir` (default `dist`) under its published name, as archives do. Otherwise, use `upload.sh`. `--latest` must come first.

1. Requires `dist-dir/metadata.json`, `dist-dir/artifacts.json`, exactly one `dist-dir/*checksums.txt`, and `registry.yaml` in the working directory.
2. Reads the version from the `tag` of `metadata.json`.
3. Sends each file listed in the checksums file, with `X-Checksum-Sha256`; then the checksums file; any `*.sig`, `*.pem`, `*.sigstore.json` of `dist-dir`; `metadata.json`; `registry.yaml`; `dist-dir/CHANGELOG.md` when it exists; `artifacts.json` last.
4. Calls publish.

```sh
scripts/publish-to-local-registry.sh --latest sonarbridge dist
```

It exits `1` with a message on stderr when a file is missing (`<dist>/<file> not found: run goreleaser first, or publish a directory of files with upload.sh --version <v> <product> <dir>`, `registry.yaml not found in the current directory: the server requires it`, `no tag in <dist>/metadata.json`, `expected exactly one *checksums.txt in <dist>`) or when the server refuses a request.
