---
title: Journal des modifications
sidebar_label: Changelog
description: Les changements de local-registry qui concernent les exploitants, les publieurs et les clients.
---

Cette page liste les changements de local-registry qui comptent pour ceux qui l'exploitent, y publient ou y lisent. Les versions suivent le [versionnage sémantique](https://semver.org). Avant `1.0.0`, les pré-releases peuvent encore changer de comportement.

## Non publié

### Ajouté

- **Le standard `local-registry/v1`.** Chaque version publiée a maintenant un `release.json` qui la décrit : produit, version, tag, commit, notes de version, et chaque fichier avec son type, son format, sa plateforme, sa taille, son sha256 et son URL de téléchargement. Chaque produit a un `index.json` avec sa fiche et toutes ses versions. Voir [Le standard local-registry/v1](concepts/standard-v1.md).
- **Routes de produit :** `GET /products`, `GET /{product}/index` et `GET /{product}/releases/{version}` (`latest` accepté). Voir [Routes de produit](reference/api/products.md).
- **La fiche produit, `registry.yaml`**, envoyée avec chaque version : nom affiché, description, responsables, liens, mots-clés, labels, instructions d'installation, dépréciation. Un schéma JSON est livré dans `schema/registry.schema.json`. Voir [Écrire registry.yaml](guides/write-registry-yaml.md).
- **Notes de version tirées de `CHANGELOG.md`** : le changelog de goreleaser (Conventional Commits) ou un fichier Keep a Changelog. Avec des titres de version, seule la section de la version publiée est lue. Voir [Écrire un changelog](guides/write-changelog.md).
- **Publication sans goreleaser.** `metadata.json`, `artifacts.json` et le fichier de checksums sont maintenant facultatifs. Sans fichier de checksums, la publication écrit `checksums.txt`. Sans `artifacts.json`, le type de chaque fichier est déduit de son nom. Voir [Publier des fichiers simples](guides/publish-plain-files.md).
- **`upload.sh` publie un répertoire** : `upload.sh --version v1.0.0 <product> <dir>` envoie chaque fichier du répertoire, avec son sha256, et la fiche produit.
- **Signatures** : les fichiers `*.sigstore.json` n'ont plus besoin d'une ligne dans le fichier de checksums.

### Modifié

- **`registry.yaml` est obligatoire** pour chaque publication. Sans lui, la publication répond `422 registry.yaml not uploaded`.
- **« Version group » devient « product »**, partout. Le champ `vcs` de la réponse de publication devient `product`, de même que le champ du journal d'audit. **Mettez à jour les clients qui lisent le champ `vcs`.** Les URL ne changent pas.
- **La dernière version est désignée par `<product>/release.json`**, une copie du `release.json` de la dernière version. `<product>/artifacts.json` est toujours écrit quand la dernière version en a un, et supprimé sinon. L'erreur `409 latest … is ambiguous` n'existe plus. Les versions publiées avant ce changement ne peuvent pas être la latest : publiez de nouveau. Voir [Latest](concepts/latest.md).
- **Noms réservés** : le produit `products`, les versions `release.json`, `index.json`, `artifacts.json` et `artifacts.goreleaser.json`, et les uploads nommés `release.json` ou `index.json` sont refusés en `400`.
- **`upload.sh` sous Windows** : les CR qu'ajoute un `jq.exe` natif sont retirés, ce qui corrige les `400 Bad Request` sous Git Bash, MSYS2 et Cygwin.
- **`upload.sh --root DIR` sans fichier** lit `DIR/dist/artifacts.json`, sinon publie `DIR` lui-même, au lieu du `dist/artifacts.json` du répertoire courant.

### Retiré

- Le flag `--require-product-file`, sa variable `LOCAL_REGISTRY_REQUIRE_PRODUCT_FILE` et sa clé `require-product-file` : la fiche produit est désormais toujours obligatoire. Retirez ce réglage de votre fichier de configuration, sinon le serveur refuse de démarrer (`unknown setting`).

### Déprécié

- `POST /releases/{version}` : il ne lit que le produit `gitlab`. Utilisez `POST /releases/{product}/{version}`.

## 1.0.0-rc.2

- Corrige le nom d'une variable d'environnement du pipeline de release. Aucun changement du serveur.

## 1.0.0-rc.1

Première pré-release.

- Routes de lecture : le catalogue, `artifacts.json` et `metadata.json` d'une version, les fichiers d'une version avec leurs URL de téléchargement, et le téléchargement des fichiers.
- Upload en deux temps : envoyer les fichiers, puis publier la version de façon atomique, avec contrôle des checksums, d'`artifacts.json` et de `metadata.json`.
- `artifacts.json` réécrit à la publication : chaque `path` devient l'URL de téléchargement du fichier.
- Token d'upload, quotas, purge du staging, journal d'audit, HTTPS, fichier de configuration et commande `config`.
- Archives de release pour Linux, macOS et Windows (amd64, arm64), et images conteneur sur GitHub Container Registry et Docker Hub.
