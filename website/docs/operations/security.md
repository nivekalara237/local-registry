---
title: Security
description: The security model of local-registry, what is public, how the upload token works, and how to deploy behind HTTPS.
---

This page describes what local-registry protects, what it leaves open on purpose, and how to deploy it safely. Read it before you expose the server beyond `localhost`.

## The model in one table

| Action | Who can do it | Protection |
| --- | --- | --- |
| Read the catalogue, `release.json`, `index.json` | Anyone who reaches the server | None: reads are public. |
| Download a published file | Anyone who reaches the server | None. Integrity comes from sha256 checksums. See [Integrity](../concepts/integrity.md). |
| Upload files, publish a version | Holders of the upload token | `Authorization: Bearer <token>`, compared in constant time. |
| Change or delete a published version | Nobody through the API | Published versions are immutable (`409`). Only a person with access to the disk can change them. |
| Read the audit log, the staging area | Nobody through the API | Never served. |

> [!IMPORTANT]
> local-registry has no read authentication. Put it on a private network, or behind a proxy that authenticates readers, if your releases are confidential.

## The upload token

There is one upload token per server: a shared secret that every publisher presents.

- **Where it comes from**: `LOCAL_REGISTRY_UPLOAD_TOKEN`, else the `upload-token` key of the configuration file. There is no flag, because flags show in `ps`.
- **Without a token**, upload is disabled: the upload routes answer `403 upload is disabled: no token configured`, and startup prints `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled`. This is the right setting for a read-only mirror.
- **A wrong or missing token** gets `401 invalid or missing bearer token` with `WWW-Authenticate: Bearer`. The attempt is written to the [audit log](audit-log.md) as `auth_failed`.
- **The token is never printed or logged.** Logs, the audit log and `local-registry config` show its `token_id` instead: the first 12 hex digits of its sha256. Compute it yourself to match a log line with a token:

  ```sh
  printf %s "$LOCAL_REGISTRY_UPLOAD_TOKEN" | shasum -a 256 | cut -c1-12
  ```

  `release.json` records the `token_id` that published each version, as `publishedBy`.

### Create and store the token

```sh
openssl rand -hex 32
```

Store it in your secret manager, then hand it to the server and to the CI pipelines that publish:

- systemd: in the configuration file, owned by the service user, mode `0600`;
- Docker: `-e LOCAL_REGISTRY_UPLOAD_TOKEN` from the environment of the shell or of your orchestrator;
- Kubernetes: a `Secret`, read through `secretKeyRef` (see [Install](install.md));
- CI: a masked, protected secret variable.

Clients should send the token on standard input, never as a command-line argument. The [upload scripts](../reference/scripts.md) already do this:

```sh
printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" |
  curl -fsS -H @- -X POST "https://registry.example.com/my-product/artifacts/v1.2.0/publish"
```

### Rotate the token

The server reads the token at startup.

1. Generate a new token and store it in your secret manager.
2. Update the server's configuration or secret, then restart it. Uploads in staging survive the restart.
3. Update the CI secrets. Publishers still using the old token get `401`.

The new `token_id` appears in the logs from the restart on, so you can tell versions published before and after the rotation apart.

> [!WARNING]
> `--disable-generate-token=false` prints a generated token on standard output. systemd, Docker, Kubernetes and CI often keep that output. Use a fixed token anywhere logs are retained.

## HTTPS

Without TLS, everything travels in clear text, the token included. When upload is enabled on an address other than loopback without TLS, the server starts with this warning:

```
warning: upload is enabled on 0.0.0.0 without TLS: the token travels in clear text (use --tls-cert/--tls-key or a TLS reverse proxy)
```

You have two options.

### Option 1: TLS in the server

```sh
local-registry serve --asset-path /srv/registry --address 0.0.0.0 --port 8443 \
  --tls-cert /etc/local-registry/fullchain.pem --tls-key /etc/local-registry/privkey.pem
```

- The server speaks HTTPS only, with TLS 1.2 or later. A plain HTTP request gets `400 Client sent an HTTP request to an HTTPS server.`
- The certificate is loaded at startup. An invalid pair stops the server with exit code `1`. Restart it to load a renewed certificate.

### Option 2: a TLS reverse proxy (recommended)

Terminate TLS in your load balancer, Ingress controller or proxy, and keep the registry on a private address. Configure the proxy for uploads:

| Proxy setting | Value |
| --- | --- |
| Request body size | At least `--max-upload-mb` (200 MiB by default). |
| Read and send timeouts on `/…/artifacts/…/files/…` and `/…/publish` | Up to 30 minutes, the server's own upload limit. |
| Request buffering | Off for uploads if your proxy supports it, so large files stream through. |

Then set `public-url` to the address clients use (`https://registry.example.com`), so the download URLs the registry writes point to the proxy.

The server logs the `X-Forwarded-For` header it receives, as `forwarded_for`, but does not trust it: the `remote` field is always the TCP peer, here your proxy.

## What is public on disk

Only two trees under the asset path are ever served:

| Tree | Content | Served |
| --- | --- | --- |
| `bin.local/assets/` | Published versions | Yes |
| `assets/` | Legacy tree, read by `POST /releases` | Yes |
| `bin.local/.uploads/` | Staging: uploaded, not yet published | Never |
| `bin.local/.uploads-tmp/` | Files being received | Never |
| `bin.local/audit.log` | Audit log, mode `0600` | Never |
| Anything else | | Never |

- Hidden files and directories (names starting with `.`) are never served.
- The file download routes refuse a symbolic link that leaves the version directory. Catalogue and upload routes follow links found on disk: do not put links in the asset path.
- The server refuses to start if `--audit-log` points into a public tree.

Grant write access to the asset path to the service user only. Anyone who can write there can change published files.

## Hardening checklist

- [ ] Upload token of 32 random bytes, stored in a secret manager, never in a repository.
- [ ] HTTPS, in the server or in a reverse proxy; `public-url` set to the HTTPS address.
- [ ] The service runs as a dedicated, unprivileged user (`registry`, uid `10001` in the image).
- [ ] The asset path and the configuration file belong to that user only; the file is mode `0600`.
- [ ] Read access is restricted at the network level if releases are confidential.
- [ ] The audit log is collected and retained. See [Audit log](audit-log.md).
- [ ] Container images are pinned to an exact version and their signature is verified. See [Install](install.md).
- [ ] One server per asset path.
