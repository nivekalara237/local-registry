---
title: Qu'est-ce que Local Registry ?
description: Un registre de versions open source qui stocke des fichiers versionnés, les vérifie et les sert par une API HTTP stable.
sidebar_label: Introduction
---

Local Registry est un registre de versions open source. Vous y publiez vos livrables versionnés (binaires Go construits par GoReleaser, migrations de base de données, paquets de configuration, n'importe quel fichier), et les serveurs, scripts et pipelines les y téléchargent.

Vous pouvez l'installer sur votre propre serveur, ou utiliser une instance exploitée par la personne ou l'équipe qui en est responsable. Dans cette documentation, le registre est joignable à l'adresse `https://registry.corp.example` : remplacez-la par l'adresse de votre instance. La publication demande le token d'upload défini dans la configuration du serveur ; la lecture n'en demande aucun.

Côté serveur, Local Registry est un binaire statique unique, sans base de données : il range tout dans un simple dossier et le sert en HTTP.

## Pourquoi l'utiliser

Les versions que vous distribuez vous-même ont besoin d'un emplacement qui ne soit ni une page de release publique, ni un gestionnaire d'artefacts complet. Local Registry répond à ce besoin avec trois garanties :

| Garantie | Ce que cela signifie pour vous |
| --- | --- |
| **Une version apparaît entière, ou pas du tout** | Les fichiers sont déposés dans une zone privée (le staging), vérifiés, puis publiés en une seule opération atomique. Un client ne voit jamais une version à moitié livrée. |
| **Ce que vous téléchargez est ce qui a été construit** | Le SHA-256 de chaque fichier est contrôlé avant la publication. Les checksums accompagnent la version. |
| **Une version publiée ne change jamais** | Une fois publiée, une version est immuable. Un nouvel envoi répond `409 Conflict`. |

En plus des fichiers, le serveur écrit une description normalisée de chaque version, `release.json`, et de chaque produit, `index.json`. Les clients lisent ces documents au lieu de deviner des noms de fichiers. C'est le [standard `local-registry/v1`](./concepts/standard-v1.md).

## Fonctionnement

![localrectl dépose les fichiers dans le staging privé avec le token et demande la publication ; après les contrôles, la version rejoint les versions publiées immuables, que les serveurs, pipelines et scripts téléchargent sans token](/img/fonctionnement-lr.gif)

1. Un éditeur dépose les fichiers d'une version avec le token d'upload, en général avec [`localrectl`](./reference/localrectl.md).
2. Il demande la publication. Le serveur contrôle les fichiers, écrit `release.json` et rend la version visible en une seule étape.
3. Toute machine du réseau lit le catalogue et télécharge les fichiers. La lecture ne demande aucun token.

Le détail est dans [Staging et publication](./concepts/staging-and-publish.md).

## À qui s'adresse cette documentation

| Vous êtes… | Commencez par |
| --- | --- |
| **Développeur ou responsable de release**, vous publiez des versions | [Démarrage rapide](./getting-started/quick-start.md), puis [Première release GoReleaser](./getting-started/first-release-goreleaser.md) ou [Publier des fichiers sans GoReleaser](./getting-started/first-release-files.md) |
| **Administrateur**, vous installez et exploitez le serveur | [Installation](./operations/install.md), [Configuration](./operations/configuration.md), [Sécurité](./operations/security.md) |
| **Intégrateur**, vous consommez les versions | [Consommer les versions](./guides/consume-releases.md) et la [référence de l'API HTTP](./reference/api/index.md) |

## Ce que ce n'est pas

Connaître le périmètre de Local Registry évite les mauvaises surprises :

- **Pas un registre de conteneurs ni de paquets.** Il ne parle ni Docker, ni npm, ni Maven, ni APT. Il sert des fichiers en HTTP simple.
- **Pas de contrôle d'accès en lecture.** Tout ce qui est publié est lisible par quiconque atteint le serveur. Le registre doit donc rester sur un réseau privé, ou derrière un reverse proxy qui authentifie. Consultez [Sécurité](./operations/security.md).
- **Pas multi-nœud.** Un serveur possède un dossier de stockage. Consultez [Limites](./limitations.md).

> [!TIP]
> Le vocabulaire (produit, version, staging, latest) est nouveau pour vous ? Le [Glossaire](./glossary.md) définit chaque terme en une ligne.
