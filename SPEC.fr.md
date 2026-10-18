# Le standard `local-registry/v1`

[English](SPEC.md)

Ce standard décrit les **produits** publiés dans local-registry et leurs **versions**. Il s'appuie sur ce que goreleaser produit, mais goreleaser est facultatif : `registry.yaml` et les fichiers suffisent. Un seul fichier s'écrit à la main, et il est obligatoire.

- **Produit** : ce qu'une équipe livre et versionne, par exemple `sonarbridge`, `billing-agent`, `db-migrations` ou `security-policies`. C'est le segment `{product}` de toutes les URLs. Son nom respecte `[A-Za-z0-9._-]+`, ne vaut ni `.`, ni `..`, ni `products`, et se choisit au moment de l'upload. Les minuscules avec `-` sont recommandées.
- **Version** : une publication d'un produit, c'est-à-dire le segment `{version}`, en général le tag git (`v1.4.2`). Elle est immuable une fois publiée.

| Document | Écrit par | Où | Rôle |
| --- | --- | --- | --- |
| `registry.yaml` | l'équipe, obligatoire | envoyé avec chaque version | Fiche du produit, et dépréciation de la version |
| `CHANGELOG.md` | goreleaser (`dist/CHANGELOG.md`) ou l'équipe, facultatif | envoyé avec chaque version | Source des notes de version |
| `release.json` | le serveur, à la publication | `binaries/<product>/<version>/release.json` | Une version, normalisée, immuable |
| `index.json` | le serveur, à chaque publication | `binaries/<product>/index.json` | Le produit et ses versions |

Chaque document généré porte `"apiVersion": "local-registry/v1"`. `artifacts.json` et `metadata.json`, s'ils sont envoyés, restent servis comme avant.

```
goreleaser release            upload.sh <product>                 serveur (publish)
──────────────────            ───────────────────                 ─────────────────
dist/*.tar.gz, *.zip, …  ───▶ PUT chaque fichier ──────────────▶ staging
dist/metadata.json       ───▶ PUT                                  │
registry.yaml            ───▶ PUT (obligatoire)                    │
dist/CHANGELOG.md        ───▶ PUT (s'il existe)                    │
dist/artifacts.json      ───▶ PUT en dernier                       │
                              POST …/publish ─────────────────▶ vérifie, puis écrit
                                                                  release.json (dans la version)
                                                                  index.json (réécrit)
```

Sans goreleaser, `upload.sh <product> DIR` envoie chaque fichier de `DIR`, puis `registry.yaml` : le serveur déduit ce qu'`artifacts.json` et `metadata.json` auraient dit (section 3) et écrit `checksums.txt`.

---

## 1. `registry.yaml` : la fiche produit

Le fichier se trouve à la racine du projet. `upload.sh` l'envoie s'il existe (`--product-file` permet d'indiquer un autre chemin). Comme il est envoyé avec chaque version, la fiche suit le code, et chaque version garde la fiche avec laquelle elle a été publiée.

```yaml
apiVersion: local-registry/v1
displayName: SonarBridge
description: Pont entre SonarQube et les outils internes.
license: Proprietary                  # identifiant SPDX (MIT, Apache-2.0…) ou "Proprietary"
owners:
  - name: Équipe Plateforme
    email: platform@corp.example
links:
  - { name: source, url: https://git.corp.example/platform/sonarbridge }
  - { name: docs,   url: https://docs.corp.example/sonarbridge }
keywords: [sonar, cli]
labels:                               # clé/valeur libres
  domain: qualité
  cost-center: "4512"
install: |                            # markdown, affiché tel quel
  curl -fsSL https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/… | tar xz
deprecated: false                     # ou un message : "remplacé par sonarbridge2"
```

| Champ | Type | Obligatoire | Règle |
| --- | --- | --- | --- |
| `apiVersion` | chaîne | **oui** | `local-registry/v1` |
| `displayName` | chaîne | non | ≤ 100 caractères. Par défaut : `project_name` de `metadata.json`, sinon le nom du produit |
| `description` | chaîne | non | ≤ 500 caractères |
| `license` | chaîne | non | — |
| `owners[]` | `{name, email?}` | non | `name` non vide ; `email` contient `@` |
| `links[]` | `{name, url}` | non | `name` non vide ; `url` en `http(s)://hôte…` |
| `keywords[]` | chaînes | non | ≤ 20, aucun vide |
| `labels` | dictionnaire de chaînes | non | ≤ 50 entrées ; nombres et booléens lus comme des chaînes |
| `install` | chaîne | non | ≤ 10 Kio |
| `deprecated` | booléen ou chaîne | non | `false` par défaut. Une chaîne vide vaut `false` |

**Validation.** Elle a lieu au `PUT …/files/registry.yaml`, et un fichier invalide reçoit `422 registry.yaml: <raison>` :

- Une clé inconnue est refusée, pour qu'une faute de frappe ne passe pas inaperçue. C'est aussi le cas de `changes` : les notes de version viennent de `CHANGELOG.md`.
- Le fichier fait au plus 64 Kio et contient un seul document YAML.
- [`schema/registry.schema.json`](schema/registry.schema.json) (JSON Schema 2020-12) permet de vérifier le fichier en CI avant l'envoi. Par exemple, avec [check-jsonschema](https://github.com/python-jsonschema/check-jsonschema) : `check-jsonschema --schemafile schema/registry.schema.json registry.yaml`.

**Obligatoire.** Chaque version a besoin de sa fiche : publier sans elle répond `422 registry.yaml not uploaded: every version needs its product card`.

## 2. `CHANGELOG.md` : les notes de version

goreleaser écrit ses notes de version dans `dist/CHANGELOG.md` ; sans goreleaser, l'équipe en écrit un. `upload.sh` envoie ce fichier s'il existe. Le serveur le vérifie à l'upload (≤ 1 Mio, UTF-8), puis le transforme en `changes` à la publication. Deux formes sont lues, et peuvent se mêler.

**Keep a Changelog.** Chaque élément de liste sous un titre `### Added`, `### Changed`, `### Deprecated`, `### Removed`, `### Fixed` ou `### Security` (toute casse) est une note de ce type, son texte tel quel. Si le fichier a des titres de version (`## [1.2.0] - 2026-10-18`, `## v1.2.0`, `## [Unreleased]`), seule la section de la version publiée est lue, avec ou sans son `v` ; une version sans section n'a pas de notes. Sans titres de version, tout le fichier est lu.

```markdown
## [1.0.0] - 2026-10-18
### Added
- orders table
### Security
- no default password
```

**goreleaser.** Chaque autre élément de liste (`* …` ou `- …`) dont le message est un [Conventional Commit](https://www.conventionalcommits.org/fr/) donne une note. Avant l'analyse, le serveur retire les ajouts de goreleaser : le sha du commit en tête, et l'auteur en fin de ligne, comme ` (@user)`, ` (Nom <mail>)` ou ` by @user in <url>`. Tout le reste est ignoré : les titres, `**Full Changelog**` et les messages sans type connu.

| Type de commit | `kind` |
| --- | --- |
| `feat`, `add` | `added` |
| `fix`, `bugfix` | `fixed` |
| `perf`, `refactor`, `change` | `changed` |
| `deprecate`, `deprecated` | `deprecated` |
| `remove`, `removed` | `removed` |
| `sec`, `security`, ou tout type de portée `security`/`sec` (`fix(security): …`) | `security` |
| tout autre type marqué cassant (`chore!: …`) | `changed` |
| `docs`, `test`, `chore`, `ci`, `build`, `style`… | pas une note |

`* 9c9d766: feat(api): export CSV (@alice)` devient `{"kind": "added", "description": "export CSV"}`.

Utilisez `changelog.use: git` ou `github` dans `.goreleaser.yaml`, et gardez les préfixes `feat:`, `fix:`… dans les messages : si les `filters` de goreleaser les retirent, il ne reste rien à lire. Sans `CHANGELOG.md`, `changes` vaut `[]`.

## 3. `release.json` : une version

Il est généré à la publication, dans la version en staging. Il apparaît donc en même temps que la version, et il est immuable comme les autres fichiers. Il est servi par `GET /{product}/releases/{version}` et par `GET /{product}/artifacts/{version}/files/release.json`.

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
    { "kind": "added", "description": "export CSV" },
    { "kind": "security", "description": "golang.org/x/net mis à jour" }
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

Toutes les clés sont toujours présentes. Une valeur inconnue vaut `null`, et la clé n'est jamais absente.

| Champ | Source |
| --- | --- |
| `product`, `version` | les segments `{product}` et `{version}` de l'URL de publication |
| `displayName` | `registry.yaml`, sinon `project_name` de `metadata.json`, sinon le produit |
| `tag`, `previousTag`, `commit`, `builtAt` | `metadata.json` : `tag`, `previous_tag`, `commit`, `date` ; `null` sans lui |
| `prerelease` | `version` est une pré-version semver (`v1.5.0-rc.1`) |
| `publishedAt` | horloge du serveur à la publication, en UTC, au format RFC 3339 |
| `publishedBy` | `token_id` du token d'upload (les 12 premiers chiffres hexadécimaux de son sha256), jamais le token |
| `deprecated` | `registry.yaml`, `false` par défaut |
| `changes` | `CHANGELOG.md` (section 2) |
| `securityUpdates` | `true` si une note est de type `security` |
| `artifacts[]` | les entrées publiées d'`artifacts.json` qui sont des fichiers à part entière (ci-dessous) ; sans lui, déduites des noms de fichiers (ci-dessous) |
| `size`, `sha256` | calculés par le serveur : identiques au fichier de checksums |
| `url` | `GET /{product}/artifacts/{version}/files/{name}`, absolue sous `--public-url`, sinon relative |
| `checksums` | l'entrée `Checksum` d'`artifacts.json`, sinon le `*checksums.txt` envoyé, sinon le `checksums.txt` écrit par la publication |

### Types (`kind`)

| Type goreleaser | `kind` | Remarque |
| --- | --- | --- |
| `Archive` | `archive` | `format` : `tar.gz`, `zip`… |
| `Binary` avec `extra.Format` | `binary` | binaire publié sans archive |
| `Linux Package` | `package` | `format` : `deb`, `rpm`, `apk`, `archlinux` |
| `File` (`release.extra_files`) | `file` | tout autre fichier : configs, scripts SQL, PDF, charts Helm… |
| `Source` | `source` | archive des sources |
| `Checksum` | — | remplit `checksums` |
| `SBOM` | — | rattaché comme `sbom` à `<name>` s'il s'appelle `<name>.sbom.json`, `.spdx.json`, `.cdx.json` ou `.sbom` |
| `Signature` | — | rattachée comme `signature` à `<name>` (ou au fichier de checksums) si elle s'appelle `<name>.sigstore.json`, `.bundle` ou `.sig` |
| `Certificate` | — | rattaché comme `certificate` de la même façon, pour `<name>.pem`, `.crt` ou `.cert` |
| `Metadata` | — | `metadata.json` : ses valeurs deviennent des champs de la version |
| `Binary` sans `Format` | ignoré | sortie de build, comme partout ailleurs |

Une pièce jointe qui ne couvre aucun fichier listé reste téléchargeable, mais n'apparaît dans aucune entrée de `artifacts[]`. Si un fichier a deux pièces jointes du même type, la première dans `artifacts.json` l'emporte.

- **`format`** : `extra.Format` quand goreleaser le renseigne, sinon l'extension du fichier (`tar.gz`, `tar.xz`, `tar.zst` et `tar.bz2` comptent comme une seule extension), sinon `null`.
- **`mediaType`** : tiré d'une table fixe, d'après la fin du nom, sans tenir compte de la casse : `.tar.gz`/`.tgz`/`.gz` `application/gzip`, `.xz` `application/x-xz`, `.zst` `application/zstd`, `.tar` `application/x-tar`, `.zip` `application/zip`, `.deb` `application/vnd.debian.binary-package`, `.rpm` `application/x-rpm`, `.exe` `application/vnd.microsoft.portable-executable`, `.msi` `application/x-msi`, `.dmg` `application/x-apple-diskimage`, `.pdf` `application/pdf`, `.sql` `application/sql`, `.yaml`/`.yml` `application/yaml`, `.json` `application/json`, `.sh` `application/x-sh`, `.md` `text/markdown`, `.txt` `text/plain`. Tout le reste donne `application/octet-stream`.
- **`os`, `arch`, `variant`** : tirés de `goos`, de `goarch` et du premier champ renseigné parmi `goamd64`, `goarm`, `goarm64`, `go386`, `gomips`, `goppc64` et `goriscv64`. Ils valent `null` pour un fichier qui ne dépend pas d'une plateforme.

### Sans `artifacts.json`

Chaque fichier en staging reçoit un type d'après son nom, puis suit le tableau ci-dessus. `registry.yaml`, `CHANGELOG.md`, `metadata.json` et les fichiers écrits par le serveur ne sont pas des entrées.

| Nom | Type |
| --- | --- |
| le fichier de checksums | `Checksum` |
| `<autre fichier en staging>` + un suffixe de SBOM, signature ou certificat (ci-dessus) | `SBOM`, `Signature`, `Certificate` |
| `.tar.gz`, `.tgz`, `.tar.xz`, `.tar.zst`, `.tar.bz2`, `.tar`, `.zip` | `Archive` |
| `.deb`, `.rpm`, `.apk` | `Linux Package` |
| tout le reste | `File` |

`os`, `arch` et `variant` valent `null` : pour les donner, envoyez un `artifacts.json`, écrit à la main au besoin (`name`, `path`, `type`, `goos`, `goarch`).

**Produits qui ne sont pas des binaires.** Envoyez les fichiers avec `registry.yaml` (`upload.sh <product> DIR`), ou utilisez goreleaser avec `builds: [{ skip: true }]` et `release.extra_files`.

## 4. `index.json` : un produit et ses versions

```json
{
  "apiVersion": "local-registry/v1",
  "product": "sonarbridge",
  "displayName": "SonarBridge",
  "description": "Pont entre SonarQube et les outils internes.",
  "license": "Proprietary",
  "owners": [{ "name": "Équipe Plateforme", "email": "platform@corp.example" }],
  "links": [{ "name": "source", "url": "https://git.corp.example/platform/sonarbridge" }],
  "keywords": ["sonar", "cli"],
  "labels": { "domain": "qualité" },
  "install": "…",
  "latest": "v1.4.2",
  "updatedAt": "2026-10-10T18:05:02Z",
  "versions": [
    { "version": "v1.5.0-rc.1", "tag": "v1.5.0-rc.1", "publishedAt": "2026-10-12T08:00:00Z", "prerelease": true, "securityUpdates": false, "deprecated": false },
    { "version": "v1.4.2", "tag": "v1.4.2", "publishedAt": "2026-10-10T18:05:02Z", "prerelease": false, "securityUpdates": true, "deprecated": false }
  ]
}
```

- **Versions.** Seules les versions qui ont un `release.json` sont listées. Les versions publiées avant ce standard ne sont pas converties : elles restent téléchargeables par les routes existantes. Les versions semver viennent en premier, de la plus récente à la plus ancienne. Les autres noms suivent, du `publishedAt` le plus récent au plus ancien.
- **`latest`.** C'est la même règle que partout ailleurs : `publish?latest=true` copie le `release.json` de la version dans `<product>/release.json`, et la version latest est la `version` qu'il nomme, si elle a un `release.json`. Il vaut `null` sinon. Pour le changer à la main, copiez `<version>/release.json` dans `<product>/release.json`.
- **Fiche.** Elle vient du `registry.yaml` de la version `latest`, sinon de la dernière version publiée. Sans fiche, les champs texte valent `""` et les listes `[]`. `displayName` est celui du `release.json` de cette version.
- **`updatedAt`.** Le `publishedAt` le plus récent.
- **Fraîcheur.** Le fichier est réécrit de façon atomique après chaque publication du produit. `GET /{product}/index` ne le relit pas : il reconstruit l'index depuis le disque à chaque requête, si bien qu'un `latest` changé à la main se voit tout de suite.

## 5. Routes

Ces routes sont en lecture seule, sans token, et répondent aussi à `HEAD`. Les erreurs sont au format `{"error": "…"}`.

| Route | Réponse | Erreurs |
| --- | --- | --- |
| `GET /products` | `[{product, displayName, description, latest, updatedAt}]`, par nom ; `[]` si vide | — |
| `GET /{product}/index` | l'index (section 4) | `404` si aucune version n'a de `release.json` |
| `GET /{product}/releases/{version}` | `release.json` ; `latest` est accepté | `404` s'il n'existe pas |

`GET /{product}/releases/{version}` et `POST /releases/{product}/{version}` (les fichiers d'une version) sont deux routes distinctes. `POST /releases/{version}`, qui suppose le produit `gitlab`, est **dépréciée** : elle répond avec les en-têtes `Deprecation: true` et `Link: </releases/gitlab/{version}>; rel="successor-version"`, et sera retirée dans une prochaine version majeure.

## 6. Règles d'upload

| Règle | Réponse |
| --- | --- |
| Produit nommé `products` | `400 product name products is reserved` |
| Version nommée `artifacts.json`, `artifacts.goreleaser.json`, `release.json` ou `index.json` | `400 version name <nom> is reserved` |
| `PUT` de `release.json`, `index.json` ou `artifacts.goreleaser.json` | `400 <name> is reserved: publishing writes it` |
| `artifacts.json` liste l'un de ces noms | `422` |
| `registry.yaml` invalide | `422 registry.yaml: <raison>` |
| `CHANGELOG.md` de plus de 1 Mio, ou pas en UTF-8 | `422` |
| Publication sans `registry.yaml` | `422 registry.yaml not uploaded: every version needs its product card` |
| Publication sans artefact (aucun fichier hors `registry.yaml`, `CHANGELOG.md`, `metadata.json`, checksums, pièces jointes) | `422 no artifact to publish: upload at least one file besides registry.yaml` |
| Plusieurs `*checksums.txt` | `422 several checksums files: a and b` |

`artifacts.json`, `metadata.json` et le fichier de checksums sont facultatifs ; chacun, s'il est envoyé, est vérifié comme avant. Avec un fichier de checksums, chaque fichier en staging doit y figurer, sauf `registry.yaml`, `CHANGELOG.md`, `metadata.json`, les fichiers écrits par le serveur et les signatures (`*.sig`, `*.pem`, `*.sigstore.json`). Avec `artifacts.json`, chaque fichier en staging sauf `registry.yaml`, `CHANGELOG.md` et `metadata.json` doit en être une entrée. Sans fichier de checksums, la publication écrit `checksums.txt` au même format (`<sha256>  <nom>`, par nom) pour les fichiers qui auraient dû y figurer.

## 7. Versions du standard

`apiVersion` ne change (`local-registry/v2`) que pour une évolution incompatible : un champ retiré ou renommé, ou un type ou un sens modifié. Ajouter un champ, un `kind`, un type de média ou un type de commit garde `v1`. Les clients doivent donc ignorer les champs qu'ils ne connaissent pas.
