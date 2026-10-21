---
title: Dépannage
description: Symptômes, causes et corrections des problèmes rencontrés en exploitant local-registry et en y publiant.
---

Cherchez votre symptôme ci-dessous : chaque ligne donne la cause et la correction. La plupart des messages viennent tels quels du serveur ou des scripts. Pour la liste complète des messages d'erreur, par route, voir la [référence des erreurs](reference/errors.md).

> [!TIP]
> Deux commandes répondent à la plupart des questions. `local-registry config` montre chaque réglage et sa provenance. La sortie d'erreur du serveur donne la cause exacte de chaque requête en échec, sur une ligne `WARN` ou `ERROR`.

## Le serveur ne démarre pas

| Symptôme | Cause | Correction |
| --- | --- | --- |
| `Error: asset-path is required (…)`, code `2` | Aucun asset path configuré. Le serveur n'utilise jamais le répertoire de travail par défaut. | Réglez `--asset-path`, `LOCAL_REGISTRY_ASSET_PATH` ou `asset-path`. |
| `Error: asset-path <dir> is not an existing directory` | Le répertoire n'existe pas, ou c'est un fichier. | Créez-le, avec l'utilisateur du service comme propriétaire. |
| `Error: config file <file>: unknown setting "…"` | Une faute de frappe ou une clé imbriquée dans le fichier de configuration. | Corrigez la clé. Les noms sont ceux des flags. |
| `Error: unknown flag: --port` | Le flag a été donné avant la commande. | Placez les flags après `serve` : `local-registry serve --port 9000`. |
| `Error: unexpected argument "false"` | Un flag booléen écrit avec une espace. | Écrivez `--disable-generate-token=false`. |
| `bind: address already in use`, code `1` | Un autre processus écoute sur le port. | Trouvez-le avec `lsof -i :8077`, ou changez `--port`. |
| `Error: loading TLS certificate: …`, code `1` | Un fichier manquant, ou une clé qui ne correspond pas au certificat. | Vérifiez les deux chemins, et que la clé appartient au certificat. |
| `Error: … staging-ttl must be 0 or at least 1h0m0s` | Le TTL est trop court pour un upload de 30 minutes. | Utilisez `0` ou au moins `1h`. |
| Un réglage a une valeur inattendue | Une variable `LOCAL_REGISTRY_*` oubliée l'emporte sur le fichier. Dans un conteneur, l'image règle l'asset path, l'adresse et le port. | Lancez `local-registry config` : la colonne `SOURCE` montre d'où vient chaque valeur. |

## Authentification

| Symptôme | Cause | Correction |
| --- | --- | --- |
| `403 upload is disabled: no token configured` | Le serveur n'a pas de token d'upload. Le démarrage a affiché `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled`. | Réglez `LOCAL_REGISTRY_UPLOAD_TOKEN` ou `upload-token` pour le serveur, puis redémarrez. Vérifiez avec `local-registry config`. |
| `401 invalid or missing bearer token` | Token faux, en-tête faux, ou token changé sans redémarrage. | Envoyez exactement `Authorization: Bearer <token>`, sans espace ni retour à la ligne en trop. Comparez les `token_id` : voir [Sécurité](operations/security.md). |
| `WARN upload token generation skipped …` | `--disable-generate-token=false` alors qu'un token est configuré. Le token configuré reste utilisé. | Normal. Retirez le flag, ou retirez le token configuré pour en obtenir un généré. |
| `curl: (60) SSL certificate problem` | Certificat auto-signé ou autorité de certification privée. | `curl --cacert ca.pem`, ou `export CURL_CA_BUNDLE=ca.pem` pour les scripts. |
| `400 Client sent an HTTP request to an HTTPS server.` | Du HTTP simple vers un serveur en TLS. | Utilisez `https://`. |

## Upload et publication

| Symptôme | Cause | Correction |
| --- | --- | --- |
| `422 registry.yaml not uploaded: every version needs its product card` | Pas de fiche produit en staging. | Ajoutez un `registry.yaml` (voir [Écrire registry.yaml](guides/write-registry-yaml.md)), ou passez `upload.sh --product-file`. Puis publiez de nouveau : le reste du staging est conservé. |
| `422 no artifact to publish: upload at least one file besides registry.yaml` | Le staging ne contient que des documents : `registry.yaml`, `metadata.json`, `CHANGELOG.md`, checksums. | Envoyez les fichiers que vous livrez. |
| `422 artifacts.json references <name>, which is not uploaded` | `artifacts.json` a été envoyé avant un fichier qu'il liste. | Envoyez ce fichier, puis de nouveau `artifacts.json`. `upload.sh` envoie les fichiers dans le bon ordre. |
| `422 <name> is uploaded but artifacts.json does not list it` | Un fichier en trop dans le staging : une tentative précédente, un binaire de build brut. | Supprimez le staging (`bin.local/.uploads/<product>/<version>`) et renvoyez, ou utilisez `upload.sh`. |
| `422 <name> is not listed in <checksums>` | Un fichier que le fichier de checksums ne couvre pas. | Ne l'envoyez pas, faites-le inclure par votre build, ou n'envoyez pas de fichier de checksums : la publication en écrit un. |
| `422 <name>: sha256 is … but … says …` | Le fichier a changé après le calcul de son checksum, ou le mauvais fichier a été envoyé. | Envoyez le bon fichier. |
| `422 metadata.json describes …, not …` | La version de l'URL n'est pas le tag de `metadata.json`. | Publiez sous le tag. Les scripts le lisent dans `metadata.json`. |
| `422 several checksums files: a and b` | Deux fichiers dont le nom finit par `checksums.txt`. | N'en gardez qu'un. |
| `409 version … is already published and cannot be changed` | Les versions publiées sont immuables. | Publiez un nouveau numéro de version. Voir [Produits et versions](concepts/products-and-versions.md). |
| `400 version name release.json is reserved` (ou `index.json`, `artifacts.json`, `artifacts.goreleaser.json`) | Ces noms sont des fichiers du répertoire du produit. | Choisissez un autre nom de version. |
| `413 file exceeds … bytes` | Un fichier dépasse `max-upload-mb`. | Relevez `max-upload-mb`. Voir [Quotas et staging](operations/quotas-and-staging.md). |
| `413 … would exceed its staging quota` / `422 … staged files (limit …)` | La version dépasse `max-version-mb`, ou a plus de fichiers que `max-version-files`. | Relevez le quota, ou retirez des fichiers du staging. |
| `507 upload staging area is full …` | Trop d'uploads non publiés, toutes versions confondues. | Publiez ou abandonnez les versions en attente, ou relevez `max-staging-mb`. |
| `507 not enough free disk space on the registry` | L'upload laisserait moins de `min-free-mb` libre. | Libérez de l'espace ou agrandissez le volume. |
| Upload coupé par un reverse proxy (`413` ou `504` venant du proxy) | Le proxy limite la taille du corps ou la durée. | Relevez la limite de taille et les délais du proxy. Voir [Sécurité](operations/security.md). |
| Le fichier téléchargé commence par `--------------------------` | Le fichier a été envoyé avec `curl -F` (multipart). | Envoyez avec `curl -T fichier`. |

## Scripts

| Symptôme | Cause | Correction |
| --- | --- | --- |
| `upload.sh: <name>: <path> not found (wrong --root?)` | Les champs `path` d'`artifacts.json` sont relatifs au répertoire de travail de goreleaser, pas à l'endroit où vous lancez le script. | Passez `--root <répertoire de travail de goreleaser>`, ou lancez le script depuis là. |
| `upload.sh: … not found` pour des fichiers d'un autre projet | Pas d'argument `artifacts.json` : le script a pris le `dist/artifacts.json` du répertoire courant. | Passez l'`artifacts.json` ou le répertoire à publier. Avec `--root DIR` sans fichier, le script lit `DIR/dist/artifacts.json`, sinon publie `DIR` lui-même. |
| `upload.sh: <dir> is a directory: archive it first …` | Le répertoire à publier contient un sous-répertoire. Le registre stocke des fichiers à plat. | Archivez le sous-répertoire, par exemple `tar czf name.tar.gz -C dir name`. Voir [Publier des fichiers simples](guides/publish-plain-files.md). |
| `upload.sh: no registry.yaml in … : pass --product-file` | Aucune fiche produit trouvée. | Passez `--product-file chemin/vers/registry.yaml`. |
| Sous Windows, les lignes de sortie commencent par `)` et les uploads échouent en `400 Bad Request` | Un `jq.exe` natif termine ses lignes par CRLF ; le CR se retrouve dans les noms et les URL. | Utilisez le `upload.sh` actuel, qui retire les CR. Voir [Windows](guides/windows.md). |

## Lectures et téléchargements

| Symptôme | Cause | Correction |
| --- | --- | --- |
| `404` sur `/<product>/releases/latest` | Aucune version de ce produit n'a été publiée avec `latest=true`, ou `<product>/release.json` désigne une version non publiée. | Publiez avec `--latest`, ou voir [Changer la dernière version](guides/change-latest.md). |
| `404` sur `/<product>/artifacts` ou `/<product>/artifacts/<version>` | La version a été publiée sans `artifacts.json` (fichiers simples). | Lisez plutôt `/<product>/releases/<version>` : il existe pour chaque version. |
| `404` sur `/artifacts/metadatas/<product>` | Pas de dernière version, ou une version publiée sans `metadata.json`. | Lisez `/<product>/releases/<version>`. |
| `404` sur `POST /releases/v1.2.3` | Cette route dépréciée ne lit que le produit `gitlab`. | Utilisez `/releases/<product>/v1.2.3`. |
| `404` sur `/assets/binary/…` | Le chemin doit être relatif à l'asset path, chaque `/` encodé en `%2F`. Le staging, les fichiers cachés et les liens qui sortent de la version ne sont jamais servis. | Utilisez plutôt les URL données par `release.json`. |
| Valeurs `path` ou `url` relatives | `public-url` n'est pas réglé. | Réglez-le. Il s'applique aux versions publiées ensuite. Voir [Configuration](operations/configuration.md). |
| Une version ou un produit manque dans `/products` ou `/<product>/index` | La version n'a pas de `release.json` : elle a été publiée avant le standard `local-registry/v1`. | Publiez-la de nouveau sous un nouveau numéro de version. |
| Téléchargement coupé après 20 secondes | Le délai d'écriture des routes de lecture. | Reprenez avec `curl -C -`. Voir [Supervision](operations/monitoring.md). |
| `301` inattendu | `..` ou `//` dans l'URL. | Nettoyez l'URL. |
