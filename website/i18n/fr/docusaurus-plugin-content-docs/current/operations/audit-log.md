---
title: Journal d'audit
description: Ce que le journal d'audit enregistre, son format, comment l'interroger et comment le conserver.
---

Le journal d'audit enregistre chaque événement sensible des routes d'upload : fichiers acceptés et refusés, versions publiées avec le sha256 de chaque fichier, authentifications échouées, et purges. Il répond à « qui a publié quoi, quand, et d'où », et permet de repérer les attaques contre le token d'upload.

## Où il se trouve

| Réglage | Défaut |
| --- | --- |
| `audit-log` | `<asset-path>/bin.local/audit.log` |

- Le serveur crée le fichier, et son répertoire, au démarrage. Le mode du fichier est `0600`.
- Le fichier n'est jamais servi. Le serveur refuse de démarrer si `audit-log` pointe dans une arborescence publique (`bin.local/assets/` ou `assets/`).
- Si le fichier ne peut pas être ouvert au démarrage, le serveur s'arrête avec le code de sortie `1`.

## Garanties

- **Un objet JSON par ligne**, en ajout.
- **Écrit et synchronisé sur le disque avant que le client reçoive sa réponse.** Une version que le client a vue publiée figure dans le journal.
- **Le token n'apparaît jamais**, seulement son `token_id` : les 12 premiers chiffres hexadécimaux de son sha256. Voir [Sécurité](security.md).
- **Une écriture ratée ne fait pas échouer la requête.** Le serveur écrit à la place `ERROR audit log write failed` sur la sortie d'erreur. Placez une alerte sur cette ligne.

> [!NOTE]
> Le journal d'audit n'est pas signé. Envoyez-le vers un stockage non modifiable si vous avez besoin d'une preuve d'intégrité.

## Format

Chaque ligne a ces champs :

| Champ | Contenu |
| --- | --- |
| `time` | UTC, RFC 3339 avec nanosecondes. |
| `event` | L'événement, listé ci-dessous. |
| `remote` | Le pair TCP : votre reverse proxy, s'il y en a un. Absent pour les événements de purge. |
| `forwarded_for` | L'en-tête `X-Forwarded-For` tel qu'envoyé, s'il est présent. Non vérifié. |

Événements :

| `event` | Quand | Autres champs |
| --- | --- | --- |
| `auth_failed` | `401` ou `403` sur une route d'upload | `method`, `path`, `status`, `error`, `authorization_sent` |
| `upload` | Un fichier est accepté en staging | `product`, `version`, `name`, `size`, `sha256`, `replaced`, `token_id` |
| `upload_refused` | Un fichier est refusé après authentification | `product`, `version`, `name`, `status`, `error`, `token_id` |
| `publish` | Une version est publiée | `product`, `version`, `latest`, `files`, `token_id` |
| `publish_refused` | Une publication est refusée après authentification | `product`, `version`, `status`, `error`, `token_id` |
| `staging_purged` | La purge a retiré une version en staging | `product`, `version`, `files`, `bytes`, `idle` |
| `tmp_purged` | La purge a retiré des fichiers temporaires | `files` |

- `files` d'un événement `publish` liste `{name, size, sha256}` pour chaque fichier publié, y compris ceux qu'écrit le serveur : le `artifacts.json` réécrit, `artifacts.goreleaser.json`, `release.json` et un `checksums.txt` généré.
- Pour un `500`, `error` contient la vraie cause. Le client n'a reçu que `internal error`.
- `authorization_sent` distingue un en-tête absent d'un token faux.

Exemples de lignes :

```json
{"time":"2026-10-18T09:12:03.418Z","event":"upload","product":"billing-api","version":"v2.4.0","name":"billing-api_2.4.0_linux_amd64.tar.gz","size":8123456,"sha256":"8f6c…","replaced":false,"token_id":"4ae3c36ce073","remote":"10.0.4.12"}
{"time":"2026-10-18T09:12:05.902Z","event":"publish","product":"billing-api","version":"v2.4.0","latest":true,"files":[{"name":"billing-api_2.4.0_checksums.txt","size":412,"sha256":"2b51…"}],"token_id":"4ae3c36ce073","remote":"10.0.4.12"}
{"time":"2026-10-18T09:20:44.017Z","event":"auth_failed","method":"PUT","path":"/billing-api/artifacts/v2.4.1/files/x","status":401,"error":"invalid or missing bearer token","authorization_sent":true,"remote":"10.0.9.80"}
```

## L'interroger

```sh
LOG=/srv/registry/bin.local/audit.log

# Chaque publication : quand, quoi, d'où
jq -r 'select(.event=="publish") | "\(.time) \(.product)/\(.version) latest=\(.latest) by \(.token_id) from \(.remote)"' $LOG

# Authentifications échouées, par adresse source
jq -r 'select(.event=="auth_failed") | .forwarded_for // .remote' $LOG | sort | uniq -c | sort -rn

# Pourquoi des publications ont été refusées
jq -r 'select(.event=="publish_refused") | "\(.time) \(.product)/\(.version) \(.status) \(.error)"' $LOG

# Quelle publication a livré un fichier donné
jq -c --arg n billing-api_2.4.0_linux_amd64.tar.gz \
  'select(.event=="publish") | select(any(.files[]; .name==$n)) | {time, product, version, token_id}' $LOG
```

## Rotation et conservation

Le serveur garde le fichier ouvert et ne fait qu'ajouter. Faites la rotation avec un outil externe qui tronque sur place, pour que le serveur continue d'écrire dans le même fichier :

```
# /etc/logrotate.d/local-registry
/srv/registry/bin.local/audit.log {
    weekly
    rotate 52
    compress
    missingok
    notifempty
    copytruncate
}
```

`copytruncate` est indispensable : une rotation par renommage et recréation laisserait le serveur écrire dans l'ancien fichier renommé jusqu'au prochain redémarrage.

Conseils de conservation :

- Conservez au moins aussi longtemps que votre politique de conservation des releases, pour relier chaque version publiée à son upload.
- Envoyez les lignes vers votre plateforme de logs (Loki, Elasticsearch, Splunk…) et alertez sur les rafales de `auth_failed` et sur `audit log write failed`.
- Sauvegardez le fichier avec l'asset path. Voir [Stockage et sauvegarde](storage-and-backup.md).
