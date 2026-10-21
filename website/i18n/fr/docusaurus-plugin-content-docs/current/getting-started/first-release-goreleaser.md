---
title: Première release goreleaser
description: Tutoriel — construire un projet Go avec goreleaser et le publier sur local-registry, avec sa fiche produit et ses notes de version.
sidebar_label: Première release goreleaser
---

Ce tutoriel publie un outil en ligne de commande écrit en Go, `sonarbridge`, construit par goreleaser. À la fin, la version `v1.4.2` est sur le registre avec ses archives, ses checksums, ses notes de version et sa fiche produit, et c'est la dernière version du produit.

Suivez ce chemin si votre projet est en Go et utilise déjà goreleaser, ou peut l'utiliser. Pour tout autre livrable, suivez [Publier des fichiers sans goreleaser](./first-release-files.md).

## Prérequis

- Un registre accessible et son token d'upload. En local, le [Démarrage rapide](./quick-start.md) vous en fournit un.
- goreleaser 2.x, git, bash, curl 7.76+ et jq.
- `scripts/upload.sh` du dépôt local-registry. Copiez-le dans votre projet ou appelez-le par son chemin.

## 1. Configurer goreleaser

Un `.goreleaser.yaml` minimal à la racine du projet :

```yaml
version: 2
project_name: sonarbridge
builds:
  - main: ./cmd/sonarbridge
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

Rien ici n'est propre à local-registry. Le serveur lit ce que goreleaser écrit dans `dist/` : les archives, le fichier de checksums, `metadata.json`, `artifacts.json` et `CHANGELOG.md`.

## 2. Écrire la fiche produit

Créez `registry.yaml` à côté de `.goreleaser.yaml`. Il décrit le produit, et le serveur l'exige avec chaque version.

```yaml
apiVersion: local-registry/v1
displayName: SonarBridge
description: Pont entre SonarQube et les outils internes.
license: Proprietary
owners:
  - name: Équipe plateforme
    email: platform@corp.example
links:
  - { name: source, url: "https://git.corp.example/platform/sonarbridge" }
keywords: [sonar, cli]
```

Versionnez-le avec le code : chaque version emporte la fiche avec laquelle elle a été publiée. Tous les champs sont dans la [référence de registry.yaml](../reference/registry-yaml.md).

## 3. Construire la release

Taguez le commit et lancez goreleaser. `--skip=publish` empêche goreleaser de pousser ailleurs ; local-registry reçoit les fichiers à l'étape suivante.

```sh
git tag v1.4.2
goreleaser release --clean --skip=publish,announce
```

> [!NOTE]
> goreleaser a besoin d'un remote git, même avec `--skip=publish`. Sans remote, il s'arrête sur `couldn't get remote URL`.

Rédigez vos messages de commit au format [Conventional Commits](https://www.conventionalcommits.org) (`feat: …`, `fix: …`). Le serveur transforme le `CHANGELOG.md` de goreleaser en notes de version : `feat` devient *added*, `fix` devient *fixed*, `fix(security)` devient *security*. Voir [Écrire les notes de version](../guides/write-changelog.md).

## 4. Publier

```sh
export LOCAL_REGISTRY_URL=https://registry.example.com
export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # depuis votre coffre de secrets, jamais en ligne de commande

scripts/upload.sh --latest sonarbridge dist/artifacts.json
```

`upload.sh` lit `dist/artifacts.json` et, dans l'ordre :

1. Vérifie en local que chaque fichier listé existe et qu'un `registry.yaml` est trouvé. Rien n'est envoyé si un contrôle échoue.
2. Envoie chaque fichier publié (archives, fichier de checksums, `metadata.json`), avec son sha256 quand goreleaser l'a noté, pour que le serveur le vérifie à l'arrivée.
3. Envoie `registry.yaml`, `dist/CHANGELOG.md`, puis `artifacts.json` en dernier.
4. Publie `v1.4.2` (le `tag` de `metadata.json`) avec `latest=true`.

> [!TIP]
> Lancez-le d'abord avec `--dry-run`. Il affiche chaque fichier qu'il enverrait, n'envoie rien et n'a pas besoin du token.

Si le serveur refuse la publication, le script affiche pourquoi, par exemple `sonarbridge_1.4.2_linux_amd64.tar.gz: sha256 is … but … says …`. Les fichiers envoyés restent en staging : corrigez la cause et relancez le script. Voir [Dépannage](../troubleshooting.md).

## 5. Vérifier le résultat

```sh
curl -s https://registry.example.com/sonarbridge/index | jq '{latest, versions: [.versions[].version]}'
curl -s https://registry.example.com/sonarbridge/releases/latest | jq '.artifacts[] | {name, os, arch, sha256}'
```

L'index donne la fiche produit et ses versions. Le document de version liste chaque archive avec sa plateforme, sa taille, son sha256 et son URL de téléchargement :

```json
{ "name": "sonarbridge_1.4.2_linux_amd64.tar.gz", "os": "linux", "arch": "amd64", "sha256": "3f1c…" }
```

Ces deux documents sont le moyen stable pour les clients de trouver et de vérifier les fichiers. Voir [Consommer les versions](../guides/consume-releases.md).

## Pour aller plus loin

- L'automatiser : [Pipelines CI](../guides/ci-pipelines.md).
- Comprendre ce qui s'est passé côté serveur : [Staging et publication](../concepts/staging-and-publish.md) et [Dernière version](../concepts/latest.md).
- Toutes les options du script d'upload : [Référence des scripts](../reference/scripts.md).
