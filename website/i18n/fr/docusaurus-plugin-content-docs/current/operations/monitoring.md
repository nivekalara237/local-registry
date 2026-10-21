---
title: Supervision
description: Health check, sortie au démarrage, logs de requêtes, codes de sortie et délais du serveur.
---

local-registry expose un point de santé, écrit une ligne de log par requête, et utilise des codes de sortie sur lesquels votre superviseur peut agir. Cette page les liste, avec ce qui mérite une alerte. Le serveur n'a pas de point de métriques : tirez vos métriques des logs de requêtes et du [journal d'audit](audit-log.md).

## Health check

```sh
curl -fsS http://localhost:8077/healthz
```

```json
{"status":"ok"}
```

`/healthz` répond `200` dès que le serveur accepte des requêtes. Il ne demande pas de token et ne lit rien sur le disque. Utilisez-le comme sonde de vie et de disponibilité, comme `HEALTHCHECK` Docker (intégré à l'image), ou comme contrôle du load balancer.

Il ne vérifie pas le disque. Surveillez l'espace libre à part (voir [Quoi surveiller](#quoi-surveiller)).

## Sortie au démarrage

Au démarrage, le serveur affiche une bannière sur la sortie standard :

```
##################################################
##          Local Registry Server               ##
##################################################

Listen      : http://127.0.0.1:8077
Start At    : 2026-10-18 10:56:12.091998 +0200 CEST m=+0.003639167
Assets Path : /srv/registry
```

`Listen` montre le schéma et l'adresse réellement liée : avec `--port 0`, le port choisi par le système.

Les avertissements partent sur la sortie d'erreur, avant la bannière :

| Ligne | Signification |
| --- | --- |
| `LOCAL_REGISTRY_UPLOAD_TOKEN is not set: upload is disabled` | Serveur en lecture seule. Normal pour un miroir, une erreur sinon. |
| `warning: upload is enabled on … without TLS: the token travels in clear text …` | Upload sur une adresse réseau sans TLS dans le serveur. Acceptable derrière un proxy TLS uniquement. |
| `warning: <file> holds the upload token but is readable by other users …` | Faites `chmod 600` sur le fichier de configuration. |
| `WARN upload token generation skipped: a token is already configured …` | `--disable-generate-token=false` avec un token configuré : le token configuré reste utilisé. |

## Logs de requêtes

Le serveur écrit une ligne par requête sur la sortie d'erreur :

```
2026/10/18 10:56:15 INFO request method=PUT path=/billing-api/artifacts/v2.4.0/files/metadata.json status=201 bytes=112 duration=6.7ms remote=10.0.4.12
```

| Champ | Contenu |
| --- | --- |
| `method`, `path` | La requête. La query string n'est pas journalisée. |
| `status` | Le statut HTTP envoyé. |
| `bytes` | Octets du corps de la réponse. |
| `duration` | Durée de traitement de la requête. |
| `remote` | Le pair TCP. |
| `forwarded_for` | `X-Forwarded-For` tel qu'envoyé, s'il est présent. Non vérifié. |

Une requête en échec ajoute une seconde ligne, `WARN` pour un `4xx` et `ERROR` pour un `5xx`, avec la cause exacte. Pour un `500`, cette ligne est le seul endroit où la cause apparaît : le client reçoit `internal error`.

```
2026/10/18 10:58:02 WARN request failed method=PUT path=/billing-api/artifacts/v2.4.0/files/artifacts.json status=422 error="artifacts.json references billing-api_2.4.0_linux_amd64.tar.gz, which is not uploaded: upload every file first, artifacts.json last"
```

Autres lignes possibles :

| Ligne | Signification |
| --- | --- |
| `INFO shutting down` | `SIGINT` ou `SIGTERM` reçu. |
| `ERROR audit log write failed` | Le journal d'audit n'a pas pu être écrit. La requête a quand même réussi. |
| `ERROR staging purge failed` | Une version en staging n'a pas pu être retirée. |

## Codes de sortie

| Code | Quand |
| --- | --- |
| `0` | Arrêt propre : après `SIGINT` ou `SIGTERM`, toutes les requêtes en cours ont fini en moins de 10 secondes. |
| `1` | Le serveur a échoué : port occupé, adresse indisponible, certificat ou clé TLS invalide, journal d'audit impossible à ouvrir, ou requêtes encore en cours 10 secondes après le signal d'arrêt (`Error: context deadline exceeded`). |
| `2` | Erreur de ligne de commande ou de configuration. Rien n'a démarré. Voir [Configuration](configuration.md). |

Avec systemd, `Restart=on-failure` redémarre sur `1` et aussi sur `2`. Une erreur de configuration tourne alors en boucle : `journalctl -u local-registry` affiche le message.

## Délais

| Délai | Valeur | Portée |
| --- | --- | --- |
| Lecture des en-têtes de requête | 5 s | Toutes les requêtes. |
| Écriture de la réponse | **20 s** | Toutes les routes sauf upload et publication, téléchargements compris. |
| Lecture et écriture | 30 min | `PUT …/files/{name}` et `POST …/publish`. |
| Connexion keep-alive inactive | 60 s | Toutes les connexions. |
| Arrêt propre | 10 s | Requêtes en cours après `SIGINT` ou `SIGTERM`. |

> [!WARNING]
> Un téléchargement doit finir en 20 secondes. Un gros fichier sur une liaison lente est coupé. Les clients peuvent reprendre avec une requête `Range`, par exemple `curl -C -`. Placez les gros téléchargements derrière un proxy de cache si cela compte pour vous.

Laissez à votre superviseur plus de 10 secondes pour arrêter le processus : `TimeoutStopSec=15` pour systemd, `terminationGracePeriodSeconds: 30` pour Kubernetes.

## Quoi surveiller

| Signal | Source | Pourquoi |
| --- | --- | --- |
| `/healthz` en échec | Sonde | Le serveur est arrêté. |
| Espace libre de l'asset path sous deux fois `min-free-mb` | Métriques du nœud ou du volume | Les uploads échouent en `507` sous `min-free-mb`. |
| `status=5xx` dans les logs de requêtes | Logs | Problèmes de disque ou de permissions ; la cause est sur la ligne `ERROR`. |
| `ERROR audit log write failed` | Logs | La trace d'audit a un trou. |
| Rafales de `auth_failed` | Journal d'audit | Quelqu'un devine le token, ou un pipeline a un ancien token. |
| Nombreux `publish_refused` | Journal d'audit | Des pipelines sont cassés ; leur staging se remplit. |
| Taille du staging proche de `max-staging-mb` | `du` sur `bin.local/.uploads` | Les prochains uploads recevront `507`. |
