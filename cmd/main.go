package main

import (
	"encoding/json"
	"net/url"
	"os"
	"path"
	localregistry "sonarbridge-go/local-registry"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ReleaseEntry struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Id         string `json:"id"`
		Name       string `json:"name"`
		Url        string `json:"url"`
		IsChecksum bool   `json:"is_checksum"`
	} `json:"assets"`
}

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
	server.AddRouteE("/releases/{version}", "POST", func(pvs ...string) (any, error) {
		version := pvs[0]
		dir, err := localregistry.NewLayout(args.absAsset).LegacyReleaseDir(version)
		if err != nil {
			return nil, err
		}
		dirs, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		releases := ReleaseEntry{TagName: version}
		for _, f := range dirs {
			if f.IsDir() {
				continue
			}
			releases.Assets = append(releases.Assets, struct {
				Id         string `json:"id"`
				Name       string `json:"name"`
				Url        string `json:"url"`
				IsChecksum bool   `json:"is_checksum"`
			}{
				Id:         uuid.New().String(),
				Name:       f.Name(),
				Url:        releaseAssetURL(version, f.Name()),
				IsChecksum: strings.Contains(f.Name(), ".checksum.txt"),
			})
		}
		return releases, nil
	})
	server.AddRouteE("/{vcs}/artifacts", "GET", fnGetArtifacts)
	server.AddRouteE("/{vcs}/artifacts/{version}", "GET", fnGetArtifacts)
	server.AddRouteE("/artifacts/metadatas", "GET", fnGetArtifacts)
	server.AddRouteE("/artifacts/all", "GET", fnGetAllArtifacts)
	return server, nil
}

// releaseAssetURL is the path that downloads a file of the legacy release tree through
// GET /assets/binary, relative to the registry's address so it works behind a proxy.
// It never reveals where the file sits on the server's disk.
func releaseAssetURL(version, name string) string {
	return "/assets/binary/" + url.PathEscape(path.Join("assets", "binaries", "gitlab", version, name))
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
