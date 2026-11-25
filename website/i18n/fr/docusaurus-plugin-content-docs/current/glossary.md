---
title: Glossaire
description: Chaque terme de Local Registry, défini en une ou deux lignes.
---

Le vocabulaire employé dans cette documentation, par ordre alphabétique.

| Terme | Définition |
| --- | --- |
| **Artefact** | Un fichier téléchargeable d'une version : une archive, un binaire, un paquet, un script SQL… Les fichiers de checksums, signatures, SBOM et documents du standard ne sont pas des artefacts ; ils les décrivent ou les couvrent. |
| **artifacts.json** | La liste de fichiers que GoReleaser écrit dans `dist/`. Facultatif. Quand vous l'envoyez, le serveur le contrôle et transforme ses chemins en URLs de téléchargement. |
| **Dernière version (latest)** | La version qu'un produit désigne comme courante, choisie explicitement à la publication. Consultez [Dernière version](./concepts/latest.md). |
| **Document de version** | `release.json` : la description normalisée d'une version, écrite par le serveur à la publication. Consultez [release.json](./reference/release-json.md). |
| **Dossier de stockage** | Le dossier donné par `--asset-path`. Il contient toutes les versions publiées, le staging et le journal d'audit. Consultez [Stockage et sauvegarde](./operations/storage-and-backup.md). |
| **Fichier de checksums** | Un fichier texte de lignes `<sha256>  <nom>`, une par artefact. Envoyé (le `*checksums.txt` de GoReleaser) ou écrit par le serveur sous le nom `checksums.txt`. |
| **Fiche produit** | `registry.yaml` : la description d'un produit (nom, responsables, liens, licence…), envoyée avec chaque version. Obligatoire. Consultez [registry.yaml](./reference/registry-yaml.md). |
| **index.json** | Le document qui décrit un produit : sa fiche et ses versions. Écrit par le serveur. Consultez [index.json](./reference/index-json.md). |
| **metadata.json** | La description d'un build par GoReleaser (tag, commit, date). Facultatif. Ses valeurs alimentent `release.json`. |
| **Notes de version** | La liste typée des changements d'une version (*added*, *fixed*, *security*…), extraite de `CHANGELOG.md`. |
| **Pièce jointe** | Une signature, un certificat ou un SBOM rattaché au fichier qu'il couvre d'après son nom (`<fichier>.sig`, `<fichier>.sbom.json`…). Consultez [Intégrité](./concepts/integrity.md). |
| **Produit** | Ce que vous livrez et versionnez, comme `bridge`. Le premier segment de chaque URL. Consultez [Produits et versions](./concepts/products-and-versions.md). |
| **Publication** | L'étape qui contrôle une version en staging et la rend visible, de façon atomique. Consultez [Staging et publication](./concepts/staging-and-publish.md). |
| **Staging** | La zone privée où les fichiers d'une version attendent sa publication. Purgée après `--staging-ttl` d'inactivité. |
| **token_id** | Les 12 premiers chiffres hexadécimaux du sha256 du token d'upload. Identifie le token dans les logs et dans `publishedBy` sans le révéler. |
| **Token d'upload** | Le secret partagé que les éditeurs envoient sous la forme `Authorization: Bearer <token>`. La lecture n'en demande pas. Consultez [Sécurité](./operations/security.md). |
| **Arborescence historique** | `assets/binaries/<product>/<version>/` dans le dossier de stockage : des fichiers déposés à la main dans le dossier de stockage, hors de l'API d'upload. Lue en repli par `POST /releases/{product}/{version}` et servie par `/assets/binary`. Consultez [Routes historiques](./reference/api/legacy.md). |
| **URL publique** | L'adresse par laquelle les clients joignent le serveur (`--public-url`), par exemple derrière un reverse proxy. Quand elle est définie, les URLs de téléchargement des documents sont absolues. |
| **Version** | Une livraison publiée et immuable d'un produit, en général nommée d'après un tag git (`v1.4.2`). |
