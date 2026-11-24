---
title: Écrire les notes de version
sidebar_label: Écrire les notes de version
description: Donner à chaque version ses notes, à partir du changelog de GoReleaser ou d'un fichier Keep a Changelog.
---

Les notes de version sont fournies dans un fichier `CHANGELOG.md` transmis avec chaque publication. Le registre analyse ce fichier et convertit son contenu en une liste `changes` dans `release.json`. Il détecte également les versions contenant des correctifs de sécurité.

Ce guide présente les deux formats pris en charge : le changelog généré par GoReleaser et le format [Keep a Changelog](https://keepachangelog.com), qui permet de rédiger les notes manuellement.

Les notes de version sont facultatives. En l'absence de fichier `CHANGELOG.md`, le champ `changes` reste une liste vide.

## Quel format choisir ?

| Votre méthode de publication | Format à utiliser | Rédigé par |
| --- | --- | --- |
| Vous utilisez GoReleaser avec des [Conventional Commits](https://www.conventionalcommits.org/) | Le fichier `dist/CHANGELOG.md` généré par GoReleaser | GoReleaser, à partir des messages de commit |
| Vous utilisez un autre outil de build ou souhaitez rédiger vos propres notes | Keep a Changelog | Votre équipe, manuellement |

Les deux formats peuvent également cohabiter dans un même fichier.

## Avec GoReleaser

GoReleaser génère un fichier `dist/CHANGELOG.md` lors de chaque release. Transmettez-le au registre avec l'option `--changelog-file dist/CHANGELOG.md` de `localrectl publish`.

Pour obtenir des notes correctement interprétées :

1. **Utilisez des messages Conventional Commits.** Par exemple : `feat(api): CSV export` ou `fix(security): bump golang.org/x/net`.

2. **Conservez les préfixes des messages.** Dans votre configuration `.goreleaser.yaml`, configurez la génération du changelog pour récupérer les messages depuis Git ou GitHub, sans supprimer les préfixes `feat:` et `fix:` :

   ```yaml
   changelog:
     use: github # ou git
     sort: asc
     filters:
       exclude:
         - "^docs:"
         - "^test:"
         - "^chore:"
   ```

3. **Générez votre release avec GoReleaser.** Les messages de commit retenus deviennent des notes de version :

   | Message de commit | Note générée |
   | --- | --- |
   | `feat(api): CSV export` | `added` : CSV export |
   | `fix: projects beyond 500` | `fixed` : projects beyond 500 |
   | `fix(security): bump golang.org/x/net` | `security` : bump golang.org/x/net |
   | `perf: faster hashing` | `changed` : faster hashing |
   | `docs: typo` | Aucune note |

La correspondance complète entre les préfixes et les types de notes est décrite dans la [référence de CHANGELOG.md](../reference/changelog-format.md).

> [!WARNING]
> Si les règles `filters` ou les titres de groupes (`groups`) de GoReleaser suppriment les préfixes `feat:` et `fix:` des messages, le registre ne pourra plus identifier leur catégorie. Le champ `changes` risque alors de rester vide.

## Manuellement, avec Keep a Changelog

Vous pouvez rédiger vous-même un fichier `CHANGELOG.md` en utilisant le format Keep a Changelog.

Pour un produit dont les versions sont publiées successivement, regroupez les notes dans un même fichier, en plaçant la version la plus récente en premier. Utilisez les titres standard suivants pour classer les changements :

- `Added` : nouvelles fonctionnalités.
- `Changed` : modifications apportées aux fonctionnalités existantes.
- `Deprecated` : fonctionnalités dont l'utilisation est déconseillée.
- `Removed` : fonctionnalités supprimées.
- `Fixed` : corrections de bugs.
- `Security` : correctifs de sécurité.

Exemple :

```markdown
# Changelog

## [Unreleased]
### Added
- invoices table

## [1.1.0] - 2026-10-18
### Added
- orders table
### Security
- remove the default password

## [1.0.0] - 2026-09-01
### Added
- users and products tables
```

Lorsque le fichier contient plusieurs sections de version, le registre extrait **uniquement celle correspondant à la version publiée**.

Par exemple, la publication de `v1.1.0` produit deux notes :

- `added` : orders table
- `security` : remove the default password

Le titre de version peut être écrit avec ou sans préfixe `v` et crochets. Les trois formes suivantes sont reconnues comme correspondant à `v1.1.0` :

- `## [1.1.0]`
- `## 1.1.0`
- `## v1.1.0`

La section `Unreleased` n'est jamais publiée.

> [!IMPORTANT]
> Ajoutez la section correspondant à la version **avant de lancer la publication**. Si cette section est absente, la version sera publiée sans notes. Une fois publiée, une version ne peut plus être modifiée.

Si votre fichier ne contient aucun titre de version, le registre analyse son contenu dans son intégralité. Cette possibilité convient notamment aux petits fichiers `CHANGELOG.md` rédigés individuellement pour chaque publication.

## Transmettre les notes de version

Le fichier `CHANGELOG.md` doit être fourni avec les artefacts de la version :

- **Avec un dossier de fichiers :** placez `CHANGELOG.md` dans le dossier publié avec `localrectl publish --root`.
- **Avec GoReleaser :** passez le fichier `dist/CHANGELOG.md` généré par GoReleaser avec `--changelog-file dist/CHANGELOG.md`.

## Vérifier le résultat

Après la publication, consultez les informations de la version pour vérifier que les notes ont été correctement interprétées :

```sh
curl -fsS https://registry.corp.example/bridge/releases/v1.1.0 \
  | jq '{securityUpdates, changes}'
```

Exemple de réponse :

```json
{
  "securityUpdates": true,
  "changes": [
    { "kind": "added", "description": "orders table" },
    { "kind": "security", "description": "remove the default password" }
  ]
}
```

Le champ `securityUpdates` vaut `true` lorsqu'au moins une note est de type `security`. Cet indicateur est également présent pour chaque version dans le fichier `index.json` du produit. Il permet ainsi aux utilisateurs d'identifier rapidement les versions contenant des correctifs de sécurité.

## Limites

- Taille maximale : 1 Mio.
- Encodage accepté : UTF-8.
- Les fins de ligne Windows sont prises en charge.
- Le serveur vérifie la taille et l'encodage du fichier lors de son envoi. En cas d'erreur, il renvoie une réponse `422`.
- L'analyse et l'extraction des notes de version sont effectuées au moment de la publication.