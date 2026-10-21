<p align="center">
  <img alt="local-registry" src="./assets/logo-fl.svg?v=3&s=300" height="300" />
</p>

<p align="center">
  <a href="https://github.com/nivekalara237/local-registry/actions/workflows/ci.yml"><img alt="ci" src="https://github.com/nivekalara237/local-registry/actions/workflows/ci.yml/badge.svg" /></a>
  <a href="https://github.com/nivekalara237/local-registry/actions/workflows/release.yml"><img alt="release" src="https://github.com/nivekalara237/local-registry/actions/workflows/release.yml/badge.svg" /></a>
  <a href="LICENSE"><img alt="licence" src="https://img.shields.io/badge/license-Apache--2.0-blue.svg" /></a>
</p>

<p align="center"><a href="README.md">English</a> | <b>Français</b></p>

# local-registry

**Un registre privé de versions pour vos livrables internes.** Les équipes y publient des fichiers versionnés (binaires Go construits par [goreleaser](https://goreleaser.com), migrations de base de données, paquets de configuration, n'importe quel fichier), et les serveurs, scripts et pipelines les y téléchargent en HTTP simple.

Un binaire statique, aucune base de données : tout tient dans un dossier que l'on sauvegarde comme n'importe quel autre.

## Pourquoi local-registry

- **Une version apparaît entière, ou pas du tout.** Les fichiers arrivent dans une zone privée (le staging), sont vérifiés, puis publiés en une seule étape atomique. Un client ne voit jamais une version à moitié.
- **Ce que vous téléchargez est ce qui a été construit.** Chaque fichier est vérifié par son sha256 avant publication ; checksums, SBOM et signatures accompagnent la version.
- **Publié veut dire immuable.** Une version ne peut plus changer une fois publiée.
- **Un contrat stable pour les clients.** Chaque version reçoit un `release.json` normalisé, chaque produit un `index.json` : les clients n'ont plus à deviner les noms de fichiers. Voir le [standard `local-registry/v1`](SPEC.fr.md).
- **Pensé pour goreleaser, sans en dépendre.** Envoyez un `dist/` goreleaser tel quel, ou n'importe quel dossier de fichiers avec une courte fiche produit.
- **Conçu pour l'exploitation.** Token d'upload, journal d'audit, quotas et purge du staging, HTTPS, images multi-architectures signées avec SBOM.

## Démarrage rapide

Lancez le serveur :

```sh
export LOCAL_REGISTRY_UPLOAD_TOKEN="$(openssl rand -hex 32)"
docker run -d --name local-registry -p 8077:8077 \
  -v local-registry-data:/data -e LOCAL_REGISTRY_UPLOAD_TOKEN \
  ghcr.io/nivekalara237/local-registry:1.0.0-rc.2
```

Publiez une version, depuis un clone de ce dépôt : une fiche produit et des fichiers.

```sh
mkdir -p out && echo "hello" > out/hello.txt
printf 'apiVersion: local-registry/v1\ndisplayName: Hello\n' > registry.yaml

LOCAL_REGISTRY_URL=http://localhost:8077 \
  scripts/upload.sh --latest --version v1.0.0 hello out
```

Consommez-la :

```sh
curl -s http://localhost:8077/hello/releases/latest     # la version, ses fichiers, leurs sha256
curl -sO http://localhost:8077/hello/artifacts/v1.0.0/files/hello.txt
```

Avec goreleaser, lancez `goreleaser release`, puis `scripts/upload.sh --latest <produit>` : le script lit `dist/artifacts.json`.

## Documentation

La documentation complète, en français et en anglais, se trouve dans [`website/`](website/README.md). C'est un site Docusaurus, et chaque page se lit aussi sur GitHub.

| Je veux… | Lire |
| --- | --- |
| Comprendre le modèle | [Introduction](website/i18n/fr/docusaurus-plugin-content-docs/current/intro.md), [Staging et publication](website/i18n/fr/docusaurus-plugin-content-docs/current/concepts/staging-and-publish.md) |
| Essayer en 5 minutes | [Démarrage rapide](website/i18n/fr/docusaurus-plugin-content-docs/current/getting-started/quick-start.md) |
| Publier depuis goreleaser ou une CI | [Publier avec goreleaser](website/i18n/fr/docusaurus-plugin-content-docs/current/guides/publish-with-goreleaser.md), [Pipelines CI](website/i18n/fr/docusaurus-plugin-content-docs/current/guides/ci-pipelines.md) |
| Publier des fichiers sans goreleaser | [Publier des fichiers simples](website/i18n/fr/docusaurus-plugin-content-docs/current/guides/publish-plain-files.md) |
| Installer et exploiter le serveur | [Installation](website/i18n/fr/docusaurus-plugin-content-docs/current/operations/install.md), [Configuration](website/i18n/fr/docusaurus-plugin-content-docs/current/operations/configuration.md), [Sécurité](website/i18n/fr/docusaurus-plugin-content-docs/current/operations/security.md) |
| Intégrer l'API | [Consommer les versions](website/i18n/fr/docusaurus-plugin-content-docs/current/guides/consume-releases.md), [Référence de l'API HTTP](website/i18n/fr/docusaurus-plugin-content-docs/current/reference/api/index.md) |
| Résoudre un problème | [Dépannage](website/i18n/fr/docusaurus-plugin-content-docs/current/troubleshooting.md) |

Le [standard `local-registry/v1`](SPEC.fr.md) est la définition normative de `registry.yaml`, `release.json`, `index.json` et des règles de publication, pour ceux qui écrivent des clients ou des outils.

## Installation

| Canal | Où l'obtenir |
| --- | --- |
| Image conteneur (linux/amd64, linux/arm64, signée) | `ghcr.io/nivekalara237/local-registry`, aussi sur Docker Hub |
| Binaires (Linux, macOS, Windows) | [Releases GitHub](https://github.com/nivekalara237/local-registry/releases) |
| Depuis les sources (Go 1.27+) | `go build -o local-registry ./cmd` |

[Installation](website/i18n/fr/docusaurus-plugin-content-docs/current/operations/install.md) couvre systemd, Docker et Kubernetes.

## Contribuer

Les issues et pull requests sont les bienvenues. Avant d'ouvrir une pull request :

```sh
gofmt -l . && go vet ./... && go test -race ./...
cd website && npm ci && npm run build    # si vous modifiez la documentation
```

## Licence

Licence Apache 2.0 : voir [LICENSE](LICENSE) et [NOTICE](NOTICE).
