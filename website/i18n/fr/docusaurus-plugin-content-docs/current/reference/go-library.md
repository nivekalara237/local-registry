---
title: Bibliothèque Go
description: Embarquer local-registry dans un programme Go, ou lire directement son stockage, avec l'API exportée du paquet racine.
sidebar_label: Bibliothèque Go
---

Le paquet racine du module, `github.com/nivekalara237/local-registry` (nom de paquet `local_registry`), est le serveur qu'exécute le binaire `local-registry`, sous forme de bibliothèque. Utilisez-le pour embarquer le registre dans un autre programme Go, pour ajouter vos propres routes, ou pour lire l'arborescence de stockage (dernière version, `release.json`, index) sans passer par HTTP. Tout ce qui est sous `internal/` est privé.

```sh
go get github.com/nivekalara237/local-registry
```

## Embarquer le serveur

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	localregistry "github.com/nivekalara237/local-registry"
)

func main() {
	const publicURL = "https://registry.example.com"

	s := localregistry.NewLocalRegistryServer(8077, "0.0.0.0", "/srv/registry")
	if err := s.SetTLS("/etc/local-registry/fullchain.pem", "/etc/local-registry/privkey.pem"); err != nil { // facultatif
		log.Fatal(err)
	}

	audit, err := localregistry.OpenAuditLog("/var/log/local-registry/audit.log")
	if err != nil {
		log.Fatal(err)
	}
	defer audit.Close()
	s.SetAuditLog(audit)

	s.AddUploadRoutes(localregistry.UploadConfig{
		Token:          os.Getenv("LOCAL_REGISTRY_UPLOAD_TOKEN"), // vide : dépôt désactivé (403)
		MaxVersionSize: 4 << 30,                                  // 0 : défaut ; < 0 : pas de limite
		StagingTTL:     48 * time.Hour,
		PublicURL:      publicURL,
	})
	s.AddReleaseRoutes(publicURL)
	s.AddProductRoutes()
	s.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")
	s.AddRoute("/healthz", "GET", func(...string) any { return map[string]string{"status": "ok"} })

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := s.ServeContext(ctx); err != nil { // nil après un arrêt propre
		log.Fatal(err)
	}
}
```

Un nouveau serveur n'a **aucune route** : enregistrez les groupes dont vous avez besoin. Le binaire les enregistre tous, plus `GET /{product}/artifacts`, `GET /{product}/artifacts/{version}` et `GET /artifacts/all`.

## Serveur

| Fonction ou méthode | Effet |
| --- | --- |
| `NewLocalRegistryServer(port int, addr, assetPath string) *LocalRegistryServer` | Un serveur sans route, qui sert l'arborescence sous `assetPath` (absolu). |
| `SetTLS(certFile, keyFile string) error` | HTTPS uniquement, TLS 1.2 ou plus. La paire est chargée immédiatement : un mauvais fichier échoue ici. |
| `TLSEnabled() bool` | Si `SetTLS` a réussi. |
| `SetAuditLog(a *AuditLog)` | Envoie les événements d'audit à `a`. Sans lui, ils vont à `slog` (message `audit`). |
| `AddUploadRoutes(cfg UploadConfig)` | `PUT …/files/{name}`, `POST …/publish`, `GET …/files/{name}`. Voir [API de dépôt](./api/upload.md). |
| `AddReleaseRoutes(publicURL string)` | `POST /releases/{product}/{version}`, `POST /releases/{version}`, `GET /artifacts/metadatas/{product}`. |
| `AddProductRoutes()` | `GET /products`, `GET /{product}/index`, `GET /{product}/releases/{version}`. |
| `AddFileRoute(uri, method, varName string)` | Sert le fichier public nommé par la variable de chemin `varName`, comme `/assets/binary`. |
| `AddRoute(uri, method string, f func(...string) any)` | Une route JSON ; `f` reçoit les variables de chemin dans l'ordre où elles apparaissent dans `uri`. |
| `AddRouteE(uri, method string, f func(...string) (any, error))` | Idem, avec erreurs : un `*HTTPError` répond son statut et son message ; `ErrNotFound` ou `fs.ErrNotExist` répond `404` ; toute autre erreur répond `500 internal error` et n'est que journalisée. |
| `ServeContext(ctx context.Context) error` | Réserve l'adresse (un port occupé échoue aussitôt), sert jusqu'à la fin de `ctx`, puis laisse jusqu'à 10 s aux requêtes en cours. Exécute la purge du staging. `nil` après un arrêt propre. |
| `Serve()` | `ServeContext` jusqu'à `SIGINT`/`SIGTERM` ; termine le processus avec le code `1` en cas d'erreur. |
| `Handler() http.Handler` | Les routes enregistrées jusque-là, sans journal des requêtes ni délais : pour `httptest`. |

## UploadConfig

| Champ | Type | Signification |
| --- | --- | --- |
| `Token` | `string` | Jeton bearer. Vide : dépôt désactivé, les routes répondent `403`. |
| `MaxFileSize` | `int64` | Plus gros fichier, en octets. `0` ou moins : `DefaultMaxFileSize`. |
| `MaxVersionSize` | `int64` | Octets en staging par version. `0` : défaut ; négatif : pas de limite. |
| `MaxVersionFiles` | `int` | Fichiers en staging par version. `0` : défaut ; négatif : pas de limite. |
| `MaxStagingSize` | `int64` | Octets en staging pour toutes les versions. `0` : défaut ; négatif : pas de limite. |
| `MinFreeSpace` | `int64` | Espace disque qu'un dépôt doit laisser libre. `0` : défaut ; négatif : pas de contrôle. |
| `StagingTTL` | `time.Duration` | Purge les versions en staging inactives depuis cette durée. `0` : défaut ; négatif : jamais. |
| `PublicURL` | `string` | Adresse par laquelle les clients joignent le serveur ; rend absolues les URL de téléchargement écrites. |

| Constante | Valeur |
| --- | --- |
| `DefaultMaxFileSize` | 200 Mio |
| `DefaultMaxVersionSize` | 2 Gio |
| `DefaultMaxVersionFiles` | 100 |
| `DefaultMaxStagingSize` | 10 Gio |
| `DefaultMinFreeSpace` | 1 Gio |
| `DefaultStagingTTL` | 24 h |
| `APIVersion` | `"local-registry/v1"` |

## Lire le stockage

Ces fonctions lisent l'arborescence directement, comme le font les routes.

| Fonction | Renvoie |
| --- | --- |
| `NewLayout(assetPath string) Layout` | Les chemins de l'arborescence : `Root`, `AssetsDir`, `BinariesDir`, `ProductDir`, `VersionDir`, `ArtifactsFile`, `StagingDir`, `UploadsDir`, `UploadTmpDir`. Les méthodes de chemin refusent les noms invalides avec une erreur qui enveloppe `ErrNotFound`. |
| `LatestVersion(l Layout, product string) (string, error)` | La version que désigne `<product>/release.json` ; `ErrNotFound` s'il n'y en a pas. |
| `ReadReleaseDocument(l Layout, product, version string) ([]byte, error)` | Le `release.json` d'une version, tel que stocké ; `latest` accepté. |
| `BuildIndex(l Layout, product string) (*ProductIndex, error)` | L'[index](./index-json.md) du produit, construit depuis le disque. |
| `ListProducts(l Layout) ([]ProductSummary, error)` | Ce que répond `GET /products`. |
| `ListRelease(l Layout, product, version, publicURL string) (*Release, error)` | Ce que répond `POST /releases/{product}/{version}`. |
| `ReadMetadata(l Layout, product, version string) (any, error)` | Le `metadata.json` décodé d'une version ; `latest` ou `""` pour la dernière. |
| `(*Asset).FindAssetPaths(dir string)`, `(*Asset).FindAssetContents(dir string)` | Le parcours du catalogue de `GET /artifacts/all` : chemins, ou contenus. |

```go
l := localregistry.NewLayout("/srv/registry")
v, err := localregistry.LatestVersion(l, "sonarbridge")
if errors.Is(err, localregistry.ErrNotFound) {
	// pas de dernière version
}
raw, err := localregistry.ReadReleaseDocument(l, "sonarbridge", v)
var doc localregistry.ReleaseDocument
err = json.Unmarshal(raw, &doc)
```

## Types et utilitaires

| Nom | Signification |
| --- | --- |
| `ReleaseDocument`, `ReleaseArtifact`, `ReleaseChecksums`, `Change` | [release.json](./release-json.md). |
| `ProductIndex`, `IndexVersion`, `ProductSummary` | [index.json](./index-json.md) et `GET /products`. |
| `ProductFile`, `Owner`, `Link` | [registry.yaml](./registry-yaml.md). |
| `Release`, `ReleaseAsset` | La réponse de `POST /releases/{product}/{version}`. |
| `Asset`, `AssetPath`, `AssetContents` | Le catalogue de `GET /artifacts/all`. |
| `HTTPError` | Une erreur dont le statut et le message sont envoyés tels quels au client. |
| `AuditLog`, `OpenAuditLog(path string) (*AuditLog, error)` | Le journal d'audit : en ajout, fichier en mode `0600`, répertoire créé si besoin. |
| `ErrNotFound` | Fait répondre `404` à un gestionnaire `AddRouteE`. |
| `ValidName(s string) bool` | La règle de nommage de toutes les variables de chemin. |
| `IsPublicFile(name string) bool` | La règle des routes de fichiers : un chemin propre dans `bin.local/assets/` ou `assets/`, sans segment commençant par `.`. |
| `TokenID(token string) string` | Le `token_id` d'un jeton : les 12 premiers chiffres hexadécimaux de son sha256. |
