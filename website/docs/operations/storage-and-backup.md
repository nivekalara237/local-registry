---
title: Storage and backup
description: The layout of the asset path, how to back it up and restore it, and how to remove a version safely.
---

local-registry keeps everything as plain files under one directory, the asset path. There is no database and no cache: the server reads the disk at each request. This page describes the layout, and the procedures that act on it directly: backup, restore, and the removal of a version.

## Layout

```
/srv/registry/                               the asset path
├── bin.local/
│   ├── assets/                              public
│   │   └── binaries/<product>/
│   │       ├── release.json                 copy of the latest version's release.json: marks the latest
│   │       ├── artifacts.json               copy of the latest version's artifacts.json, when it has one
│   │       ├── index.json                   the product and its versions, rewritten at each publish
│   │       └── <version>/                   one published version, immutable
│   │           ├── registry.yaml            product card
│   │           ├── release.json             the version, as the server describes it
│   │           ├── artifacts.json           when uploaded, rewritten with download URLs
│   │           ├── artifacts.goreleaser.json   artifacts.json as uploaded
│   │           ├── metadata.json, CHANGELOG.md   when uploaded
│   │           ├── *checksums.txt           uploaded, or written by publish
│   │           └── archives, packages, files…
│   ├── .uploads/<product>/<version>/        private: staging, uploaded but not published
│   ├── .uploads-tmp/                        private: files being received
│   └── audit.log                            private, mode 0600
├── assets/binaries/<product>/<version>/     public: legacy tree, read by POST /releases
└── anything else                            private, never served
```

What each file means is described in [The local-registry/v1 standard](../concepts/standard-v1.md). How "latest" works is described in [Latest](../concepts/latest.md).

### Rules for the file system

- **Keep `bin.local/` on one file system.** Publishing renames the staging directory into `bin.local/assets/`, and received files are renamed from `.uploads-tmp/` into staging. A rename is atomic only within one file system. Do not mount `.uploads`, `.uploads-tmp` or `assets` separately.
- **One server per asset path.** The upload lock lives in the process. Two servers writing to the same directory, or a shared network volume mounted by two hosts, can interleave their publishes.
- **Files you copy by hand are visible at once.** There is no cache to clear.

## Capacity planning

Disk usage is the sum of:

- every published version, which is never removed automatically;
- the staging area, capped by `max-staging-mb`;
- the audit log, which grows with each upload.

Keep `min-free-mb` of free space at all times: uploads that would go below it are refused with `507`. See [Quotas and staging](quotas-and-staging.md).

## Backup

Published versions never change, and the files that do change (`<product>/release.json`, `<product>/artifacts.json`, `<product>/index.json`) are replaced atomically. You can back up a running server with a file-level tool.

Back up:

| Path | Why |
| --- | --- |
| `bin.local/assets/` | Every published version, and the latest marks. |
| `bin.local/audit.log`, or your `audit-log` path | The trace of every publication. |
| `assets/` | Only if you still use the legacy tree. |
| Your configuration file | Settings. It holds the token: store the copy as a secret. |

Skip `bin.local/.uploads/` and `bin.local/.uploads-tmp/`: they hold unpublished work that the publishers can resend.

```sh
# Nightly, incremental copy to a backup host
rsync -a --delete \
  --exclude 'bin.local/.uploads/' --exclude 'bin.local/.uploads-tmp/' --exclude '.tmp-*' \
  /srv/registry/ backup.example.com:/backups/local-registry/
```

For a point-in-time copy, prefer a file-system or volume snapshot (LVM, ZFS, a cloud disk snapshot, a Kubernetes `VolumeSnapshot`).

> [!TIP]
> A publish that runs during the copy is either fully in the backup or not at all for its version directory. Only the latest mark of that product may be one publish behind. Run the backup outside your release windows, or take a snapshot, if that matters to you.

## Restore

1. Stop the server.
2. Restore the backup into the asset path.
3. Give it back to the service user: `chown -R registry:registry /srv/registry` (uid `10001` for the container image).
4. Start the server, then check a product:

   ```sh
   curl -s https://registry.example.com/products
   curl -s https://registry.example.com/<product>/releases/latest
   ```

## Remove a published version

There is no API to delete a version, on purpose: clients may already depend on it. Prefer publishing a fixed version and making it the latest. If you must remove a version, for example after a leak:

1. Check whether it is the latest:

   ```sh
   cd /srv/registry/bin.local/assets/binaries/<product>
   jq -r .version release.json
   ```

2. If it is, first make another version the latest. See [Change the latest version](../guides/change-latest.md).
3. Remove the version directory:

   ```sh
   rm -rf /srv/registry/bin.local/assets/binaries/<product>/<version>
   ```

4. `GET /<product>/index` and `GET /products` are built from the disk at each request: they no longer list the version. The file `<product>/index.json` is rewritten only at the next publish of that product. Until then it still lists the removed version: delete it, or publish a version.

Do not reuse a removed version number for different content: caches and clients may still hold the old files under that name.

## Clean up after a crash

A crash can leave temporary files behind. They are ignored by every route, and you can delete them when no upload runs:

| Files | Where | Cleaned by |
| --- | --- | --- |
| `upload-*` | `bin.local/.uploads-tmp/` | The staging purge, after `staging-ttl`. |
| `.tmp-*` | `bin.local/assets/binaries/<product>/` | You, by hand. |
