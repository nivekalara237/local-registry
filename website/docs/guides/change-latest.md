---
title: Change the latest version
sidebar_label: Change the latest version
description: Choose which version consumers get as latest, at publish time or afterwards, and roll back a bad release.
---

Consumers that ask for `latest` get one version of the product. This guide shows how to choose it when you publish, how to change it afterwards, and how to roll back a bad release. How `latest` works is explained in [Latest](../concepts/latest.md).

## At publish time

Pass `--latest` to `upload.sh`, or `?latest=true` to the publish call:

```sh
ci/upload.sh --latest sonarbridge dist/artifacts.json
```

A version published without `--latest` is available by its number, but does not change `latest`. Use this for pre-releases and for fixes to older lines (for example a `v1.3.9` published after `v1.4.2`).

> [!NOTE]
> You cannot publish a version twice: once published, `publish` answers `409` for that version, with or without `latest`. To change `latest` afterwards, use the procedure below.

## Afterwards: on the server

There is no API route to change `latest`. An operator does it on the server, in the published tree, by copying two files.

The latest version is the one named by `<product>/release.json`. For goreleaser clients, `<product>/artifacts.json` is a copy of that version's `artifacts.json`. Change both together.

1. **Go to the product's directory.**

   ```sh
   cd <asset-path>/bin.local/assets/binaries/sonarbridge
   ls     # one directory per published version, plus release.json, index.json…
   ```

2. **Copy the chosen version's files.** Write through a temporary file and `mv`, so a reader never sees half a file.

   ```sh
   v=v1.4.2
   cp "$v/release.json" .release.json.tmp && mv .release.json.tmp release.json
   if [ -f "$v/artifacts.json" ]; then
     cp "$v/artifacts.json" .artifacts.json.tmp && mv .artifacts.json.tmp artifacts.json
   else
     rm -f artifacts.json   # the version was published without goreleaser
   fi
   ```

3. **Check.** The change is visible at once: the routes read these files at each request.

   ```sh
   curl -fsS https://registry.example.com/sonarbridge/releases/latest | jq -r .version
   curl -fsS https://registry.example.com/sonarbridge/index | jq -r .latest
   ```

The `index.json` file on disk is only rewritten at the next publish of the product. `GET /sonarbridge/index` does not read it: it always rebuilds the index from the disk.

## Roll back a bad release

Say `v1.5.0` was published as latest and is broken. You have two options.

| Option | When | How |
| --- | --- | --- |
| **Publish a fix** (recommended) | You can build `v1.5.1` quickly | Publish `v1.5.1` with `--latest`. The history stays clear, and every consumer moves forward. |
| **Point latest back** | You need the old version now | Follow the procedure above with `v=v1.4.2`. `v1.5.0` stays downloadable by its number. |

To warn users who already took `v1.5.0`, publish the next version with a `deprecated` message in its card. See [Write the product card](write-registry-yaml.md).

## Remove `latest`

Delete `<product>/release.json` and `<product>/artifacts.json`. `/releases/latest` then answers `404`, and `latest` is `null` in the index, until the next publish with `--latest`.

## Things to know

- Versions published before the `release.json` standard cannot be latest: they have no `release.json` to copy.
- A `<product>/release.json` naming a version that does not exist designates no version: `latest` answers `404`.
- Keep a record of manual changes: they do not go through the API, so the [audit log](../operations/audit-log.md) does not see them.
