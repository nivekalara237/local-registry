---
title: Go library
description: Embed local-registry in a Go program, or read its storage directly, with the exported API of the root package.
sidebar_label: Go library
---

The root package of the module, `github.com/nivekalara237/local-registry` (package name `local_registry`), is the server the `local-registry` binary runs, as a library. Use it to embed the registry in another Go program, to add routes of your own, or to read the storage tree (latest version, `release.json`, index) without HTTP. Everything under `internal/` is private.

```sh
go get github.com/nivekalara237/local-registry
```

## Embedding the server

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
	if err := s.SetTLS("/etc/local-registry/fullchain.pem", "/etc/local-registry/privkey.pem"); err != nil { // optional
		log.Fatal(err)
	}

	audit, err := localregistry.OpenAuditLog("/var/log/local-registry/audit.log")
	if err != nil {
		log.Fatal(err)
	}
	defer audit.Close()
	s.SetAuditLog(audit)

	s.AddUploadRoutes(localregistry.UploadConfig{
		Token:          os.Getenv("LOCAL_REGISTRY_UPLOAD_TOKEN"), // empty: upload disabled (403)
		MaxVersionSize: 4 << 30,                                  // 0: default; < 0: no limit
		StagingTTL:     48 * time.Hour,
		PublicURL:      publicURL,
	})
	s.AddReleaseRoutes(publicURL)
	s.AddProductRoutes()
	s.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")
	s.AddRoute("/healthz", "GET", func(...string) any { return map[string]string{"status": "ok"} })

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := s.ServeContext(ctx); err != nil { // nil after a clean stop
		log.Fatal(err)
	}
}
```

A new server has **no route**: register the groups you need. The binary registers all of them, plus `GET /{product}/artifacts`, `GET /{product}/artifacts/{version}` and `GET /artifacts/all`.

## Server

| Function or method | Effect |
| --- | --- |
| `NewLocalRegistryServer(port int, addr, assetPath string) *LocalRegistryServer` | A server with no route, serving the tree under `assetPath` (absolute). |
| `SetTLS(certFile, keyFile string) error` | HTTPS only, TLS 1.2 or later. The pair is loaded now: a bad file fails here. |
| `TLSEnabled() bool` | Whether `SetTLS` succeeded. |
| `SetAuditLog(a *AuditLog)` | Send audit events to `a`. Without it, they go to `slog` (message `audit`). |
| `AddUploadRoutes(cfg UploadConfig)` | `PUT …/files/{name}`, `POST …/publish`, `GET …/files/{name}`. See [Upload API](./api/upload.md). |
| `AddReleaseRoutes(publicURL string)` | `POST /releases/{product}/{version}`, `POST /releases/{version}`, `GET /artifacts/metadatas/{product}`. |
| `AddProductRoutes()` | `GET /products`, `GET /{product}/index`, `GET /{product}/releases/{version}`. |
| `AddFileRoute(uri, method, varName string)` | Serve the public file named by the path variable `varName`, as `/assets/binary` does. |
| `AddRoute(uri, method string, f func(...string) any)` | A JSON route; `f` gets the path variables in the order they appear in `uri`. |
| `AddRouteE(uri, method string, f func(...string) (any, error))` | Same, with errors: an `*HTTPError` answers its status and message; `ErrNotFound` or `fs.ErrNotExist` answers `404`; any other error answers `500 internal error` and is only logged. |
| `ServeContext(ctx context.Context) error` | Bind the address (a busy port fails at once), serve until `ctx` is done, then drain requests for up to 10 s. Runs the staging purge. `nil` after a clean stop. |
| `Serve()` | `ServeContext` until `SIGINT`/`SIGTERM`; exits the process with code `1` on error. |
| `Handler() http.Handler` | The routes registered so far, without request logging nor timeouts: for `httptest`. |

## UploadConfig

| Field | Type | Meaning |
| --- | --- | --- |
| `Token` | `string` | Bearer token. Empty: upload disabled, routes answer `403`. |
| `MaxFileSize` | `int64` | Largest file, bytes. `0` or less: `DefaultMaxFileSize`. |
| `MaxVersionSize` | `int64` | Bytes staged per version. `0`: default; negative: no limit. |
| `MaxVersionFiles` | `int` | Files staged per version. `0`: default; negative: no limit. |
| `MaxStagingSize` | `int64` | Bytes staged for all versions. `0`: default; negative: no limit. |
| `MinFreeSpace` | `int64` | Disk space an upload must leave free. `0`: default; negative: no check. |
| `StagingTTL` | `time.Duration` | Purge staged versions untouched for this long. `0`: default; negative: never. |
| `PublicURL` | `string` | Address clients reach the server at; makes written download URLs absolute. |

| Constant | Value |
| --- | --- |
| `DefaultMaxFileSize` | 200 MiB |
| `DefaultMaxVersionSize` | 2 GiB |
| `DefaultMaxVersionFiles` | 100 |
| `DefaultMaxStagingSize` | 10 GiB |
| `DefaultMinFreeSpace` | 1 GiB |
| `DefaultStagingTTL` | 24 h |
| `APIVersion` | `"local-registry/v1"` |

## Reading the storage

These functions read the tree directly, as the routes do.

| Function | Returns |
| --- | --- |
| `NewLayout(assetPath string) Layout` | The paths of the tree: `Root`, `AssetsDir`, `BinariesDir`, `ProductDir`, `VersionDir`, `ArtifactsFile`, `StagingDir`, `UploadsDir`, `UploadTmpDir`. Path methods refuse invalid names with an error wrapping `ErrNotFound`. |
| `LatestVersion(l Layout, product string) (string, error)` | The version `<product>/release.json` designates; `ErrNotFound` when none. |
| `ReadReleaseDocument(l Layout, product, version string) ([]byte, error)` | The `release.json` of a version, as stored; `latest` allowed. |
| `BuildIndex(l Layout, product string) (*ProductIndex, error)` | The product's [index](./index-json.md), built from the disk. |
| `ListProducts(l Layout) ([]ProductSummary, error)` | What `GET /products` answers. |
| `ListRelease(l Layout, product, version, publicURL string) (*Release, error)` | What `POST /releases/{product}/{version}` answers. |
| `ReadMetadata(l Layout, product, version string) (any, error)` | The decoded `metadata.json` of a version; `latest` or `""` for the latest. |
| `(*Asset).FindAssetPaths(dir string)`, `(*Asset).FindAssetContents(dir string)` | The catalogue scan of `GET /artifacts/all`: paths, or contents. |

```go
l := localregistry.NewLayout("/srv/registry")
v, err := localregistry.LatestVersion(l, "sonarbridge")
if errors.Is(err, localregistry.ErrNotFound) {
	// no latest version
}
raw, err := localregistry.ReadReleaseDocument(l, "sonarbridge", v)
var doc localregistry.ReleaseDocument
err = json.Unmarshal(raw, &doc)
```

## Types and helpers

| Name | Meaning |
| --- | --- |
| `ReleaseDocument`, `ReleaseArtifact`, `ReleaseChecksums`, `Change` | [release.json](./release-json.md). |
| `ProductIndex`, `IndexVersion`, `ProductSummary` | [index.json](./index-json.md) and `GET /products`. |
| `ProductFile`, `Owner`, `Link` | [registry.yaml](./registry-yaml.md). |
| `Release`, `ReleaseAsset` | The answer of `POST /releases/{product}/{version}`. |
| `Asset`, `AssetPath`, `AssetContents` | The catalogue of `GET /artifacts/all`. |
| `HTTPError` | An error whose status and message are sent to the client as is. |
| `AuditLog`, `OpenAuditLog(path string) (*AuditLog, error)` | The audit log: appended, file mode `0600`, directory created if needed. |
| `ErrNotFound` | Makes an `AddRouteE` handler answer `404`. |
| `ValidName(s string) bool` | The name rule of every path variable. |
| `IsPublicFile(name string) bool` | The rule of file routes: a clean path inside `bin.local/assets/` or `assets/`, with no segment starting with `.`. |
| `TokenID(token string) string` | The `token_id` of a token: the first 12 hex digits of its sha256. |
