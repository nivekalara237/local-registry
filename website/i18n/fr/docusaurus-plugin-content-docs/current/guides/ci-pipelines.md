---
title: Publier depuis la CI
sidebar_label: Publier depuis la CI
description: Publier chaque release taguée depuis GitHub Actions ou GitLab CI, avec le token gardé en secret masqué.
---

Ce guide automatise la publication : chaque tag poussé sur votre dépôt construit la release et la publie dans le registre. Les exemples utilisent GitHub Actions et GitLab CI. Les deux mêmes étapes valent pour toute CI : lancer goreleaser, puis lancer `upload.sh`.

## Avant de commencer

1. **Stockez le token comme secret.** Nommez-le `LOCAL_REGISTRY_UPLOAD_TOKEN`, masqué, et disponible seulement pour les pipelines de tag ou un environnement protégé. Ne l'écrivez jamais dans le fichier de pipeline, et ne l'affichez jamais.
2. **Stockez l'adresse du registre** dans une variable simple `LOCAL_REGISTRY_URL`, par exemple `https://registry.example.com`.
3. **Commitez `upload.sh`** dans votre dépôt, par exemple en `ci/upload.sh`, pris dans une archive de release de local-registry. Une copie figée rend chaque build reproductible.
4. **Commitez `registry.yaml`** à la racine de votre dépôt. Voir [Écrire la fiche produit](write-registry-yaml.md).

> [!TIP]
> Ne marquez comme dernière version que les versions stables. Un tag de pré-version comme `v1.5.0-rc.1` contient un `-` : publiez-le sans `--latest`, pour que les utilisateurs de `latest` gardent la version stable.

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
          fetch-depth: 0 # goreleaser a besoin des tags pour son changelog

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: goreleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          version: "~> v2"
          args: release --clean --skip=publish

      - name: publish to the registry
        env:
          LOCAL_REGISTRY_URL: ${{ vars.LOCAL_REGISTRY_URL }}
          LOCAL_REGISTRY_UPLOAD_TOKEN: ${{ secrets.LOCAL_REGISTRY_UPLOAD_TOKEN }}
        run: |
          latest=
          case "$GITHUB_REF_NAME" in *-*) ;; *) latest=--latest ;; esac
          bash ci/upload.sh $latest sonarbridge dist/artifacts.json
```

- `--skip=publish` empêche goreleaser de créer une release GitHub. Retirez-le si vous en voulez une aussi.
- `environment: registry` permet de réserver le secret aux tags et d'exiger une approbation. Figez les actions sur un sha de commit si votre politique l'exige.

## GitLab CI

```yaml
# .gitlab-ci.yml
release:
  stage: deploy
  image:
    name: goreleaser/goreleaser:v2.12.0
    entrypoint: [""]
  rules:
    - if: $CI_COMMIT_TAG =~ /^v/
  variables:
    GIT_DEPTH: 0 # goreleaser a besoin des tags pour son changelog
  before_script:
    - apk add --no-cache bash curl jq
  script:
    - goreleaser release --clean --skip=publish
    - |
      latest=
      case "$CI_COMMIT_TAG" in *-*) ;; *) latest=--latest ;; esac
      bash ci/upload.sh $latest sonarbridge dist/artifacts.json
```

Définissez `LOCAL_REGISTRY_UPLOAD_TOKEN` comme variable masquée et protégée, et protégez vos tags `v*` : seuls les tags protégés peuvent alors publier. Choisissez le tag d'image goreleaser qu'utilise votre projet.

## Fichiers sans goreleaser

Pour un produit construit sans goreleaser, remplacez l'étape goreleaser par les commandes qui produisent vos fichiers, et passez un dossier et une version. Voir [Publier des fichiers sans goreleaser](publish-plain-files.md).

```sh
tar czf out/billing-db-migrations_${CI_COMMIT_TAG#v}.tar.gz -C migrations changelog-master.xml changes
bash ci/upload.sh --latest --version "$CI_COMMIT_TAG" --product-file migrations/registry.yaml billing-db-migrations out
```

## Valider avant de publier

Ajoutez ces contrôles aux merge requests, pour qu'une release n'échoue jamais à la dernière étape :

```sh
# La fiche produit respecte le schéma fourni avec local-registry
check-jsonschema --schemafile schema/registry.schema.json registry.yaml

# La release se lit et chaque fichier existe ; n'envoie rien, sans token
goreleaser release --snapshot --clean --skip=publish
bash ci/upload.sh --dry-run --version v0.0.0-dryrun sonarbridge dist/artifacts.json
```

## Comportements sur lesquels compter

- **Un job en échec peut être relancé.** Une publication refusée laisse les fichiers en staging, et l'exécution suivante les remplace. Voir [Staging et publication](../concepts/staging-and-publish.md).
- **Une version est immuable.** Relancer le job après une publication réussie répond `409 … already published`. Traitez-le comme « déjà fait », ou publiez un nouveau tag.
- **Codes de sortie** d'`upload.sh` : `0` publié, `1` un contrôle local a échoué, une variable manque ou le serveur a refusé (son message est affiché), `2` mauvaise utilisation. Voir [Scripts](../reference/scripts.md).
- **HTTPS auto-signé.** Indiquez votre autorité de certification à curl avec `CURL_CA_BUNDLE=/path/to/ca.pem`.
