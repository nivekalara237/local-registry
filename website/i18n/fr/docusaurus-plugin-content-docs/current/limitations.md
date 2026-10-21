---
title: Garanties et limites
sidebar_label: Limites
description: Ce que local-registry garantit, comment, et les limites à connaître avant d'en dépendre.
---

Cette page dit ce que local-registry promet, et où il s'arrête. Servez-vous-en pour décider si le registre convient à un usage, et de ce qu'il faut mettre autour.

## Garanties

| Garantie | Comment |
| --- | --- |
| Un fichier en cours de réception n'est jamais visible. | Il est écrit dans `bin.local/.uploads-tmp/`, puis renommé dans le staging une fois complet et vérifié. |
| Une version apparaît complète, ou pas du tout. | La publication renomme tout le répertoire de staging dans l'arborescence publique en une seule opération. |
| Les fichiers publiés correspondent à leurs checksums. | Avant la publication, chaque fichier est vérifié contre le fichier de checksums envoyé. Sans lui, la publication calcule le sha256 de chaque fichier et écrit `checksums.txt`. |
| Les URL de téléchargement d'`artifacts.json` pointent vers les fichiers de la version. | Elles sont réécrites en staging, après la vérification de chaque fichier listé, avant que la version devienne visible. |
| La dernière version change en une étape. | Le nouveau `<product>/release.json` est écrit à côté de l'ancien, puis renommé par-dessus. |
| Une version publiée ne change jamais par l'API. | L'upload et la publication d'une version existante répondent `409`. |
| Des uploads en parallèle ne peuvent pas dépasser un quota ensemble. | Les quotas sont revérifiés sous le verrou d'upload. |
| Chaque publication est traçable. | Le journal d'audit l'enregistre, avec le sha256 de chaque fichier et le `token_id`, avant que le client reçoive sa réponse. |

Ces garanties tiennent à deux conditions :

- `bin.local/` est sur **un seul système de fichiers**, car les renommages ne sont atomiques qu'au sein d'un même système ;
- **un seul serveur** écrit dans un asset path donné, car le verrou d'upload vit dans le processus.

## Limites

### Contrôle d'accès

- **Les lectures sont publiques.** Tout ce qui est sous `bin.local/assets/` et `assets/` se télécharge sans authentification. Restreignez l'accès au niveau réseau, ou par un proxy qui authentifie, si les releases sont confidentielles.
- **Un token par serveur.** Tous les publieurs partagent le même token d'upload. Il n'y a ni token par équipe, ni portée, ni permission par produit. Le journal d'audit distingue les publications par adresse et par date, pas par équipe.
- **Sans TLS, le token circule en clair.** Activez HTTPS dans le serveur ou dans un proxy. Voir [Sécurité](operations/security.md).
- **`X-Forwarded-For` est enregistré, pas vérifié.**

### Disponibilité et montée en charge

- **Une instance par asset path.** Il n'y a ni cluster ni réplication. La haute disponibilité passe par un redémarrage rapide sur le même volume, et par des sauvegardes.
- **Pas de cache.** Chaque requête lit le disque. Les routes de produit et d'index parcourent les répertoires de version d'un produit à chaque appel.
- **Les téléchargements doivent finir en 20 secondes.** Un gros fichier sur une liaison lente est coupé. Les clients peuvent reprendre avec une requête `Range` (`curl -C -`). Placez un proxy de cache devant pour les gros fichiers.
- **Pas de point de métriques.** Supervisez par `/healthz`, les logs de requêtes et le journal d'audit. Voir [Supervision](operations/monitoring.md).

### Contenu

- **Pas de route de suppression**, ni pour le staging ni pour les versions publiées. Retirer une version est une opération manuelle sur le disque. Voir [Stockage et sauvegarde](operations/storage-and-backup.md).
- **Fichiers à plat uniquement.** Une version contient des fichiers, pas des répertoires : archivez un répertoire avant de le publier.
- **Sans `artifacts.json`, `os` et `arch` sont inconnus.** Le serveur déduit le type de chaque fichier de son nom seulement. Envoyez un `artifacts.json`, écrit à la main si besoin, quand les consommateurs ont besoin de la plateforme. Voir [Publier des fichiers simples](guides/publish-plain-files.md).
- **Les versions publiées avant le standard `local-registry/v1`** n'ont pas de `release.json`. Elles ne sont pas listées par `/products` et `/<product>/index`, et ne peuvent pas être la latest.
- **Le journal d'audit n'est pas signé**, et une écriture ratée ne fait pas échouer la requête.
- **Les liens symboliques** ne sont contrôlés que sur les routes de téléchargement. Les routes du catalogue et d'upload suivent les liens trouvés sur le disque : gardez les liens hors de l'asset path.
- **La configuration et les certificats TLS sont lus au démarrage.** Un changement demande un redémarrage.

### Déprécié

- `POST /releases/{version}` ne lit que le produit `gitlab` et sera retiré. Utilisez `POST /releases/{product}/{version}`. Voir [Routes historiques](reference/api/legacy.md).
- `/artifacts/all` renvoie le contenu brut de chaque `artifacts.json`, `metadata.json` et fichier de checksums. Il est conservé pour les clients existants ; les nouveaux clients doivent utiliser [les routes de produit](reference/api/products.md).
