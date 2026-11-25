---
title: Intégrité
description: Comment le registre garantit que chaque fichier publié est exactement celui qui a été construit, et comment les consommateurs le vérifient.
---

Un registre n'est utile que si l'on peut se fier à ce qu'on y télécharge. Local Registry contrôle chaque fichier avant de le publier, enregistre son SHA-256 et donne aux consommateurs de quoi le vérifier eux-mêmes. Cette page décrit les contrôles, et ce qui reste de votre responsabilité.

## Trois niveaux de contrôle

![Les trois niveaux de contrôle : à l'envoi, à la publication, au téléchargement](/img/three-layers-of-checks.svg)

**1. À l'envoi.** Chaque requête `PUT` peut inclure le SHA-256 calculé par l'éditeur dans l'en-tête `X-Checksum-Sha256`. Le serveur calcule également le SHA-256 du fichier reçu et rejette la requête en cas de différence (`422 sha256 mismatch`), avant même que le fichier ne soit placé en staging.

Cet en-tête est facultatif. Pour l'envoyer depuis vos propres clients, consultez l'[API d'upload](../reference/api/upload.md).

**2. À la publication.** Le serveur recalcule le SHA-256 de chaque fichier présent en staging, puis vérifie la cohérence de l'ensemble avec le fichier de checksums :

- Si vous avez fourni un fichier de checksums (par exemple, celui généré par GoReleaser avec le motif `*checksums.txt`), chaque fichier doit y être référencé avec une somme SHA-256 correspondante. Toute entrée manquante ou toute différence entraîne le rejet de la publication.
- Si aucun fichier de checksums n'a été fourni, le serveur génère lui-même un fichier `checksums.txt`, au format `<sha256>  <nom>`, à partir des fichiers qu'il vient de vérifier.

Exemple de contenu d'un fichier de checksums (sommes abrégées) :

```text
3f1c…  bridge_1.4.2_darwin_arm64.tar.gz
8a07…  bridge_1.4.2_linux_amd64.tar.gz
c9e2…  bridge_1.4.2_windows_amd64.zip
```

Le fichier de checksums ne se liste pas lui-même.

Dans les deux cas, chaque version publiée dispose ainsi d'un fichier de checksums cohérent avec l'ensemble de ses fichiers.

**3. Au téléchargement.** Le consommateur peut vérifier l'intégrité des fichiers téléchargés en comparant leur SHA-256 à celui indiqué dans `release.json` ou dans le fichier de checksums fourni avec la version.

Par exemple, pour télécharger une archive et vérifier son intégrité à l'aide du fichier de checksums :

```sh
curl -fsSLO https://registry.corp.example/bridge/artifacts/v1.4.2/files/bridge_1.4.2_linux_amd64.tar.gz

curl -fsSL https://registry.corp.example/bridge/artifacts/v1.4.2/files/bridge_1.4.2_checksums.txt \
  | sha256sum --check --ignore-missing
```

Consultez [Consommer les versions](../guides/consume-releases.md) pour des scripts clients complets.

## Les fichiers sans ligne de checksum

Certains fichiers sont volontairement exclus du fichier de checksums :

- La fiche produit (`registry.yaml`).
- Les notes de version (`CHANGELOG.md`).
- Les métadonnées générées par GoReleaser (`metadata.json`, `artifacts.json`).
- Les documents générés par le serveur (`index.json`, `release.json`, etc.).
- Les fichiers de signature (`*.sig`, `*.pem`, `*.sigstore.json`), qui servent notamment à signer le fichier de checksums lui-même.

Bien qu'ils soient exclus du fichier de checksums, leurs sommes SHA-256 sont systématiquement enregistrées dans le [journal d'audit](../operations/audit-log.md) lors de la publication.

## Signatures et SBOM

Lorsque vous publiez des signatures, des certificats ou des SBOM (via les directives `signs:` et `sboms:` de GoReleaser), le serveur associe chaque fichier au fichier correspondant qu'il couvre, en se basant sur son nom. Il reconnaît notamment les suffixes suivants : `<fichier>.sig`, `<fichier>.pem`, `<fichier>.sbom.json`, ainsi que leurs variantes équivalentes.

Le fichier `release.json` expose alors, pour chaque artefact, les URLs associées à son SBOM (`sbom`), sa signature (`signature`) et son certificat (`certificate`). Ce mécanisme s'applique également au fichier de checksums.

> [!IMPORTANT]
> Le serveur se contente d'associer les signatures et les certificats aux fichiers correspondants : il ne vérifie pas leur validité cryptographique. La vérification des signatures (avec Cosign ou GPG), conformément à votre politique de confiance, relève de la responsabilité du consommateur.
>
> Il est recommandé de signer le fichier de checksums : une seule signature permet ainsi de couvrir l'ensemble des fichiers référencés, sans avoir à signer chaque artefact individuellement.

## Ce dont le registre ne vous protège pas

Les limites, sans détour :

| Menace | Couverte ? |
| --- | --- |
| Un fichier corrompu en route vers le registre, ou un mauvais fichier envoyé | **Oui**, par les contrôles à l'envoi et à la publication |
| Un fichier publié modifié par l'API | **Oui** : les versions publiées sont immuables |
| Quelqu'un ayant un accès en écriture au dossier de stockage qui modifie un fichier | **Non** : protégez le dossier et sauvegardez-le. Consultez [Stockage et sauvegarde](../operations/storage-and-backup.md) |
| Un fichier altéré sur le réseau vers le consommateur | **Seulement avec HTTPS**, ou en vérifiant les signatures. Consultez [Sécurité](../operations/security.md) |
| Un build malveillant publié avec un token valide | **Non** : protégez le token, signez vos versions |

## Voir aussi

- [Staging et publication](./staging-and-publish.md)
- [Règles d'upload](../reference/upload-rules.md)
