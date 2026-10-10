<p align="center">
  <img alt="GoReleaser Logo" src="[./assets/logo-fl.svg](https://raw.githubusercontent.com/nivekalara237/local-registry/ba87dd439dabe95f0e04d9956a3d74f9d33daabb/assets/logo-fl.svg?v=3&s=200" height="200" />
</p>

---

**English** | [Français](README.fr.md)

A local HTTP registry for [goreleaser](https://goreleaser.com) releases. It serves the release catalogue, the binaries and their checksums, and accepts new releases in two steps: upload the files, then publish them atomically.

## Contents

- [Quick start](#quick-start)
- [Running the server](#running-the-server)
- [Storage layout](#storage-layout)
- [API conventions](#api-conventions)
- [API reference](#api-reference)
- [Publishing a release](#publishing-a-release)
- [Quotas and staging purge](#quotas-and-staging-purge)
- [Audit log](#audit-log)
- [Guarantees](#guarantees)
- [Operations](#operations)
- [Go library](#go-library)
- [Known limitations](#known-limitations)
- [Troubleshooting](#troubleshooting)

## Quick start

```sh
go build -o local-registry ./cmd

export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"
./local-registry serve --asset-path /srv/registry

# In the goreleaser project
goreleaser release --clean
LOCAL_REGISTRY_URL=http://localhost:8077 \
  scripts/upload.sh --latest github dist/artifacts.json

curl -s http://localhost:8077/github/artifacts                 # artifacts.json of the latest version
curl -s -X POST http://localhost:8077/releases/github/latest   # its files and download URLs
```

---

## Running the server

### Requirements

| Tool | Version | Used for |
| --- | --- | --- |
| Go | 1.25.1+ (`go.mod`) | building |
| bash, curl, jq | curl 7.76+ (`--fail-with-body`) | `scripts/upload.sh` |
| goreleaser | tested with 2.18.2 | producing releases |

### Build

```sh
go build -o local-registry ./cmd
go build -ldflags "-X main.version=v1.2.3" -o local-registry ./cmd   # with a version number
```

The binary has no runtime dependency.

### Commands

| Command | Effect |
| --- | --- |
| `serve` | Run the server. |
| `config` | Resolve and validate the configuration exactly like `serve`, print each setting and its source, start nothing. |
| `version` (or `--version`, `-v`) | Print `local-registry v1.2.3 (go1.25.1, darwin/arm64, commit 9c9d7664ceef)`. |
| `completion bash\|zsh\|fish\|powershell` | Print a shell completion script. |
| `help [command]` (or `-h`, `--help`) | Help. `serve --help` lists every flag with its environment variable and default. |

### Configuration

Each setting is taken from the first source that defines it:

1. command-line flag (`--port 9000`);
2. environment variable: `LOCAL_REGISTRY_` + flag name in upper case, `-` → `_` (`LOCAL_REGISTRY_PORT=9000`);
3. YAML file given by `--config` (`port: 9000`);
4. default.

| Flag | Default | Description |
| --- | --- | --- |
| `--config` | — | YAML configuration file. |
| `--asset-path` | **required** | Root of all data; must be an existing directory. |
| `--address` | `localhost` | Listen address (`0.0.0.0` for all interfaces). |
| `--port` | `8077` | Listen port, 0–65535 (`0`: any free port). |
| `--tls-cert`, `--tls-key` | — | PEM certificate chain and key. Both or neither. Enables [HTTPS](#https) only. |
| `--max-upload-mb` | `200` | Largest uploaded file, MiB. `≤ 0` means 200. |
| `--max-version-mb` | `2048` | Staged bytes per version, MiB. `0`: no limit. |
| `--max-version-files` | `100` | Staged files per version. `0`: no limit. |
| `--max-staging-mb` | `10240` | Staged bytes for all versions, MiB. `0`: no limit. |
| `--min-free-mb` | `1024` | Disk space an upload must leave free, MiB. `0`: no check. |
| `--staging-ttl` | `24h` | Purge a staged version idle for this long. Go duration, `0` (never) or ≥ `1h`. |
| `--audit-log` | `<asset-path>/bin.local/audit.log` | [Audit log](#audit-log) file. Refused inside `bin.local/assets/` or `assets/` (downloadable). |
| `--public-url` | — | Address clients reach the server at (`https://registry.example.com`, a path prefix is allowed). Makes the URLs written in `artifacts.json` and returned by `/releases` absolute. Empty: relative URLs. |
| `--disable-generate-token` | `true` | `=false`: generate an upload token at startup. See [Generated token](#generated-token). |
| *(no flag)* `LOCAL_REGISTRY_UPLOAD_TOKEN` | — | Upload token; config key `upload-token`. See [Upload token](#upload-token). |

Flags are written `--port 8077` or `--port=8077`; single-dash `-port` is rejected. A boolean must use `=`: `--disable-generate-token=false`.

**Paths.** `config`, `asset-path`, `tls-cert`, `tls-key` and `audit-log` expand `$VAR`, `${VAR}`, `~` and `~/`, whatever their source (useful where no shell runs: config file, systemd, single quotes). An unset variable is an error, not an empty string; `~user` is rejected. Relative paths are resolved from the working directory, not from the config file's directory.

**Config file.** Same keys as the flags, plus `upload-token`. Unknown keys (typos, nested keys) are rejected. No file is read without `--config`/`LOCAL_REGISTRY_CONFIG`.

```yaml
# /etc/local-registry/config.yaml — chmod 600 if it holds the token
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

#### Upload token

Read from `LOCAL_REGISTRY_UPLOAD_TOKEN`, else from the `upload-token` key. There is no `--upload-token` flag (it would show in `ps`).

- Empty: upload routes answer 403; startup prints `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled`.
- In a file readable by other users: startup warns `warning: <file> holds the upload token but is readable by other users (mode -rw-r--r--): chmod 600 <file>`.
- Never printed or logged. Logs and the audit log show its `token_id`: the first 12 hex digits of its sha256 (`printf %s "$TOKEN" | shasum -a 256 | cut -c1-12`).

#### Generated token

With `--disable-generate-token=false` and no configured token, `serve` generates a random token (32 bytes, 64 hex digits) and prints it on **stdout only**, before the banner. It changes at each start.

```
Upload token: 9f2c4e…
  generated for this run only and saved nowhere: it changes at each start (token_id 4ae3c36ce073)
```

With a configured token, nothing is generated, the configured token stays in use, and stderr shows:

```
WARN upload token generation skipped: a token is already configured (env LOCAL_REGISTRY_UPLOAD_TOKEN) and stays in use; unset LOCAL_REGISTRY_UPLOAD_TOKEN and upload-token to get a generated one, or drop --disable-generate-token=false
```

The token is not written to any file by the server, but stdout may be kept by systemd (`journalctl`), Docker, Kubernetes or CI logs. In those cases, use a fixed token.

#### Checking the configuration

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

`-` means empty. `upload-token` is one of `not set: upload disabled`, `set (token_id …)`, `generated at startup, printed on stdout only`, `set (token_id …); generation skipped`.

#### Configuration errors

Any error stops the program before it starts, with exit code `2` and `Error: <message>`:

| Message | Cause |
| --- | --- |
| `asset-path is required (…)` | No asset path. The working directory is never used by default. |
| `asset-path <dir> is not an existing directory` | Missing directory, or a file. |
| `<setting>: environment variable $X is not set` | A path uses an unset variable. |
| `<setting>: ~user/…: only ~ and ~/ are expanded, not ~user` | `~user` is not supported. |
| `port 70000 is out of range 0-65535` | Invalid port. |
| `tls-cert and tls-key go together` | Only one of them is set. |
| `max-version-mb cannot be negative` (and other quotas) | Negative value. |
| `staging-ttl must be 0 or at least 1h0m0s` | The purge could delete an upload still running (uploads may last 30 min). |
| `audit-log … would be downloadable: put it outside bin.local/assets and assets` | Audit log inside a public tree. |
| `public-url "…" must be an http or https URL with no user, query or fragment, …` | Invalid `--public-url`. |
| `port: "abc" is not a whole number`, `staging-ttl: "x" is not a duration such as 90m or 24h`, `disable-generate-token: "x" is not true or false` | Unparsable value from the environment or the file. |
| `config file <file>: unknown setting "max-verion-mb"` | Unknown key in the file. |
| `config file <file>: …` | File missing, unreadable or invalid YAML. |
| `unknown flag: --x`, `unexpected argument "x" for "local-registry serve"`, `unknown command "x"` | Command-line error. |

### HTTPS

With `--tls-cert` and `--tls-key`, the server speaks HTTPS only (TLS 1.2+, HTTP/2). A plain HTTP request gets `400 Client sent an HTTP request to an HTTPS server.` Certificates are loaded at startup; an invalid pair exits with code `1` (`Error: loading TLS certificate: …`). Restart to change them.

```sh
# Self-signed certificate for local use
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 365 \
  -keyout key.pem -out cert.pem -subj /CN=localhost -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
./local-registry serve --asset-path /srv/registry --tls-cert cert.pem --tls-key key.pem
curl --cacert cert.pem https://localhost:8077/github/artifacts     # scripts: export CURL_CA_BUNDLE=cert.pem
```

Without TLS, with upload enabled on a non-loopback address, startup warns `warning: upload is enabled on 0.0.0.0 without TLS: the token travels in clear text (…)` and starts anyway (for a TLS reverse proxy).

### systemd

```ini
# /etc/systemd/system/local-registry.service
[Unit]
Description=local-registry
After=network-online.target

[Service]
ExecStart=/usr/local/bin/local-registry serve --config /etc/local-registry/config.yaml
User=registry
Restart=on-failure
TimeoutStopSec=15

[Install]
WantedBy=multi-user.target
```

### Output

On startup (stdout):

```
##################################################
##          Local Registry Server               ##
##################################################

Listen      : http://127.0.0.1:8077
Start At    : 2026-10-10 10:56:12.091998 +0200 CEST m=+0.003639167
Assets Path : /srv/registry
```

`Listen` shows the actual scheme and bound address. Then one line per request on stderr:

```
2026/10/10 10:56:15 INFO request method=PUT path=/github/artifacts/v1.2.3/files/metadata.json status=201 bytes=112 duration=6.7ms remote=127.0.0.1
```

`remote` is the TCP peer; `forwarded_for` is added when `X-Forwarded-For` is present, as sent (not trusted). A failed request adds a `WARN` (4xx) or `ERROR` (5xx) line with the exact cause; clients only get the message listed in each route's errors.

### Exit codes and timeouts

| Situation | Exit code |
| --- | --- |
| `SIGINT`/`SIGTERM`: stop accepting connections, give running requests 10 s | `0` |
| Running requests exceed 10 s at shutdown (`Error: context deadline exceeded`) | `1` |
| Port in use, bad address, invalid TLS pair, audit log cannot be opened | `1` |
| Command or configuration error | `2` |

| Timeout | Value | Scope |
| --- | --- | --- |
| Reading request headers | 5 s | all requests |
| Writing the response | **20 s** | all routes except upload and publish (downloads included) |
| Read and write | 30 min | `PUT …/files/{name}`, `POST …/publish` |
| Idle keep-alive connection | 60 s | all connections |

---

## Storage layout

```
<asset-path>/
├── bin.local/
│   ├── assets/                              public, scanned by GET /artifacts/all
│   │   └── binaries/<vcs>/
│   │       ├── artifacts.json               copy of the latest version's artifacts.json
│   │       └── <version>/                   published version, immutable
│   │           ├── artifacts.json           rewritten: each path is a download URL
│   │           ├── artifacts.goreleaser.json   artifacts.json as uploaded
│   │           ├── metadata.json
│   │           ├── <project>_<version>_checksums.txt
│   │           └── archives, binaries…
│   ├── .uploads/<vcs>/<version>/            private: staging (uploaded, not published)
│   ├── .uploads-tmp/                        private: files being received
│   └── audit.log                            private, mode 0600
├── assets/binaries/<vcs>/<version>/         public: legacy tree, read by POST /releases
└── anything else                            private, never served
```

- `<vcs>` is a free name grouping the versions of one product (`github`, `gitlab`…).
- The server reads the disk at each request (no cache): files copied by hand are visible at once.
- The upload never writes into the legacy tree `assets/`.

---

## API conventions

**Names.** `{vcs}`, `{version}` and `{name}` must match `[A-Za-z0-9._-]+` and not be `.` or `..`. A refused name gives `404` on read routes and `400` on upload routes. `latest` is a valid read version (the latest one) but not an upload version.

**One segment per variable.** In `/assets/binary/{filePath}`, encode `/` as `%2F`. Names are decoded once: `%252F` stays `%2F`. A literal `..` or `//` in the URL gets a `301` to the cleaned path.

**Methods.** Every `GET` route also answers `HEAD`. Wrong method: `405` with `Allow`. Unknown path: `404 page not found` (text).

**Errors.** JSON routes answer `Content-Type: application/json` and, on error, `{"error":"<message>"}`. File routes and routing errors answer plain text.

| Status | Meaning |
| --- | --- |
| `400` | Invalid name or parameter |
| `401` | Missing or wrong token (with `WWW-Authenticate: Bearer`) |
| `403` | Upload disabled (no token configured) |
| `404` | Not found, or name refused on a read route |
| `409` | Version already published; ambiguous latest |
| `413` | File or version too large |
| `422` | Content refused (checksums, `artifacts.json`, `metadata.json`) |
| `500` | Internal error; the cause is only in the server log |
| `507` | Staging full or disk almost full |

---

## API reference

| Method | Route | Auth | Returns |
| --- | --- | --- | --- |
| `GET` | [`/{vcs}/artifacts`](#get-vcsartifacts) | — | `artifacts.json` of the latest version |
| `GET` | [`/{vcs}/artifacts/{version}`](#get-vcsartifactsversion) | — | `artifacts.json` of a version |
| `GET` | [`/artifacts/metadatas/{vcs}`](#get-artifactsmetadatasvcs) | — | `metadata.json` of a version (default: latest) |
| `POST` | [`/releases/{vcs}/{version}`](#post-releasesvcsversion) | — | Files of a version with their download URL |
| `POST` | [`/releases/{version}`](#post-releasesversion-deprecated) | — | Deprecated: `/releases/gitlab/{version}` |
| `GET` | [`/{vcs}/artifacts/{version}/files/{name}`](#get-vcsartifactsversionfilesname) | — | A file of a published version |
| `GET` | [`/assets/binary/{filePath}`](#get-assetsbinaryfilepath) | — | A public file by path |
| `GET` | [`/assets/checksum/{filename}`](#get-assetsbinaryfilepath) | — | Same as `/assets/binary` |
| `GET` | [`/artifacts/all`](#get-artifactsall) | — | Raw catalogue files of every directory |
| `PUT` | [`/{vcs}/artifacts/{version}/files/{name}`](#put-vcsartifactsversionfilesname) | token | Upload one file to staging |
| `POST` | [`/{vcs}/artifacts/{version}/publish`](#post-vcsartifactsversionpublish) | token | Check and publish a version |

### Latest version

`publish?latest=true` copies the version's `artifacts.json` to `<vcs>/artifacts.json`. Where a route needs the latest version **number**, it is the published version whose `artifacts.json` has the same bytes as that copy. None: `404`. Several (files copied by hand): `409 latest of <vcs> is ambiguous: v1, v2 have the same artifacts.json`. Versions published through the API cannot collide: their rewritten `artifacts.json` contains the version.

### `GET /{vcs}/artifacts`

`bin.local/assets/binaries/<vcs>/artifacts.json`: the `artifacts.json` of the latest version.

```sh
curl -s http://localhost:8077/github/artifacts
```

```json
[{"internal_type":36,"name":"metadata.json","path":"/github/artifacts/v1.2.3/files/metadata.json","type":"Metadata"},{"extra":{"Checksum":"sha256:2b51…","Format":"binary"},"goarch":"arm64","goos":"linux","name":"ci-bridge-vcs-github_Linux_arm64","path":"/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_Linux_arm64","type":"Binary"}]
```

The JSON is returned as stored (re-encoded, same content). For releases published through the API it is the [rewritten](#artifactsjson-rewrite) file.

| Status | Cause |
| --- | --- |
| `404` | Invalid `vcs`, or no version published with `latest=true`. |
| `500` | Stored file is not valid JSON. |

### `GET /{vcs}/artifacts/{version}`

`bin.local/assets/binaries/<vcs>/<version>/artifacts.json`. `latest` behaves like [`GET /{vcs}/artifacts`](#get-vcsartifacts).

```sh
curl -s http://localhost:8077/github/artifacts/v1.2.3
```

| Status | Cause |
| --- | --- |
| `404` | Invalid name, version not published (staging included), file missing. |
| `500` | Stored file is not valid JSON. |

### `GET /artifacts/metadatas/{vcs}`

`metadata.json` of a published version of `<vcs>`.

| Parameter | In | Description |
| --- | --- | --- |
| `vcs` | path | Version group. |
| `version` | query | Exact version. Absent, empty or `latest`: the [latest version](#latest-version). |

```sh
curl -s http://localhost:8077/artifacts/metadatas/github
curl -s "http://localhost:8077/artifacts/metadatas/github?version=v1.2.3"
```

```json
{"project_name":"sonarbridge-go","tag":"v1.2.3","previous_tag":"v1.2.2","version":"1.2.3","commit":"5c8e4a1…","date":"2026-10-10T10:56:12Z","runtime":{"goos":"darwin","goarch":"arm64"}}
```

| Status | Cause |
| --- | --- |
| `404` | Invalid name, version not published, no `metadata.json`, or no latest version. |
| `409` | Ambiguous [latest version](#latest-version). |
| `500` | Stored file is not valid JSON (its content is not returned). |

### `POST /releases/{vcs}/{version}`

Lists the files of a version, each with the URL that downloads it. Read-only despite `POST`; no request body.

| Parameter | In | Description |
| --- | --- | --- |
| `vcs` | path | Version group. |
| `version` | path | Exact version, or `latest` (see [Latest version](#latest-version)). |

Files are read from the first directory that exists:

| Order | Directory | `url` |
| --- | --- | --- |
| 1 | Published version `bin.local/assets/binaries/<vcs>/<version>/` | `/{vcs}/artifacts/{version}/files/{name}` |
| 2 | Legacy tree `assets/binaries/<vcs>/<version>/` | `/assets/binary/assets%2Fbinaries%2F<vcs>%2F<version>%2F<name>` |

```sh
curl -s -X POST http://localhost:8077/releases/github/latest
```

```json
{
  "tag_name": "v1.2.3",
  "assets": [
    {"id": "d7417cd3-…", "name": "ci-bridge-vcs-github_Linux_arm64", "url": "/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_Linux_arm64", "is_checksum": false},
    {"id": "8b1e0c52-…", "name": "ci-bridge-vcs-github_checksums.txt", "url": "/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_checksums.txt", "is_checksum": true}
  ]
}
```

| Field | Content |
| --- | --- |
| `tag_name` | The version listed (the real number for `latest`). |
| `assets` | One entry per regular file, sorted by name; directories, symbolic links and hidden files (`.x`) are skipped. `[]` when empty. |
| `id` | UUID v5 of `vcs/version/name`: stable across calls and restarts. |
| `url` | Download URL, name percent-encoded. Absolute under `--public-url` if set, else relative. |
| `is_checksum` | Name ends with `checksums.txt` or `checksum.txt`, any case. |

Download every file of a release:

```sh
BASE=http://localhost:8077
curl -s -X POST $BASE/releases/github/latest | jq -r '.assets[] | [.name, .url] | @tsv' |
  while IFS=$'\t' read -r name url; do
    case $url in http*) ;; *) url=$BASE$url ;; esac
    curl -fsS -o "$name" "$url"
  done
```

| Status | Cause |
| --- | --- |
| `404` | Invalid name, version in neither tree, or no latest version. |
| `405` | `GET` used. |
| `409` | Ambiguous latest version. |

### `POST /releases/{version}` (deprecated)

Same answer as `POST /releases/gitlab/{version}`, plus `Deprecation: true` and `Link: </releases/gitlab/{version}>; rel="successor-version"`. Will be removed.

### `GET /{vcs}/artifacts/{version}/files/{name}`

Downloads a file of a **published** version. This is the URL written in the rewritten `artifacts.json` and returned by `/releases`.

```sh
curl -fsSO http://localhost:8077/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_Linux_arm64
curl -fsS -C - -O http://localhost:8077/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_Windows_x86_64.zip   # resume
```

Headers: `Content-Type: application/octet-stream`, `Content-Disposition: attachment; filename=<name>`, `Content-Length`, `Last-Modified`, `Accept-Ranges: bytes`. `Range` and `If-Modified-Since` are honoured.

`404` (text): version not published (staged files are never served), `version` is `latest`, invalid or hidden name, missing file, directory, or symbolic link leaving the version directory.

### `GET /assets/binary/{filePath}`

Downloads a public file by its path relative to `--asset-path`, encoded as one segment. `GET /assets/checksum/{filename}` is identical.

| Served | Not served (`404`, text) |
| --- | --- |
| `bin.local/assets/…` | staging `bin.local/.uploads*/…` |
| `assets/…` (legacy tree) | any other path under `--asset-path` |
| | any path with a segment starting with `.` |
| | a symbolic link leading outside `--asset-path` (checked with `os.Root` at open time) |

```sh
curl -fsS -OJ "http://localhost:8077/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgithub%2Fv1.2.3%2Fci-bridge-vcs-github_checksums.txt"
```

Encode the path with your language's path escaping (`encodeURIComponent`, `url.PathEscape`, `urllib.parse.quote(p, safe="")`). Same headers as above. Also `404`: double-encoded or non-normalised path (`a%2F%2Fb`), directory.

### `GET /artifacts/all`

Scans `bin.local/assets/` and returns, for each directory holding at least one known file, the raw content of those files as strings.

| Field | Source |
| --- | --- |
| `provider` | Name of the directory (the vcs for the latest copy, **the version** for a version directory). |
| `artifacts_json` | `artifacts.json` |
| `metadata_json` | `metadata.json` |
| `checksums_text` | file ending in `_checksums.txt` (any case) |

```json
[{"provider":"github","artifacts_json":"[…]","metadata_json":"","checksums_text":""},
 {"provider":"v1.2.3","artifacts_json":"[…]","metadata_json":"{\"tag\":\"v1.2.3\",…}","checksums_text":"2b51…  ci-bridge-vcs-github_Linux_arm64\n…"}]
```

Sorted by directory path. Missing or unreadable file: `""`. No known file at all: `null`. Staging is not scanned. `404` if `bin.local/assets/` does not exist; `500` on a read error while scanning.

### Upload authentication

`PUT` and `publish` require `Authorization: Bearer <token>`, checked in constant time before anything else.

| Situation | Answer |
| --- | --- |
| Server has no token | `403 upload is disabled: no token configured` |
| Header missing, other scheme, wrong token | `401 invalid or missing bearer token` + `WWW-Authenticate: Bearer` |

Pass the token to curl on stdin so it does not appear in `ps`: `printf 'Authorization: Bearer %s\n' "$TOKEN" | curl -H @- …`.

### `PUT /{vcs}/artifacts/{version}/files/{name}`

Uploads one file into the staging area of `<vcs>/<version>`. Nothing is visible before [publish](#post-vcsartifactsversionpublish).

| Parameter | In | Description |
| --- | --- | --- |
| `vcs`, `version` | path | `version` cannot be `latest`. |
| `name` | path | Published file name. `artifacts.goreleaser.json` is reserved. |
| `X-Checksum-Sha256` | header, optional | Expected sha256 (hex, any case). Mismatch: `422`, nothing kept. |
| body | — | Raw file content (`curl -T file`). Not multipart: `curl -F` would store the envelope. |

```sh
printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" |
  curl -fsS -H @- -H "X-Checksum-Sha256: $(shasum -a 256 tool.tar.gz | cut -d' ' -f1)" \
    -T tool.tar.gz http://localhost:8077/github/artifacts/v1.2.3/files/tool_1.2.3_linux_amd64.tar.gz
```

`201 Created`: `{"name":"tool_1.2.3_linux_amd64.tar.gz","size":1843202,"sha256":"8f6c…"}`.

- The body is streamed to `bin.local/.uploads-tmp/`, then moved into staging once complete and checked. An interrupted or refused upload leaves nothing.
- Uploading the same name again replaces the staged file.
- **`artifacts.json` must be uploaded last.** On receipt it is [checked](#artifactsjson-checks): every file it publishes must already be staged, with the sha256 of its `extra.Checksum`. Uploading it again re-checks it.

Errors, in check order:

| Status | Message |
| --- | --- |
| `403`, `401` | see [Upload authentication](#upload-authentication) |
| `400` | `invalid vcs or version name`, `invalid file name: use [A-Za-z0-9._-] only`, `artifacts.goreleaser.json is reserved: publishing writes it` |
| `409` | `version github/v1.2.3 is already published and cannot be changed` |
| `422` | `version github/v1.2.3 already has 100 staged files (limit 100)` (replacing a staged file is still allowed) |
| `413` | `file exceeds 209715200 bytes`, `version github/v1.2.3 would exceed its staging quota of 2147483648 bytes` |
| `507` | `upload staging area is full (limit … bytes): publish or abandon pending versions`, `not enough free disk space on the registry` |
| `422` | `sha256 mismatch: got <actual>, X-Checksum-Sha256 says <expected>` |
| `422` | `artifacts.json …`: see [artifacts.json checks](#artifactsjson-checks) |
| `500` | `internal error` (disk error, client disconnected) |

### `POST /{vcs}/artifacts/{version}/publish`

Checks the staged files as a complete goreleaser release, then makes them visible at once. No body.

| Parameter | In | Description |
| --- | --- | --- |
| `vcs`, `version` | path | `version` cannot be `latest`. |
| `latest` | query | `true`: also make it the latest version. `true/false`, `1/0`, `t/f` (any case). Default `false`. |

```sh
printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" |
  curl -fsS -H @- -X POST "http://localhost:8077/github/artifacts/v1.2.3/publish?latest=true"
```

`200 OK`: `{"vcs":"github","version":"v1.2.3","files":["artifacts.goreleaser.json","artifacts.json",…],"latest":true}` (`files` sorted).

**Checks.** All must pass, otherwise nothing is published and staging is kept:

1. Exactly one checksums file (name ends with `checksums.txt`, any case).
2. `artifacts.json` and `metadata.json` are staged.
3. Each line of the checksums file is `<64-hex sha256>  <name>` (`*name` accepted).
4. Each staged file is listed in the checksums file with a matching sha256, except the checksums file, `artifacts.json`, `metadata.json`, `*.sig`, `*.pem`. Lines naming files not uploaded are allowed.
5. `artifacts.json` passes its [checks](#artifactsjson-checks) again (catches files removed or replaced after it).
6. Each staged file is a published entry of `artifacts.json`, except `artifacts.json` and `metadata.json`.
7. `metadata.json` is a JSON object with `tag` or `version`, and `{version}` equals `tag`, `version`, or `v` + `version`.

**Then**, in order:

1. In staging: the uploaded `artifacts.json` is copied to `artifacts.goreleaser.json` and replaced by its [rewrite](#artifactsjson-rewrite).
2. The staging directory is renamed to `bin.local/assets/binaries/<vcs>/<version>/`: the version appears complete in one operation.
3. With `latest=true`, the rewritten `artifacts.json` is copied atomically (temp file + rename) to `<vcs>/artifacts.json`.

| Status | Message |
| --- | --- |
| `400` | `invalid vcs or version name`, `latest must be true or false` |
| `409` | `version … is already published and cannot be changed` |
| `404` | `nothing uploaded for github/v1.2.3` |
| `422` | check 1: `no checksums file (*checksums.txt) uploaded`, `several checksums files: a and b` |
| `422` | check 2: `artifacts.json not uploaded`, `metadata.json not uploaded` |
| `422` | checks 3–4: `<file>: line 3 is not "<sha256>  <name>"`, `<name> is not listed in <checksums>`, `<name>: sha256 is <actual> but <checksums> says <expected>` |
| `422` | check 5: see [artifacts.json checks](#artifactsjson-checks) |
| `422` | check 6: `<name> is uploaded but artifacts.json does not list it` |
| `422` | check 7: `metadata.json is not valid JSON`, `metadata.json has neither tag nor version`, `metadata.json describes v0.2.0 (version 0.2.0), not v1.2.3` |
| `500` | `internal error`. If it happens while updating latest, the version **is** published but latest is unchanged (logged as `version published but latest not updated`). |

#### artifacts.json checks

An entry is a **published file** when its `type` is `Archive`, `File`, `Source`, `Linux Package`, `Checksum`, `Signature`, `Certificate`, `SBOM` or `Metadata`, or `Binary` **with** `extra.Format` (goreleaser `formats: [binary]`). Other entries are ignored, notably `Binary` without `extra.Format`: the raw build output an archive is made from, never in the checksums file, often several with the same name.

| Rule | `422` message |
| --- | --- |
| JSON array of objects, at most 32 MiB | `artifacts.json is not a JSON array of objects`, `artifacts.json: entry 3 is null`, `artifacts.json is larger than 32 MiB` |
| `name`, `path`, `type` are strings | `artifacts.json: name, path and type must be strings: …` |
| Each published file has a unique, [valid](#api-conventions) name, other than `artifacts.json` and `artifacts.goreleaser.json` | `artifacts.json: a Archive entry has no name`, `artifacts.json lists <name> twice`, `artifacts.json lists "<name>", a name that cannot be uploaded` |
| Each published file is staged | `artifacts.json references <name>, which is not uploaded: upload every file first, artifacts.json last` |
| sha256 matches `extra.Checksum` when it starts with `sha256:` | `<name>: sha256 is <actual> but artifacts.json says <expected>` |

#### artifacts.json rewrite

The published `artifacts.json` keeps only the published files, in their original order. In each entry only `path` changes: it becomes the download URL `/{vcs}/artifacts/{version}/files/{name}`, prefixed by `--public-url` when set. Other fields are kept as uploaded; keys are re-ordered alphabetically, indentation is 2 spaces.

Uploaded entry:

```json
{"name": "ci-bridge-vcs-github_Linux_arm64", "path": "bin.local/assets/binaries/github/github_linux_arm64_v8.0/ci-bridge-vcs-github",
 "goos": "linux", "goarch": "arm64", "internal_type": 2, "type": "Binary",
 "extra": {"Binary": "ci-bridge-vcs-github", "Checksum": "sha256:2b51…", "Format": "binary", "ID": "vcs-github-plugin-archive"}}
```

Published entry, with `--public-url https://registry.example.com`:

```json
{"extra": {"Binary": "ci-bridge-vcs-github", "Checksum": "sha256:2b51…", "Format": "binary", "ID": "vcs-github-plugin-archive"},
 "goarch": "arm64", "goos": "linux", "internal_type": 2, "name": "ci-bridge-vcs-github_Linux_arm64",
 "path": "https://registry.example.com/github/artifacts/v1.2.3/files/ci-bridge-vcs-github_Linux_arm64", "type": "Binary"}
```

A relative `path` resolves against the address `artifacts.json` was read from. Set `--public-url` behind a reverse proxy that changes the host or adds a prefix: the server never trusts the `Host` header. Versions published before this rewrite existed are not changed.

---

## Publishing a release

goreleaser runs its `publishers:` before writing `artifacts.json`, so publishing must run **after** `goreleaser release`, e.g. in the same CI step.

### `scripts/upload.sh` (recommended)

Uploads every published file of an `artifacts.json` (under its `name`, read from its `path`), `artifacts.json` last, then publishes. Works whatever the layout of the goreleaser output. Needs bash (3.2+), curl, jq.

```
upload.sh [--latest] [--dry-run] [--root DIR] [--version V] <vcs> [artifacts.json]
```

| Argument | Description |
| --- | --- |
| `--latest` | Publish with `latest=true`. |
| `--dry-run` | Print what would be sent; send nothing; no URL or token needed. Files are still checked. |
| `--root DIR` | Directory the `path` fields are relative to (goreleaser's working directory). Default: deduced from where `artifacts.json` sits (it strips the directory of the `Metadata` or `Checksum` entry's `path`), else the working directory. |
| `--version V` | Version to publish. Default: `tag`, else `version`, of the listed `metadata.json`. |
| `<vcs>` | Version group. |
| `[artifacts.json]` | Default `dist/artifacts.json`. |

Environment: `LOCAL_REGISTRY_URL` (trailing `/` removed) and `LOCAL_REGISTRY_UPLOAD_TOKEN` (sent on curl's stdin).

It selects published files with the [server's rule](#artifactsjson-checks), and before sending anything checks that names are unique, every file exists, and a checksums file is listed. Each file is sent with `X-Checksum-Sha256` when its entry has a `sha256:` checksum.

```
$ scripts/upload.sh --latest github bin.local/assets/binaries/github/artifacts.json
== github v1.2.3 from bin.local/assets/binaries/github/artifacts.json (root /home/me/sonarbridge-go)
PUT metadata.json <- /home/me/sonarbridge-go/bin.local/assets/binaries/github/metadata.json
PUT ci-bridge-vcs-github_Linux_arm64 <- /home/me/sonarbridge-go/bin.local/assets/binaries/github/github_linux_arm64_v8.0/ci-bridge-vcs-github (sha256 2b51…)
…
PUT artifacts.json <- bin.local/assets/binaries/github/artifacts.json
POST publish github/v1.2.3 (latest=true)
{"vcs":"github","version":"v1.2.3","files":[…],"latest":true}
```

Exit codes: `0` published (or dry run OK); `1` local check failed, variable missing, or server refusal (its message is printed); `2` usage error. After a failure, fix the cause and run it again: staged files are replaced.

### `scripts/publish-to-local-registry.sh`

```
publish-to-local-registry.sh [--latest] <vcs> [dist-dir]
```

Reads the version from `metadata.json`'s `tag`, uploads each file listed in the checksums file from `dist-dir` (with `X-Checksum-Sha256`), then the checksums file, `*.sig`/`*.pem`, `metadata.json`, `artifacts.json`, and publishes. Only works when every file of the checksums file sits at the root of `dist-dir` under its published name (archives); otherwise use `upload.sh`. Same environment variables. `--latest` must come first.

### By hand

```sh
URL=http://localhost:8077 V=v1.2.3
auth() { printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN"; }
while read -r sum name; do
  auth | curl -fsS -H @- -H "X-Checksum-Sha256: $sum" -T "dist/$name" "$URL/github/artifacts/$V/files/$name"
done < dist/tool_checksums.txt
for f in tool_checksums.txt metadata.json artifacts.json; do      # artifacts.json last
  auth | curl -fsS -H @- -T "dist/$f" "$URL/github/artifacts/$V/files/$f"
done
auth | curl -fsS -H @- -X POST "$URL/github/artifacts/$V/publish?latest=true"
```

---

## Quotas and staging purge

| Limit | Flag | Default | Beyond |
| --- | --- | --- | --- |
| One file | `--max-upload-mb` | 200 MiB | `413` |
| Staged bytes of one version | `--max-version-mb` | 2 GiB | `413` |
| Staged files of one version | `--max-version-files` | 100 | `422` |
| Staged bytes of all versions | `--max-staging-mb` | 10 GiB | `507` |
| Free disk space left | `--min-free-mb` | 1 GiB | `507` |

- Only staging counts; published versions do not.
- Replacing a file counts only the size difference, not an extra file.
- Without `Content-Length` (chunked), the disk check assumes the largest size the quotas allow.
- Quotas are checked before receiving and again under the upload lock before entering staging.
- The free-space check works on Unix systems (Linux, macOS…) and is skipped elsewhere.

**Purge.** With `--staging-ttl` > 0, at startup and then every `min(ttl/2, 1h)`, the server deletes each staged version whose newest file is older than the TTL, and files of `bin.local/.uploads-tmp/` older than the TTL. Published versions are never touched. Each deletion is audited (`staging_purged`, `tmp_purged`). The purge takes the upload lock.

---

## Audit log

One JSON object per line, appended and synced to disk before the client gets the answer. File mode `0600`; never served; rotation is external (`logrotate` with `copytruncate`). If writing fails, the request still succeeds and stderr shows `ERROR audit log write failed`. The token never appears, only its `token_id`.

| Field | Content |
| --- | --- |
| `time` | UTC, RFC 3339 with nanoseconds. |
| `event` | See below. |
| `remote`, `forwarded_for` | TCP peer; `X-Forwarded-For` as sent. Absent for purge events. |

| `event` | When | Fields |
| --- | --- | --- |
| `auth_failed` | 401/403 on an upload route | `method`, `path`, `status`, `error`, `authorization_sent` |
| `upload` | `PUT` accepted | `vcs`, `version`, `name`, `size`, `sha256`, `replaced`, `token_id` |
| `upload_refused` | `PUT` refused after authentication | `vcs`, `version`, `name`, `status`, `error`, `token_id` |
| `publish` | publish succeeded | `vcs`, `version`, `latest`, `files` (`{name, size, sha256}` of every published file: rewritten `artifacts.json`, uploaded `artifacts.goreleaser.json`), `token_id` |
| `publish_refused` | publish refused after authentication | `vcs`, `version`, `status`, `error`, `token_id` |
| `staging_purged` | staged version purged | `vcs`, `version`, `files`, `bytes`, `idle` |
| `tmp_purged` | temporary files purged | `files` |

For a `500`, `error` holds the real cause; the client only got `internal error`.

```sh
LOG=/srv/registry/bin.local/audit.log
jq -r 'select(.event=="publish") | "\(.time) \(.vcs)/\(.version) latest=\(.latest) from \(.remote)"' $LOG
jq -r 'select(.event=="auth_failed") | .remote' $LOG | sort | uniq -c | sort -rn
```

---

## Guarantees

| Guarantee | Mechanism |
| --- | --- |
| A file being received is never visible | Received in `.uploads-tmp/`, renamed into staging once complete and checked. |
| A version appears complete or not at all | Publish renames the staging directory in one operation. |
| Published files match the checksums | Every file is checked before publish. |
| `artifacts.json` paths point to the version's files | Rewritten in staging before the rename, after checking each file it lists. |
| Latest changes in one step | New `artifacts.json` written next to the old one, then renamed over it. |
| A published version never changes | `PUT` and publish answer `409`. |
| Parallel uploads cannot exceed a quota together | Quotas re-checked under the upload lock. |

Renames are atomic because `.uploads-tmp`, `.uploads` and `assets` are on the same file system under `bin.local/`: do not mount them separately. The lock is per process: run **one** uploading server per `--asset-path`.

---

## Operations

| Task | How |
| --- | --- |
| Fix a refused publish | Staging is kept: upload the missing or corrected file under the same name (then `artifacts.json` again if needed), call publish again, or rerun the script. |
| Abandon an upload | Wait for the purge, or `rm -rf <asset-path>/bin.local/.uploads/<vcs>/<version>`. |
| Change latest to an older version | `cp <vcs>/<old>/artifacts.json <vcs>/artifacts.json` under `bin.local/assets/binaries/`. |
| Remove a published version | No API. `rm -rf bin.local/assets/binaries/<vcs>/<version>` (and fix `<vcs>/artifacts.json` if it was latest). Prefer publishing a new version. |
| Change the token | Stop, change `LOCAL_REGISTRY_UPLOAD_TOKEN`, restart. |
| After a crash | `upload-*` files in `bin.local/.uploads-tmp/` (purged after the TTL) and `.tmp-*` files in `bin.local/assets/binaries/<vcs>/` are ignored and can be deleted when no upload runs. |

---

## Go library

```go
import localregistry "sonarbridge-go/local-registry" // package local_registry

s := localregistry.NewLocalRegistryServer(8077, "localhost", "/srv/registry")
if err := s.SetTLS("fullchain.pem", "privkey.pem"); err != nil { // optional
	log.Fatal(err)
}
s.AddUploadRoutes(localregistry.UploadConfig{
	Token:           os.Getenv("LOCAL_REGISTRY_UPLOAD_TOKEN"), // empty: upload disabled (403)
	MaxFileSize:     500 << 20,      // bytes; ≤ 0: DefaultMaxFileSize
	MaxVersionSize:  4 << 30,        // 0: default; < 0: no limit (same for the fields below)
	MaxVersionFiles: 0,
	MaxStagingSize:  -1,
	MinFreeSpace:    0,
	StagingTTL:      48 * time.Hour, // < 0: no purge
	PublicURL:       "https://registry.example.com",
})
s.AddReleaseRoutes("https://registry.example.com")
s.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")
audit, err := localregistry.OpenAuditLog("/var/log/local-registry/audit.log")
if err != nil {
	log.Fatal(err)
}
defer audit.Close()
s.SetAuditLog(audit)
if err := s.ServeContext(ctx); err != nil { // nil after a clean stop
	log.Fatal(err)
}
```

| Function | Effect |
| --- | --- |
| `NewLocalRegistryServer(port, addr, assetPath)` | Server with no route. |
| `AddRoute(uri, method, f)` / `AddRouteE(uri, method, f)` | JSON route; `f` gets the path variables in order. `AddRouteE` errors: `*HTTPError` → its status and message; `ErrNotFound` or `fs.ErrNotExist` → `404`; other → `500 internal error`. |
| `AddFileRoute(uri, method, varName)` | Serves the public file named by `varName`, as `/assets/binary`. |
| `AddUploadRoutes(cfg)` | `PUT …/files/{name}`, `POST …/publish`, `GET …/files/{name}`. |
| `AddReleaseRoutes(publicURL)` | `POST /releases/{vcs}/{version}`, `POST /releases/{version}`, `GET /artifacts/metadatas/{vcs}`. |
| `SetTLS(cert, key) error`, `TLSEnabled() bool` | HTTPS only, TLS 1.2+. |
| `OpenAuditLog(path)`, `SetAuditLog(a)` | Audit log; without it, events go to `slog` (message `audit`). |
| `ServeContext(ctx) error` | Serve until `ctx` is done, then shut down (10 s); runs the purge. |
| `Serve()` | `ServeContext` with SIGINT/SIGTERM; exits the process with code 1 on error. |
| `Handler() http.Handler` | Routes without logging or timeouts, for `httptest`. |
| `NewLayout(root)` | Paths: `AssetsDir`, `BinariesDir`, `VCSDir`, `VersionDir`, `ArtifactsFile`, `StagingDir`, `UploadsDir`, `UploadTmpDir`; reads: `LatestVersion(vcs)`, `ListRelease(vcs, version, publicURL)`, `ReadMetadata(vcs, version)`. |
| `ValidName(s)`, `IsPublicFile(name)`, `TokenID(token)` | Name rule, file-route rule, audit token id. |
| `Asset.FindAssetPaths(dir)`, `Asset.FindAssetContents(dir)` | Catalogue scan (`/artifacts/all`). |

Constants: `DefaultMaxFileSize` (200 MiB), `DefaultMaxVersionSize` (2 GiB), `DefaultMaxVersionFiles` (100), `DefaultMaxStagingSize` (10 GiB), `DefaultMinFreeSpace` (1 GiB), `DefaultStagingTTL` (24 h).

---

## Known limitations

- Everything under `bin.local/assets/` and `assets/` is public; reads need no authentication.
- Without TLS everything, the token included, travels in clear text.
- Responses of read routes, downloads included, must finish within **20 s**: a large file over a slow link is cut (resume with `Range`, `curl -C -`).
- Symbolic links are checked on file routes only; catalogue routes and upload follow links found on disk.
- No delete route, for staging or published versions.
- `/artifacts/all`: `provider` is the directory name (the version for a version directory); unreadable files give `""`; an empty catalogue gives `null`; only `_checksums.txt` names are recognised.
- `POST /releases/{version}` is deprecated (gitlab only) and will be removed.
- A `<vcs>/artifacts.json` edited by hand no longer designates any version: `latest` lookups answer `404`.
- The audit log is not signed; a failed audit write does not fail the request; `X-Forwarded-For` is not verified.
- One uploading server per `--asset-path` (in-process lock).

---

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled`, then `403` | Set the variable or `upload-token` in the server's environment; check with `local-registry config`. |
| `401` with the right token | Exact `Bearer ` prefix; no extra space or newline; server restarted after a token change. |
| `WARN upload token generation skipped …` | A token is configured and stays in use. Unset it or drop `--disable-generate-token=false`. |
| `unexpected argument "false"` | Write `--disable-generate-token=false`. |
| `unknown flag: --port` | Flags belong to a command: `local-registry serve --port …`. |
| A setting has an unexpected value | `local-registry config` shows each value and its source; a forgotten `LOCAL_REGISTRY_*` variable wins over the file. |
| `bind: address already in use`, exit 1 | Another process uses the port: `lsof -i :8077`. |
| `Error: loading TLS certificate`, exit 1 | Missing file, or key not matching the certificate. |
| `curl: (60) SSL certificate problem` | Self-signed: `curl --cacert cert.pem`, or `CURL_CA_BUNDLE` for the scripts. |
| `Client sent an HTTP request to an HTTPS server.` | Use `https://`. |
| `422 artifacts.json references …, which is not uploaded` | `artifacts.json` sent before a file it lists: send that file, then `artifacts.json` again. `upload.sh` does it in order. |
| `422 … is uploaded but artifacts.json does not list it` | Extra file in staging (earlier attempt, raw build binary): abandon the staging and resend, or use `upload.sh`. |
| `422 … is not listed in …checksums.txt` | Uploaded file not in the checksums file: do not send it, or have goreleaser checksum it. |
| `422 … sha256 is … but … says …` | Wrong or changed file: resend the right one. |
| `422 metadata.json describes …, not …` | URL version differs from the goreleaser tag; the scripts read it from `metadata.json`. |
| `409 … already published` | Publish a new version, or see [Operations](#operations). |
| `413` / `422 … staged files` / `507` | Raise the matching [quota](#quotas-and-staging-purge), or clear old staging. |
| `404` on `/releases/<vcs>/latest` or `/artifacts/metadatas/<vcs>` | No version published with `latest=true`, or `<vcs>/artifacts.json` edited by hand. |
| `409 latest of … is ambiguous` | Two versions have the same `artifacts.json` (hand copies): fix one, or ask for an exact version. |
| `404` on `POST /releases/v1.2.3` | Deprecated route reads `gitlab` only: use `/releases/<vcs>/v1.2.3`. |
| `404` on `/assets/binary/…` | Path is relative to `--asset-path`, starts with `bin.local%2Fassets%2F` or `assets%2F`, every `/` as `%2F`; staging, hidden files and outside links are never served. |
| Relative `path` in `artifacts.json` | Default. Prefix with the registry address, or set `--public-url` (affects versions published afterwards). |
| Downloaded file starts with `--------------------------` | Uploaded with `curl -F`: resend with `curl -T`. |
| Download cut after 20 s | See [Known limitations](#known-limitations): resume with `curl -C -`. |
| Unexpected `301` | `..` or `//` in clear in the URL. |
