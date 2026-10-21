---
title: Quotas and staging
description: Size the staging area, protect the disk, and let the server purge abandoned uploads.
---

Uploaded files wait in a staging area until the version is published. Quotas cap how much this area can hold, and a purge removes uploads nobody finished. Tune them when your releases are large, or when many teams publish at once.

For how staging and publish work, see [Staging and publish](../concepts/staging-and-publish.md).

## Quotas

| Limit | Setting | Default | Beyond the limit |
| --- | --- | --- | --- |
| One file | `max-upload-mb` | 200 MiB | `413 file exceeds <n> bytes` |
| Staged bytes of one version | `max-version-mb` | 2048 MiB (2 GiB) | `413 version <p>/<v> would exceed its staging quota of <n> bytes` |
| Staged files of one version | `max-version-files` | 100 | `422 version <p>/<v> already has <n> staged files (limit <n>)` |
| Staged bytes of all versions | `max-staging-mb` | 10240 MiB (10 GiB) | `507 upload staging area is full (limit <n> bytes): publish or abandon pending versions` |
| Free disk space left after the upload | `min-free-mb` | 1024 MiB (1 GiB) | `507 not enough free disk space on the registry` |

`0` disables a limit, except `max-upload-mb`, where `0` or less means the default of 200 MiB. Negative values are refused at startup.

How the quotas count:

- **Only staging counts.** Published versions do not use the staging quotas. They use disk space, which `min-free-mb` protects.
- **Replacing a file** counts the size difference only, not an extra file.
- **Chunked uploads** (no `Content-Length`): the free-space check assumes the largest size the quotas allow.
- **Parallel uploads** cannot exceed a quota together: the quotas are checked before the body is received, and again under the upload lock before the file enters staging.
- The free-space check works on Unix systems (Linux, macOS). On other systems it is skipped.

### Choosing values

- Set `max-upload-mb` above your largest single file, typically the largest archive or package.
- Set `max-version-mb` above the total size of one release, and `max-version-files` above its file count. A goreleaser release with many platforms, SBOMs and signatures can exceed 100 files.
- Set `max-staging-mb` to what you accept on disk for unpublished work: roughly the number of versions published at the same time, multiplied by the size of one version.
- Keep `min-free-mb` large enough for the server to finish publishing and writing its logs.

```yaml
# Large releases, several teams publishing at once
max-upload-mb: 1024
max-version-mb: 8192
max-version-files: 300
max-staging-mb: 40960
min-free-mb: 4096
```

## Staging purge

A staged version that nobody publishes stays on disk. With `staging-ttl` greater than `0` (the default is `24h`), the server purges:

- every staged version whose most recent file is older than the TTL;
- the files of `bin.local/.uploads-tmp/` older than the TTL, left by an interrupted upload or a crash.

The purge runs at startup, then every `min(ttl / 2, 1h)`. It never touches a published version. Each removal is written to the [audit log](audit-log.md), as `staging_purged` (with `product`, `version`, `files`, `bytes`, `idle`) or `tmp_purged` (with `files`).

| `staging-ttl` | Effect |
| --- | --- |
| `24h` (default) | An upload untouched for a day is removed. |
| `0` | No purge. Clean staging yourself. |
| Less than `1h` | Refused at startup: an upload may run for 30 minutes, and the purge must never remove it while it runs. |

The TTL counts from the **last file uploaded** to that version. A pipeline that keeps uploading files keeps its staging alive.

## Abandon an upload by hand

To free space at once, remove the staged version while no upload runs for it:

```sh
rm -rf /srv/registry/bin.local/.uploads/<product>/<version>
```

To fix a refused publish instead, keep the staging: upload the missing or corrected file under the same name, then publish again. See [Troubleshooting](../troubleshooting.md).

## Watch the staging area

```sh
du -sh /srv/registry/bin.local/.uploads/*/*          # staged versions, by size
find /srv/registry/bin.local/.uploads -mindepth 2 -maxdepth 2 -type d -mtime +1   # older than a day
jq -c 'select(.event=="staging_purged")' /srv/registry/bin.local/audit.log
```

A staging area that fills up regularly means pipelines upload and never publish, usually because publish is refused. The `publish_refused` events of the audit log tell you why.
