---
title: Première release GoReleaser
description: Tutoriel — construire un projet Go avec GoReleaser et le publier sur le registre, avec sa fiche produit et ses notes de version.
sidebar_label: Première release GoReleaser
---

Ce tutoriel publie un outil en ligne de commande écrit en Go, `bridge`, construit par GoReleaser. À la fin, la version `v1.4.2` est sur le registre avec ses archives, ses checksums, ses notes de version et sa fiche produit, et c'est la dernière version du produit.

Suivez ce parcours si votre projet est écrit en Go et utilise déjà GoReleaser, ou peut l'adopter. Pour tout autre livrable, suivez [Publier des fichiers sans GoReleaser](./first-release-files.md).

## Prérequis

- L'adresse du registre et son token d'upload. Si vous n'exploitez pas le registre vous-même, demandez-les à la personne ou à l'équipe qui en est responsable. Pour vous entraîner sur votre machine, le [Démarrage rapide](./quick-start.md) lance un registre d'essai.
- GoReleaser 2.x et git.
- [`localrectl`](../reference/localrectl.md), le client en ligne de commande du registre.
- curl et jq, pour vérifier le résultat.

## 1. Configurer GoReleaser

Un `.goreleaser.yaml` minimal à la racine du projet :

```yaml
version: 2
project_name: bridge
builds:
  - main: ./cmd/bridge
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
checksum:
  name_template: "{{ .ProjectName }}_{{ .Version }}_checksums.txt"
changelog:
  use: git
```

Rien ici n'est propre au registre. Le serveur lit ce que GoReleaser écrit dans `dist/` : les archives, le fichier de checksums, `metadata.json`, `artifacts.json` et `CHANGELOG.md`.

## 2. Écrire la fiche produit

Créez `registry.yaml` à côté de `.goreleaser.yaml`. Ce fichier décrit le produit ; le serveur l'exige avec chaque version.

```yaml
apiVersion: local-registry/v1
displayName: Bridge
description: "Sonar Bridge : pont entre SonarQube et les outils internes."
license: Proprietary
owners:
  - name: Équipe plateforme
    email: platform@corp.example
links:
  - { name: source, url: "https://git.corp.example/platform/sonar-bridge" }
keywords: [sonar, cli]
```

Versionnez-le avec le code : chaque version emporte la fiche avec laquelle elle a été publiée. Tous les champs sont dans la [référence de registry.yaml](../reference/registry-yaml.md).

## 3. Construire la release

Taguez le commit et lancez GoReleaser. `--skip=publish` empêche GoReleaser de publier ailleurs ; le registre reçoit les fichiers à l'étape suivante.

```sh
git tag v1.4.2
goreleaser release --clean --skip=publish,announce
```

Rédigez vos messages de commit au format [Conventional Commits](https://www.conventionalcommits.org) (`feat: …`, `fix: …`). Le serveur transforme le `CHANGELOG.md` de GoReleaser en notes de version : `feat` devient *added*, `fix` devient *fixed*, `fix(security)` devient *security*. Consultez [Écrire les notes de version](../guides/write-changelog.md).

## 4. Publier

```sh
export LOCAL_REGISTRY_URL=https://registry.corp.example
export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # depuis votre coffre de secrets, jamais en ligne de commande

localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml \
  --changelog-file dist/CHANGELOG.md --latest
```

`localrectl` lit le build GoReleaser de `dist/`, envoie ses fichiers, la fiche produit et les notes de version, puis publie la version indiquée par `dist/metadata.json`, ici `v1.4.2`. `--latest` en fait la dernière version du produit.

> [!TIP]
> Lancez d'abord la commande avec `--dry-run` : elle affiche ce qui serait envoyé, n'envoie rien et n'a pas besoin du token.

Si le serveur refuse la publication, `localrectl` affiche l'erreur et sort avec le code `1`. Le serveur indique la cause, par exemple `bridge_1.4.2_linux_amd64.tar.gz: sha256 is … but … says …`. Les fichiers envoyés restent en staging : corrigez la cause et relancez la commande. Consultez [Dépannage](../troubleshooting.md).

## 5. Vérifier le résultat

```sh
curl -s https://registry.corp.example/bridge/index | jq '{latest, versions: [.versions[].version]}'
curl -s https://registry.corp.example/bridge/releases/latest | jq '.artifacts[] | {name, os, arch, sha256}'
```

L'index donne la fiche produit et ses versions. Le document de version liste chaque archive avec sa plateforme, sa taille, son sha256 et son URL de téléchargement :

```json
{ "name": "bridge_1.4.2_linux_amd64.tar.gz", "os": "linux", "arch": "amd64", "sha256": "3f1c…" }
```

Ces deux documents sont le moyen stable pour les clients de trouver et de vérifier les fichiers. Consultez [Consommer les versions](../guides/consume-releases.md).

## Pour aller plus loin

- L'automatiser : [Pipelines CI](../guides/ci-pipelines.md).
- Comprendre ce qui s'est passé côté serveur : [Staging et publication](../concepts/staging-and-publish.md) et [Dernière version](../concepts/latest.md).
- Toutes les options de publication : [référence de localrectl](../reference/localrectl.md).
