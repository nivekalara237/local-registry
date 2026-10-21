---
title: registry.yaml
description: Référence normative de registry.yaml, la fiche produit déposée avec chaque version.
sidebar_label: registry.yaml
---

`registry.yaml` est la fiche produit : qui possède le produit, ce qu'il fait, où sont ses sources, comment l'installer, et si cette version est dépréciée. C'est le seul fichier du [standard local-registry/v1](../concepts/standard-v1.md) écrit à la main, et il est **obligatoire** : chaque version est publiée avec sa propre copie. Pour un guide commenté, voir [Écrire registry.yaml](../guides/write-registry-yaml.md).

## Exemple

```yaml
apiVersion: local-registry/v1
displayName: SonarBridge
description: Bridge between SonarQube and internal tools.
license: Proprietary                  # identifiant SPDX (MIT, Apache-2.0…) ou "Proprietary"
owners:
  - name: Platform team
    email: platform@corp.example
links:
  - { name: source, url: https://git.corp.example/platform/sonarbridge }
  - { name: docs,   url: https://docs.corp.example/sonarbridge }
keywords: [sonar, cli]
labels:                               # paires clé/valeur libres
  domain: quality
  cost-center: "4512"
install: |                            # Markdown, affiché tel quel
  curl -fsSL https://registry.example.com/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz | tar xz
deprecated: false                     # ou un message : "replaced by sonarbridge2"
```

La plus petite fiche valide tient en une ligne :

```yaml
apiVersion: local-registry/v1
```

## Champs

| Champ | Type | Obligatoire | Règle |
| --- | --- | --- | --- |
| `apiVersion` | chaîne | **oui** | Exactement `local-registry/v1`. |
| `displayName` | chaîne | non | 100 caractères au plus. Par défaut : `project_name` de `metadata.json`, sinon le nom du produit. |
| `description` | chaîne | non | 500 caractères au plus. |
| `license` | chaîne | non | Texte libre ; un identifiant SPDX ou `Proprietary` est recommandé. |
| `owners` | liste de `{name, email}` | non | `name` non vide ; `email`, facultatif, contient `@`. |
| `links` | liste de `{name, url}` | non | `name` non vide ; `url` est une URL `http://` ou `https://` avec un hôte. |
| `keywords` | liste de chaînes | non | 20 au plus ; aucun vide. |
| `labels` | table de chaînes | non | 50 entrées au plus. Les nombres et booléens sont lus comme des chaînes (`4512` devient `"4512"`). |
| `install` | chaîne | non | 10 Kio au plus. Markdown, affiché tel quel par les clients. |
| `deprecated` | booléen ou chaîne | non | `false` (défaut), `true`, ou un message. Une chaîne vide vaut `false`. |

Le fichier entier fait au plus 64 Kio et contient un seul document YAML. Toute autre clé est refusée, pour qu'une faute de frappe ne passe pas inaperçue. Cela inclut `changes` : les notes de version viennent de [`CHANGELOG.md`](./changelog-format.md).

## Où vont les valeurs

| Champ | Affiché dans |
| --- | --- |
| `displayName` | `release.json` de la version ; `index.json` et `GET /products` quand cette version fournit la fiche. |
| `deprecated` | `release.json` de la version, et son entrée dans `index.json`. |
| tous les autres champs | `index.json` et `GET /products`, à partir de la fiche de la dernière version (sinon de la version publiée en dernier). |

Voir [release.json](./release-json.md) et [index.json](./index-json.md).

## Validation

Le fichier est validé au dépôt (`PUT …/files/registry.yaml`), puis de nouveau à la publication. Un fichier invalide répond `422` et n'est pas conservé.

| Message | Cause |
| --- | --- |
| `registry.yaml is larger than 64 KiB` | Fichier trop gros. |
| `registry.yaml: empty` | Aucun document YAML. |
| `registry.yaml: holds several YAML documents` | Un séparateur `---` suivi d'un second document. |
| `registry.yaml: unmarshal errors: line <n>: field <key> not found in type endpoints.ProductFile` | Clé inconnue (faute de frappe, `changes`…). |
| `registry.yaml: line <n>: <reason>`, `registry.yaml: unmarshal errors: line <n>: cannot unmarshal …` | YAML invalide, ou valeur du mauvais type. |
| `registry.yaml: apiVersion must be local-registry/v1` | `apiVersion` absent ou différent. |
| `registry.yaml: displayName is longer than 100 characters` | |
| `registry.yaml: description is longer than 500 characters` | |
| `registry.yaml: more than 20 keywords` | |
| `registry.yaml: more than 50 labels` | |
| `registry.yaml: install is larger than 10 KiB` | |
| `registry.yaml: owners[<i>] has no name` | |
| `registry.yaml: owners[<i>].email "<email>" is not an email address` | Pas de `@` dans l'adresse. |
| `registry.yaml: links[<i>] needs a name and an http(s) url` | |
| `registry.yaml: keywords[<i>] is empty` | |
| `registry.yaml: deprecated must be true, false or a message` | Une liste, une table ou un nombre. |

À la publication, une version sans ce fichier répond `422 registry.yaml not uploaded: every version needs its product card`.

## Schéma JSON

[`schema/registry.schema.json`](https://github.com/nivekalara237/local-registry/blob/main/schema/registry.schema.json) (JSON Schema 2020-12) décrit les mêmes règles, pour que la CI puisse vérifier le fichier avant de l'envoyer. Un test du projet vérifie que le schéma et le serveur acceptent les mêmes clés.

```sh
check-jsonschema --schemafile schema/registry.schema.json registry.yaml
```

Le schéma est aussi livré dans chaque archive de version de local-registry.
