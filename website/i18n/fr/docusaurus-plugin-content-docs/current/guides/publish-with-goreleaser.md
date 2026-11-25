---
title: Publier une release GoReleaser
sidebar_label: Publier avec GoReleaser
description: Envoyer la sortie d'une release GoReleaser au registre et la publier comme une version.
---

Ce guide publie la sortie de `goreleaser release` comme une version d'un produit. Utilisez-le quand votre projet est déjà construit avec GoReleaser : [`localrectl`](../reference/localrectl.md) lit le build GoReleaser de `dist/`, quelle que soit l'organisation de vos archives.

![GoReleaser construit dist/, localrectl envoie chaque fichier en staging, puis publie la version](/img/goreleaser-release-flow.svg)

## Avant de commencer

- L'adresse du registre, par exemple `https://registry.corp.example`, et son token d'upload. Si vous n'exploitez pas le registre vous-même, demandez-les à la personne ou à l'équipe qui en est responsable.
- `localrectl` sur la machine qui publie. Consultez [localrectl](../reference/localrectl.md#installation).
- Une fiche produit `registry.yaml` dans votre projet. Le serveur refuse une version sans elle. Consultez [Écrire la fiche produit](write-registry-yaml.md).

> [!IMPORTANT]
> Publiez **après** la fin de `goreleaser release`. GoReleaser écrit `dist/artifacts.json` après ses propres `publishers:` : un publisher GoReleaser ne peut donc pas l'envoyer.

## Étapes

1. **Construisez la release.** Depuis la racine du projet, sur un tag :

   ```sh
   git tag v1.4.2
   goreleaser release --clean
   ```

   Si GoReleaser ne doit pas créer de release GitHub ou GitLab, ajoutez `--skip=publish`. `dist/artifacts.json`, `dist/metadata.json` et `dist/CHANGELOG.md` sont quand même écrits.

2. **Vérifiez ce qui serait envoyé.** Une simulation affiche ce qui serait envoyé et n'envoie rien. Elle ne demande pas de token.

   ```sh
   localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml \
     --changelog-file dist/CHANGELOG.md --dry-run
   ```

   La version vient du `metadata.json` de GoReleaser. Passez `--version` pour en choisir une autre.

3. **Publiez.** Définissez l'adresse du registre et le token, puis relancez la commande sans `--dry-run`. Ajoutez `--latest` pour faire de cette version la dernière version du produit.

   ```sh
   export LOCAL_REGISTRY_URL=https://registry.corp.example
   export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # depuis votre coffre de secrets, jamais en ligne de commande

   localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml \
     --changelog-file dist/CHANGELOG.md --latest
   ```

   `localrectl` lit le token dans `LOCAL_REGISTRY_UPLOAD_TOKEN` et ne l'affiche jamais. Avec `-o json`, il affiche le résultat sous la forme `{"total", "total_size", "status", "version", "product"}`, où `status` vaut `PUBLISHED`.

## Vérifier le résultat

```sh
curl -fsS https://registry.corp.example/bridge/releases/v1.4.2 | jq '{version, tag, changes, files: [.artifacts[].name]}'
curl -fsS https://registry.corp.example/bridge/index | jq '{latest, versions: [.versions[].version]}'
```

`release.json` liste chaque fichier avec sa plateforme, sa taille et son SHA-256. Si vous avez utilisé `--latest`, le champ `latest` de l'index vaut maintenant `v1.4.2`.

## Ce qui est publié

- Chaque fichier que GoReleaser marque comme faisant partie de la release : archives, paquets, checksums, signatures, certificats, SBOM, `metadata.json`, et binaires publiés tels quels (`formats: [binary]`).
- Les sorties de build brutes (entrées `Binary` sans format) ne sont pas publiées : ce sont les fichiers à partir desquels vos archives sont construites.
- La fiche produit `registry.yaml`, les notes de version `CHANGELOG.md` et `artifacts.json`. Le serveur conserve l'`artifacts.json` envoyé sous le nom `artifacts.goreleaser.json` et en écrit une version dont chaque `path` est l'URL de téléchargement du fichier.

Les règles complètes sont dans [Règles d'upload](../reference/upload-rules.md).

## En cas de problème

- **`localrectl` s'arrête avant d'envoyer quoi que ce soit.** Une vérification locale a échoué. Corrigez la cause indiquée et relancez.
- **Le serveur refuse la publication (`422`).** Rien n'est publié et les fichiers envoyés restent en staging. Corrigez la cause et relancez la commande : les fichiers en staging sont remplacés. Consultez [Staging et publication](../concepts/staging-and-publish.md).
- **`already published`.** Une version est immuable. Publiez une nouvelle version, ou ajoutez `--if-exists skip` si cette version a déjà été publiée par une exécution précédente.
- **`metadata.json describes …, not …`.** La version demandée n'est pas celle du `metadata.json` de GoReleaser. Retirez `--version`, ou passez le tag.

Chaque message est expliqué dans [Dépannage](../troubleshooting.md) et [Erreurs](../reference/errors.md).

## Et ensuite

- Automatiser : [Publier depuis la CI](ci-pipelines.md).
- Changer la version désignée comme dernière : [Changer la dernière version](change-latest.md).
