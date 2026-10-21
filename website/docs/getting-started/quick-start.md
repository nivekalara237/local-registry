---
title: Quick start
description: Run a registry on your machine, publish a first version and download it, in about five minutes.
---

This page takes you from nothing to a published, downloadable version in about five minutes. You run the server locally, publish two small files, and fetch them back. No goreleaser needed.

## Before you start

| Tool | Why |
| --- | --- |
| Go 1.27+ | Build the server (or take a released binary instead) |
| bash, curl 7.76+, jq | Run `scripts/upload.sh` |
| A clone of the repository | The server sources and the upload script |

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
> The token is the only secret. Publishers send it, readers never need it. Without a token, the server still serves downloads but refuses every upload with `403`.

## 2. Prepare a version

A version is a set of files plus a product card, `registry.yaml`. Create both:

```sh
mkdir -p out
echo "hello from v0.1.0" > out/hello.txt
tar czf out/hello_0.1.0.tar.gz -C out hello.txt

cat > registry.yaml <<'YAML'
apiVersion: local-registry/v1
displayName: Hello
description: A first product, to try local-registry.
YAML
```

## 3. Publish it

In the second terminal, export the same token and the server address, then run the upload script:

```sh
export LOCAL_REGISTRY_UPLOAD_TOKEN="<the token from step 1>"
export LOCAL_REGISTRY_URL=http://localhost:8077

scripts/upload.sh --latest --version v0.1.0 --product-file registry.yaml hello out
```

The script uploads each file of `out/` with its sha256, then the product card, then asks the server to publish `v0.1.0` and make it the latest version:

```
== hello v0.1.0 from /home/me/out
PUT hello.txt <- /home/me/out/hello.txt (sha256 5d1f…)
PUT hello_0.1.0.tar.gz <- /home/me/out/hello_0.1.0.tar.gz (sha256 a8c2…)
PUT registry.yaml <- registry.yaml (sha256 0b7e…)
POST publish hello/v0.1.0 (latest=true)
{"product":"hello","version":"v0.1.0","files":["checksums.txt","hello.txt","hello_0.1.0.tar.gz","registry.yaml","release.json"],"latest":true}
```

Notice two files you did not upload: `checksums.txt` and `release.json`. The server wrote them. See [Integrity](../concepts/integrity.md) and [The local-registry/v1 standard](../concepts/standard-v1.md).

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

- Publish a real Go project: [Your first goreleaser release](./first-release-goreleaser.md).
- Publish files that are not binaries: [Publish files without goreleaser](./first-release-files.md).
- Run it for your team: [Install](../operations/install.md) and [Security](../operations/security.md).
