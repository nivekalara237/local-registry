---
title: Changer la dernière version
sidebar_label: Changer la dernière version
description: Choisir la version que les utilisateurs reçoivent comme dernière, à la publication ou ensuite, et revenir en arrière après une mauvaise release.
---

Les utilisateurs qui demandent `latest` reçoivent une version du produit. Ce guide montre comment la choisir à la publication, comment la changer ensuite, et comment revenir en arrière après une mauvaise release. Le fonctionnement de `latest` est expliqué dans [Dernière version](../concepts/latest.md).

## À la publication

Passez `--latest` à `upload.sh`, ou `?latest=true` à l'appel de publication :

```sh
ci/upload.sh --latest sonarbridge dist/artifacts.json
```

Une version publiée sans `--latest` est disponible par son numéro, mais ne change pas `latest`. C'est ce qu'il faut pour les pré-versions et pour les correctifs d'anciennes branches (par exemple une `v1.3.9` publiée après la `v1.4.2`).

> [!NOTE]
> On ne publie pas deux fois une version : une fois publiée, `publish` répond `409` pour elle, avec ou sans `latest`. Pour changer `latest` ensuite, suivez la procédure ci-dessous.

## Ensuite : sur le serveur

Aucune route de l'API ne change `latest`. Un exploitant le fait sur le serveur, dans l'arborescence publiée, en copiant deux fichiers.

La dernière version est celle que désigne `<product>/release.json`. Pour les clients goreleaser, `<product>/artifacts.json` est une copie de l'`artifacts.json` de cette version. Changez les deux ensemble.

1. **Allez dans le dossier du produit.**

   ```sh
   cd <asset-path>/bin.local/assets/binaries/sonarbridge
   ls     # un dossier par version publiée, plus release.json, index.json…
   ```

2. **Copiez les fichiers de la version choisie.** Passez par un fichier temporaire et `mv`, pour qu'aucun lecteur ne voie un fichier à moitié écrit.

   ```sh
   v=v1.4.2
   cp "$v/release.json" .release.json.tmp && mv .release.json.tmp release.json
   if [ -f "$v/artifacts.json" ]; then
     cp "$v/artifacts.json" .artifacts.json.tmp && mv .artifacts.json.tmp artifacts.json
   else
     rm -f artifacts.json   # la version a été publiée sans goreleaser
   fi
   ```

3. **Vérifiez.** Le changement est visible tout de suite : les routes lisent ces fichiers à chaque requête.

   ```sh
   curl -fsS https://registry.example.com/sonarbridge/releases/latest | jq -r .version
   curl -fsS https://registry.example.com/sonarbridge/index | jq -r .latest
   ```

Le fichier `index.json` sur disque n'est réécrit qu'à la prochaine publication du produit. `GET /sonarbridge/index` ne le lit pas : il reconstruit toujours l'index depuis le disque.

## Revenir en arrière après une mauvaise release

Supposons que `v1.5.0` a été publiée comme dernière version et qu'elle est défectueuse. Vous avez deux options.

| Option | Quand | Comment |
| --- | --- | --- |
| **Publier un correctif** (recommandé) | Vous pouvez construire `v1.5.1` rapidement | Publiez `v1.5.1` avec `--latest`. L'historique reste clair, et tous les utilisateurs avancent. |
| **Faire pointer latest en arrière** | Il vous faut l'ancienne version tout de suite | Suivez la procédure ci-dessus avec `v=v1.4.2`. `v1.5.0` reste téléchargeable par son numéro. |

Pour prévenir ceux qui ont déjà pris `v1.5.0`, publiez la version suivante avec un message `deprecated` dans sa fiche. Voir [Écrire la fiche produit](write-registry-yaml.md).

## Supprimer `latest`

Supprimez `<product>/release.json` et `<product>/artifacts.json`. `/releases/latest` répond alors `404`, et `latest` vaut `null` dans l'index, jusqu'à la prochaine publication avec `--latest`.

## À savoir

- Les versions publiées avant le standard `release.json` ne peuvent pas être la dernière version : elles n'ont pas de `release.json` à copier.
- Un `<product>/release.json` qui nomme une version inexistante ne désigne aucune version : `latest` répond `404`.
- Gardez une trace des changements manuels : ils ne passent pas par l'API, le [journal d'audit](../operations/audit-log.md) ne les voit donc pas.
