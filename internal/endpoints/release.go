package endpoints

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/nivekalara237/local-registry/internal/core"
)

// Release lists the files of one version, as POST /releases/{product}/{version} answers it.
type Release struct {
	TagName string         `json:"tag_name"`
	Assets  []ReleaseAsset `json:"assets"`
}

// ReleaseAsset is one file of a Release.
type ReleaseAsset struct {
	// ID is stable: the same product, version and name always give the same UUID (v5).
	ID         string `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	IsChecksum bool   `json:"is_checksum"`
}

// assetNamespace scopes the ReleaseAsset IDs.
var assetNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("local-registry/release-asset"))

// isChecksumName reports a checksums file: its name ends with checksums.txt, the rule publish
// applies (goreleaser's <project>_<version>_checksums.txt), or with checksum.txt, as legacy
// trees name them (tool.checksum.txt). In any case.
func isChecksumName(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, "checksums.txt") || strings.HasSuffix(n, "checksum.txt")
}

// LatestVersion is the version that <product>/release.json of l designates (publish?latest=true
// copies the release.json of the version there), when that version is published. It answers
// ErrNotFound when no version is latest.
func LatestVersion(l core.Layout, product string) (string, error) {
	productDir, err := l.ProductDir(product)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(productDir, releaseFile))
	if errors.Is(err, os.ErrNotExist) {
		return "", core.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	var doc struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("%s/%s: %w", product, releaseFile, err)
	}
	if !core.ValidName(doc.Version) || reservedFile(doc.Version) {
		return "", core.ErrNotFound
	}
	if _, err := os.Stat(filepath.Join(productDir, doc.Version, releaseFile)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", core.ErrNotFound
		}
		return "", err
	}
	return doc.Version, nil
}

// resolveVersion turns "latest" (or "") into the latest published version of product.
func resolveVersion(l core.Layout, product, version string) (string, error) {
	if version == "" || version == "latest" {
		return LatestVersion(l, product)
	}
	if !core.ValidName(version) {
		return "", core.ErrNotFound
	}
	return version, nil
}

// ListRelease lists the files of <product>/<version> in l ("latest" allowed). It reads the published
// version (bin.local/assets/binaries/<product>/<version>), whose files download through
// GET /{product}/artifacts/{version}/files/{name}; failing that, the legacy tree
// (assets/binaries/<product>/<version>), whose files download through GET /assets/binary.
// URLs are relative, or absolute under publicURL when it is set.
// Only regular files are listed, hidden ones (".x") excepted, sorted by name.
func ListRelease(l core.Layout, product, version, publicURL string) (*Release, error) {
	if !core.ValidName(product) {
		return nil, core.ErrNotFound
	}
	version, err := resolveVersion(l, product, version)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(publicURL, "/")

	dir, _ := l.VersionDir(product, version)
	fileURL := func(name string) string {
		return releaseFilesURL(publicURL, product, version) + "/" + url.PathEscape(name)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		dir = filepath.Join(l.Root(), "assets", "binaries", product, version)
		fileURL = func(name string) string {
			return base + "/assets/binary/" + url.PathEscape(path.Join("assets", "binaries", product, version, name))
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	rel := &Release{TagName: version, Assets: []ReleaseAsset{}}
	for _, e := range entries {
		if !e.Type().IsRegular() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		rel.Assets = append(rel.Assets, ReleaseAsset{
			ID:         uuid.NewSHA1(assetNamespace, []byte(product+"/"+version+"/"+e.Name())).String(),
			Name:       e.Name(),
			URL:        fileURL(e.Name()),
			IsChecksum: isChecksumName(e.Name()),
		})
	}
	return rel, nil
}

// ReadMetadata returns the metadata.json of the published version <product>/<version> in l
// ("latest" or "" for the latest one), decoded.
func ReadMetadata(l core.Layout, product, version string) (any, error) {
	if !core.ValidName(product) {
		return nil, core.ErrNotFound
	}
	version, err := resolveVersion(l, product, version)
	if err != nil {
		return nil, err
	}
	dir, err := l.VersionDir(product, version)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, metadataFile))
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// AddReleaseRoutes registers the read-only release routes on reg:
//
//	POST /releases/{product}/{version}            files of a version, with their download URL
//	POST /releases/{version}                  deprecated: the same for product "gitlab"
//	GET  /artifacts/metadatas/{product}?version=  metadata.json of a version (default: latest)
//
// publicURL, when set, makes the download URLs absolute (see UploadConfig.PublicURL).
func AddReleaseRoutes(reg *core.Registry, publicURL string) {
	l := reg.Layout
	reg.AddRouteE("/releases/{product}/{version}", "POST", func(pvs ...string) (any, error) {
		return ListRelease(l, pvs[0], pvs[1], publicURL)
	})
	reg.Mux.HandleFunc("POST /releases/{version}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Link", `</releases/gitlab/`+url.PathEscape(r.PathValue("version"))+`>; rel="successor-version"`)
		rel, err := ListRelease(l, "gitlab", r.PathValue("version"), publicURL)
		if err != nil {
			core.WriteError(w, r, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, rel)
	})
	reg.Mux.HandleFunc("GET /artifacts/metadatas/{product}", func(w http.ResponseWriter, r *http.Request) {
		meta, err := ReadMetadata(l, r.PathValue("product"), r.URL.Query().Get("version"))
		if err != nil {
			core.WriteError(w, r, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, meta)
	})
}
