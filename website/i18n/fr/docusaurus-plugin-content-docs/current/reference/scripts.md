---
title: Scripts de publication
description: Options, modes, environnement et codes de sortie de scripts/upload.sh et scripts/publish-to-local-registry.sh.
sidebar_label: Scripts
---

Le dépôt fournit deux scripts shell qui déposent une version et la publient, pour que la CI n'ait pas besoin de client à elle. `upload.sh` est le script recommandé : il gère les sorties goreleaser de toute organisation, et les répertoires de fichiers simples. `publish-to-local-registry.sh` est un script POSIX `sh` plus petit pour les sorties goreleaser simples. Pour un pas à pas, voir [Publier avec goreleaser](../guides/publish-with-goreleaser.md) et [Publier des fichiers simples](../guides/publish-plain-files.md).

Les deux scripts lisent le même environnement :

| Variable | Description |
| --- | --- |
| `LOCAL_REGISTRY_URL` | Adresse du serveur, comme `https://registry.example.com`. Un `/` final est retiré (`upload.sh`). |
| `LOCAL_REGISTRY_UPLOAD_TOKEN` | Jeton de dépôt. Passé à curl sur son entrée standard, jamais sur une ligne de commande, jamais affiché. |

## upload.sh

```
upload.sh [--latest] [--dry-run] [--root DIR] [--version V] [--product-file F] <product> [artifacts.json | DIR]
```

Nécessite bash 3.2 ou plus, curl 7.76 ou plus (`--fail-with-body`) et jq ; pour un répertoire, `sha256sum` ou `shasum` aussi.

| Argument | Description |
| --- | --- |
| `--latest` | Publie avec `latest=true` : la version devient la dernière du produit. |
| `--dry-run` | Affiche ce qui serait envoyé et n'envoie rien. Ni URL ni jeton nécessaires. Les vérifications locales s'exécutent quand même. |
| `--root DIR` | Répertoire auquel les champs `path` d'`artifacts.json` sont relatifs : le répertoire de travail de goreleaser. Par défaut : déduit de l'emplacement d'`artifacts.json` (le répertoire de son entrée `Metadata` ou `Checksum` en est retiré), sinon le répertoire courant. |
| `--version V` | Version à publier. Par défaut : `tag`, sinon `version`, de `metadata.json` : celui que liste `artifacts.json`, ou `DIR/metadata.json` pour un répertoire. |
| `--product-file F` | Fiche produit, envoyée sous le nom `registry.yaml`. Par défaut : `registry.yaml` à la racine (ou dans `DIR`), sinon dans le répertoire courant. Obligatoire : le script s'arrête s'il n'en trouve pas. |
| `<product>` | Nom du produit. |
| `artifacts.json` ou `DIR` | Ce qu'il faut publier. Par défaut : `dist/artifacts.json` ; avec `--root`, `ROOT/dist/artifacts.json` s'il existe, sinon `ROOT` comme répertoire. Un répertoire qui contient un `artifacts.json` désigne ce fichier. |

Les options s'écrivent `--root DIR` ou `--root=DIR`. `-h` ou `--help` affiche l'usage.

### Avec artifacts.json (goreleaser)

1. Sélectionne les fichiers publiés d'`artifacts.json` avec la règle du serveur : types `Archive`, `File`, `Source`, `Linux Package`, `Checksum`, `Signature`, `Certificate`, `SBOM`, `Metadata`, et `Binary` avec `extra.Format`.
2. Avant tout envoi, vérifie que les noms sont uniques, qu'aucune entrée ne s'appelle `artifacts.json`, que chaque fichier existe sous la racine et qu'une fiche produit est trouvée.
3. Envoie chaque fichier sous son `name`, lu depuis son `path`, avec `X-Checksum-Sha256` quand son entrée a un checksum `sha256:`.
4. Envoie `registry.yaml`, puis le `CHANGELOG.md` placé à côté d'`artifacts.json` s'il existe, puis `artifacts.json` lui-même, en dernier.
5. Appelle la publication.

```sh
export LOCAL_REGISTRY_URL=https://registry.example.com
scripts/upload.sh --latest sonarbridge dist/artifacts.json
```

```
== sonarbridge v1.4.2 from dist/artifacts.json (root /builds/platform/sonarbridge)
PUT metadata.json <- /builds/platform/sonarbridge/dist/metadata.json
PUT sonarbridge_1.4.2_linux_amd64.tar.gz <- /builds/platform/sonarbridge/dist/sonarbridge_1.4.2_linux_amd64.tar.gz (sha256 3f1c…)
…
PUT registry.yaml <- /builds/platform/sonarbridge/registry.yaml
PUT CHANGELOG.md <- /builds/platform/sonarbridge/dist/CHANGELOG.md
PUT artifacts.json <- dist/artifacts.json
POST publish sonarbridge/v1.4.2 (latest=true)
{"product":"sonarbridge","version":"v1.4.2","files":[…],"latest":true}
```

### Avec un répertoire (sans goreleaser)

1. Liste chaque fichier ordinaire de `DIR`. Un sous-répertoire arrête le script : archivez-le d'abord. Un fichier nommé `release.json`, `index.json` ou `artifacts.goreleaser.json` l'arrête aussi : le serveur les écrit.
2. Envoie chaque fichier avec `X-Checksum-Sha256`, calculé localement.
3. Envoie la fiche produit sous le nom `registry.yaml` (un `registry.yaml` présent dans `DIR` est cette fiche, sauf si `--product-file` en désigne une autre).
4. Appelle la publication. Le serveur déduit le type de chaque fichier de son nom et écrit `checksums.txt`.

```sh
mkdir -p out
tar czf out/billing-db-migrations_1.0.0.tar.gz -C migrations changelog-master.xml changes
cp migrations/CHANGELOG.md out/
scripts/upload.sh --latest --version v1.0.0 --product-file migrations/registry.yaml billing-db-migrations out
```

### Erreurs et codes de sortie

| Code de sortie | Quand |
| --- | --- |
| `0` | Publié, ou simulation réussie. |
| `1` | Une vérification locale a échoué, une variable manque, ou le serveur a refusé une requête : son message d'erreur est affiché. |
| `2` | Erreur d'usage (option inconnue, mauvais nombre d'arguments). |

Messages, tous préfixés par `upload.sh: ` :

| Message | Correction |
| --- | --- |
| `jq is required`, `curl is required`, `sha256sum or shasum is required` | Installez l'outil. |
| `LOCAL_REGISTRY_URL is not set`, `LOCAL_REGISTRY_UPLOAD_TOKEN is not set` | Exportez la variable, ou utilisez `--dry-run`. |
| `<file> not found: run goreleaser first, or pass a directory of files` | Mauvais chemin, ou goreleaser n'a pas tourné. |
| `<file> is not a JSON array` | Ce n'est pas un `artifacts.json` de goreleaser. |
| `--root <dir> is not a directory` | Mauvais `--root`. |
| `no uploadable file in <file>` | `artifacts.json` ne liste que des sorties de build. |
| `several entries of <file> share a name: <names>` | Deux fichiers publiés de même nom : renommez-les dans `.goreleaser.yaml`. |
| `<file> lists a file named artifacts.json` | Renommez ce fichier. |
| `<name>: <path> not found (wrong --root?)` | Passez le répertoire de travail de goreleaser en `--root`. |
| `--product-file <file> not found` | Mauvais chemin. |
| `no registry.yaml in <dir> nor in the current directory: pass --product-file` | Écrivez un `registry.yaml` (voir [registry.yaml](./registry-yaml.md)) ou indiquez-le. |
| `<dir>/<sub> is a directory: archive it first, e.g. tar czf …` | Archivez le sous-répertoire. |
| `<path> is not a regular file` | Retirez le fichier spécial de `DIR`. |
| `<path>: <name> is reserved, the server writes it` | Retirez le fichier de `DIR`. |
| `no file to publish in <dir>` | `DIR` est vide. |
| `<file> lists no metadata.json: pass --version`, `no metadata.json in <dir>: pass --version`, `no tag nor version in <file>: pass --version` | Passez `--version`. |

Après un échec, corrigez la cause et relancez le script : les fichiers en staging sont remplacés.

> [!NOTE]
> Sous Windows (Git Bash, MSYS2, Cygwin), le `jq.exe` natif termine ses lignes par CRLF ; le script retire les retours chariot. Voir [Windows](../guides/windows.md).

## publish-to-local-registry.sh

```
publish-to-local-registry.sh [--latest] <product> [dist-dir]
```

Un script POSIX `sh` réservé aux sorties goreleaser, quand chaque fichier du fichier de checksums se trouve à la racine de `dist-dir` (par défaut `dist`) sous son nom publié, comme les archives. Sinon, utilisez `upload.sh`. `--latest` doit venir en premier.

1. Exige `dist-dir/metadata.json`, `dist-dir/artifacts.json`, exactement un `dist-dir/*checksums.txt`, et `registry.yaml` dans le répertoire courant.
2. Lit la version dans le `tag` de `metadata.json`.
3. Envoie chaque fichier listé dans le fichier de checksums, avec `X-Checksum-Sha256` ; puis le fichier de checksums ; les `*.sig`, `*.pem`, `*.sigstore.json` de `dist-dir` ; `metadata.json` ; `registry.yaml` ; `dist-dir/CHANGELOG.md` s'il existe ; `artifacts.json` en dernier.
4. Appelle la publication.

```sh
scripts/publish-to-local-registry.sh --latest sonarbridge dist
```

Il sort avec le code `1` et un message sur stderr quand un fichier manque (`<dist>/<file> not found: run goreleaser first, or publish a directory of files with upload.sh --version <v> <product> <dir>`, `registry.yaml not found in the current directory: the server requires it`, `no tag in <dist>/metadata.json`, `expected exactly one *checksums.txt in <dist>`) ou quand le serveur refuse une requête.
