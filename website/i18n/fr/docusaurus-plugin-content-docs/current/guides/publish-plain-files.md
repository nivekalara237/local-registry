---
title: Publier des fichiers sans GoReleaser
sidebar_label: Publier des fichiers
description: Publier des migrations de base de données, des paquets de configuration ou tout ensemble de fichiers comme un produit versionné.
---

GoReleaser est facultatif. Ce guide publie un dossier de fichiers, par exemple des migrations de base de données, un paquet de configuration ou des documents, comme une version d'un produit. Le registre vérifie et sert ces fichiers exactement comme une release GoReleaser.

## Avant de commencer

- L'adresse du registre et son token d'upload. Si vous n'exploitez pas le registre vous-même, demandez-les à la personne ou à l'équipe qui en est responsable.
- [`localrectl`](../reference/localrectl.md) sur la machine qui publie.

## Étapes

L'exemple publie des changelogs Liquibase comme produit `billing-db-migrations`, en version `v1.0.0`.

1. **Préparez un dossier avec les fichiers à publier.** Le registre stocke des fichiers, pas des dossiers : un dossier à publier ne peut pas contenir de sous-dossier. Archivez d'abord chaque sous-dossier, ou laissez `localrectl` le faire avec `--compress-subdirs`.

   ```sh
   mkdir -p out
   tar czf out/billing-db-migrations_1.0.0.tar.gz -C migrations changelog-master.xml changes
   ```

   Nommez les fichiers comme vos utilisateurs les chercheront. Un nom ne peut utiliser que `A-Z a-z 0-9 . _ -`.

2. **Écrivez la fiche produit.** Gardez `registry.yaml` hors du dossier à publier, avec vos sources, et indiquez-le avec `--product-file`. Consultez [Écrire la fiche produit](write-registry-yaml.md).

   ```yaml
   apiVersion: local-registry/v1
   displayName: Billing database migrations
   description: Liquibase changelogs of the billing database.
   license: Proprietary
   owners:
     - name: Billing team
       email: billing@corp.example
   ```

3. **Ajoutez des notes de version (facultatif).** Un `CHANGELOG.md` dans le dossier devient les notes de la version. Consultez [Écrire les notes de version](write-changelog.md).

   ```markdown
   ## [1.0.0] - 2026-10-18
   ### Added
   - users, products and orders tables
   ```

4. **Vérifiez ce qui serait envoyé.**

   ```sh
   localrectl publish billing-db-migrations --root out --version v1.0.0 --no-semver-standard \
     --product-file migrations/registry.yaml --dry-run
   ```

   La simulation affiche ce qui serait envoyé, n'envoie rien et ne demande pas de token. `--version` est obligatoire hors build GoReleaser ; `--no-semver-standard` autorise le préfixe `v`.

5. **Publiez.**

   ```sh
   export LOCAL_REGISTRY_URL=https://registry.corp.example
   export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # depuis votre coffre de secrets

   localrectl publish billing-db-migrations --root out --version v1.0.0 --no-semver-standard \
     --product-file migrations/registry.yaml --latest
   ```

## Vérifier le résultat

```sh
curl -fsS https://registry.corp.example/billing-db-migrations/releases/v1.0.0 \
  | jq '{version, changes, checksums: .checksums.name, artifacts: [.artifacts[] | {name, kind, format}]}'
```

```json
{
  "version": "v1.0.0",
  "changes": [{ "kind": "added", "description": "users, products and orders tables" }],
  "checksums": "checksums.txt",
  "artifacts": [{ "name": "billing-db-migrations_1.0.0.tar.gz", "kind": "archive", "format": "tar.gz" }]
}
```

## Ce que le serveur fait pour vous

- **Il écrit `checksums.txt`** quand vous n'envoyez pas de fichier de checksums. Le fichier liste le sha256 de chaque fichier publié, au format de GoReleaser et de `sha256sum`. Chaque envoi qui porte un SHA-256 est aussi vérifié à la réception.
- **Il déduit le type de chaque fichier de son nom.** `.tar.gz`, `.tgz`, `.zip` et les autres archives donnent `archive` ; `.deb`, `.rpm` et `.apk` donnent `package` ; tout le reste donne `file`. Un fichier nommé comme un autre fichier suivi de `.sig`, `.pem` ou `.sbom.json` lui est rattaché comme signature, certificat ou SBOM.
- **`tag`, `commit` et `builtAt` valent `null`** dans `release.json`, faute de `metadata.json` GoReleaser.

Les règles complètes sont dans [Règles d'upload](../reference/upload-rules.md).

## Donner une plateforme à chaque fichier

Sans GoReleaser, `os` et `arch` valent `null` : le registre ne devine pas de plateforme à partir d'un nom de fichier. Si vos utilisateurs en ont besoin, par exemple pour choisir le bon binaire, écrivez un `artifacts.json` à la main, placez-le dans le dossier à publier, et publiez ce dossier : `localrectl` le traite alors comme un build GoReleaser.

```json
[
  { "name": "agent_1.0.0_linux_amd64.tar.gz", "path": "agent_1.0.0_linux_amd64.tar.gz", "type": "Archive", "goos": "linux", "goarch": "amd64" },
  { "name": "agent_1.0.0_windows_amd64.zip", "path": "agent_1.0.0_windows_amd64.zip", "type": "Archive", "goos": "windows", "goarch": "amd64" }
]
```

```sh
localrectl publish agent --root out --version v1.0.0 --no-semver-standard \
  --product-file registry.yaml --latest
```

- `name` est le nom publié, `path` l'emplacement du fichier. La façon dont `localrectl` résout `path` est décrite dans son [README](https://github.com/nivekalara237/localrectl/blob/master/README.md).
- `type` est l'un des types GoReleaser, par exemple `Archive`, `File`, `Linux Package`, `Checksum` ou `Signature`.
- Chaque fichier en staging doit être listé, sauf `registry.yaml`, `CHANGELOG.md` et `metadata.json`.

## En cas de problème

| Situation | Que faire |
| --- | --- |
| Le dossier contient un sous-dossier | Archivez-le avec `tar czf`, comme à l'étape 1, ou ajoutez `--compress-subdirs` (ou `--skip-subdirs` pour l'ignorer). |
| `--product-file` ou `--version` manque | `localrectl` refuse de publier : ajoutez l'option manquante. |
| `422 no artifact to publish` | Seuls la fiche, les notes ou les checksums ont été envoyés : ajoutez les fichiers eux-mêmes. |
| `release.json is reserved` | Renommez ou retirez ce fichier, ou excluez-le avec `--exclude` : c'est le serveur qui l'écrit. |

Plus de cas dans [Dépannage](../troubleshooting.md).
