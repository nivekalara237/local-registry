---
title: Consume releases
sidebar_label: Consume releases
description: Find products and versions, download the right file and verify it, from a shell script or any HTTP client.
---

This guide is for integrators: installers, deployment scripts and portals that read the registry. Every recipe uses `curl` and `jq` against the read routes, which need no token.

```sh
R=https://registry.corp.example
```

## The URLs to rely on

| What | URL | Changes over time |
| --- | --- | --- |
| Every product, with its latest version | `GET $R/products` | yes |
| The products that match criteria, page by page | `GET $R/products/search?…` | yes |
| A product, its card and its versions | `GET $R/{product}/index` | yes |
| One version: files, platforms, sha256, notes | `GET $R/{product}/releases/{version}` | never: a version is immutable |
| The latest version | `GET $R/{product}/releases/latest` | when a new latest is published |
| One file | `GET $R/{product}/artifacts/{version}/files/{name}` | never |

`release.json`, served by `/releases/…`, is the reference description of a version. Its format is in the [release.json reference](../reference/release-json.md).

> [!NOTE]
> Download URLs take an exact version. `…/artifacts/latest/files/…` answers `404`: resolve `latest` to a version number first, as shown below.

## List products

```sh
curl -fsS "$R/products" | jq -r '.[] | [.product, (.latest // "-"), .displayName] | @tsv'
```

```
billing-db-migrations  v1.0.0  Billing database migrations
bridge            v1.4.2  Bridge
```

## Search products

`/products/search` filters the catalogue on the server and returns it one page at a time. Criteria combine: every one must match.

```sh
curl -fsS -G "$R/products/search" --data-urlencode "keyword=db" --data-urlencode "label=team=billing" \
  | jq -r '.items[] | [.product, (.latest // "-")] | @tsv'
```

To read every result, follow `next` until it is `null`. `-G` with `--data-urlencode` encodes each value, the cursor included:

```sh
cursor=
while :; do
  page=$(curl -fsS -G "$R/products/search" --data-urlencode "keyword=db" \
    --data-urlencode "limit=100" --data-urlencode "cursor=$cursor")
  jq -r '.items[] | [.product, (.latest // "-")] | @tsv' <<<"$page"
  cursor=$(jq -r '.next // empty' <<<"$page")
  [ -n "$cursor" ] || break
done
```

Keep the same criteria from page to page: a cursor sent with other criteria answers `400`. Every parameter is in [Search products](../reference/api/products.md#search-products).

## Find the latest version

```sh
version=$(curl -fsS "$R/bridge/releases/latest" | jq -r .version)
```

`404` means that no version of the product was published as latest. To choose among all versions, read the index instead. It lists versions newest first, and marks pre-releases:

```sh
# The newest stable version
curl -fsS "$R/bridge/index" | jq -r '[.versions[] | select(.prerelease | not)][0].version'
```

## Download the file for a platform

```sh
rel=$(curl -fsS "$R/bridge/releases/latest")
url=$(jq -r '.artifacts[] | select(.kind == "archive" and .os == "linux" and .arch == "amd64") | .url' <<<"$rel")
sha=$(jq -r '.artifacts[] | select(.kind == "archive" and .os == "linux" and .arch == "amd64") | .sha256' <<<"$rel")

# The URL is relative when the server runs without --public-url
case $url in http*) ;; *) url=$R$url ;; esac

curl -fsSLo bridge.tar.gz "$url"
echo "$sha  bridge.tar.gz" | sha256sum -c -   # macOS: shasum -a 256 -c -
```

`os` and `arch` are `null` for files published without GoReleaser platform information. Select those by `name` or `kind`.

## Verify against the checksums file

Every version has a checksums file, uploaded or written by the server. Its name is in `release.json`:

```sh
v=v1.4.2
rel=$(curl -fsS "$R/bridge/releases/$v")
sums=$(jq -r .checksums.name <<<"$rel")

curl -fsSO "$R/bridge/artifacts/$v/files/$sums"
curl -fsSO "$R/bridge/artifacts/$v/files/bridge_1.4.2_linux_amd64.tar.gz"
sha256sum -c --ignore-missing "$sums"
```

When the publisher signed the checksums file or the archives, `release.json` gives the URL of each signature and certificate (`checksums.signature`, `artifacts[].signature`). See [Integrity](../concepts/integrity.md).

## Watch for security updates and deprecation

```sh
# Versions that carry a security fix
curl -fsS "$R/bridge/index" | jq -r '.versions[] | select(.securityUpdates) | .version'

# Is the version you run deprecated? false, true or a message
curl -fsS "$R/bridge/releases/v1.4.2" | jq .deprecated
```
