---
title: Démarrage rapide
description: Lancez un registre sur votre machine, publiez une première version et téléchargez-la, en cinq minutes environ.
---

Cette page vous mène de zéro à une version publiée et téléchargeable en cinq minutes environ. Vous lancez le serveur en local, publiez deux petits fichiers et les récupérez. goreleaser n'est pas nécessaire.

## Avant de commencer

| Outil | Pourquoi |
| --- | --- |
| Go 1.27+ | Construire le serveur (ou prendre un binaire publié) |
| bash, curl 7.76+, jq | Lancer `scripts/upload.sh` |
| Un clone du dépôt | Les sources du serveur et le script d'upload |

## 1. Lancer le serveur

Construisez le binaire, créez un dossier de stockage et choisissez un token d'upload.

```sh
go build -o local-registry ./cmd
mkdir -p ~/registry-data

export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"
./local-registry serve --asset-path ~/registry-data
```

Le serveur écoute sur `http://localhost:8077`. Laissez-le tourner et ouvrez un second terminal pour la suite.

> [!NOTE]
> Le token est le seul secret. Les éditeurs l'envoient, les lecteurs n'en ont jamais besoin. Sans token, le serveur sert toujours les téléchargements mais refuse tout upload avec `403`.

## 2. Préparer une version

Une version, c'est un ensemble de fichiers plus une fiche produit, `registry.yaml`. Créez les deux :

```sh
mkdir -p out
echo "hello from v0.1.0" > out/hello.txt
tar czf out/hello_0.1.0.tar.gz -C out hello.txt

cat > registry.yaml <<'YAML'
apiVersion: local-registry/v1
displayName: Hello
description: Un premier produit, pour essayer local-registry.
YAML
```

## 3. La publier

Dans le second terminal, exportez le même token et l'adresse du serveur, puis lancez le script d'upload :

```sh
export LOCAL_REGISTRY_UPLOAD_TOKEN="<le token de l'étape 1>"
export LOCAL_REGISTRY_URL=http://localhost:8077

scripts/upload.sh --latest --version v0.1.0 --product-file registry.yaml hello out
```

Le script envoie chaque fichier de `out/` avec son sha256, puis la fiche produit, puis demande au serveur de publier `v0.1.0` et d'en faire la dernière version :

```
== hello v0.1.0 from /home/me/out
PUT hello.txt <- /home/me/out/hello.txt (sha256 5d1f…)
PUT hello_0.1.0.tar.gz <- /home/me/out/hello_0.1.0.tar.gz (sha256 a8c2…)
PUT registry.yaml <- registry.yaml (sha256 0b7e…)
POST publish hello/v0.1.0 (latest=true)
{"product":"hello","version":"v0.1.0","files":["checksums.txt","hello.txt","hello_0.1.0.tar.gz","registry.yaml","release.json"],"latest":true}
```

Deux fichiers que vous n'avez pas envoyés apparaissent : `checksums.txt` et `release.json`. Le serveur les a écrits. Voir [Intégrité](../concepts/integrity.md) et [Le standard local-registry/v1](../concepts/standard-v1.md).

## 4. Lire et télécharger

La lecture ne demande pas de token :

```sh
curl -s http://localhost:8077/products                    # tous les produits
curl -s http://localhost:8077/hello/releases/latest       # la dernière version, décrite
curl -fsSO http://localhost:8077/hello/artifacts/v0.1.0/files/hello_0.1.0.tar.gz
```

`/hello/releases/latest` renvoie le `release.json` de `v0.1.0` : ses fichiers, leur taille, leur sha256 et leur URL de téléchargement.

## Ce que vous venez de faire

Vous avez parcouru le cycle complet : dépôt en staging, publication atomique, lecture par l'API. Tout le reste de cette documentation s'appuie dessus.

## Pour aller plus loin

- Publier un vrai projet Go : [Première release goreleaser](./first-release-goreleaser.md).
- Publier des fichiers qui ne sont pas des binaires : [Publier des fichiers sans goreleaser](./first-release-files.md).
- Le faire tourner pour votre équipe : [Installation](../operations/install.md) et [Sécurité](../operations/security.md).
