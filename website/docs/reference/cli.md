---
title: Command line
description: Every command, flag, environment variable and configuration key of the local-registry binary, with its exit codes.
sidebar_label: Command line
---

`local-registry` is a single binary. `serve` runs the server; `config` shows the configuration `serve` would use. This page lists every setting; [Configuration](../operations/configuration.md) explains how to choose them.

## Commands

| Command | Effect |
| --- | --- |
| `serve` | Run the server until `SIGINT` or `SIGTERM`. |
| `config` | Resolve and validate the configuration exactly as `serve` does, print each setting with its source, start nothing. |
| `version` (or `--version`, `-v`) | Print `local-registry v1.4.2 (go1.27.2, linux/amd64, commit 9c9d7664ceef)`. |
| `completion bash\|zsh\|fish\|powershell` | Print a shell completion script. |
| `help [command]` (or `-h`, `--help`) | Help. `serve --help` lists every flag with its environment variable and default. |

`local-registry` alone prints the help.

```sh
local-registry serve --asset-path /srv/registry
local-registry config --config /etc/local-registry/config.yaml
```

## Where settings come from

Each setting is taken from the first source that defines it:

1. the command-line flag: `--port 9000`;
2. the environment variable: `LOCAL_REGISTRY_` + the flag name in upper case, `-` replaced by `_`: `LOCAL_REGISTRY_PORT=9000`;
3. the YAML file given by `--config` (or `LOCAL_REGISTRY_CONFIG`): `port: 9000`;
4. the default.

Flags are written `--port 8077` or `--port=8077`; a single dash (`-port`) is refused. A boolean takes `=`: `--disable-generate-token=false`.

## Settings

| Flag | Environment variable | Default | Description |
| --- | --- | --- | --- |
| `--config` | `LOCAL_REGISTRY_CONFIG` | — | YAML configuration file. No file is read without it. |
| `--asset-path` | `LOCAL_REGISTRY_ASSET_PATH` | **required** | Root of all data; must be an existing directory. The working directory is never used by default. |
| `--address` | `LOCAL_REGISTRY_ADDRESS` | `localhost` | Listen address; `0.0.0.0` for every interface. |
| `--port` | `LOCAL_REGISTRY_PORT` | `8077` | Listen port, 0 to 65535; `0` picks a free port. |
| `--tls-cert` | `LOCAL_REGISTRY_TLS_CERT` | — | PEM certificate (chain). With `--tls-key`, the server speaks HTTPS only. |
| `--tls-key` | `LOCAL_REGISTRY_TLS_KEY` | — | PEM private key. Both TLS settings or neither. |
| `--max-upload-mb` | `LOCAL_REGISTRY_MAX_UPLOAD_MB` | `200` | Largest uploaded file, MiB. `0` or less means 200. |
| `--max-version-mb` | `LOCAL_REGISTRY_MAX_VERSION_MB` | `2048` | Bytes staged per version, MiB. `0`: no limit. |
| `--max-version-files` | `LOCAL_REGISTRY_MAX_VERSION_FILES` | `100` | Files staged per version. `0`: no limit. |
| `--max-staging-mb` | `LOCAL_REGISTRY_MAX_STAGING_MB` | `10240` | Bytes staged for all versions together, MiB. `0`: no limit. |
| `--min-free-mb` | `LOCAL_REGISTRY_MIN_FREE_MB` | `1024` | Disk space an upload must leave free, MiB. `0`: no check. |
| `--staging-ttl` | `LOCAL_REGISTRY_STAGING_TTL` | `24h` | Purge staged versions untouched for this long. A Go duration: `0` (never) or at least `1h`. |
| `--audit-log` | `LOCAL_REGISTRY_AUDIT_LOG` | `<asset-path>/bin.local/audit.log` | Audit log file. Refused inside `bin.local/assets/` or `assets/`, which are downloadable. |
| `--public-url` | `LOCAL_REGISTRY_PUBLIC_URL` | — | Address clients reach the server at, such as `https://registry.example.com` (a path prefix is allowed). Makes the download URLs the server writes absolute. Empty: relative URLs. |
| `--disable-generate-token` | `LOCAL_REGISTRY_DISABLE_GENERATE_TOKEN` | `true` | `=false`: generate a random upload token at startup when none is configured. |
| *(no flag)* | `LOCAL_REGISTRY_UPLOAD_TOKEN` | — | Upload token; config key `upload-token`. Empty: upload is disabled (`403`). |

The quotas are explained in [Quotas and staging](../operations/quotas-and-staging.md).

**Paths.** `config`, `asset-path`, `tls-cert`, `tls-key` and `audit-log` expand `$VAR`, `${VAR}`, `~` and `~/`, whatever their source: this helps where no shell runs (configuration file, systemd unit, single quotes). An unset variable is an error, not an empty string; `~user` is refused. A relative path is resolved from the working directory, not from the configuration file's directory.

## Configuration file

The same keys as the flags, plus `upload-token`. An unknown key, such as a typo or a nested key, is refused.

```yaml
# /etc/local-registry/config.yaml (chmod 600: it holds the token)
asset-path: /srv/registry
address: 0.0.0.0
port: 8443
tls-cert: /etc/local-registry/fullchain.pem
tls-key: /etc/local-registry/privkey.pem
public-url: https://registry.example.com:8443
max-version-mb: 4096
staging-ttl: 48h
audit-log: /var/log/local-registry/audit.log
upload-token: change-me
```

## Upload token

The token is read from `LOCAL_REGISTRY_UPLOAD_TOKEN`, else from the `upload-token` key. There is no `--upload-token` flag: it would show in the process list.

| Situation | Effect |
| --- | --- |
| No token | Upload routes answer `403`. Startup prints `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled` on stderr. |
| Token in a file other users can read | Startup warns `warning: <file> holds the upload token but is readable by other users (mode <mode>): chmod 600 <file>`. |
| Token set, server listening on a non-loopback address without TLS | Startup warns `warning: upload is enabled on <address> without TLS: the token travels in clear text (use --tls-cert/--tls-key or a TLS reverse proxy)` and starts anyway, for a TLS reverse proxy. |

The token is never printed nor logged. Logs and the audit log show its `token_id`: the first 12 hex digits of its sha256.

```sh
printf %s "$LOCAL_REGISTRY_UPLOAD_TOKEN" | shasum -a 256 | cut -c1-12
```

### Generated token

With `--disable-generate-token=false` and no configured token, `serve` draws a random token (32 bytes, 64 hex digits) and prints it on **stdout only**, before the banner. It is saved nowhere and changes at each start.

```
Upload token: 9f2c4e…
  generated for this run only and saved nowhere: it changes at each start (token_id 4ae3c36ce073)
```

With a configured token, nothing is generated, the configured token stays in use, and stderr shows:

```
WARN upload token generation skipped: a token is already configured (env LOCAL_REGISTRY_UPLOAD_TOKEN) and stays in use; unset LOCAL_REGISTRY_UPLOAD_TOKEN and upload-token to get a generated one, or drop --disable-generate-token=false
```

> [!WARNING]
> systemd (`journalctl`), Docker, Kubernetes and CI runners may keep stdout. Where they do, configure a fixed token instead.

## `config` output

```sh
LOCAL_REGISTRY_MAX_VERSION_FILES=50 local-registry config --config /etc/local-registry/config.yaml --staging-ttl 48h
```

```
SETTING                 VALUE                               SOURCE
config                  /etc/local-registry/config.yaml     flag
asset-path              /srv/registry                       config file
address                 localhost                           default
port                    8443                                config file
tls-cert                -                                   default
tls-key                 -                                   default
max-upload-mb           200                                 default
max-version-mb          2048                                default
max-version-files       50                                  env LOCAL_REGISTRY_MAX_VERSION_FILES
max-staging-mb          10240                               default
min-free-mb             1024                                default
staging-ttl             48h0m0s                             flag
audit-log               /srv/registry/bin.local/audit.log   default
public-url              https://registry.example.com        config file
disable-generate-token  true                                default
upload-token            set (token_id e2186dbdb1bb)         config file
```

`-` means empty. `upload-token` reads `not set: upload disabled`, `set (token_id …)`, `generated at startup, printed on stdout only`, or `set (token_id …); generation skipped`. The token itself is never printed.

## Configuration errors

Any error stops the program before it starts, with exit code `2`, `Error: <message>` and `Run '<command> --help' for usage.` on stderr.

| Message | Cause |
| --- | --- |
| `asset-path is required (--asset-path, LOCAL_REGISTRY_ASSET_PATH or asset-path in the config file)` | No asset path. |
| `asset-path <dir> is not an existing directory` | Missing directory, or a file. |
| `<setting>: environment variable <X> is not set` | A path uses an unset variable. |
| `<setting>: <path>: only ~ and ~/ are expanded, not ~user` | `~user` is not supported. |
| `port <n> is out of range 0-65535` | Invalid port. |
| `tls-cert and tls-key go together` | Only one of the two is set. |
| `<setting> cannot be negative` | A negative quota. |
| `staging-ttl must be 0 or at least 1h0m0s` | The purge could delete an upload still running: uploads may last 30 minutes. |
| `audit-log <file> would be downloadable: put it outside bin.local/assets and assets` | Audit log inside a public tree. |
| `public-url "<url>" must be an http or https URL with no user, query or fragment, such as https://registry.example.com` | Invalid `--public-url`. |
| `<setting>: "<value>" is not a whole number` | Unparsable number from the environment or the file. |
| `staging-ttl: "<value>" is not a duration such as 90m or 24h` | Unparsable duration. |
| `disable-generate-token: "<value>" is not true or false` | Unparsable boolean. |
| `config file <file>: unknown setting "<key>"` | Unknown key in the file. |
| `config file <file>: <reason>` | File missing, unreadable, or invalid YAML. |
| `unknown flag: --<x>`, `unexpected argument "<x>" for "local-registry serve"`, `unknown command "<x>" for "local-registry"` | Command-line error. |

## HTTPS

With `--tls-cert` and `--tls-key`, the server speaks HTTPS only: TLS 1.2 or later, HTTP/2. A plain HTTP request gets `400 Client sent an HTTP request to an HTTPS server.` The certificate is loaded at startup; an invalid pair exits with code `1` (`Error: loading TLS certificate: …`). Restart the server to change it. See [Security](../operations/security.md).

## Output

At startup, on stdout:

```
##################################################
##          Local Registry Server               ##
##################################################

Listen      : https://0.0.0.0:8443
Start At    : 2026-10-10 10:56:12.091998 +0200 CEST m=+0.003639167
Assets Path : /srv/registry
```

`Listen` shows the actual scheme and bound address. Then one line per request on stderr:

```
2026/10/10 10:56:15 INFO request method=PUT path=/sonarbridge/artifacts/v1.4.2/files/metadata.json status=201 bytes=112 duration=6.7ms remote=10.0.4.12
```

`remote` is the TCP peer. `forwarded_for` is added when the request carries `X-Forwarded-For`, as sent: it is not trusted. A failed request adds a `WARN` (4xx) or `ERROR` (5xx) line with the exact cause; the client only gets the message documented for the route.

## Exit codes

| Situation | Exit code |
| --- | --- |
| `SIGINT` or `SIGTERM`: stop accepting connections, give running requests 10 s, stop | `0` |
| Running requests still busy after 10 s at shutdown (`Error: context deadline exceeded`) | `1` |
| Port in use, bad address, invalid TLS pair, audit log that cannot be opened | `1` |
| Command or configuration error | `2` |
