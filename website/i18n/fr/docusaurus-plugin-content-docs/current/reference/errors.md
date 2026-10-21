---
title: Erreurs
description: Tous les messages d'erreur du serveur, groupés par phase, avec leur cause et leur correction.
sidebar_label: Erreurs
---

Toute erreur d'une route JSON est un code de statut et un corps `{"error": "<message>"}`. Les messages ci-dessous sont les chaînes exactes du serveur, en anglais ; `<…>` marque une valeur remplie. Un `500` dit toujours `internal error` : la vraie cause est dans le journal du serveur, et dans le journal d'audit pour les routes de dépôt. Pour partir des symptômes plutôt que des messages, voir [Dépannage](../troubleshooting.md).

## Authentification

Routes de dépôt uniquement (`PUT …/files/{name}`, `POST …/publish`).

| Statut | Message | Cause | Correction |
| --- | --- | --- | --- |
| `403` | `upload is disabled: no token configured` | Le serveur tourne sans `LOCAL_REGISTRY_UPLOAD_TOKEN` ni `upload-token`. | Configurez un jeton sur le serveur et redémarrez-le. Voir [Sécurité](../operations/security.md). |
| `401` | `invalid or missing bearer token` | Pas d'en-tête `Authorization`, autre schéma que `Bearer`, ou mauvais jeton. | Envoyez `Authorization: Bearer <token>` avec le jeton du serveur. Comparez les `token_id` avec `local-registry config`. |

## Noms

| Statut | Message | Cause | Correction |
| --- | --- | --- | --- |
| `400` | `invalid product or version name` | Un nom hors de `[A-Za-z0-9._-]`, `.`, `..`, ou la version `latest` sur une route de dépôt. | Utilisez un nom valide : un tag git comme `v1.4.2` l'est. |
| `400` | `product name products is reserved` | `products` entrerait en conflit avec `GET /products`. | Choisissez un autre nom de produit. |
| `400` | `version name <version> is reserved` | `artifacts.json`, `artifacts.goreleaser.json`, `release.json` ou `index.json` comme version. | Choisissez un autre nom de version. |
| `400` | `invalid file name: use [A-Za-z0-9._-] only` | Un nom de fichier avec des espaces, des `/` ou d'autres caractères. | Renommez le fichier. |
| `400` | `<name> is reserved: publishing writes it` | Dépôt de `artifacts.goreleaser.json`, `release.json` ou `index.json`. | Ne l'envoyez pas : le serveur l'écrit. |
| `400` | `latest must be true or false` | `publish?latest=` avec une autre valeur. | Utilisez `true` ou `false`. |
| `404` | `not found` | Sur les routes de lecture, un nom invalide donne `404`. | Vérifiez le nom. |

## État de la version

| Statut | Message | Cause | Correction |
| --- | --- | --- | --- |
| `409` | `version <product>/<version> is already published and cannot be changed` | Une version publiée est immuable. | Publiez une nouvelle version. |
| `404` | `nothing uploaded for <product>/<version>` | Publication appelée avant tout dépôt, ou après la purge du staging. | Déposez les fichiers, puis publiez. |

## Quotas et disque

Levées pendant le dépôt. Voir [Quotas et staging](../operations/quotas-and-staging.md).

| Statut | Message | Cause | Correction |
| --- | --- | --- | --- |
| `413` | `file exceeds <bytes> bytes` | Un fichier au-delà de `--max-upload-mb`. | Relevez la limite, ou découpez le fichier. |
| `413` | `version <product>/<version> would exceed its staging quota of <bytes> bytes` | Les fichiers en staging de la version dépasseraient `--max-version-mb`. | Relevez la limite, ou publiez moins de fichiers. |
| `422` | `version <product>/<version> already has <n> staged files (limit <n>)` | Plus de fichiers que `--max-version-files`. Remplacer un fichier en staging reste permis. | Relevez la limite, ou regroupez des fichiers dans une archive. |
| `507` | `upload staging area is full (limit <bytes> bytes): publish or abandon pending versions` | L'ensemble des versions en staging dépasse `--max-staging-mb`. | Publiez les versions en attente, supprimez les versions abandonnées, ou relevez la limite. |
| `507` | `not enough free disk space on the registry` | Le dépôt laisserait moins que `--min-free-mb` de libre. | Libérez de l'espace ou agrandissez le volume. |

## Contenu des fichiers

Levées au dépôt du fichier.

| Statut | Message | Cause | Correction |
| --- | --- | --- | --- |
| `422` | `sha256 mismatch: got <actual>, X-Checksum-Sha256 says <expected>` | Les octets reçus ne sont pas ceux annoncés : transfert corrompu, ou mauvais fichier. | Renvoyez le fichier ; vérifiez le fichier local. |
| `422` | `registry.yaml: <reason>`, `registry.yaml is larger than 64 KiB` | Fiche produit invalide. | Corrigez le fichier : chaque raison est listée dans [registry.yaml](./registry-yaml.md). |
| `422` | `CHANGELOG.md is larger than 1 MiB` | Notes de version trop volumineuses. | Ne gardez que les versions récentes, ou la section de cette version. |
| `422` | `CHANGELOG.md is not UTF-8 text` | Un fichier binaire, ou un autre encodage. | Enregistrez-le en UTF-8. |

### artifacts.json

Levées au dépôt d'`artifacts.json`, puis de nouveau à la publication. Voir [Règles de dépôt](./upload-rules.md).

| Statut | Message | Cause | Correction |
| --- | --- | --- | --- |
| `422` | `artifacts.json is larger than 32 MiB` | Trop volumineux. | — |
| `422` | `artifacts.json is not a JSON array of objects` | Ce n'est pas le format de goreleaser. | Envoyez `dist/artifacts.json` tel que goreleaser l'a écrit. |
| `422` | `artifacts.json: entry <i> is null` | Un `null` dans le tableau. | Retirez-le. |
| `422` | `artifacts.json: name, path and type must be strings: <reason>` | Un champ du mauvais type. | Corrigez l'entrée. |
| `422` | `artifacts.json: a <type> entry has no name` | Une entrée publiée sans `name`. | Corrigez l'entrée. |
| `422` | `artifacts.json lists <name> twice` | Deux fichiers publiés de même nom. | Renommez-les dans `.goreleaser.yaml`. |
| `422` | `artifacts.json lists "<name>", a name that cannot be uploaded` | Un nom invalide ou réservé. | Renommez le fichier. |
| `422` | `artifacts.json references <name>, which is not uploaded: upload every file first, artifacts.json last` | `artifacts.json` envoyé avant un fichier qu'il liste, ou ce fichier manque. | Déposez le fichier, puis `artifacts.json` à nouveau. |
| `422` | `<name>: sha256 is <actual> but artifacts.json says <expected>` | Le fichier en staging n'est pas celui que goreleaser a construit. | Déposez le bon fichier. |

## Publication

Levées par `POST …/publish`, dans cet ordre. Rien n'est publié ; le staging est conservé.

| Statut | Message | Cause | Correction |
| --- | --- | --- | --- |
| `422` | `several checksums files: <a> and <b>` | Deux fichiers finissant par `checksums.txt`. | N'en gardez qu'un. |
| `422` | `registry.yaml not uploaded: every version needs its product card` | Pas de fiche produit. | Déposez `registry.yaml`, puis publiez à nouveau. |
| `422` | `<checksums>: line <n> is not "<sha256>  <name>"` | Une ligne mal formée dans le fichier de checksums. | Envoyez le fichier écrit par goreleaser ou `sha256sum`. |
| `422` | `<name> is not listed in <checksums>` | Un fichier en staging que le fichier de checksums ne couvre pas. | Ajoutez-le aux checksums, ou ne le déposez pas ; ou n'envoyez aucun fichier de checksums et laissez la publication en écrire un. |
| `422` | `<name>: sha256 is <actual> but <checksums> says <expected>` | Le fichier en staging diffère des checksums. | Déposez le bon fichier. |
| `422` | `metadata.json is not valid JSON` | `metadata.json` corrompu. | Envoyez celui de goreleaser, ou aucun. |
| `422` | `metadata.json has neither tag nor version` | Ce n'est pas le `metadata.json` de goreleaser. | Envoyez celui de goreleaser, ou aucun. |
| `422` | `metadata.json describes <tag> (version <v>), not <version>` | `metadata.json` d'un autre build. | Publiez sous la version que goreleaser a construite, ou envoyez le bon fichier. |
| `422` | `<name> is uploaded but artifacts.json does not list it` | Avec `artifacts.json`, un fichier en staging qu'il ne publie pas. | Listez le fichier (`release.extra_files`) ou ne le déposez pas. |
| `422` | `no artifact to publish: upload at least one file besides registry.yaml` | Seuls la fiche, les notes, les métadonnées, les checksums ou des pièces jointes sont en staging. | Déposez les fichiers de la version. |
| `500` | `internal error` | Erreur de disque ou de droits. Quand le journal du serveur indique `version published but latest not updated` ou `version published but index.json not updated`, la version **est** publiée. | Lisez le journal du serveur ; voir [Règles de dépôt](./upload-rules.md). |

## Routes de lecture

| Statut | Message | Cause |
| --- | --- | --- |
| `404` | `not found` | Produit ou version inconnus, pas de dernière version, version sans le fichier demandé (`artifacts.json`, `metadata.json`, `release.json`), ou nom invalide. |
| `404` | `404 page not found` (texte brut) | Chemin inconnu. |
| `404` | `404 page not found` (texte brut) | Sur les routes de téléchargement : fichier absent, en staging, caché, hors des arborescences publiques, ou `latest` utilisé comme version. |
| `405` | `Method Not Allowed` (texte brut) | Mauvaise méthode, comme `GET /releases/…`. L'en-tête `Allow` donne la bonne. |
| `500` | `<product>/<version>/release.json is not valid JSON` | Un `release.json` endommagé sur le disque. |
| `500` | `internal error` | Disque illisible, ou `artifacts.json` ou `metadata.json` stocké qui n'est pas du JSON valide. |

## Démarrage du serveur

Les erreurs de démarrage sont écrites sur stderr sous la forme `Error: <message>`, et le processus sort avec le code `2` (configuration) ou `1` (exécution). Elles sont listées dans [Ligne de commande](./cli.md).
