---
title: Publier depuis la CI
sidebar_label: Publier depuis la CI
description: Publier chaque release taguée depuis GitHub Actions ou GitLab CI, avec le token gardé en secret masqué.
---

Ce guide automatise le processus de publication : chaque tag poussé sur votre dépôt déclenche la construction de la release, puis sa publication dans le registre.

Les exemples présentés utilisent GitHub Actions et GitLab CI. Quelle que soit la plateforme CI utilisée, le processus repose sur deux étapes : exécuter GoReleaser, puis lancer `localrectl publish`.

## Avant de commencer

1. **Stockez le token comme secret.** Nommez-le `LOCAL_REGISTRY_UPLOAD_TOKEN`. Configurez-le comme variable masquée et limitez son accès aux pipelines déclenchés par des tags ou exécutés dans un environnement protégé. Ne l'inscrivez jamais directement dans le fichier de configuration du pipeline et ne l'affichez jamais dans les journaux d'exécution.
2. **Configurez l'adresse du registre** dans une variable d'environnement nommée `LOCAL_REGISTRY_URL`, par exemple `https://registry.corp.example`.
3. **Rendez `localrectl` disponible dans le job.** Ajoutez à l'image de CI un binaire pris sur la [page des releases de localrectl](https://github.com/nivekalara237/localrectl/releases), en vérifiant son fichier de checksums, ou utilisez son image Docker. Consultez [localrectl](../reference/localrectl.md#installation).
4. **Versionnez `registry.yaml` dans votre dépôt.** Consultez le guide [Écrire la fiche produit](write-registry-yaml.md).

> [!TIP]
> Ne désignez comme dernière version que les versions stables. Les tags de préversion, tels que `v1.5.0-rc.1`, contiennent un tiret (`-`). Publiez-les sans l'option `--latest` afin que les utilisateurs qui ciblent `latest` continuent d'obtenir la dernière version stable.

## GitHub Actions

```yaml
# .github/workflows/release.yml
name: release

on:
  push:
    tags: ["v*"]

permissions:
  contents: read

jobs:
  publish:
    runs-on: ubuntu-latest
    environment: registry # contient le secret LOCAL_REGISTRY_UPLOAD_TOKEN
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0 # GoReleaser a besoin des tags pour générer le changelog

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: goreleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          version: "~> v2"
          args: release --clean --skip=publish

      # localrectl doit être installé sur le runner : voir « Avant de commencer ».
      - name: publish to the registry
        env:
          LOCAL_REGISTRY_URL: ${{ vars.LOCAL_REGISTRY_URL }}
          LOCAL_REGISTRY_UPLOAD_TOKEN: ${{ secrets.LOCAL_REGISTRY_UPLOAD_TOKEN }}
        run: |
          latest=
          case "$GITHUB_REF_NAME" in *-*) ;; *) latest=--latest ;; esac
          localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml \
            --changelog-file dist/CHANGELOG.md $latest
```

- `--skip=publish` empêche GoReleaser de publier la release sur GitHub. Retirez cette option si vous souhaitez également créer une release GitHub.
- `environment: registry` permet d'associer le job à un environnement GitHub afin de contrôler l'accès aux secrets et, si nécessaire, d'exiger une approbation avant son exécution. Épinglez les actions sur des SHA de commit si votre politique de sécurité l'exige.

## GitLab CI

```yaml
# .gitlab-ci.yml
release:
  stage: deploy
  image:
    name: goreleaser/goreleaser:v2.12.0 # localrectl doit être présent dans l'image : voir « Avant de commencer »
    entrypoint: [""]
  rules:
    - if: $CI_COMMIT_TAG =~ /^v/
  variables:
    GIT_DEPTH: 0 # GoReleaser a besoin des tags pour générer le changelog
  script:
    - goreleaser release --clean --skip=publish
    - |
      latest=
      case "$CI_COMMIT_TAG" in *-*) ;; *) latest=--latest ;; esac
      localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml \
        --changelog-file dist/CHANGELOG.md $latest
```

Définissez `LOCAL_REGISTRY_UPLOAD_TOKEN` comme variable masquée et protégée, puis protégez vos tags correspondant au motif `v*`. Ainsi, seuls les pipelines autorisés à utiliser cette variable pourront publier des releases.

Choisissez également le tag de l'image GoReleaser correspondant à la version utilisée par votre projet.

## Fichiers sans GoReleaser

Si votre produit est construit sans GoReleaser, remplacez l'étape GoReleaser par les commandes nécessaires à la génération de vos fichiers. Passez ensuite un dossier contenant les artefacts à publier ainsi que la version cible.

Consultez [Publier des fichiers sans GoReleaser](publish-plain-files.md).

```sh
tar czf out/billing-db-migrations_${CI_COMMIT_TAG#v}.tar.gz -C migrations changelog-master.xml changes

localrectl publish billing-db-migrations --root out --version "$CI_COMMIT_TAG" --no-semver-standard \
  --product-file migrations/registry.yaml --latest
```

## Valider avant de publier

Ajoutez les contrôles suivants à vos pipelines de merge requests afin de détecter les erreurs avant l'exécution d'une publication :

```sh
# Vérifie que la fiche produit respecte le schéma fourni avec Local Registry
check-jsonschema --schemafile schema/registry.schema.json registry.yaml

# Vérifie la configuration de la release et les fichiers à envoyer, sans publier ni utiliser de token
goreleaser release --snapshot --clean --skip=publish

localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml --dry-run
```

## Comportements à connaître

- **Un job en échec peut être relancé.** Une publication refusée laisse les fichiers en staging. Une nouvelle exécution remplace les fichiers précédemment envoyés. Consultez [Staging et publication](../concepts/staging-and-publish.md).
- **Une version publiée est immuable.** Relancer le job après une publication réussie échoue avec `already published`. Ajoutez `--if-exists skip` pour que ce cas réussisse sans rien envoyer, ou créez un nouveau tag.
- **Codes de sortie de `localrectl` :**
    - `0` : succès.
    - `1` : échec, par exemple un contrôle local ou un refus du serveur (le message d'erreur est affiché sur stderr).
    - `2` : mauvaise utilisation de la commande.

  Consultez [localrectl](../reference/localrectl.md) pour plus de détails.
- **HTTPS avec un certificat d'une autorité privée.** Indiquez à `localrectl` le certificat de cette autorité de certification avec la variable `LOCAL_REGISTRY_CA_FILE`, par exemple `LOCAL_REGISTRY_CA_FILE=/path/to/ca.pem`.
