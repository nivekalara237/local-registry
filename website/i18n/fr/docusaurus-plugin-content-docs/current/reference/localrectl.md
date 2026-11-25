---
title: localrectl
description: Le client en ligne de commande de Local Registry. Réglages, commandes, publication, téléchargement et codes de sortie.
sidebar_label: localrectl
---

`localrectl` est le client en ligne de commande du registre. Il lit les produits, les versions et les releases, télécharge les fichiers en vérifiant leur SHA-256, valide les fiches produit et publie de nouvelles versions. La lecture ne demande aucun token ; la publication demande le token d'upload.

Cette page résume ce qu'utilisent les guides. Toutes les options et tous les exemples sont dans le [README de localrectl](https://github.com/nivekalara237/localrectl/blob/master/README.md).

## Installation

- **Binaires** pour Linux, macOS et Windows, accompagnés d'un fichier de checksums, sur la [page des releases](https://github.com/nivekalara237/localrectl/releases).
- **Image Docker** [`nivekalara237/localrectl`](https://hub.docker.com/r/nivekalara237/localrectl) :

  ```sh
  docker run --rm -e LOCAL_REGISTRY_URL=https://registry.corp.example nivekalara237/localrectl ping
  ```

## Utilisation

```
localrectl <commande> [arguments] [options]
```

Chaque commande accepte `--help`. Le résultat est écrit sur stdout ; les barres de progression, les journaux et les erreurs sur stderr. Ajoutez `-o json` pour obtenir un résultat exploitable par un script.

## Configuration

Chaque réglage provient, par ordre de priorité, d'une option, d'une variable d'environnement, du fichier de configuration, puis de la valeur par défaut. Le fichier de configuration est celui indiqué par `--config`, sinon `config`, `config.yaml` ou `config.yml`, recherché dans `/etc/localrectl/`, `~/.lr/`, puis dans le dossier courant.

| Option | Variable d'environnement | Clé du fichier | Description |
| --- | --- | --- | --- |
| `--server-url` | `LOCAL_REGISTRY_URL` | `server.url` | Adresse du registre. Par défaut : `http://localhost:8077`. |
| — | `LOCAL_REGISTRY_UPLOAD_TOKEN` | `server.upload-token` | Token d'upload. Nécessaire uniquement pour `publish`. |
| `--server-ca-file` | `LOCAL_REGISTRY_CA_FILE` | `server.ca-file` | Certificat de l'autorité de certification d'un registre dont le certificat HTTPS vient d'une autorité privée. |
| `-o, --output text\|json` | `LOCAL_REGISTRY_OUTPUT` | — | Format de sortie. |

Autres options : `-v, --verbose`, `-q, --quiet`, `--no-color`, `--timeout` (en secondes, 300 par défaut), `--retries` (3 par défaut), `--exit-code` (1 par défaut).

`localrectl config` affiche les réglages utilisés et la provenance de chacun.

> [!WARNING]
> Ne passez jamais le token d'upload en ligne de commande : définissez `LOCAL_REGISTRY_UPLOAD_TOKEN`. `localrectl` ne l'affiche jamais.

## Commandes

| Commande | Rôle |
| --- | --- |
| `ping` | Vérifie que le registre répond (`GET /healthz`). |
| `products` | Liste tous les produits, toutes pages lues. |
| `products search` | Recherche des produits, une page à la fois : `-Q, --query`, `-k, --keyword` (répétable), `-l, --label clé=valeur` (répétable), `--owner`, `--license`, `--deprecated[=false]`, `--prerelease[=false]`, `--updated-since DATE`, `--sort`, `--limit`, `-c, --cursor`, `--all`. |
| `versions <produit>` | Liste les versions d'un produit. `--prerelease` ajoute les préversions. |
| `release <produit> <version\|latest>` | Affiche le `release.json` d'une version : ses artefacts et son fichier de checksums. |
| `validate <registry.yaml>` | Vérifie une fiche produit avec le schéma du registre. `--artifacts dist/artifacts.json` vérifie aussi un `artifacts.json` de GoReleaser. |
| `download <produit> [version]` | Télécharge des fichiers d'une version (`latest` par défaut) et vérifie le SHA-256 de chacun. Consultez [download](#download). |
| `publish <produit>` | Envoie les fichiers d'une nouvelle version, puis la publie. Consultez [publish](#publish). |
| `config`, `version`, `completion bash\|zsh\|fish\|powershell` | Réglages utilisés, version du client, complétion du shell. |

## publish

Obligatoire : `--product-file registry.yaml`, et `--version X.Y.Z`, sauf pour un build GoReleaser dont le `metadata.json` fournit la version. Le token d'upload est nécessaire, sauf avec `--dry-run`.

Provenance des fichiers :

- **Un build GoReleaser :** `--goreleaser-dir dist` ; ou `--goreleaser-artifacts-file` et `--goreleaser-metadata-file` ; ou un dossier racine contenant un `artifacts.json`.
- **Un dossier :** `-R, --root DIR` (par défaut : le dossier courant). Chaque fichier ordinaire du dossier est envoyé ; les fichiers cachés ne le sont jamais. Un sous-dossier fait échouer la commande, sauf avec `--skip-subdirs` ou `--compress-subdirs` (`--compress-type tar.gz|zip`).

| Option | Description |
| --- | --- |
| `--changelog-file F` | Envoie `F` sous le nom `CHANGELOG.md`. |
| `--exclude NOM` (répétable), `--exclude-match 'MOTIF'` | Fichiers ou sous-dossiers du dossier racine à ne pas envoyer. Interdit avec un build GoReleaser. |
| `--latest` | Fait de la version la dernière version. |
| `--if-exists fail\|skip` | Si la version est déjà publiée : échouer (par défaut), ou réussir sans rien envoyer. |
| `--no-semver-standard` | Accepte une version comme `v1.2.0`. |
| `--no-validate` | Ignore les vérifications locales. |
| `--dry-run` | Affiche ce qui serait envoyé, sans rien envoyer. |

```sh
localrectl publish billing-agent --root ./out --version 1.2.0 --product-file registry.yaml --dry-run
localrectl publish billing-agent --goreleaser-dir dist --product-file registry.yaml --latest --if-exists skip
```

Une version publiée ne change jamais : la publier de nouveau échoue avec `already published`, sauf avec `--if-exists skip`.

## download

Fichiers sélectionnés, d'après le `release.json` de la version :

| Sélection | Fichiers |
| --- | --- |
| `--file NOM` (répétable) | Ces fichiers, par leur nom. |
| `--all` | Tous les fichiers, fichier de checksums compris. |
| `--os linux\|darwin\|macos\|windows`, `--arch amd64\|arm64\|…`, `--kind archive\|binary\|package\|file\|source` | Tous les fichiers correspondants. |
| aucune | L'archive de la machine : zip sous Windows, tar.gz ailleurs. |

`--all`, `--file` et les filtres ne se combinent pas.

| Option | Description |
| --- | --- |
| `-d, --dir DIR` | Dossier de destination, créé s'il manque. Par défaut : le dossier courant. |
| `--force` | Remplace un fichier ou une entrée extraite qui diffère. Sans cette option, un fichier existant différent fait échouer la commande ; un fichier de même SHA-256 est ignoré (`up-to-date`). |
| `--extract` | Extrait les archives tar.gz ou zip téléchargées dans `--dir` et conserve les archives. Les liens et les entrées qui sortent de `--dir` sont refusés. |
| `--no-verify` | Ignore la vérification du SHA-256. |
| `--dry-run` | Affiche ce qui serait téléchargé, sans rien écrire. |
| `--verify-signature` | Vérifie la signature Sigstore du fichier de checksums de la release avec `cosign` (qui doit être dans le `PATH`), puis vérifie chaque fichier avec ce fichier signé. Nécessite `--key CLE_PUBLIQUE`, ou `--certificate-identity ID` (ou `--certificate-identity-regexp RE`) avec `--certificate-oidc-issuer URL`. La signature doit figurer dans le journal de transparence Rekor. |

```sh
localrectl download billing-agent --extract -d /opt/billing-agent
localrectl download billing-agent v1.2.0 --os linux --arch arm64 -o json
localrectl download billing-agent --verify-signature --key cosign.pub
```

## Codes de sortie

| Code | Signification |
| --- | --- |
| `0` | Succès. |
| `1` | Échec, ou la valeur de `--exit-code`. |
| `2` | Mauvaise utilisation de la commande : option inconnue, argument manquant, réglage incorrect. |
