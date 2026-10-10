package main

import (
	"encoding/json"
	localregistry "github.com/nivekalara237/local-registry"
	"os"
	"time"
)

type arguments struct {
	port        int
	addr        string
	absAsset    string
	maxUploadMB int64
	uploadToken string
	tlsCert     string
	tlsKey      string

	maxVersionMB    int64
	maxVersionFiles int
	maxStagingMB    int64
	minFreeMB       int64
	stagingTTL      time.Duration
	auditLog        string
	publicURL       string

	generateToken bool   // --disable-generate-token=false
	tokenSource   string // where a configured token comes from, for the warning
}

// limit turns a command-line value where 0 means "no limit" into an UploadConfig value,
// where 0 means "default" and a negative value means "no limit".
func limit[T int | int64 | time.Duration](v T) T {
	if v == 0 {
		return -1
	}
	return v
}

// args is the configuration the route handlers read.
var args arguments

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, serveRegistry))
}

func newServer() (*localregistry.LocalRegistryServer, error) {
	server := localregistry.NewLocalRegistryServer(args.port, args.addr, args.absAsset)
	if args.tlsCert != "" {
		if err := server.SetTLS(args.tlsCert, args.tlsKey); err != nil {
			return nil, err
		}
	}
	if args.auditLog != "" {
		audit, err := localregistry.OpenAuditLog(args.auditLog)
		if err != nil {
			return nil, err
		}
		server.SetAuditLog(audit)
	}
	server.AddUploadRoutes(localregistry.UploadConfig{
		Token:           args.uploadToken,
		MaxFileSize:     args.maxUploadMB << 20,
		MaxVersionSize:  limit(args.maxVersionMB << 20),
		MaxVersionFiles: limit(args.maxVersionFiles),
		MaxStagingSize:  limit(args.maxStagingMB << 20),
		MinFreeSpace:    limit(args.minFreeMB << 20),
		StagingTTL:      limit(args.stagingTTL),
		PublicURL:       args.publicURL,
	})

	// curl --request GET localhost:8077/assets/binary/vcs-gitlab-latest -o  bitlab.bin
	server.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")
	server.AddFileRoute("/assets/checksum/{filename}", "GET", "filename")
	server.AddReleaseRoutes(args.publicURL)
	// Liveness probe (Docker HEALTHCHECK, load balancers): answers as soon as the server serves.
	server.AddRoute("/healthz", "GET", func(...string) any { return map[string]string{"status": "ok"} })
	server.AddRouteE("/{vcs}/artifacts", "GET", fnGetArtifacts)
	server.AddRouteE("/{vcs}/artifacts/{version}", "GET", fnGetArtifacts)
	server.AddRouteE("/artifacts/all", "GET", fnGetAllArtifacts)
	return server, nil
}

func fnGetAllArtifacts(vps ...string) (any, error) {
	asset := &localregistry.Asset{}

	return asset.FindAssetContents(localregistry.NewLayout(args.absAsset).AssetsDir())
}

func fnGetArtifacts(pvs ...string) (any, error) {
	var version string
	var vcs string
	if len(pvs) > 0 {
		vcs = pvs[0]
	}
	if len(pvs) > 1 {
		version = pvs[1]
	}
	artifactsFile, err := localregistry.NewLayout(args.absAsset).ArtifactsFile(vcs, version)
	if err != nil {
		return nil, err
	}
	file, err := os.ReadFile(artifactsFile)
	if err != nil {
		return nil, err
	}
	var res any
	if err := json.Unmarshal(file, &res); err != nil {
		return nil, err
	}
	return res, nil
}
