---
title: Stockage et sauvegarde
description: L'organisation de l'asset path, comment le sauvegarder et le restaurer, et comment retirer une version sans risque.
---

local-registry garde tout sous forme de fichiers simples dans un seul répertoire, l'asset path. Il n'y a ni base de données ni cache : le serveur lit le disque à chaque requête. Cette page décrit l'organisation des fichiers, et les procédures qui agissent directement dessus : sauvegarde, restauration, et retrait d'une version.

## Organisation

```
/srv/registry/                               l'asset path
├── bin.local/
│   ├── assets/                              public
│   │   └── binaries/<product>/
│   │       ├── release.json                 copie du release.json de la dernière version : marque la latest
│   │       ├── artifacts.json               copie de l'artifacts.json de la dernière version, s'il existe
│   │       ├── index.json                   le produit et ses versions, réécrit à chaque publication
│   │       └── <version>/                   une version publiée, immuable
│   │           ├── registry.yaml            fiche produit
│   │           ├── release.json             la version, telle que le serveur la décrit
│   │           ├── artifacts.json           si envoyé, réécrit avec les URL de téléchargement
│   │           ├── artifacts.goreleaser.json   artifacts.json tel qu'envoyé
│   │           ├── metadata.json, CHANGELOG.md   si envoyés
│   │           ├── *checksums.txt           envoyé, ou écrit par la publication
│   │           └── archives, paquets, fichiers…
│   ├── .uploads/<product>/<version>/        privé : staging, envoyé mais pas publié
│   ├── .uploads-tmp/                        privé : fichiers en cours de réception
│   └── audit.log                            privé, mode 0600
├── assets/binaries/<product>/<version>/     public : ancienne arborescence, lue par POST /releases
└── tout le reste                            privé, jamais servi
```

Le sens de chaque fichier est décrit dans [Le standard local-registry/v1](../concepts/standard-v1.md). Le fonctionnement de « latest » est décrit dans [Latest](../concepts/latest.md).

### Règles pour le système de fichiers

- **Gardez `bin.local/` sur un seul système de fichiers.** La publication renomme le répertoire de staging dans `bin.local/assets/`, et les fichiers reçus sont renommés de `.uploads-tmp/` vers le staging. Un renommage n'est atomique qu'au sein d'un même système de fichiers. Ne montez pas `.uploads`, `.uploads-tmp` ou `assets` séparément.
- **Un serveur par asset path.** Le verrou d'upload vit dans le processus. Deux serveurs qui écrivent dans le même répertoire, ou un volume réseau partagé monté par deux machines, peuvent entremêler leurs publications.
- **Les fichiers copiés à la main sont visibles tout de suite.** Il n'y a pas de cache à vider.

## Prévoir la capacité

L'occupation du disque est la somme de :

- chaque version publiée, jamais retirée automatiquement ;
- la zone de staging, plafonnée par `max-staging-mb` ;
- le journal d'audit, qui grossit à chaque upload.

Gardez en permanence `min-free-mb` d'espace libre : les uploads qui passeraient en dessous sont refusés en `507`. Voir [Quotas et staging](quotas-and-staging.md).

## Sauvegarde

Les versions publiées ne changent jamais, et les fichiers qui changent (`<product>/release.json`, `<product>/artifacts.json`, `<product>/index.json`) sont remplacés de façon atomique. Vous pouvez sauvegarder un serveur en marche avec un outil au niveau fichier.

À sauvegarder :

| Chemin | Pourquoi |
| --- | --- |
| `bin.local/assets/` | Toutes les versions publiées, et les marques de latest. |
| `bin.local/audit.log`, ou votre chemin `audit-log` | La trace de chaque publication. |
| `assets/` | Seulement si vous utilisez encore l'ancienne arborescence. |
| Votre fichier de configuration | Les réglages. Il contient le token : stockez la copie comme un secret. |

Ignorez `bin.local/.uploads/` et `bin.local/.uploads-tmp/` : ils contiennent du travail non publié que les publieurs peuvent renvoyer.

```sh
# Chaque nuit, copie incrémentale vers une machine de sauvegarde
rsync -a --delete \
  --exclude 'bin.local/.uploads/' --exclude 'bin.local/.uploads-tmp/' --exclude '.tmp-*' \
  /srv/registry/ backup.example.com:/backups/local-registry/
```

Pour une copie à un instant précis, préférez un instantané du système de fichiers ou du volume (LVM, ZFS, instantané de disque cloud, `VolumeSnapshot` Kubernetes).

> [!TIP]
> Une publication pendant la copie est entièrement dans la sauvegarde, ou pas du tout, pour son répertoire de version. Seule la marque de latest de ce produit peut avoir une publication de retard. Lancez la sauvegarde en dehors de vos fenêtres de release, ou prenez un instantané, si cela compte pour vous.

## Restauration

1. Arrêtez le serveur.
2. Restaurez la sauvegarde dans l'asset path.
3. Rendez-la à l'utilisateur du service : `chown -R registry:registry /srv/registry` (uid `10001` pour l'image conteneur).
4. Démarrez le serveur, puis vérifiez un produit :

   ```sh
   curl -s https://registry.example.com/products
   curl -s https://registry.example.com/<product>/releases/latest
   ```

## Retirer une version publiée

Il n'y a pas d'API pour supprimer une version, volontairement : des clients en dépendent peut-être déjà. Préférez publier une version corrigée et en faire la latest. Si vous devez retirer une version, par exemple après une fuite :

1. Vérifiez si c'est la latest :

   ```sh
   cd /srv/registry/bin.local/assets/binaries/<product>
   jq -r .version release.json
   ```

2. Si c'est le cas, faites d'abord d'une autre version la latest. Voir [Changer la dernière version](../guides/change-latest.md).
3. Supprimez le répertoire de la version :

   ```sh
   rm -rf /srv/registry/bin.local/assets/binaries/<product>/<version>
   ```

4. `GET /<product>/index` et `GET /products` sont construits depuis le disque à chaque requête : ils ne listent plus la version. Le fichier `<product>/index.json` n'est réécrit qu'à la prochaine publication de ce produit. D'ici là, il liste encore la version retirée : supprimez-le, ou publiez une version.

Ne réutilisez pas un numéro de version retiré pour un autre contenu : des caches et des clients peuvent encore garder les anciens fichiers sous ce nom.

## Nettoyer après un crash

Un crash peut laisser des fichiers temporaires. Toutes les routes les ignorent, et vous pouvez les supprimer quand aucun upload ne tourne :

| Fichiers | Où | Nettoyés par |
| --- | --- | --- |
| `upload-*` | `bin.local/.uploads-tmp/` | La purge du staging, après `staging-ttl`. |
| `.tmp-*` | `bin.local/assets/binaries/<product>/` | Vous, à la main. |
