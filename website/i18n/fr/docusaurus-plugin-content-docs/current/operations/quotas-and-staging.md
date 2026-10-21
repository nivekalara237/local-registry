---
title: Quotas et staging
description: Dimensionner la zone de staging, protéger le disque, et laisser le serveur purger les uploads abandonnés.
---

Les fichiers envoyés attendent dans une zone de staging jusqu'à la publication de la version. Les quotas limitent ce que cette zone peut contenir, et une purge retire les uploads que personne n'a terminés. Ajustez-les quand vos releases sont volumineuses, ou quand beaucoup d'équipes publient en même temps.

Pour le fonctionnement du staging et de la publication, voir [Staging et publication](../concepts/staging-and-publish.md).

## Quotas

| Limite | Réglage | Défaut | Au-delà |
| --- | --- | --- | --- |
| Un fichier | `max-upload-mb` | 200 Mio | `413 file exceeds <n> bytes` |
| Octets en staging d'une version | `max-version-mb` | 2048 Mio (2 Gio) | `413 version <p>/<v> would exceed its staging quota of <n> bytes` |
| Fichiers en staging d'une version | `max-version-files` | 100 | `422 version <p>/<v> already has <n> staged files (limit <n>)` |
| Octets en staging de toutes les versions | `max-staging-mb` | 10240 Mio (10 Gio) | `507 upload staging area is full (limit <n> bytes): publish or abandon pending versions` |
| Espace disque laissé libre après l'upload | `min-free-mb` | 1024 Mio (1 Gio) | `507 not enough free disk space on the registry` |

`0` désactive une limite, sauf `max-upload-mb`, où `0` ou moins signifie le défaut de 200 Mio. Les valeurs négatives sont refusées au démarrage.

Comment les quotas comptent :

- **Seul le staging compte.** Les versions publiées n'entrent pas dans les quotas de staging. Elles occupent du disque, que `min-free-mb` protège.
- **Remplacer un fichier** ne compte que la différence de taille, pas un fichier de plus.
- **Uploads en chunked** (sans `Content-Length`) : le contrôle d'espace libre suppose la plus grande taille que les quotas autorisent.
- **Des uploads en parallèle** ne peuvent pas dépasser un quota ensemble : les quotas sont vérifiés avant la réception du corps, puis de nouveau sous le verrou d'upload avant l'entrée du fichier en staging.
- Le contrôle d'espace libre fonctionne sur les systèmes Unix (Linux, macOS). Ailleurs, il est ignoré.

### Choisir les valeurs

- Réglez `max-upload-mb` au-dessus de votre plus gros fichier, en général la plus grosse archive ou le plus gros paquet.
- Réglez `max-version-mb` au-dessus de la taille totale d'une release, et `max-version-files` au-dessus de son nombre de fichiers. Une release goreleaser avec beaucoup de plateformes, de SBOM et de signatures peut dépasser 100 fichiers.
- Réglez `max-staging-mb` sur ce que vous acceptez sur disque pour le travail non publié : environ le nombre de versions publiées en même temps, multiplié par la taille d'une version.
- Gardez `min-free-mb` assez grand pour que le serveur finisse de publier et d'écrire ses logs.

```yaml
# Grosses releases, plusieurs équipes qui publient en même temps
max-upload-mb: 1024
max-version-mb: 8192
max-version-files: 300
max-staging-mb: 40960
min-free-mb: 4096
```

## Purge du staging

Une version en staging que personne ne publie reste sur le disque. Avec `staging-ttl` supérieur à `0` (le défaut est `24h`), le serveur purge :

- chaque version en staging dont le fichier le plus récent est plus vieux que le TTL ;
- les fichiers de `bin.local/.uploads-tmp/` plus vieux que le TTL, laissés par un upload interrompu ou un crash.

La purge tourne au démarrage, puis toutes les `min(ttl / 2, 1h)`. Elle ne touche jamais une version publiée. Chaque suppression est écrite dans le [journal d'audit](audit-log.md), en `staging_purged` (avec `product`, `version`, `files`, `bytes`, `idle`) ou `tmp_purged` (avec `files`).

| `staging-ttl` | Effet |
| --- | --- |
| `24h` (défaut) | Un upload non touché depuis un jour est retiré. |
| `0` | Pas de purge. Nettoyez le staging vous-même. |
| Moins de `1h` | Refusé au démarrage : un upload peut durer 30 minutes, et la purge ne doit jamais le retirer en cours. |

Le TTL part du **dernier fichier envoyé** pour cette version. Un pipeline qui continue d'envoyer des fichiers garde son staging en vie.

## Abandonner un upload à la main

Pour libérer de la place tout de suite, supprimez la version en staging pendant qu'aucun upload ne tourne pour elle :

```sh
rm -rf /srv/registry/bin.local/.uploads/<product>/<version>
```

Pour corriger une publication refusée, gardez au contraire le staging : envoyez le fichier manquant ou corrigé sous le même nom, puis publiez de nouveau. Voir [Dépannage](../troubleshooting.md).

## Surveiller la zone de staging

```sh
du -sh /srv/registry/bin.local/.uploads/*/*          # versions en staging, par taille
find /srv/registry/bin.local/.uploads -mindepth 2 -maxdepth 2 -type d -mtime +1   # plus vieilles qu'un jour
jq -c 'select(.event=="staging_purged")' /srv/registry/bin.local/audit.log
```

Une zone de staging qui se remplit régulièrement signifie que des pipelines envoient sans jamais publier, en général parce que la publication est refusée. Les événements `publish_refused` du journal d'audit disent pourquoi.
