---
title: Publier des fichiers sans goreleaser
sidebar_label: Publier des fichiers
description: Publier des migrations de base de données, des paquets de configuration ou tout ensemble de fichiers comme un produit versionné.
---

goreleaser est optionnel. Ce guide publie un dossier de fichiers, par exemple des migrations de base de données, un paquet de configuration ou des documents, comme une version d'un produit. Le registre vérifie et sert ces fichiers exactement comme une release goreleaser.

## Avant de commencer

- Un registre joignable et son token d'upload.
- `bash`, `curl`, `jq`, et `sha256sum` ou `shasum` sur la machine qui publie.
- `scripts/upload.sh`, pris dans une archive de release de local-registry.

## Étapes

L'exemple publie des changelogs Liquibase comme produit `billing-db-migrations`, en version `v1.0.0`.

1. **Préparez un dossier avec les fichiers à publier.** Le registre stocke des fichiers, pas des dossiers : un dossier à publier ne peut pas contenir de sous-dossier. Archivez d'abord chaque dossier.

   ```sh
   mkdir -p out
   tar czf out/billing-db-migrations_1.0.0.tar.gz -C migrations changelog-master.xml changes
   ```

   Nommez les fichiers comme vos utilisateurs les chercheront. Un nom ne peut utiliser que `A-Z a-z 0-9 . _ -`.

2. **Ajoutez la fiche produit.** Écrivez un `registry.yaml` dans le dossier, ou gardez-le ailleurs et passez `--product-file`. Voir [Écrire la fiche produit](write-registry-yaml.md).

   ```yaml
   apiVersion: local-registry/v1
   displayName: Billing database migrations
   description: Liquibase changelogs of the billing database.
   license: Proprietary
   owners:
     - name: Billing team
       email: billing@corp.example
   ```

3. **Ajoutez des notes de version (optionnel).** Un `CHANGELOG.md` dans le dossier devient les notes de la version. Voir [Écrire les notes de version](write-changelog.md).

   ```markdown
   ## [1.0.0] - 2026-10-18
   ### Added
   - users, products and orders tables
   ```

4. **Vérifiez ce qui serait envoyé.**

   ```sh
   ci/upload.sh --dry-run --version v1.0.0 --product-file migrations/registry.yaml billing-db-migrations out
   ```

   ```
   == billing-db-migrations v1.0.0 from /home/me/billing/out
   PUT CHANGELOG.md <- /home/me/billing/out/CHANGELOG.md (sha256 d58c…)
   PUT billing-db-migrations_1.0.0.tar.gz <- /home/me/billing/out/billing-db-migrations_1.0.0.tar.gz (sha256 613f…)
   PUT registry.yaml <- /home/me/billing/migrations/registry.yaml (sha256 56b5…)
   POST publish billing-db-migrations/v1.0.0 (latest=true)
   ```

   `--version` est obligatoire, sauf si le dossier contient un `metadata.json` avec un `tag`.

5. **Publiez.**

   ```sh
   export LOCAL_REGISTRY_URL=https://registry.example.com
   export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # depuis votre coffre de secrets
   ci/upload.sh --latest --version v1.0.0 --product-file migrations/registry.yaml billing-db-migrations out
   ```

## Vérifier le résultat

```sh
curl -fsS https://registry.example.com/billing-db-migrations/releases/v1.0.0 \
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

- **Il écrit `checksums.txt`** quand vous n'envoyez pas de fichier de checksums. Le fichier liste le sha256 de chaque fichier publié, au format de goreleaser et de `sha256sum`. Chaque envoi est aussi vérifié à la réception, avec le sha256 qu'envoie `upload.sh`.
- **Il déduit le type de chaque fichier de son nom.** `.tar.gz`, `.tgz`, `.zip` et les autres archives donnent `archive` ; `.deb`, `.rpm` et `.apk` donnent `package` ; tout le reste donne `file`. Un fichier nommé comme un autre fichier suivi de `.sig`, `.pem` ou `.sbom.json` lui est rattaché comme signature, certificat ou SBOM.
- **`tag`, `commit` et `builtAt` valent `null`** dans `release.json`, faute de `metadata.json` goreleaser.

Les règles complètes sont dans [Règles d'upload](../reference/upload-rules.md).

## Donner une plateforme à chaque fichier

Sans goreleaser, `os` et `arch` valent `null` : le registre ne devine pas de plateforme à partir d'un nom de fichier. Si vos utilisateurs en ont besoin, par exemple pour choisir le bon binaire, écrivez un `artifacts.json` à la main et publiez-le à la place du dossier.

```json
[
  { "name": "agent_1.0.0_linux_amd64.tar.gz", "path": "agent_1.0.0_linux_amd64.tar.gz", "type": "Archive", "goos": "linux", "goarch": "amd64" },
  { "name": "agent_1.0.0_windows_amd64.zip", "path": "agent_1.0.0_windows_amd64.zip", "type": "Archive", "goos": "windows", "goarch": "amd64" }
]
```

```sh
ci/upload.sh --latest --version v1.0.0 --product-file registry.yaml agent out/artifacts.json
```

- `name` est le nom publié, `path` l'endroit où `upload.sh` lit le fichier. Ici, `path` est relatif au dossier d'`artifacts.json`. Utilisez `--root` pour choisir un autre dossier de base.
- `type` est l'un des types goreleaser, par exemple `Archive`, `File`, `Linux Package`, `Checksum` ou `Signature`.
- Chaque fichier en staging doit être listé, sauf `registry.yaml`, `CHANGELOG.md` et `metadata.json`.

## En cas de problème

| Message | Que faire |
| --- | --- |
| `… is a directory: archive it first` | Archivez le dossier avec `tar czf`, comme à l'étape 1. |
| `no registry.yaml in … nor in the current directory` | Ajoutez `registry.yaml` au dossier, ou passez `--product-file`. |
| `no metadata.json in …: pass --version` | Passez `--version v1.0.0`. |
| `422 no artifact to publish` | Seuls la fiche, les notes ou les checksums ont été envoyés : ajoutez les fichiers eux-mêmes. |
| `release.json is reserved` | Renommez ou retirez ce fichier : c'est le serveur qui l'écrit. |

Plus de cas dans [Dépannage](../troubleshooting.md).
