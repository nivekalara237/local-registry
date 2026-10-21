---
title: Publish from Windows
sidebar_label: Windows
description: Run upload.sh on Windows with Git Bash, and avoid the line-ending and path pitfalls.
---

`upload.sh` is a bash script. On Windows, run it from **Git Bash** (installed with Git for Windows), MSYS2 or WSL. This page lists what to install and the pitfalls specific to Windows. The steps themselves are the same as in [Publish with goreleaser](publish-with-goreleaser.md) and [Publish plain files](publish-plain-files.md).

## Install the tools

| Tool | Where it comes from |
| --- | --- |
| `bash`, `sha256sum`, `tar` | Git for Windows (Git Bash) |
| `curl` | Windows 10 and later, and Git for Windows |
| `jq` | Not included: install `jq.exe` from [jqlang.org](https://jqlang.org/download/), or with a package manager such as `winget` or `scoop`, and make sure it is on the `PATH` of Git Bash |

Check them in Git Bash:

```sh
bash --version && curl --version && jq --version && sha256sum --version
```

## Publish

```sh
export LOCAL_REGISTRY_URL=https://registry.example.com
export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # from your secret store
./ci/upload.sh --latest --root . sonarbridge dist/artifacts.json
```

Run it from Git Bash, not from PowerShell or `cmd.exe`. In WSL, everything behaves as on Linux: install the Linux `jq` there.

## Pitfalls

### `jq.exe` and line endings

The native `jq.exe` ends its lines with CRLF. A stray carriage return in a name or a checksum breaks the URL, and the server answers `400 Bad Request`. `upload.sh` removes these carriage returns itself. If you see `)UT …` lines or a `400` with an older copy of the script, update it from a recent local-registry release.

> [!WARNING]
> `publish-to-local-registry.sh` does not handle `jq.exe` output, and is meant for POSIX systems. Use `upload.sh` on Windows.

### Scripts checked out with CRLF

If Git converts line endings (`core.autocrlf=true`), a script checked out with CRLF fails with `$'\r': command not found`. Keep shell scripts in LF with a `.gitattributes` file at the root of your repository:

```
*.sh text eol=lf
```

Then check the files out again: `git rm --cached -r . && git reset --hard`. Commit or stash your changes before you run this.

`registry.yaml`, `CHANGELOG.md` and checksums files may use CRLF: the server accepts both line endings.

### Paths

- Use forward slashes and relative paths: `dist/artifacts.json`, `--root .`.
- Pass `--root .` when you run the script from the directory goreleaser ran in. The script then reads every `path` of `artifacts.json` from there.
- File names published to the registry may only use `A-Z a-z 0-9 . _ -`. Rename files with spaces or accents before you publish.

### Self-signed certificates

If the registry uses a certificate from your company's CA, point curl at it:

```sh
export CURL_CA_BUNDLE=/c/certs/corp-ca.pem
```

More fixes in [Troubleshooting](../troubleshooting.md).
