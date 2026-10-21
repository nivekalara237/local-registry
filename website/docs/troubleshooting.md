---
title: Troubleshooting
description: Symptoms, causes and fixes for the problems met when running local-registry and publishing to it.
---

Find your symptom below: each row gives the cause and the fix. Most messages come straight from the server or the scripts. For the complete list of error messages, by route, see the [error reference](reference/errors.md).

> [!TIP]
> Two commands answer most questions. `local-registry config` shows every setting and where it comes from. The server's standard error shows the exact cause of every failed request, on a `WARN` or `ERROR` line.

## The server does not start

| Symptom | Cause | Fix |
| --- | --- | --- |
| `Error: asset-path is required (…)`, exit `2` | No asset path configured. The server never uses the working directory by default. | Set `--asset-path`, `LOCAL_REGISTRY_ASSET_PATH` or `asset-path`. |
| `Error: asset-path <dir> is not an existing directory` | The directory does not exist, or is a file. | Create it, owned by the service user. |
| `Error: config file <file>: unknown setting "…"` | A typo or a nested key in the configuration file. | Fix the key. The names are the flag names. |
| `Error: unknown flag: --port` | The flag was given before the command. | Put flags after `serve`: `local-registry serve --port 9000`. |
| `Error: unexpected argument "false"` | A boolean flag written with a space. | Write `--disable-generate-token=false`. |
| `bind: address already in use`, exit `1` | Another process listens on the port. | Find it with `lsof -i :8077`, or change `--port`. |
| `Error: loading TLS certificate: …`, exit `1` | A missing file, or a key that does not match the certificate. | Check both paths, and that the key belongs to the certificate. |
| `Error: … staging-ttl must be 0 or at least 1h0m0s` | The TTL is too short for a 30-minute upload. | Use `0` or at least `1h`. |
| A setting has an unexpected value | A forgotten `LOCAL_REGISTRY_*` variable wins over the file. In a container, the image sets the asset path, address and port. | Run `local-registry config`: the `SOURCE` column shows where each value comes from. |

## Authentication

| Symptom | Cause | Fix |
| --- | --- | --- |
| `403 upload is disabled: no token configured` | The server has no upload token. Startup printed `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled`. | Set `LOCAL_REGISTRY_UPLOAD_TOKEN` or `upload-token` for the server, then restart. Check with `local-registry config`. |
| `401 invalid or missing bearer token` | Wrong token, wrong header, or a token changed without a restart. | Send exactly `Authorization: Bearer <token>`, with no extra space or newline. Compare `token_id` values: see [Security](operations/security.md). |
| `WARN upload token generation skipped …` | `--disable-generate-token=false` while a token is configured. The configured one stays in use. | Expected. Drop the flag, or unset the configured token to get a generated one. |
| `curl: (60) SSL certificate problem` | Self-signed or private certificate authority. | `curl --cacert ca.pem`, or `export CURL_CA_BUNDLE=ca.pem` for the scripts. |
| `400 Client sent an HTTP request to an HTTPS server.` | Plain HTTP to a server running TLS. | Use `https://`. |

## Upload and publish

| Symptom | Cause | Fix |
| --- | --- | --- |
| `422 registry.yaml not uploaded: every version needs its product card` | No product card in staging. | Add a `registry.yaml` (see [Write registry.yaml](guides/write-registry-yaml.md)), or pass `upload.sh --product-file`. Then publish again: the rest of the staging is kept. |
| `422 no artifact to publish: upload at least one file besides registry.yaml` | Staging holds only documents: `registry.yaml`, `metadata.json`, `CHANGELOG.md`, checksums. | Upload the files you deliver themselves. |
| `422 artifacts.json references <name>, which is not uploaded` | `artifacts.json` was sent before a file it lists. | Send that file, then `artifacts.json` again. `upload.sh` sends files in the right order. |
| `422 <name> is uploaded but artifacts.json does not list it` | An extra file in staging: an earlier attempt, a raw build binary. | Remove the staging (`bin.local/.uploads/<product>/<version>`) and upload again, or use `upload.sh`. |
| `422 <name> is not listed in <checksums>` | A file the checksums file does not cover. | Do not send it, have your build checksum it, or send no checksums file: publish then writes one. |
| `422 <name>: sha256 is … but … says …` | The file changed after its checksum was computed, or the wrong file was sent. | Send the right file. |
| `422 metadata.json describes …, not …` | The version in the URL is not the tag in `metadata.json`. | Publish under the tag. The scripts read it from `metadata.json`. |
| `422 several checksums files: a and b` | Two files whose names end with `checksums.txt`. | Keep one. |
| `409 version … is already published and cannot be changed` | Published versions are immutable. | Publish a new version number. See [Products and versions](concepts/products-and-versions.md). |
| `400 version name release.json is reserved` (or `index.json`, `artifacts.json`, `artifacts.goreleaser.json`) | These names are files of the product directory. | Choose another version name. |
| `413 file exceeds … bytes` | One file is larger than `max-upload-mb`. | Raise `max-upload-mb`. See [Quotas and staging](operations/quotas-and-staging.md). |
| `413 … would exceed its staging quota` / `422 … staged files (limit …)` | The version is larger than `max-version-mb`, or has more files than `max-version-files`. | Raise the quota, or remove files from staging. |
| `507 upload staging area is full …` | Too many unpublished uploads, all versions together. | Publish or abandon pending versions, or raise `max-staging-mb`. |
| `507 not enough free disk space on the registry` | The upload would leave less than `min-free-mb` free. | Free disk space or grow the volume. |
| Upload cut through a reverse proxy (`413` or `504` from the proxy) | The proxy limits the body size or the duration. | Raise the proxy's body size limit and timeouts. See [Security](operations/security.md). |
| Downloaded file starts with `--------------------------` | The file was uploaded with `curl -F` (multipart). | Upload with `curl -T file`. |

## Scripts

| Symptom | Cause | Fix |
| --- | --- | --- |
| `upload.sh: <name>: <path> not found (wrong --root?)` | The `path` fields of `artifacts.json` are relative to goreleaser's working directory, not to where you run the script. | Pass `--root <goreleaser working directory>`, or run the script from there. |
| `upload.sh: … not found` for files of another project | No `artifacts.json` argument: the script took `dist/artifacts.json` of the current directory. | Pass the `artifacts.json` or the directory to publish. With `--root DIR` and no file, the script reads `DIR/dist/artifacts.json`, else publishes `DIR` itself. |
| `upload.sh: <dir> is a directory: archive it first …` | The directory to publish holds a subdirectory. The registry stores flat files. | Archive the subdirectory, for example `tar czf name.tar.gz -C dir name`. See [Publish plain files](guides/publish-plain-files.md). |
| `upload.sh: no registry.yaml in … : pass --product-file` | No product card found. | Pass `--product-file path/to/registry.yaml`. |
| On Windows, output lines start with `)` and uploads fail with `400 Bad Request` | A native `jq.exe` ends its lines with CRLF; the CR ends up in names and URLs. | Use the current `upload.sh`, which removes the CRs. See [Windows](guides/windows.md). |

## Reads and downloads

| Symptom | Cause | Fix |
| --- | --- | --- |
| `404` on `/<product>/releases/latest` | No version of this product was published with `latest=true`, or `<product>/release.json` names a version that is not published. | Publish with `--latest`, or see [Change the latest version](guides/change-latest.md). |
| `404` on `/<product>/artifacts` or `/<product>/artifacts/<version>` | The version was published without `artifacts.json` (plain files). | Read `/<product>/releases/<version>` instead: it exists for every version. |
| `404` on `/artifacts/metadatas/<product>` | No latest version, or a version published without `metadata.json`. | Read `/<product>/releases/<version>`. |
| `404` on `POST /releases/v1.2.3` | This deprecated route reads the `gitlab` product only. | Use `/releases/<product>/v1.2.3`. |
| `404` on `/assets/binary/…` | The path must be relative to the asset path and every `/` encoded as `%2F`. Staging, hidden files and links leaving the version are never served. | Use the URLs given in `release.json` instead. |
| Relative `path` or `url` values | `public-url` is not set. | Set it. It applies to versions published afterwards. See [Configuration](operations/configuration.md). |
| A version or a product is missing from `/products` or `/<product>/index` | The version has no `release.json`: it was published before the `local-registry/v1` standard. | Publish it again under a new version number. |
| Download cut after 20 seconds | The write timeout of read routes. | Resume with `curl -C -`. See [Monitoring](operations/monitoring.md). |
| Unexpected `301` | `..` or `//` in the URL. | Clean the URL. |
