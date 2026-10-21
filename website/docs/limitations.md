---
title: Guarantees and limitations
sidebar_label: Limitations
description: What local-registry guarantees, how, and the limits you should know before you depend on it.
---

This page states what local-registry promises, and where it stops. Use it to decide whether the registry fits a use case, and what to put around it.

## Guarantees

| Guarantee | How |
| --- | --- |
| A file being received is never visible. | It is written to `bin.local/.uploads-tmp/`, then renamed into staging once complete and checked. |
| A version appears complete, or not at all. | Publish renames the whole staging directory into the public tree in one operation. |
| Published files match their checksums. | Before publishing, every file is checked against the uploaded checksums file. Without one, publish computes the sha256 of every file and writes `checksums.txt`. |
| Download URLs in `artifacts.json` point to the version's own files. | They are rewritten in staging, after each listed file was checked, before the version becomes visible. |
| The latest version changes in one step. | The new `<product>/release.json` is written beside the old one, then renamed over it. |
| A published version never changes through the API. | Upload and publish of an existing version answer `409`. |
| Parallel uploads cannot exceed a quota together. | Quotas are checked again under the upload lock. |
| Every publication is traceable. | The audit log records it, with the sha256 of each file and the `token_id`, before the client gets its answer. |

These guarantees hold under two conditions:

- `bin.local/` is on **one file system**, because renames are atomic only within one;
- **one server** writes to a given asset path, because the upload lock lives in the process.

## Limitations

### Access control

- **Reads are public.** Everything under `bin.local/assets/` and `assets/` can be downloaded without authentication. Restrict access at the network level, or with an authenticating proxy, if releases are confidential.
- **One token per server.** Every publisher shares the same upload token. There are no per-team tokens, scopes or per-product permissions. The audit log tells publications apart by address and time, not by team.
- **Without TLS, the token travels in clear text.** Run HTTPS in the server or in a proxy. See [Security](operations/security.md).
- **`X-Forwarded-For` is recorded, not verified.**

### Availability and scale

- **One instance per asset path.** There is no clustering or replication. High availability means a fast restart on the same volume, and backups.
- **No cache.** Each request reads the disk. Product and index routes scan the version directories of a product at each call.
- **Downloads must finish within 20 seconds.** A large file over a slow link is cut. Clients can resume with a `Range` request (`curl -C -`). Put a caching proxy in front for large files.
- **No metrics endpoint.** Monitor through `/healthz`, the request logs and the audit log. See [Monitoring](operations/monitoring.md).

### Content

- **No delete route**, for staging or published versions. Removing a version is a manual operation on disk. See [Storage and backup](operations/storage-and-backup.md).
- **Flat files only.** A version holds files, not directories: archive a directory before you publish it.
- **Without `artifacts.json`, `os` and `arch` are unknown.** The server infers the kind of each file from its name only. Upload an `artifacts.json`, hand-written if needed, when consumers need the platform. See [Publish plain files](guides/publish-plain-files.md).
- **Versions published before the `local-registry/v1` standard** have no `release.json`. They are not listed by `/products` and `/<product>/index`, and cannot be the latest.
- **The audit log is not signed**, and a failed audit write does not fail the request.
- **Symbolic links** are checked on the file download routes only. Catalogue and upload routes follow links found on disk: keep links out of the asset path.
- **Configuration and TLS certificates are read at startup.** A change needs a restart.

### Deprecated

- `POST /releases/{version}` reads the `gitlab` product only and will be removed. Use `POST /releases/{product}/{version}`. See [Legacy routes](reference/api/legacy.md).
- `/artifacts/all` returns the raw content of every `artifacts.json`, `metadata.json` and checksums file. It is kept for existing clients; new clients should use [the product routes](reference/api/products.md).
