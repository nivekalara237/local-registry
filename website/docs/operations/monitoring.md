---
title: Monitoring
description: Health check, startup output, request logs, exit codes and timeouts of the server.
---

local-registry exposes a health endpoint, writes one log line per request, and uses exit codes your supervisor can act on. This page lists them, with what to alert on. The server has no metrics endpoint: derive metrics from the request logs and the [audit log](audit-log.md).

## Health check

```sh
curl -fsS http://localhost:8077/healthz
```

```json
{"status":"ok"}
```

`/healthz` answers `200` as soon as the server accepts requests. It needs no token and reads nothing on disk. Use it as the liveness and readiness probe, the Docker `HEALTHCHECK` (built into the image), or the load balancer check.

It does not check the disk. Watch free space separately (see [What to alert on](#what-to-alert-on)).

## Startup output

On startup, the server prints a banner on standard output:

```
##################################################
##          Local Registry Server               ##
##################################################

Listen      : http://127.0.0.1:8077
Start At    : 2026-10-18 10:56:12.091998 +0200 CEST m=+0.003639167
Assets Path : /srv/registry
```

`Listen` shows the scheme and the address actually bound: with `--port 0`, the port the system picked.

Warnings go to standard error before the banner:

| Line | Meaning |
| --- | --- |
| `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled` | Read-only server. Expected for a mirror, a mistake otherwise. |
| `warning: upload is enabled on … without TLS: the token travels in clear text …` | Upload on a network address without TLS in the server. Fine behind a TLS proxy only. |
| `warning: <file> holds the upload token but is readable by other users …` | `chmod 600` the configuration file. |
| `WARN upload token generation skipped: a token is already configured …` | `--disable-generate-token=false` with a configured token: the configured one stays in use. |

## Request logs

The server writes one line per request on standard error:

```
2026/10/18 10:56:15 INFO request method=PUT path=/billing-api/artifacts/v2.4.0/files/metadata.json status=201 bytes=112 duration=6.7ms remote=10.0.4.12
```

| Field | Content |
| --- | --- |
| `method`, `path` | The request. The query string is not logged. |
| `status` | The HTTP status sent. |
| `bytes` | Bytes of the response body. |
| `duration` | Time to serve the request. |
| `remote` | The TCP peer. |
| `forwarded_for` | `X-Forwarded-For` as sent, when present. Not verified. |

A request that fails adds a second line, `WARN` for a `4xx` and `ERROR` for a `5xx`, with the exact cause. For a `500`, this line is the only place the cause appears: the client receives `internal error`.

```
2026/10/18 10:58:02 WARN request failed method=PUT path=/billing-api/artifacts/v2.4.0/files/artifacts.json status=422 error="artifacts.json references billing-api_2.4.0_linux_amd64.tar.gz, which is not uploaded: upload every file first, artifacts.json last"
```

Other lines you may see:

| Line | Meaning |
| --- | --- |
| `INFO shutting down` | `SIGINT` or `SIGTERM` received. |
| `ERROR audit log write failed` | The audit log could not be written. The request succeeded anyway. |
| `ERROR staging purge failed` | A staged version could not be removed. |

## Exit codes

| Code | When |
| --- | --- |
| `0` | Clean stop: after `SIGINT` or `SIGTERM`, every running request finished within 10 seconds. |
| `1` | The server failed: port in use, address not available, invalid TLS certificate or key, audit log that cannot be opened, or requests still running 10 seconds after the stop signal (`Error: context deadline exceeded`). |
| `2` | Command-line or configuration error. Nothing was started. See [Configuration](configuration.md). |

With systemd, `Restart=on-failure` restarts on `1` and also on `2`. A configuration error then loops: `journalctl -u local-registry` shows the message.

## Timeouts

| Timeout | Value | Scope |
| --- | --- | --- |
| Reading the request headers | 5 s | All requests. |
| Writing the response | **20 s** | All routes but upload and publish, downloads included. |
| Reading and writing | 30 min | `PUT …/files/{name}` and `POST …/publish`. |
| Idle keep-alive connection | 60 s | All connections. |
| Graceful shutdown | 10 s | Running requests after `SIGINT` or `SIGTERM`. |

> [!WARNING]
> A download must finish within 20 seconds. A large file over a slow link is cut. Clients can resume with a `Range` request, for example `curl -C -`. Put large downloads behind a caching proxy if this matters to you.

Give your supervisor more than 10 seconds to stop the process: `TimeoutStopSec=15` for systemd, `terminationGracePeriodSeconds: 30` for Kubernetes.

## What to alert on

| Signal | Source | Why |
| --- | --- | --- |
| `/healthz` failing | Probe | The server is down. |
| Free space on the asset path below twice `min-free-mb` | Node or volume metrics | Uploads start failing with `507` below `min-free-mb`. |
| `status=5xx` in request logs | Logs | Disk or permission problems; the cause is on the `ERROR` line. |
| `ERROR audit log write failed` | Logs | The audit trail has a gap. |
| Bursts of `auth_failed` | Audit log | Someone is guessing the token, or a pipeline has an old one. |
| Many `publish_refused` | Audit log | Pipelines are broken; their staging fills up. |
| Staging size close to `max-staging-mb` | `du` on `bin.local/.uploads` | Next uploads will get `507`. |
