---
title: Écrire la fiche produit
sidebar_label: Écrire registry.yaml
description: Écrire registry.yaml, la fiche produit que chaque version doit porter, et la vérifier avant de publier.
---

`registry.yaml` est la fiche de votre produit : son nom, ses responsables, ses liens et la façon de l'installer. Chaque version doit en porter une : le registre refuse de publier sans elle. Ce guide écrit une bonne fiche et repère les erreurs avant la publication.

## Où la placer

Gardez `registry.yaml` à la racine de votre dépôt et commitez-la avec le code. `upload.sh` la trouve là, ou prend le fichier passé avec `--product-file`. La fiche est envoyée avec chaque version : chaque version garde donc la fiche avec laquelle elle a été publiée. La page du produit (`index.json`) affiche la fiche de la dernière version.

## Un exemple complet

```yaml
# Obligatoire. La seule valeur acceptée.
apiVersion: local-registry/v1

# Le nom affiché. Jusqu'à 100 caractères.
displayName: SonarBridge

# Une ou deux phrases : ce que fait le produit, pour qui. Jusqu'à 500 caractères.
description: Bridge between SonarQube and internal tools.

# Un identifiant SPDX (MIT, Apache-2.0…) ou "Proprietary".
license: Proprietary

# Qui contacter. "email" est optionnel.
owners:
  - name: Platform team
    email: platform@corp.example

# Pages utiles. Les URL doivent commencer par http:// ou https://.
links:
  - { name: source, url: https://git.corp.example/platform/sonarbridge }
  - { name: docs, url: https://docs.corp.example/sonarbridge }

# Jusqu'à 20 mots pour chercher et filtrer.
keywords: [sonar, cli]

# Paires clé/valeur libres pour votre inventaire. Jusqu'à 50.
labels:
  domain: quality
  cost-center: "4512"

# Markdown, affiché tel quel. Jusqu'à 10 Kio.
install: |
  Download the archive of your platform and unpack it:

      curl -fsSL https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz | tar xz

# false, true, ou un message.
deprecated: false
```

Seul `apiVersion` est obligatoire. Chaque champ et chaque limite sont dans la [référence de registry.yaml](../reference/registry-yaml.md).

## La vérifier avant de publier

Le serveur vérifie la fiche à la réception et répond `422 registry.yaml: <raison>` si elle est incorrecte. Vérifiez-la plus tôt, dans vos merge requests, avec le schéma JSON fourni dans chaque release de local-registry (`schema/registry.schema.json`) :

```sh
pip install check-jsonschema
check-jsonschema --schemafile schema/registry.schema.json registry.yaml
```

## Erreurs fréquentes

| Erreur | Effet | Correction |
| --- | --- | --- |
| Fiche copiée d'un autre produit, `displayName` inchangé | Votre produit s'affiche sous le nom d'un autre | Relisez `displayName`, `description` et `links` quand vous copiez une fiche |
| Une faute dans une clé, comme `displayname` | `422 registry.yaml: …` : les clés inconnues sont refusées | Utilisez exactement les clés ci-dessus |
| Une section `changes:` | `422` : les notes de version n'ont pas leur place dans la fiche | Écrivez un `CHANGELOG.md` : voir [Écrire les notes de version](write-changelog.md) |
| `license: Prioritary` | Accepté, mais faux dans tous les inventaires | Utilisez un identifiant SPDX ou `Proprietary` |
| Un lien sans schéma, comme `git.corp.example/…` | `422` | Écrivez l'URL complète `https://…` |
| `install` qui pointe vers un autre produit ou une autre version | Les utilisateurs installent la mauvaise chose | Gardez les noms de produit et de fichiers de ce produit |
| Plusieurs documents YAML (`---`) | `422` | Un seul document par fichier |

> [!TIP]
> Les nombres et booléens de `labels` sont lus comme du texte : `cost-center: 4512` devient `"4512"`. Mettez-les quand même entre guillemets, pour que ce soit explicite.

## Déprécier un produit

Renseignez `deprecated` dans la fiche d'une nouvelle version, avec un message qui dit aux utilisateurs quoi faire :

```yaml
deprecated: replaced by sonarbridge2, see https://docs.corp.example/sonarbridge2
```

La valeur est enregistrée dans le `release.json` de cette version. Les versions déjà publiées gardent la fiche qu'elles avaient.
