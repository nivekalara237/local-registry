---
title: Publier depuis Windows
sidebar_label: Windows
description: Publier depuis Windows avec le binaire natif de localrectl, en évitant les pièges des chemins et des certificats.
---

[`localrectl`](../reference/localrectl.md) existe en binaire Windows natif : ni bash, ni curl, ni jq ne sont nécessaires pour publier. Cette page couvre ce qui est propre à Windows. Les étapes elles-mêmes sont celles de [Publier avec GoReleaser](publish-with-goreleaser.md) et de [Publier des fichiers](publish-plain-files.md).

## Installer localrectl

Téléchargez le binaire Windows sur la [page des releases de localrectl](https://github.com/nivekalara237/localrectl/releases), vérifiez-le avec le fichier de checksums publié avec lui, puis placez-le dans un dossier de votre `PATH`.

Vérifiez qu'il joint le registre :

```powershell
$env:LOCAL_REGISTRY_URL = "https://registry.corp.example"
localrectl ping
```

`localrectl completion powershell` affiche le script de complétion pour PowerShell.

## Publier

```powershell
$env:LOCAL_REGISTRY_URL = "https://registry.corp.example"
$env:LOCAL_REGISTRY_UPLOAD_TOKEN = "…"   # depuis votre coffre de secrets, jamais en ligne de commande

localrectl publish bridge --goreleaser-dir dist --product-file registry.yaml --changelog-file dist/CHANGELOG.md --latest
```

`localrectl` lit le token dans `LOCAL_REGISTRY_UPLOAD_TOKEN` et ne l'affiche jamais. Ajoutez `--dry-run` pour voir ce qui serait envoyé, sans rien envoyer.

## Télécharger

Sans sélection, `localrectl download` prend l'archive de la machine : le zip sous Windows.

```powershell
localrectl download bridge --extract -d C:\tools\bridge
```

## Pièges

### Fins de ligne

`registry.yaml`, `CHANGELOG.md` et les fichiers de checksums peuvent utiliser CRLF : le serveur accepte les deux fins de ligne.

### Noms de fichiers

Les noms de fichiers publiés sur le registre ne peuvent utiliser que `A-Z a-z 0-9 . _ -`. Renommez les fichiers qui contiennent des espaces ou des accents avant de publier.

### Certificats privés

Si le registre utilise un certificat émis par une autorité de certification privée, indiquez le certificat de cette autorité à `localrectl` :

```powershell
$env:LOCAL_REGISTRY_CA_FILE = "C:\certs\corp-ca.pem"
```

D'autres solutions dans [Dépannage](../troubleshooting.md).
