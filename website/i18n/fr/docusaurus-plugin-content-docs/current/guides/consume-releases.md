---
title: Consommer les releases
sidebar_label: Consommer les releases
description: Trouver les produits et les versions, télécharger le bon fichier et le vérifier, depuis un script shell ou n'importe quel client HTTP.
---

Ce guide s'adresse aux intégrateurs : installeurs, scripts de déploiement et portails qui lisent le registre. Chaque recette utilise `curl` et `jq` sur les routes de lecture, qui ne demandent pas de token.

```sh
R=https://registry.corp.example
```

## Les URL sur lesquelles compter

| Quoi | URL | Change dans le temps |
| --- | --- | --- |
| Tous les produits, avec leur dernière version | `GET $R/products` | oui |
| Les produits qui répondent à des critères, page par page | `GET $R/products/search?…` | oui |
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
bridge            v1.4.2  Bridge
```

## Rechercher des produits

`/products/search` filtre le catalogue côté serveur et le renvoie une page à la fois. Les critères se combinent : tous doivent correspondre.

```sh
curl -fsS -G "$R/products/search" --data-urlencode "keyword=db" --data-urlencode "label=team=billing" \
  | jq -r '.items[] | [.product, (.latest // "-")] | @tsv'
```

Pour lire tous les résultats, suivez `next` jusqu'à ce qu'il vaille `null`. `-G` avec `--data-urlencode` encode chaque valeur, curseur compris :

```sh
cursor=
while :; do
  page=$(curl -fsS -G "$R/products/search" --data-urlencode "keyword=db" \
    --data-urlencode "limit=100" --data-urlencode "cursor=$cursor")
  jq -r '.items[] | [.product, (.latest // "-")] | @tsv' <<<"$page"
  cursor=$(jq -r '.next // empty' <<<"$page")
  [ -n "$cursor" ] || break
done
```

Gardez les mêmes critères d'une page à l'autre : un curseur envoyé avec d'autres critères répond `400`. Tous les paramètres sont dans [Rechercher des produits](../reference/api/products.md#rechercher-des-produits).

## Trouver la dernière version

```sh
version=$(curl -fsS "$R/bridge/releases/latest" | jq -r .version)
```

Un `404` signifie qu'aucune version du produit n'a été publiée comme dernière. Pour choisir parmi toutes les versions, lisez plutôt l'index. Il liste les versions de la plus récente à la plus ancienne, et signale les pré-versions :

```sh
# La version stable la plus récente
curl -fsS "$R/bridge/index" | jq -r '[.versions[] | select(.prerelease | not)][0].version'
```

## Télécharger le fichier d'une plateforme

```sh
rel=$(curl -fsS "$R/bridge/releases/latest")
url=$(jq -r '.artifacts[] | select(.kind == "archive" and .os == "linux" and .arch == "amd64") | .url' <<<"$rel")
sha=$(jq -r '.artifacts[] | select(.kind == "archive" and .os == "linux" and .arch == "amd64") | .sha256' <<<"$rel")

# L'URL est relative quand le serveur tourne sans --public-url
case $url in http*) ;; *) url=$R$url ;; esac

curl -fsSLo bridge.tar.gz "$url"
echo "$sha  bridge.tar.gz" | sha256sum -c -   # macOS : shasum -a 256 -c -
```

`os` et `arch` valent `null` pour les fichiers publiés sans information de plateforme goreleaser. Sélectionnez-les par `name` ou par `kind`.

## Vérifier avec le fichier de checksums

Chaque version a un fichier de checksums, envoyé ou écrit par le serveur. Son nom est dans `release.json` :

```sh
v=v1.4.2
rel=$(curl -fsS "$R/bridge/releases/$v")
sums=$(jq -r .checksums.name <<<"$rel")

curl -fsSO "$R/bridge/artifacts/$v/files/$sums"
curl -fsSO "$R/bridge/artifacts/$v/files/bridge_1.4.2_linux_amd64.tar.gz"
sha256sum -c --ignore-missing "$sums"
```

Quand l'éditeur a signé le fichier de checksums ou les archives, `release.json` donne l'URL de chaque signature et de chaque certificat (`checksums.signature`, `artifacts[].signature`). Consultez [Intégrité](../concepts/integrity.md).

## Surveiller les mises à jour de sécurité et les dépréciations

```sh
# Les versions qui contiennent un correctif de sécurité
curl -fsS "$R/bridge/index" | jq -r '.versions[] | select(.securityUpdates) | .version'

# La version que vous utilisez est-elle dépréciée ? false, true ou un message
curl -fsS "$R/bridge/releases/v1.4.2" | jq .deprecated
```
