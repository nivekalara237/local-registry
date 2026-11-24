---
title: Démarrage rapide
description: Lancez un registre sur votre machine, publiez une première version et téléchargez-la, en cinq minutes environ.
---

Cette page vous mène de zéro à une version publiée et téléchargeable en cinq minutes environ. Vous lancez un registre d'essai sur votre machine, y publiez deux petits fichiers et les récupérez. GoReleaser n'est pas nécessaire.

Pour publier sur un autre registre, la démarche est la même : seuls l'adresse et le token d'upload changent. Si vous n'exploitez pas ce registre vous-même, la personne ou l'équipe qui en est responsable vous les fournit.

## Avant de commencer

| Outil | Pourquoi |
| --- | --- |
| Go 1.27+ et un clone du dépôt | Construire le serveur (ou prendre un binaire publié, consultez [Installation](../operations/install.md)) |
| [`localrectl`](../reference/localrectl.md) | Publier la version |
| curl | Lire et télécharger |

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
> Le token est le seul secret. Les éditeurs (Upload, Publish, etc.) l'envoient, les lecteurs (téléchargement, API GET, etc.) n'en ont jamais besoin. Sans token, le serveur sert toujours les téléchargements mais refuse tout upload avec `403`.

## 2. Préparer une version

Une version, c'est un ensemble de fichiers plus une fiche produit, `registry.yaml`. Créez les deux :

```sh
mkdir -p out
echo "hello from v0.1.0" > out/hello.txt
tar czf out/hello_0.1.0.tar.gz -C out hello.txt

cat > registry.yaml <<'YAML'
apiVersion: local-registry/v1
displayName: Hello
description: Un premier produit, pour essayer Local Registry.
YAML
```

## 3. La publier

Dans le second terminal, exportez le même token et l'adresse du serveur, puis lancez `localrectl` :

```sh
export LOCAL_REGISTRY_UPLOAD_TOKEN="<le token de l'étape 1>"
export LOCAL_REGISTRY_URL=http://localhost:8077

localrectl publish hello --root out --version v0.1.0 --no-semver-standard \
  --product-file registry.yaml --latest
```

`localrectl` envoie chaque fichier de `out/` et la fiche produit, puis demande au serveur de publier `v0.1.0` et d'en faire la dernière version. `--no-semver-standard` autorise le préfixe `v` dans le nom de version.

> [!TIP]
> Ajoutez `--dry-run` pour voir ce qui serait envoyé, sans rien envoyer et sans token.

La version publiée contient deux fichiers que vous n'avez pas envoyés : `checksums.txt` et `release.json`. Le serveur les a écrits. Consultez [Intégrité](../concepts/integrity.md) et [Le standard local-registry/v1](../concepts/standard-v1.md).

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

- Publier un vrai projet Go : [Première release GoReleaser](./first-release-goreleaser.md).
- Publier des fichiers qui ne sont pas des binaires : [Publier des fichiers sans GoReleaser](./first-release-files.md).
- Exploiter votre propre registre : [Installation](../operations/install.md) et [Sécurité](../operations/security.md).
