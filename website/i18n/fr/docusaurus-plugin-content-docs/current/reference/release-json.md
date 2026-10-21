---
title: release.json
description: Référence normative de release.json, le document que le serveur écrit pour chaque version publiée.
sidebar_label: release.json
---

`release.json` décrit une version publiée : ses fichiers avec plateforme, taille, sha256 et URL de téléchargement, ses signatures et SBOM, son fichier de checksums et ses notes de version. Le serveur l'écrit à la publication, dans la version : il apparaît avec elle et ne change plus jamais. C'est le document que les clients devraient lire ; il existe pour toute version, construite avec goreleaser ou non.

Il est servi par `GET /{product}/releases/{version}` (`latest` accepté, voir [API des produits](./api/products.md)) et comme fichier, `GET /{product}/artifacts/{version}/files/release.json`.

## Exemple

```json
{
  "apiVersion": "local-registry/v1",
  "product": "sonarbridge",
  "displayName": "SonarBridge",
  "version": "v1.4.2",
  "tag": "v1.4.2",
  "previousTag": "v1.4.1",
  "prerelease": false,
  "commit": "9c9d7664ceefa1b2…",
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
      "sbom": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz.sbom.json",
      "signature": null,
      "certificate": null
    },
    {
      "name": "migrations.sql",
      "kind": "file",
      "format": "sql",
      "mediaType": "application/sql",
      "os": null,
      "arch": null,
      "variant": null,
      "size": 18213,
      "sha256": "9ab0…",
      "url": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/migrations.sql",
      "sbom": null,
      "signature": null,
      "certificate": null
    }
  ],
  "checksums": {
    "name": "sonarbridge_1.4.2_checksums.txt",
    "url": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_checksums.txt",
    "signature": "https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_checksums.txt.sigstore.json",
    "certificate": null
  }
}
```

**Toutes les clés sont toujours présentes.** Une valeur inconnue vaut `null`, jamais une clé absente. Les clients doivent ignorer les clés qu'ils ne connaissent pas : de nouvelles peuvent être ajoutées au sein de `local-registry/v1`.

## Champs de la version

| Champ | Type | Source |
| --- | --- | --- |
| `apiVersion` | chaîne | Toujours `local-registry/v1`. |
| `product` | chaîne | Le segment `{product}` de l'URL de publication. |
| `displayName` | chaîne | `displayName` de `registry.yaml`, sinon `project_name` de `metadata.json`, sinon le produit. |
| `version` | chaîne | Le segment `{version}` de l'URL de publication (`v1.4.2`) : utilisable tel quel dans les URL. |
| `tag` | chaîne ou null | `tag` de `metadata.json` ; `null` sans lui. |
| `previousTag` | chaîne ou null | `previous_tag` de `metadata.json`. |
| `prerelease` | booléen | `true` quand `version` est une pré-version semver, comme `v1.5.0-rc.1`. |
| `commit` | chaîne ou null | `commit` de `metadata.json`. |
| `builtAt` | chaîne ou null | `date` de `metadata.json`, telle que goreleaser l'a écrite. |
| `publishedAt` | chaîne | Horloge du serveur à la publication, UTC, RFC 3339. |
| `publishedBy` | chaîne | `token_id` du jeton de dépôt : les 12 premiers chiffres hexadécimaux de son sha256, jamais le jeton. |
| `deprecated` | booléen ou chaîne | `deprecated` de `registry.yaml` : `false`, `true` ou un message. |
| `securityUpdates` | booléen | `true` quand une des `changes` est de type `security`. |
| `changes` | liste | Notes de version tirées de `CHANGELOG.md`, dans l'ordre ; `[]` sans lui. Voir [CHANGELOG.md](./changelog-format.md). |
| `artifacts` | liste | Les fichiers téléchargeables de la version (ci-dessous). |
| `checksums` | objet ou null | Le fichier de checksums (ci-dessous). |

Chaque entrée de `changes` :

| Champ | Contenu |
| --- | --- |
| `kind` | `added`, `changed`, `deprecated`, `removed`, `fixed` ou `security`. |
| `description` | La note, en une ligne de texte. |

## Artefacts

Une entrée par fichier autonome : archives, binaires, paquets, autres fichiers, archives de sources. Les SBOM, signatures et certificats ne sont pas des entrées : ils sont rattachés au fichier qu'ils couvrent.

| Champ | Type | Contenu |
| --- | --- | --- |
| `name` | chaîne | Nom du fichier. |
| `kind` | chaîne | `archive`, `binary`, `package`, `file` ou `source` (voir Types). |
| `format` | chaîne ou null | `extra.Format` d'`artifacts.json` s'il est défini, sinon l'extension sans le point (`tar.gz`, `tar.xz`, `tar.zst` et `tar.bz2` comptent pour une), sinon `null`. |
| `mediaType` | chaîne | Déduit de la fin du nom (voir Types de média). |
| `os` | chaîne ou null | `goos` d'`artifacts.json`. |
| `arch` | chaîne ou null | `goarch` d'`artifacts.json`. |
| `variant` | chaîne ou null | Le premier défini parmi `goamd64`, `goarm`, `goarm64`, `go386`, `gomips`, `goppc64`, `goriscv64`. |
| `size` | nombre | Octets, mesurés par le serveur. |
| `sha256` | chaîne | Calculé par le serveur : le même que dans le fichier de checksums. |
| `url` | chaîne | `/{product}/artifacts/{version}/files/{name}`, absolue sous `--public-url` s'il est défini. |
| `sbom` | chaîne ou null | URL du SBOM qui couvre ce fichier. |
| `signature` | chaîne ou null | URL de sa signature. |
| `certificate` | chaîne ou null | URL de son certificat. |

`os`, `arch` et `variant` valent `null` pour un fichier qui ne dépend d'aucune plateforme, et pour tous les fichiers d'une version publiée sans `artifacts.json`.

### Checksums

| Champ | Type | Contenu |
| --- | --- | --- |
| `name` | chaîne | Nom du fichier de checksums. |
| `url` | chaîne | Son URL de téléchargement. |
| `signature` | chaîne ou null | URL de sa signature (par exemple `<name>.sigstore.json`). |
| `certificate` | chaîne ou null | URL de son certificat. |

Le fichier de checksums est l'entrée `Checksum` d'`artifacts.json`, sinon le `*checksums.txt` déposé, sinon le `checksums.txt` écrit par la publication. Toute version en a un.

## Types

Avec un `artifacts.json`, le `type` goreleaser de chaque entrée publiée donne son rôle :

| Type goreleaser | `kind` | Remarque |
| --- | --- | --- |
| `Archive` | `archive` | `format` : `tar.gz`, `zip`… |
| `Binary` avec `extra.Format` | `binary` | Un binaire publié sans archive. |
| `Linux Package` | `package` | `format` : `deb`, `rpm`, `apk`, `archlinux`. |
| `File` | `file` | Tout autre fichier (`release.extra_files`) : configuration, SQL, PDF, chart Helm… |
| `Source` | `source` | Archive de sources. |
| `Checksum` | — | Remplit `checksums`. |
| `SBOM` | — | Rattaché comme `sbom` à `<name>` quand il s'appelle `<name>.sbom.json`, `<name>.spdx.json`, `<name>.cdx.json` ou `<name>.sbom`. |
| `Signature` | — | Rattachée comme `signature` à `<name>`, ou au fichier de checksums, quand elle s'appelle `<name>.sigstore.json`, `<name>.bundle` ou `<name>.sig`. |
| `Certificate` | — | Rattaché comme `certificate` de la même façon, pour `<name>.pem`, `<name>.crt` ou `<name>.cert`. |
| `Metadata` | — | `metadata.json` : ses valeurs remplissent les champs de la version. |
| `Binary` sans `extra.Format` | ignoré | Une sortie de build dont une archive a été faite. |

Une pièce jointe qui ne couvre aucun fichier listé reste téléchargeable mais n'apparaît dans aucune entrée. Quand un fichier a deux pièces jointes du même type, la première dans `artifacts.json` l'emporte.

### Sans artifacts.json

Chaque fichier en staging est typé d'après son nom, puis suit le tableau ci-dessus. `registry.yaml`, `CHANGELOG.md`, `metadata.json` et les fichiers écrits par le serveur ne sont pas des entrées.

| Nom | Type |
| --- | --- |
| Le fichier de checksums | `Checksum` |
| Le nom d'un autre fichier en staging + un suffixe de SBOM, de signature ou de certificat (ci-dessus) | `SBOM`, `Signature` ou `Certificate` |
| Finit par `.tar.gz`, `.tgz`, `.tar.xz`, `.tar.zst`, `.tar.bz2`, `.tar` ou `.zip` | `Archive` |
| Finit par `.deb`, `.rpm` ou `.apk` | `Linux Package` |
| Tout le reste | `File` |

Un fichier `.sig` ou `.pem` qui ne couvre aucun autre fichier en staging est un simple `File`. Pour fournir `os` et `arch`, déposez un `artifacts.json`, écrit à la main au besoin, avec `name`, `path`, `type`, `goos` et `goarch` pour chaque fichier.

## Types de média

Comparés à la fin du nom, casse ignorée, dans cet ordre :

| Suffixe | `mediaType` |
| --- | --- |
| `.tar.gz`, `.tgz`, `.gz` | `application/gzip` |
| `.tar.xz`, `.xz` | `application/x-xz` |
| `.tar.zst`, `.zst` | `application/zstd` |
| `.tar` | `application/x-tar` |
| `.zip` | `application/zip` |
| `.deb` | `application/vnd.debian.binary-package` |
| `.rpm` | `application/x-rpm` |
| `.exe` | `application/vnd.microsoft.portable-executable` |
| `.msi` | `application/x-msi` |
| `.dmg` | `application/x-apple-diskimage` |
| `.pdf` | `application/pdf` |
| `.sql` | `application/sql` |
| `.yaml`, `.yml` | `application/yaml` |
| `.json` | `application/json` |
| `.sh` | `application/x-sh` |
| `.md` | `text/markdown` |
| `.txt` | `text/plain` |
| tout le reste | `application/octet-stream` |
