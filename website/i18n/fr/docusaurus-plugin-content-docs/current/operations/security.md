---
title: Sécurité
description: Le modèle de sécurité de local-registry, ce qui est public, le fonctionnement du token d'upload, et le déploiement derrière HTTPS.
---

Cette page décrit ce que local-registry protège, ce qu'il laisse ouvert volontairement, et comment le déployer sans risque. Lisez-la avant d'exposer le serveur au-delà de `localhost`.

## Le modèle en un tableau

| Action | Qui peut la faire | Protection |
| --- | --- | --- |
| Lire le catalogue, `release.json`, `index.json` | Quiconque atteint le serveur | Aucune : les lectures sont publiques. |
| Télécharger un fichier publié | Quiconque atteint le serveur | Aucune. L'intégrité vient des checksums sha256. Voir [Intégrité](../concepts/integrity.md). |
| Envoyer des fichiers, publier une version | Les détenteurs du token d'upload | `Authorization: Bearer <token>`, comparé en temps constant. |
| Modifier ou supprimer une version publiée | Personne par l'API | Les versions publiées sont immuables (`409`). Seule une personne ayant accès au disque peut les modifier. |
| Lire le journal d'audit, la zone de staging | Personne par l'API | Jamais servis. |

> [!IMPORTANT]
> local-registry n'a pas d'authentification en lecture. Placez-le sur un réseau privé, ou derrière un proxy qui authentifie les lecteurs, si vos releases sont confidentielles.

## Le token d'upload

Il y a un token d'upload par serveur : un secret partagé que chaque publieur présente.

- **D'où il vient** : `LOCAL_REGISTRY_UPLOAD_TOKEN`, sinon la clé `upload-token` du fichier de configuration. Il n'y a pas de flag, car les flags apparaissent dans `ps`.
- **Sans token**, l'upload est désactivé : les routes d'upload répondent `403 upload is disabled: no token configured`, et le démarrage affiche `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled`. C'est le bon réglage pour un miroir en lecture seule.
- **Un token faux ou absent** reçoit `401 invalid or missing bearer token` avec `WWW-Authenticate: Bearer`. La tentative est écrite dans le [journal d'audit](audit-log.md) en `auth_failed`.
- **Le token n'est jamais affiché ni journalisé.** Les logs, le journal d'audit et `local-registry config` montrent son `token_id` à la place : les 12 premiers chiffres hexadécimaux de son sha256. Calculez-le vous-même pour rapprocher une ligne de log d'un token :

  ```sh
  printf %s "$LOCAL_REGISTRY_UPLOAD_TOKEN" | shasum -a 256 | cut -c1-12
  ```

  `release.json` enregistre le `token_id` qui a publié chaque version, dans `publishedBy`.

### Créer et conserver le token

```sh
openssl rand -hex 32
```

Rangez-le dans votre gestionnaire de secrets, puis transmettez-le au serveur et aux pipelines CI qui publient :

- systemd : dans le fichier de configuration, appartenant à l'utilisateur du service, mode `0600` ;
- Docker : `-e LOCAL_REGISTRY_UPLOAD_TOKEN` depuis l'environnement du shell ou de votre orchestrateur ;
- Kubernetes : un `Secret`, lu par `secretKeyRef` (voir [Installation](install.md)) ;
- CI : une variable secrète, masquée et protégée.

Les clients doivent envoyer le token sur l'entrée standard, jamais en argument de commande. Les [scripts d'upload](../reference/scripts.md) le font déjà :

```sh
printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" |
  curl -fsS -H @- -X POST "https://registry.example.com/my-product/artifacts/v1.2.0/publish"
```

### Changer de token

Le serveur lit le token au démarrage.

1. Générez un nouveau token et rangez-le dans votre gestionnaire de secrets.
2. Mettez à jour la configuration ou le secret du serveur, puis redémarrez-le. Les uploads en staging survivent au redémarrage.
3. Mettez à jour les secrets de la CI. Les publieurs qui utilisent encore l'ancien token reçoivent `401`.

Le nouveau `token_id` apparaît dans les logs à partir du redémarrage : vous distinguez les versions publiées avant et après le changement.

> [!WARNING]
> `--disable-generate-token=false` affiche un token généré sur la sortie standard. systemd, Docker, Kubernetes et la CI conservent souvent cette sortie. Utilisez un token fixe partout où les logs sont conservés.

## HTTPS

Sans TLS, tout circule en clair, token compris. Quand l'upload est activé sur une adresse autre que loopback sans TLS, le serveur démarre avec cet avertissement :

```
warning: upload is enabled on 0.0.0.0 without TLS: the token travels in clear text (use --tls-cert/--tls-key or a TLS reverse proxy)
```

Vous avez deux options.

### Option 1 : TLS dans le serveur

```sh
local-registry serve --asset-path /srv/registry --address 0.0.0.0 --port 8443 \
  --tls-cert /etc/local-registry/fullchain.pem --tls-key /etc/local-registry/privkey.pem
```

- Le serveur ne parle que HTTPS, en TLS 1.2 ou plus. Une requête HTTP simple reçoit `400 Client sent an HTTP request to an HTTPS server.`
- Le certificat est chargé au démarrage. Une paire invalide arrête le serveur avec le code de sortie `1`. Redémarrez-le pour charger un certificat renouvelé.

### Option 2 : un reverse proxy TLS (recommandé)

Terminez TLS dans votre load balancer, votre contrôleur Ingress ou votre proxy, et gardez le registre sur une adresse privée. Configurez le proxy pour les uploads :

| Réglage du proxy | Valeur |
| --- | --- |
| Taille du corps de requête | Au moins `--max-upload-mb` (200 Mio par défaut). |
| Délais de lecture et d'envoi sur `/…/artifacts/…/files/…` et `/…/publish` | Jusqu'à 30 minutes, la limite d'upload du serveur. |
| Mise en mémoire tampon des requêtes | Désactivée pour les uploads si votre proxy le permet, pour que les gros fichiers passent en flux. |

Réglez ensuite `public-url` sur l'adresse qu'utilisent les clients (`https://registry.example.com`), pour que les URL de téléchargement écrites par le registre pointent vers le proxy.

Le serveur journalise l'en-tête `X-Forwarded-For` qu'il reçoit, dans `forwarded_for`, sans lui faire confiance : le champ `remote` est toujours le pair TCP, ici votre proxy.

## Ce qui est public sur le disque

Seules deux arborescences de l'asset path sont servies :

| Arborescence | Contenu | Servie |
| --- | --- | --- |
| `bin.local/assets/` | Versions publiées | Oui |
| `assets/` | Ancienne arborescence, lue par `POST /releases` | Oui |
| `bin.local/.uploads/` | Staging : envoyé, pas encore publié | Jamais |
| `bin.local/.uploads-tmp/` | Fichiers en cours de réception | Jamais |
| `bin.local/audit.log` | Journal d'audit, mode `0600` | Jamais |
| Tout le reste | | Jamais |

- Les fichiers et répertoires cachés (noms commençant par `.`) ne sont jamais servis.
- Les routes de téléchargement refusent un lien symbolique qui sort du répertoire de la version. Les routes du catalogue et d'upload suivent les liens trouvés sur le disque : ne mettez pas de liens dans l'asset path.
- Le serveur refuse de démarrer si `--audit-log` pointe dans une arborescence publique.

N'accordez l'écriture dans l'asset path qu'à l'utilisateur du service. Qui peut y écrire peut modifier les fichiers publiés.

## Liste de contrôle de durcissement

- [ ] Token d'upload de 32 octets aléatoires, dans un gestionnaire de secrets, jamais dans un dépôt.
- [ ] HTTPS, dans le serveur ou dans un reverse proxy ; `public-url` réglé sur l'adresse HTTPS.
- [ ] Le service tourne sous un utilisateur dédié et sans privilège (`registry`, uid `10001` dans l'image).
- [ ] L'asset path et le fichier de configuration appartiennent à cet utilisateur seul ; le fichier est en mode `0600`.
- [ ] L'accès en lecture est restreint au niveau réseau si les releases sont confidentielles.
- [ ] Le journal d'audit est collecté et conservé. Voir [Journal d'audit](audit-log.md).
- [ ] Les images conteneur sont épinglées sur une version exacte et leur signature est vérifiée. Voir [Installation](install.md).
- [ ] Un serveur par asset path.
