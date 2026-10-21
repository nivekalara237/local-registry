---
title: API HTTP
description: Conventions communes à toutes les routes de l'API HTTP de local-registry, et liste de toutes les routes.
sidebar_label: Vue d'ensemble
---

L'API HTTP de local-registry sert les versions publiées et en accepte de nouvelles. Cette page donne les règles que suivent toutes les routes ; chaque groupe de routes a sa propre page.

## URL de base

Par défaut, le serveur écoute sur `http://localhost:8077` (`--address`, `--port`), ou en `https://` quand TLS est activé. Derrière un reverse proxy, renseignez `--public-url` (par exemple `https://registry.example.com`) : les URL de téléchargement que le serveur écrit (`release.json`, `artifacts.json` réécrit, réponses de `POST /releases`) deviennent alors des URL absolues sous cette adresse. Sans cette option, ce sont des chemins relatifs comme `/sonarbridge/artifacts/v1.4.2/files/sonarbridge_1.4.2_linux_amd64.tar.gz`.

## Routes

| Méthode | Route | Auth | Renvoie | Page |
| --- | --- | --- | --- | --- |
| `GET` | `/products` | — | Tous les produits, avec leur dernière version | [Produits](./products.md) |
| `GET` | `/{product}/index` | — | `index.json` : la fiche du produit et ses versions | [Produits](./products.md) |
| `GET` | `/{product}/releases/{version}` | — | `release.json` d'une version (`latest` accepté) | [Produits](./products.md) |
| `GET` | `/{product}/artifacts` | — | `artifacts.json` de la dernière version | [Artifacts](./artifacts.md) |
| `GET` | `/{product}/artifacts/{version}` | — | `artifacts.json` d'une version | [Artifacts](./artifacts.md) |
| `GET` | `/artifacts/metadatas/{product}` | — | `metadata.json` d'une version (par défaut : la dernière) | [Artifacts](./artifacts.md) |
| `GET` | `/artifacts/all` | — | Fichiers bruts du catalogue, pour chaque répertoire | [Artifacts](./artifacts.md) |
| `GET` | `/healthz` | — | Sonde de vivacité | [Artifacts](./artifacts.md) |
| `GET` | `/{product}/artifacts/{version}/files/{name}` | — | Un fichier d'une version publiée | [Téléchargements](./downloads.md) |
| `GET` | `/assets/binary/{filePath}` | — | Un fichier public, par son chemin | [Téléchargements](./downloads.md) |
| `GET` | `/assets/checksum/{filename}` | — | Identique à `/assets/binary` | [Téléchargements](./downloads.md) |
| `PUT` | `/{product}/artifacts/{version}/files/{name}` | jeton | Déposer un fichier en staging | [Dépôt](./upload.md) |
| `POST` | `/{product}/artifacts/{version}/publish` | jeton | Vérifier et publier une version | [Dépôt](./upload.md) |
| `POST` | `/releases/{product}/{version}` | — | Fichiers d'une version avec leur URL de téléchargement | [Historique](./legacy.md) |
| `POST` | `/releases/{version}` | — | Dépréciée : `/releases/gitlab/{version}` | [Historique](./legacy.md) |

Les routes de lecture ne demandent aucune authentification. Seuls `PUT` et `publish` exigent le jeton de dépôt.

## Noms

`{product}`, `{version}` et `{name}` doivent respecter `[A-Za-z0-9._-]+` et n'être ni `.` ni `..`.

- Un nom refusé donne `404` sur les routes de lecture et `400` sur les routes de dépôt.
- `latest` est une version valide **en lecture** : la dernière version du produit (voir [Dernière version](../../concepts/latest.md)). Ce n'est pas une version de dépôt.
- Le nom de produit `products` est réservé.
- Les noms de version `artifacts.json`, `artifacts.goreleaser.json`, `release.json` et `index.json` sont réservés : leur répertoire prendrait la place d'un fichier du produit.

Chaque variable de chemin est **un seul segment**. Dans `/assets/binary/{filePath}`, encodez `/` en `%2F`. Les noms sont décodés une seule fois : `%252F` reste `%2F`. Un `..` ou un `//` littéral dans l'URL reçoit une redirection `301` vers le chemin nettoyé.

## Méthodes

Toute route `GET` répond aussi à `HEAD`. Une mauvaise méthode répond `405` avec un en-tête `Allow`. Un chemin inconnu répond `404 page not found` en texte brut.

## Réponses et erreurs

Les routes JSON répondent `Content-Type: application/json`. En cas d'erreur, le corps est :

```json
{"error": "version sonarbridge/v1.4.2 is already published and cannot be changed"}
```

Les routes de fichiers (téléchargements) et les erreurs de routage répondent en texte brut.

Le message `error` est destiné aux humains et aux scripts. Pour un `500`, il vaut toujours `internal error` : la vraie cause n'est écrite que dans le journal du serveur (et dans le journal d'audit pour les routes de dépôt). Tous les messages sont listés dans [Erreurs](../errors.md).

| Statut | Signification |
| --- | --- |
| `200` | OK |
| `201` | Fichier déposé en staging |
| `301` | Chemin nettoyé (`..`, `//`) |
| `400` | Nom ou paramètre invalide, nom réservé |
| `401` | Jeton absent ou faux, avec `WWW-Authenticate: Bearer` |
| `403` | Dépôt désactivé : aucun jeton configuré sur le serveur |
| `404` | Introuvable, ou nom refusé sur une route de lecture |
| `405` | Mauvaise méthode |
| `409` | Version déjà publiée |
| `413` | Fichier ou version trop volumineux |
| `422` | Contenu refusé : `registry.yaml`, `CHANGELOG.md`, checksums, `artifacts.json`, `metadata.json`, ou quota de fichiers |
| `500` | Erreur interne ; la cause n'est que dans le journal du serveur |
| `507` | Zone de staging pleine, ou disque presque plein |

## Authentification

`PUT` et `publish` exigent `Authorization: Bearer <token>`, comparé en temps constant avant toute autre chose. Voir [Dépôt](./upload.md).

Passez l'en-tête à curl sur son entrée standard, pour que le jeton n'apparaisse jamais dans la liste des processus :

```sh
printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" |
  curl -fsS -H @- -X POST https://registry.example.com/sonarbridge/artifacts/v1.4.2/publish
```

## Fraîcheur

Le serveur lit le disque à chaque requête et ne garde aucun cache : un fichier copié à la main sous le répertoire de données est visible immédiatement.

## Délais

| Délai | Valeur | Portée |
| --- | --- | --- |
| Lecture des en-têtes de la requête | 5 s | toutes les requêtes |
| Écriture de la réponse | **20 s** | toutes les routes sauf le dépôt et la publication, téléchargements compris |
| Lecture et écriture | 30 min | `PUT …/files/{name}`, `POST …/publish` |
| Connexion keep-alive inactive | 60 s | toutes les connexions |

Un téléchargement sur un lien lent qui dépasse 20 s est coupé : reprenez-le avec `Range` (`curl -C -`).
