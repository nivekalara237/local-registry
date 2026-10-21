---
title: Publier depuis Windows
sidebar_label: Windows
description: Lancer upload.sh sous Windows avec Git Bash, en évitant les pièges des fins de ligne et des chemins.
---

`upload.sh` est un script bash. Sous Windows, lancez-le depuis **Git Bash** (installé avec Git for Windows), MSYS2 ou WSL. Cette page liste les outils à installer et les pièges propres à Windows. Les étapes elles-mêmes sont celles de [Publier avec goreleaser](publish-with-goreleaser.md) et de [Publier des fichiers](publish-plain-files.md).

## Installer les outils

| Outil | D'où il vient |
| --- | --- |
| `bash`, `sha256sum`, `tar` | Git for Windows (Git Bash) |
| `curl` | Windows 10 et suivants, et Git for Windows |
| `jq` | Non fourni : installez `jq.exe` depuis [jqlang.org](https://jqlang.org/download/), ou avec un gestionnaire de paquets comme `winget` ou `scoop`, et vérifiez qu'il est dans le `PATH` de Git Bash |

Vérifiez-les dans Git Bash :

```sh
bash --version && curl --version && jq --version && sha256sum --version
```

## Publier

```sh
export LOCAL_REGISTRY_URL=https://registry.example.com
export LOCAL_REGISTRY_UPLOAD_TOKEN=…   # depuis votre coffre de secrets
./ci/upload.sh --latest --root . sonarbridge dist/artifacts.json
```

Lancez-le depuis Git Bash, pas depuis PowerShell ni `cmd.exe`. Dans WSL, tout se passe comme sous Linux : installez-y le `jq` Linux.

## Pièges

### `jq.exe` et les fins de ligne

Le `jq.exe` natif termine ses lignes par CRLF. Un retour chariot égaré dans un nom ou un checksum casse l'URL, et le serveur répond `400 Bad Request`. `upload.sh` retire lui-même ces retours chariot. Si vous voyez des lignes `)UT …` ou un `400` avec une ancienne copie du script, mettez-la à jour depuis une release récente de local-registry.

> [!WARNING]
> `publish-to-local-registry.sh` ne gère pas la sortie de `jq.exe`, et vise les systèmes POSIX. Utilisez `upload.sh` sous Windows.

### Scripts récupérés en CRLF

Si Git convertit les fins de ligne (`core.autocrlf=true`), un script récupéré en CRLF échoue avec `$'\r': command not found`. Gardez les scripts shell en LF avec un fichier `.gitattributes` à la racine de votre dépôt :

```
*.sh text eol=lf
```

Puis récupérez à nouveau les fichiers : `git rm --cached -r . && git reset --hard`. Commitez ou mettez de côté vos modifications avant de lancer cette commande.

`registry.yaml`, `CHANGELOG.md` et les fichiers de checksums peuvent être en CRLF : le serveur accepte les deux fins de ligne.

### Chemins

- Utilisez des barres obliques et des chemins relatifs : `dist/artifacts.json`, `--root .`.
- Passez `--root .` quand vous lancez le script depuis le dossier où goreleaser a tourné. Le script lit alors chaque `path` d'`artifacts.json` à partir de là.
- Les noms de fichiers publiés ne peuvent utiliser que `A-Z a-z 0-9 . _ -`. Renommez les fichiers qui contiennent des espaces ou des accents avant de publier.

### Certificats auto-signés

Si le registre utilise un certificat de l'autorité de certification de votre entreprise, indiquez-la à curl :

```sh
export CURL_CA_BUNDLE=/c/certs/corp-ca.pem
```

Plus de solutions dans [Dépannage](../troubleshooting.md).
