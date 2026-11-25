---
title: Publish from Windows
sidebar_label: Windows
description: Publish from Windows with the native localrectl binary, and avoid the path and certificate pitfalls.
---

[`localrectl`](../reference/localrectl.md) ships a native Windows binary: no bash, curl or jq is needed to publish. This page covers what is specific to Windows. The steps themselves are the same as in [Publish with GoReleaser](publish-with-goreleaser.md) and [Publish plain files](publish-plain-files.md).

## Install localrectl

Download the Windows binary from the [localrectl Releases page](https://github.com/nivekalara237/localrectl/releases), check it against the checksums file published with it, and put it in a folder of your `PATH`.

Check that it reaches the registry:

```powershell
$env:LOCAL_REGISTRY_URL = "https://registry.corp.example"
localrectl ping
```

`localrectl completion powershell` prints the completion script for PowerShell.

## Publish

```powershell
$env:LOCAL_REGISTRY_URL = "https://registry.corp.example"
$env:LOCAL_REGISTRY_UPLOAD_TOKEN = "…"   # from your secret store, never on the command line

localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml --changelog-file dist/CHANGELOG.md --latest
```

`localrectl` reads the token from `LOCAL_REGISTRY_UPLOAD_TOKEN` and never prints it. Add `--dry-run` to see what would be sent without sending anything.

## Download

Without a selection, `localrectl download` takes the archive for the current machine: the zip on Windows.

```powershell
localrectl download bridge --extract -d C:\tools\bridge
```

## Pitfalls

### Line endings

`registry.yaml`, `CHANGELOG.md` and checksums files may use CRLF: the server accepts both line endings.

### File names

File names published to the registry may only use `A-Z a-z 0-9 . _ -`. Rename files with spaces or accents before you publish.

### Private certificates

If the registry uses a certificate from a private CA, give that CA's certificate to `localrectl`:

```powershell
$env:LOCAL_REGISTRY_CA_FILE = "C:\certs\corp-ca.pem"
```

More fixes in [Troubleshooting](../troubleshooting.md).
