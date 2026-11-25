---
title: Écrire la fiche produit
sidebar_label: Écrire registry.yaml
description: Écrire registry.yaml, la fiche produit que chaque version doit porter, et la vérifier avant de publier.
---

`registry.yaml` est la fiche descriptive de votre produit. Elle contient son nom, ses responsables, ses liens et ses instructions d'installation. Chaque version doit être accompagnée de cette fiche : le registre refuse toute publication qui n'en contient pas. Ce guide explique comment rédiger une fiche valide et détecter les erreurs avant la publication.

## Où placer le fichier

Vous pouvez créer `registry.yaml` dans le répertoire de votre choix et le versionner avec le code. Lors de la publication, indiquez son chemin à `localrectl` avec l'option `--product-file`.

La fiche est transmise avec chaque version publiée. Chaque version conserve ainsi les informations du produit telles qu'elles étaient au moment de sa publication. La page du produit (`index.json`) présente la fiche associée à la dernière version publiée.

## Un exemple complet

```yaml
# Obligatoire. La seule valeur acceptée.
apiVersion: local-registry/v1

# Le nom affiché. Jusqu'à 100 caractères.
displayName: Bridge

# Une ou deux phrases : ce que fait le produit, pour qui. Jusqu'à 500 caractères.
description: "Sonar Bridge : pont entre SonarQube et les outils internes."

# Un identifiant SPDX (MIT, Apache-2.0…) ou "Proprietary".
license: Proprietary

# Qui contacter. "email" est facultatif.
owners:
  - name: Équipe plateforme
    email: platform@corp.example

# Pages utiles. Les URL doivent commencer par http:// ou https://.
links:
  - { name: source, url: https://git.corp.example/platform/sonar-bridge }
  - { name: docs, url: https://docs.corp.example/bridge }

# Jusqu'à 20 mots-clés pour rechercher et filtrer le produit.
keywords: [sonar, cli]

# Paires clé/valeur libres pour votre inventaire. Jusqu'à 50.
labels:
  domain: quality
  cost-center: "4512"

# Markdown, affiché tel quel. Jusqu'à 10 Kio.
install: |
  Téléchargez l'archive de votre plateforme et décompressez-la :

      curl -fsSL https://registry.corp.example/bridge/artifacts/v1.4.2/files/bridge_1.4.2_linux_amd64.tar.gz | tar xz

# false, true, ou un message.
deprecated: false
```

Seul `apiVersion` est obligatoire. La description de chaque champ et les limites associées sont disponibles dans la [référence de registry.yaml](../reference/registry-yaml.md).

## Vérifier la fiche avant de publier

Le serveur valide automatiquement la fiche à sa réception et renvoie une erreur `422 registry.yaml: <raison>` si son contenu est incorrect.

Vous pouvez également effectuer cette vérification localement, avant de lancer la publication, à l'aide du schéma JSON fourni dans les archives de release de Local Registry (`schema/registry.schema.json`).

Installez d'abord l'outil `check-jsonschema` :

```sh
pip install check-jsonschema
```

Puis indiquez le chemin du fichier à vérifier et celui du schéma :

```sh
check-jsonschema --schemafile schema/registry.schema.json registry.yaml
```

Remplacez `registry.yaml` par le chemin de votre fichier s'il se trouve dans un autre répertoire.

Si vous avez accès au registre, `localrectl validate registry.yaml` vérifie aussi la fiche avec le schéma du serveur. Consultez [localrectl](../reference/localrectl.md).

## Erreurs fréquentes

| Erreur | Effet | Correction |
| --- | --- | --- |
| Fiche copiée d'un autre produit, `displayName` inchangé | Votre produit s'affiche sous le nom d'un autre | Vérifiez `displayName`, `description` et `links` lorsque vous réutilisez une fiche existante |
| Une faute dans une clé, comme `displayname` | `422 registry.yaml: …` : les clés inconnues sont refusées | Utilisez exactement les noms de clés documentés |
| Une section `changes:` | `422` : les notes de version n'ont pas leur place dans la fiche | Utilisez un fichier `CHANGELOG.md` : consultez [Écrire les notes de version](write-changelog.md) |
| `license: Prioritary` | Accepté, mais incorrect dans les inventaires | Utilisez un identifiant SPDX ou `Proprietary` |
| Un lien sans schéma, comme `git.corp.example/…` | `422` | Indiquez une URL complète commençant par `https://` |
| `install` qui pointe vers un autre produit ou une autre version | Les utilisateurs installent le mauvais produit ou la mauvaise version | Vérifiez les noms de produit, les versions et les URLs des fichiers |
| Plusieurs documents YAML (`---`) | `422` | Limitez le fichier à un seul document YAML |

> [!TIP]
> Les nombres et les booléens dans `labels` sont interprétés comme du texte : `cost-center: 4512` devient `"4512"`. Il est néanmoins recommandé de les entourer de guillemets pour rendre cette intention explicite.

## Déprécier un produit

Pour signaler qu'un produit est déprécié, renseignez le champ `deprecated` dans la fiche fournie avec une nouvelle version. Ajoutez un message indiquant aux utilisateurs quelle solution privilégier :

```yaml
deprecated: replaced by bridge2, see https://docs.corp.example/bridge2
```

Cette information est enregistrée dans le `release.json` de la version concernée. Les versions déjà publiées conservent la fiche produit qui leur était associée lors de leur publication.