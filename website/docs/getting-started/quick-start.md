---
title: Quick start
description: Run a registry on your machine, publish a first version and download it, in about five minutes.
---

This page takes you from nothing to a published, downloadable version in about five minutes. You run a trial registry on your machine, publish two small files to it, and fetch them back. No GoReleaser needed.

Publishing to another registry works the same way: only the address and the upload token change. If you do not run that registry yourself, the person or team responsible for it gives you both.

## Before you start

| Tool | Why |
| --- | --- |
| Go 1.27+ and a clone of the repository | Build the server (or take a released binary, see [Install](../operations/install.md)) |
| [`localrectl`](../reference/localrectl.md) | Publish the version |
| curl | Read and download |

## 1. Start the server

Build the binary, create a storage directory, and choose an upload token.

```sh
go build -o local-registry ./cmd
mkdir -p ~/registry-data

export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"
./local-registry serve --asset-path ~/registry-data
```

The server listens on `http://localhost:8077`. Leave it running and open a second terminal for the next steps.

> [!NOTE]
> The token is the only secret. Publishers (upload, publish) send it; readers (downloads, GET API) never need it. Without a token, the server still serves downloads but refuses every upload with `403`.

## 2. Prepare a version

A version is a set of files plus a product card, `registry.yaml`. Create both:

```sh
mkdir -p out
echo "hello from v0.1.0" > out/hello.txt
tar czf out/hello_0.1.0.tar.gz -C out hello.txt

cat > registry.yaml <<'YAML'
apiVersion: local-registry/v1
displayName: Hello
description: A first product, to try Local Registry.
YAML
```

## 3. Publish it

In the second terminal, export the same token and the server address, then run `localrectl`:

```sh
export LOCAL_REGISTRY_UPLOAD_TOKEN="<the token from step 1>"
export LOCAL_REGISTRY_URL=http://localhost:8077

localrectl publish hello --root out --version v0.1.0 --no-semver-standard \
  --product-file registry.yaml --latest
```

`localrectl` uploads each file of `out/` and the product card, then asks the server to publish `v0.1.0` and make it the latest version. `--no-semver-standard` allows the `v` prefix in the version name.

> [!TIP]
> Add `--dry-run` to see what would be sent, without sending anything and without a token.

The published version holds two files you did not upload: `checksums.txt` and `release.json`. The server wrote them. See [Integrity](../concepts/integrity.md) and [The local-registry/v1 standard](../concepts/standard-v1.md).

## 4. Read and download it

Reads need no token:

```sh
curl -s http://localhost:8077/products                    # every product
curl -s http://localhost:8077/hello/releases/latest       # the latest version, described
curl -fsSO http://localhost:8077/hello/artifacts/v0.1.0/files/hello_0.1.0.tar.gz
```

`/hello/releases/latest` returns the `release.json` of `v0.1.0`: its files, their size, sha256 and download URL.

## What you just did

You used the full publication cycle: upload to staging, publish atomically, read through the API. Everything else in this documentation builds on it.

## Next steps

- Publish a real Go project: [Your first GoReleaser release](./first-release-goreleaser.md).
- Publish files that are not binaries: [Publish files without GoReleaser](./first-release-files.md).
- Run a registry of your own: [Install](../operations/install.md) and [Security](../operations/security.md).
