# The `local-registry/v1` standard

[Français](SPEC.fr.md)

This standard describes the **products** published to local-registry and their **versions**. It builds on what goreleaser produces, but goreleaser is optional: `registry.yaml` and the files are enough. Only one file is written by hand, and it is required.

- **Product**: what a team delivers and versions, such as `sonarbridge`, `billing-agent`, `db-migrations` or `security-policies`. It is the `{product}` segment of every URL. Its name matches `[A-Za-z0-9._-]+`, is neither `.` nor `..` nor `products`, and is chosen at upload time. Lowercase with `-` is recommended.
- **Version**: one published release of a product: the `{version}` segment, usually the git tag (`v1.4.2`). It is immutable once published.

| Document | Written by | Where | Role |
| --- | --- | --- | --- |
| `registry.yaml` | the team, required | uploaded with each version | Product card, and deprecation of the version |
| `CHANGELOG.md` | goreleaser (`dist/CHANGELOG.md`) or the team, optional | uploaded with each version | Source of the release notes |
| `release.json` | the server, at publish | `binaries/<product>/<version>/release.json` | One version, normalised, immutable |
| `index.json` | the server, at each publish | `binaries/<product>/index.json` | The product and its versions |

Every generated document carries `"apiVersion": "local-registry/v1"`. `artifacts.json` and `metadata.json`, when uploaded, are still served as before.

```
goreleaser release            upload.sh <product>                 server (publish)
──────────────────            ───────────────────                 ────────────────
dist/*.tar.gz, *.zip, …  ───▶ PUT each file ───────────────────▶ staging
dist/metadata.json       ───▶ PUT                                  │
registry.yaml            ───▶ PUT (required)                       │
dist/CHANGELOG.md        ───▶ PUT (if present)                     │
dist/artifacts.json      ───▶ PUT last                             │
                              POST …/publish ─────────────────▶ checks, then writes
                                                                  release.json (in the version)
                                                                  index.json (rewritten)
```

Without goreleaser, `upload.sh <product> DIR` sends every file of `DIR`, then `registry.yaml`: the server infers what `artifacts.json` and `metadata.json` would have said (section 3) and writes `checksums.txt`.

---

## 1. `registry.yaml`: the product card

The file lives at the root of the project. `upload.sh` sends it when it exists (`--product-file` to pick another path). Because it is uploaded with each version, the card follows the code, and each version keeps the card it was published with.

```yaml
apiVersion: local-registry/v1
displayName: SonarBridge
description: Bridge between SonarQube and internal tools.
license: Proprietary                  # SPDX identifier (MIT, Apache-2.0…) or "Proprietary"
owners:
  - name: Platform team
    email: platform@corp.example
links:
  - { name: source, url: https://git.corp.example/platform/sonarbridge }
  - { name: docs,   url: https://docs.corp.example/sonarbridge }
keywords: [sonar, cli]
labels:                               # free key/value pairs
  domain: quality
  cost-center: "4512"
install: |                            # markdown, shown as is
  curl -fsSL https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/… | tar xz
deprecated: false                     # or a message: "replaced by sonarbridge2"
```

| Field | Type | Required | Rule |
| --- | --- | --- | --- |
| `apiVersion` | string | **yes** | `local-registry/v1` |
| `displayName` | string | no | ≤ 100 characters. Default: `project_name` of `metadata.json`, else the product name |
| `description` | string | no | ≤ 500 characters |
| `license` | string | no | — |
| `owners[]` | `{name, email?}` | no | `name` not empty; `email` contains `@` |
| `links[]` | `{name, url}` | no | `name` not empty; `url` is `http(s)://host…` |
| `keywords[]` | strings | no | ≤ 20, none blank |
| `labels` | map of strings | no | ≤ 50 entries; numbers and booleans are read as strings |
| `install` | string | no | ≤ 10 KiB |
| `deprecated` | boolean or string | no | Default `false`. An empty string means `false` |

**Validation.** It happens at `PUT …/files/registry.yaml`, and an invalid file answers `422 registry.yaml: <reason>`:

- An unknown key is refused, so a typo cannot pass unnoticed. This includes `changes`: release notes come from `CHANGELOG.md`.
- The file is at most 64 KiB and holds a single YAML document.
- [`schema/registry.schema.json`](schema/registry.schema.json) (JSON Schema 2020-12) lets CI check the file before sending it. For example, with [check-jsonschema](https://github.com/python-jsonschema/check-jsonschema): `check-jsonschema --schemafile schema/registry.schema.json registry.yaml`.

**Required.** Every version needs its card: publishing without it answers `422 registry.yaml not uploaded: every version needs its product card`.

## 2. `CHANGELOG.md`: the release notes

goreleaser writes its release notes to `dist/CHANGELOG.md`; without goreleaser, the team writes one. `upload.sh` sends that file when it exists. The server checks it at upload (≤ 1 MiB, UTF-8), then turns it into `changes` at publish. Two forms are read, and may mix.

**Keep a Changelog.** Each list item under an `### Added`, `### Changed`, `### Deprecated`, `### Removed`, `### Fixed` or `### Security` heading (any case) is a note of that kind, its text as is. When the file has version headings (`## [1.2.0] - 2026-10-18`, `## v1.2.0`, `## [Unreleased]`), only the section of the published version is read, with or without its `v`; a version without a section has no notes. Without version headings, the whole file is read.

```markdown
## [1.0.0] - 2026-10-18
### Added
- orders table
### Security
- no default password
```

**goreleaser.** Each other list item (`* …` or `- …`) whose message is a [Conventional Commit](https://www.conventionalcommits.org/) becomes one note. Before parsing, goreleaser's decorations are stripped: the leading commit sha, and a trailing author such as ` (@user)`, ` (Name <mail>)` or ` by @user in <url>`. Everything else is ignored: headings, `**Full Changelog**`, and messages without a known type.

| Commit type | `kind` |
| --- | --- |
| `feat`, `add` | `added` |
| `fix`, `bugfix` | `fixed` |
| `perf`, `refactor`, `change` | `changed` |
| `deprecate`, `deprecated` | `deprecated` |
| `remove`, `removed` | `removed` |
| `sec`, `security`, or any type with scope `security`/`sec` (`fix(security): …`) | `security` |
| any other type marked breaking (`chore!: …`) | `changed` |
| `docs`, `test`, `chore`, `ci`, `build`, `style`… | not a note |

`* 9c9d766: feat(api): CSV export (@alice)` becomes `{"kind": "added", "description": "CSV export"}`.

Use `changelog.use: git` or `github` in `.goreleaser.yaml`, and keep the `feat:`, `fix:`… prefixes in the messages: if goreleaser's `filters` drop them, nothing remains to read. Without `CHANGELOG.md`, `changes` is `[]`.

## 3. `release.json`: one version

It is generated at publish, inside the staged version, so it appears with the version and is immutable like every other file. It is served by `GET /{product}/releases/{version}` and `GET /{product}/artifacts/{version}/files/release.json`.

```json
{
  "apiVersion": "local-registry/v1",
  "product": "sonarbridge",
  "displayName": "SonarBridge",
  "version": "v1.4.2",
  "tag": "v1.4.2",
  "previousTag": "v1.4.1",
  "prerelease": false,
  "commit": "9c9d7664ceef…",
  "builtAt": "2026-10-10T20:01:27+02:00",
  "publishedAt": "2026-10-10T18:05:02Z",
  "publishedBy": "e2186dbdb1bb",
  "deprecated": false,
  "securityUpdates": true,
  "changes": [
    { "kind": "added", "description": "CSV export" },
    { "kind": "security", "description": "bump golang.org/x/net" }
  ],
  "artifacts": [
    {
      "name": "sonarbridge_1.4.2_linux_amd64.tar.gz",
      "kind": "archive",
      "format": "tar.gz",
      "mediaType": "application/gzip",
      "os": "linux",
      "arch": "amd64",
      "variant": "v1",
      "size": 4821337,
      "sha256": "3f1c…",
      "url": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz",
      "sbom": "https://…/files/sonarbridge_1.4.2_linux_amd64.tar.gz.sbom.json",
      "signature": null,
      "certificate": null
    },
    {
      "name": "migrations.sql",
      "kind": "file",
      "format": "sql",
      "mediaType": "application/sql",
      "os": null, "arch": null, "variant": null,
      "size": 18213,
      "sha256": "9ab0…",
      "url": "https://…/files/migrations.sql",
      "sbom": null, "signature": null, "certificate": null
    }
  ],
  "checksums": {
    "name": "sonarbridge_1.4.2_checksums.txt",
    "url": "https://…/files/sonarbridge_1.4.2_checksums.txt",
    "signature": "https://…/files/sonarbridge_1.4.2_checksums.txt.sigstore.json",
    "certificate": null
  }
}
```

Every key is always present. An unknown value is `null`, never a missing key.

| Field | Source |
| --- | --- |
| `product`, `version` | the `{product}` and `{version}` segments of the publish URL |
| `displayName` | `registry.yaml`, else `project_name` of `metadata.json`, else the product |
| `tag`, `previousTag`, `commit`, `builtAt` | `metadata.json`: `tag`, `previous_tag`, `commit`, `date`; `null` without it |
| `prerelease` | `version` is a semver pre-release (`v1.5.0-rc.1`) |
| `publishedAt` | server clock at publish, UTC, RFC 3339 |
| `publishedBy` | `token_id` of the upload token (first 12 hex digits of its sha256), never the token |
| `deprecated` | `registry.yaml`, default `false` |
| `changes` | `CHANGELOG.md` (section 2) |
| `securityUpdates` | `true` if a note has kind `security` |
| `artifacts[]` | the published entries of `artifacts.json` that are files of their own (below); without it, inferred from the file names (below) |
| `size`, `sha256` | computed by the server: the same as the checksums file |
| `url` | `GET /{product}/artifacts/{version}/files/{name}`, absolute under `--public-url`, else relative |
| `checksums` | the `Checksum` entry of `artifacts.json`, else the uploaded `*checksums.txt`, else the `checksums.txt` written by publish |

### Kinds

| goreleaser type | `kind` | Note |
| --- | --- | --- |
| `Archive` | `archive` | `format`: `tar.gz`, `zip`… |
| `Binary` with `extra.Format` | `binary` | binary published without an archive |
| `Linux Package` | `package` | `format`: `deb`, `rpm`, `apk`, `archlinux` |
| `File` (`release.extra_files`) | `file` | any other file: configs, SQL scripts, PDF, Helm charts… |
| `Source` | `source` | source archive |
| `Checksum` | — | fills `checksums` |
| `SBOM` | — | attached as `sbom` to `<name>` when named `<name>.sbom.json`, `.spdx.json`, `.cdx.json` or `.sbom` |
| `Signature` | — | attached as `signature` to `<name>` (or to the checksums file) when named `<name>.sigstore.json`, `.bundle` or `.sig` |
| `Certificate` | — | attached as `certificate` the same way, for `<name>.pem`, `.crt` or `.cert` |
| `Metadata` | — | `metadata.json`: its values become fields of the version |
| `Binary` without `Format` | ignored | build output, as everywhere else |

An attachment that covers no listed file stays downloadable but appears in no `artifacts[]` entry. When a file has two attachments of the same kind, the first one in `artifacts.json` wins.

- **`format`**: `extra.Format` when goreleaser sets it, else the file extension (`tar.gz`, `tar.xz`, `tar.zst` and `tar.bz2` count as one), else `null`.
- **`mediaType`**: taken from a fixed table on the end of the name, ignoring case: `.tar.gz`/`.tgz`/`.gz` `application/gzip`, `.xz` `application/x-xz`, `.zst` `application/zstd`, `.tar` `application/x-tar`, `.zip` `application/zip`, `.deb` `application/vnd.debian.binary-package`, `.rpm` `application/x-rpm`, `.exe` `application/vnd.microsoft.portable-executable`, `.msi` `application/x-msi`, `.dmg` `application/x-apple-diskimage`, `.pdf` `application/pdf`, `.sql` `application/sql`, `.yaml`/`.yml` `application/yaml`, `.json` `application/json`, `.sh` `application/x-sh`, `.md` `text/markdown`, `.txt` `text/plain`. Anything else is `application/octet-stream`.
- **`os`, `arch`, `variant`**: from `goos`, `goarch` and the first of `goamd64`, `goarm`, `goarm64`, `go386`, `gomips`, `goppc64`, `goriscv64`. They are `null` for a file that does not depend on a platform.

### Without `artifacts.json`

Each staged file is typed from its name, then follows the table above. `registry.yaml`, `CHANGELOG.md`, `metadata.json` and the files the server writes are not entries.

| Name | Type |
| --- | --- |
| the checksums file | `Checksum` |
| `<other staged file>` + an SBOM, signature or certificate suffix (above) | `SBOM`, `Signature`, `Certificate` |
| `.tar.gz`, `.tgz`, `.tar.xz`, `.tar.zst`, `.tar.bz2`, `.tar`, `.zip` | `Archive` |
| `.deb`, `.rpm`, `.apk` | `Linux Package` |
| anything else | `File` |

`os`, `arch` and `variant` are `null`: to give them, upload an `artifacts.json`, written by hand if need be (`name`, `path`, `type`, `goos`, `goarch`).

**Products that are not binaries.** Upload the files with `registry.yaml` (`upload.sh <product> DIR`), or use goreleaser with `builds: [{ skip: true }]` and `release.extra_files`.

## 4. `index.json`: a product and its versions

```json
{
  "apiVersion": "local-registry/v1",
  "product": "sonarbridge",
  "displayName": "SonarBridge",
  "description": "Bridge between SonarQube and internal tools.",
  "license": "Proprietary",
  "owners": [{ "name": "Platform team", "email": "platform@corp.example" }],
  "links": [{ "name": "source", "url": "https://git.corp.example/platform/sonarbridge" }],
  "keywords": ["sonar", "cli"],
  "labels": { "domain": "quality" },
  "install": "…",
  "latest": "v1.4.2",
  "updatedAt": "2026-10-10T18:05:02Z",
  "versions": [
    { "version": "v1.5.0-rc.1", "tag": "v1.5.0-rc.1", "publishedAt": "2026-10-12T08:00:00Z", "prerelease": true, "securityUpdates": false, "deprecated": false },
    { "version": "v1.4.2", "tag": "v1.4.2", "publishedAt": "2026-10-10T18:05:02Z", "prerelease": false, "securityUpdates": true, "deprecated": false }
  ]
}
```

- **Versions.** Only versions that have a `release.json` are listed. Versions published before this standard are not converted: they stay downloadable through the existing routes. Semver versions come first, newest first. Other names follow, newest `publishedAt` first.
- **`latest`.** The same rule as everywhere else: `publish?latest=true` copies the version's `release.json` to `<product>/release.json`, and the latest is the `version` it names, when that version has a `release.json`. It is `null` otherwise. To change it by hand, copy `<version>/release.json` to `<product>/release.json`.
- **Card.** It comes from the `registry.yaml` of the `latest` version, else from the version published last. Without a card, the text fields are `""` and the lists are `[]`. `displayName` is the one in that version's `release.json`.
- **`updatedAt`.** The most recent `publishedAt`.
- **Freshness.** The file is rewritten atomically after each publish of the product. `GET /{product}/index` does not read it back: it rebuilds the index from the disk at each request, so a `latest` changed by hand shows at once.

## 5. Routes

These routes are read-only and need no token. `HEAD` works too. Errors are `{"error": "…"}`.

| Route | Answer | Errors |
| --- | --- | --- |
| `GET /products` | `[{product, displayName, description, latest, updatedAt}]`, by name; `[]` when empty | — |
| `GET /{product}/index` | the index (section 4) | `404` if no version has a `release.json` |
| `GET /{product}/releases/{version}` | `release.json`; `latest` is accepted | `404` if there is none |

`GET /{product}/releases/{version}` and `POST /releases/{product}/{version}` (the files of a version) are distinct routes. `POST /releases/{version}`, which assumes the product `gitlab`, is **deprecated**: it answers with `Deprecation: true` and a `Link: </releases/gitlab/{version}>; rel="successor-version"` header, and will be removed in a later major version.

## 6. Upload rules

| Rule | Answer |
| --- | --- |
| Product named `products` | `400 product name products is reserved` |
| Version named `artifacts.json`, `artifacts.goreleaser.json`, `release.json` or `index.json` | `400 version name <name> is reserved` |
| `PUT` of `release.json`, `index.json` or `artifacts.goreleaser.json` | `400 <name> is reserved: publishing writes it` |
| `artifacts.json` lists one of those names | `422` |
| Invalid `registry.yaml` | `422 registry.yaml: <reason>` |
| `CHANGELOG.md` over 1 MiB, or not UTF-8 | `422` |
| Publish without `registry.yaml` | `422 registry.yaml not uploaded: every version needs its product card` |
| Publish with no artifact (no file besides `registry.yaml`, `CHANGELOG.md`, `metadata.json`, checksums, attachments) | `422 no artifact to publish: upload at least one file besides registry.yaml` |
| Several `*checksums.txt` | `422 several checksums files: a and b` |

`artifacts.json`, `metadata.json` and the checksums file are optional; each one uploaded is checked as before. With a checksums file, every staged file must be listed in it except `registry.yaml`, `CHANGELOG.md`, `metadata.json`, the files the server writes, and signatures (`*.sig`, `*.pem`, `*.sigstore.json`). With `artifacts.json`, every staged file except `registry.yaml`, `CHANGELOG.md` and `metadata.json` must be one of its entries. Without a checksums file, publish writes `checksums.txt` in the same format (`<sha256>  <name>`, by name) for the files that would need a line.

## 7. Versioning of the standard

`apiVersion` changes (`local-registry/v2`) only for an incompatible change: a field removed or renamed, or a type or meaning changed. Adding a field, a `kind`, a media type or a commit type keeps `v1`. Clients must therefore ignore fields they do not know.
