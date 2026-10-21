---
title: Audit log
description: What the audit log records, its format, how to query it, and how to retain it.
---

The audit log records every security-relevant event of the upload routes: files accepted and refused, versions published with the sha256 of each file, failed authentications, and purges. Use it to answer "who published what, when, and from where", and to detect attacks on the upload token.

## Where it is

| Setting | Default |
| --- | --- |
| `audit-log` | `<asset-path>/bin.local/audit.log` |

- The server creates the file, and its directory, at startup. The file mode is `0600`.
- The file is never served. The server refuses to start if `audit-log` points into a public tree (`bin.local/assets/` or `assets/`).
- If the file cannot be opened at startup, the server stops with exit code `1`.

## Guarantees

- **One JSON object per line**, appended.
- **Written and synced to disk before the client gets its answer.** A version the client saw as published is in the log.
- **The token never appears**, only its `token_id`: the first 12 hex digits of its sha256. See [Security](security.md).
- **A failed write does not fail the request.** The server logs `ERROR audit log write failed` on standard error instead. Alert on that line.

> [!NOTE]
> The audit log is not signed. Ship it to a write-once store if you need tamper evidence.

## Format

Every line has these fields:

| Field | Content |
| --- | --- |
| `time` | UTC, RFC 3339 with nanoseconds. |
| `event` | The event, listed below. |
| `remote` | The TCP peer: your reverse proxy, if there is one. Absent for purge events. |
| `forwarded_for` | The `X-Forwarded-For` header as sent, when present. Not verified. |

Events:

| `event` | When | Other fields |
| --- | --- | --- |
| `auth_failed` | `401` or `403` on an upload route | `method`, `path`, `status`, `error`, `authorization_sent` |
| `upload` | A file is accepted into staging | `product`, `version`, `name`, `size`, `sha256`, `replaced`, `token_id` |
| `upload_refused` | A file is refused after authentication | `product`, `version`, `name`, `status`, `error`, `token_id` |
| `publish` | A version is published | `product`, `version`, `latest`, `files`, `token_id` |
| `publish_refused` | A publish is refused after authentication | `product`, `version`, `status`, `error`, `token_id` |
| `staging_purged` | The purge removed a staged version | `product`, `version`, `files`, `bytes`, `idle` |
| `tmp_purged` | The purge removed temporary files | `files` |

- `files` of a `publish` event lists `{name, size, sha256}` for every published file, including the files the server writes: the rewritten `artifacts.json`, `artifacts.goreleaser.json`, `release.json` and a generated `checksums.txt`.
- For a `500`, `error` holds the real cause. The client only received `internal error`.
- `authorization_sent` tells a missing header apart from a wrong token.

Example lines:

```json
{"time":"2026-10-18T09:12:03.418Z","event":"upload","product":"billing-api","version":"v2.4.0","name":"billing-api_2.4.0_linux_amd64.tar.gz","size":8123456,"sha256":"8f6c…","replaced":false,"token_id":"4ae3c36ce073","remote":"10.0.4.12"}
{"time":"2026-10-18T09:12:05.902Z","event":"publish","product":"billing-api","version":"v2.4.0","latest":true,"files":[{"name":"billing-api_2.4.0_checksums.txt","size":412,"sha256":"2b51…"}],"token_id":"4ae3c36ce073","remote":"10.0.4.12"}
{"time":"2026-10-18T09:20:44.017Z","event":"auth_failed","method":"PUT","path":"/billing-api/artifacts/v2.4.1/files/x","status":401,"error":"invalid or missing bearer token","authorization_sent":true,"remote":"10.0.9.80"}
```

## Query it

```sh
LOG=/srv/registry/bin.local/audit.log

# Every publication: when, what, from where
jq -r 'select(.event=="publish") | "\(.time) \(.product)/\(.version) latest=\(.latest) by \(.token_id) from \(.remote)"' $LOG

# Failed authentications, by source address
jq -r 'select(.event=="auth_failed") | .forwarded_for // .remote' $LOG | sort | uniq -c | sort -rn

# Why publishes were refused
jq -r 'select(.event=="publish_refused") | "\(.time) \(.product)/\(.version) \(.status) \(.error)"' $LOG

# Which publication delivered a given file
jq -c --arg n billing-api_2.4.0_linux_amd64.tar.gz \
  'select(.event=="publish") | select(any(.files[]; .name==$n)) | {time, product, version, token_id}' $LOG
```

## Rotation and retention

The server keeps the file open and only appends. Rotate it with an external tool that truncates in place, so the server keeps writing to the same file:

```
# /etc/logrotate.d/local-registry
/srv/registry/bin.local/audit.log {
    weekly
    rotate 52
    compress
    missingok
    notifempty
    copytruncate
}
```

`copytruncate` is required: a rename-and-create rotation would leave the server writing to the old, renamed file until the next restart.

Retention advice:

- Keep at least as long as your release retention policy, so every published version can be traced to its upload.
- Ship the lines to your log platform (Loki, Elasticsearch, Splunk…) and alert on `auth_failed` bursts and on `audit log write failed`.
- Back up the file with the asset path. See [Storage and backup](storage-and-backup.md).
