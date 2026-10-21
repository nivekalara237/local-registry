---
title: Consommer les releases
sidebar_label: Consommer les releases
description: Trouver les produits et les versions, télécharger le bon fichier et le vérifier, depuis un script shell ou n'importe quel client HTTP.
---

Ce guide s'adresse aux intégrateurs : installeurs, scripts de déploiement et portails internes qui lisent le registre. Chaque recette utilise `curl` et `jq` sur les routes de lecture, qui ne demandent pas de token.

```sh
R=https://registry.example.com
```

## Les URL sur lesquelles compter

| Quoi | URL | Change dans le temps |
| --- | --- | --- |
| Tous les produits, avec leur dernière version | `GET $R/products` | oui |
| Un produit, sa fiche et ses versions | `GET $R/{product}/index` | oui |
| Une version : fichiers, plateformes, sha256, notes | `GET $R/{product}/releases/{version}` | jamais : une version est immuable |
| La dernière version | `GET $R/{product}/releases/latest` | quand une nouvelle dernière version est publiée |
| Un fichier | `GET $R/{product}/artifacts/{version}/files/{name}` | jamais |

`release.json`, servi par `/releases/…`, est la description de référence d'une version. Son format est dans la [référence de release.json](../reference/release-json.md).

> [!NOTE]
> Les URL de téléchargement prennent une version exacte. `…/artifacts/latest/files/…` répond `404` : résolvez d'abord `latest` en numéro de version, comme ci-dessous.

## Lister les produits

```sh
curl -fsS "$R/products" | jq -r '.[] | [.product, (.latest // "-"), .displayName] | @tsv'
```

```
billing-db-migrations  v1.0.0  Billing database migrations
sonarbridge            v1.4.2  SonarBridge
```

## Trouver la dernière version

```sh
version=$(curl -fsS "$R/sonarbridge/releases/latest" | jq -r .version)
```

Un `404` signifie qu'aucune version du produit n'a été publiée comme dernière. Pour choisir parmi toutes les versions, lisez plutôt l'index. Il liste les versions de la plus récente à la plus ancienne, et signale les pré-versions :

```sh
# La version stable la plus récente
curl -fsS "$R/sonarbridge/index" | jq -r '[.versions[] | select(.prerelease | not)][0].version'
```

## Télécharger le fichier d'une plateforme

```sh
rel=$(curl -fsS "$R/sonarbridge/releases/latest")
url=$(jq -r '.artifacts[] | select(.kind == "archive" and .os == "linux" and .arch == "amd64") | .url' <<<"$rel")
sha=$(jq -r '.artifacts[] | select(.kind == "archive" and .os == "linux" and .arch == "amd64") | .sha256' <<<"$rel")

# L'URL est relative quand le serveur tourne sans --public-url
case $url in http*) ;; *) url=$R$url ;; esac

curl -fsSLo sonarbridge.tar.gz "$url"
echo "$sha  sonarbridge.tar.gz" | sha256sum -c -   # macOS : shasum -a 256 -c -
```

`os` et `arch` valent `null` pour les fichiers publiés sans information de plateforme goreleaser. Sélectionnez-les par `name` ou par `kind`.

## Vérifier avec le fichier de checksums

Chaque version a un fichier de checksums, envoyé ou écrit par le serveur. Son nom est dans `release.json` :

```sh
v=v1.4.2
rel=$(curl -fsS "$R/sonarbridge/releases/$v")
sums=$(jq -r .checksums.name <<<"$rel")

curl -fsSO "$R/sonarbridge/artifacts/$v/files/$sums"
curl -fsSO "$R/sonarbridge/artifacts/$v/files/sonarbridge_1.4.2_linux_amd64.tar.gz"
sha256sum -c --ignore-missing "$sums"
```

Quand l'éditeur a signé le fichier de checksums ou les archives, `release.json` donne l'URL de chaque signature et de chaque certificat (`checksums.signature`, `artifacts[].signature`). Voir [Intégrité](../concepts/integrity.md).

## Surveiller les mises à jour de sécurité et les dépréciations

```sh
# Les versions qui contiennent un correctif de sécurité
curl -fsS "$R/sonarbridge/index" | jq -r '.versions[] | select(.securityUpdates) | .version'

# La version que vous utilisez est-elle dépréciée ? false, true ou un message
curl -fsS "$R/sonarbridge/releases/v1.4.2" | jq .deprecated
```

## Routes plus anciennes

Les clients écrits avant le standard `local-registry/v1` lisent directement les fichiers de goreleaser : `GET /{product}/artifacts` (l'`artifacts.json` de la dernière version), `GET /{product}/artifacts/{version}` et `GET /artifacts/metadatas/{product}`. Elles fonctionnent toujours pour les releases goreleaser, mais répondent `404` pour une version publiée sans `artifacts.json` ou sans `metadata.json`. Les nouveaux clients doivent lire `release.json`. Voir l'[API HTTP](../reference/api/index.md).

## Bonnes pratiques

- **Mettez en cache par version.** Une version publiée ne change jamais : vous pouvez garder son `release.json` et ses fichiers indéfiniment. Seuls `/products`, `/{product}/index` et `/releases/latest` changent.
- **Vérifiez toujours le sha256** avant d'installer un fichier.
- **Échouez sur les erreurs HTTP** : `curl -f` transforme un `404` en code de sortie au lieu d'enregistrer une page d'erreur.
- **Un téléchargement doit se terminer en 20 secondes.** Sur un lien lent, un gros fichier est coupé : reprenez-le avec `curl -C -`, qui envoie une requête `Range`. Voir [Limites](../limitations.md).
