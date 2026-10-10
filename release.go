package local_registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Release lists the files of one version, as POST /releases/{vcs}/{version} answers it.
type Release struct {
	TagName string         `json:"tag_name"`
	Assets  []ReleaseAsset `json:"assets"`
}

// ReleaseAsset is one file of a Release.
type ReleaseAsset struct {
	// ID is stable: the same vcs, version and name always give the same UUID (v5).
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

// LatestVersion is the version that <vcs>/artifacts.json designates: the published version
// whose own artifacts.json has the same bytes (publish?latest=true copies it). It answers
// ErrNotFound when no version is latest, and 409 when several versions match.
func (l Layout) LatestVersion(vcs string) (string, error) {
	latest, err := l.ArtifactsFile(vcs, "latest")
	if err != nil {
		return "", err
	}
	want, err := os.ReadFile(latest)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	vcsDir, _ := l.VCSDir(vcs)
	entries, err := os.ReadDir(vcsDir)
	if err != nil {
		return "", err
	}
	var found []string
	for _, e := range entries {
		if !e.IsDir() || !ValidName(e.Name()) || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		got, err := os.ReadFile(filepath.Join(vcsDir, e.Name(), artifactsFile))
		if err == nil && bytes.Equal(got, want) {
			found = append(found, e.Name())
		}
	}
	switch len(found) {
	case 0:
		return "", ErrNotFound
	case 1:
		return found[0], nil
	}
	return "", httpError(http.StatusConflict, "latest of %s is ambiguous: %s have the same artifacts.json", vcs, strings.Join(found, ", "))
}

// resolveVersion turns "latest" (or "") into the latest published version of vcs.
func (l Layout) resolveVersion(vcs, version string) (string, error) {
	if version == "" || version == "latest" {
		return l.LatestVersion(vcs)
	}
	if !ValidName(version) {
		return "", ErrNotFound
	}
	return version, nil
}

// ListRelease lists the files of <vcs>/<version> ("latest" allowed). It reads the published
// version (bin.local/assets/binaries/<vcs>/<version>), whose files download through
// GET /{vcs}/artifacts/{version}/files/{name}; failing that, the legacy tree
// (assets/binaries/<vcs>/<version>), whose files download through GET /assets/binary.
// URLs are relative, or absolute under publicURL when it is set.
// Only regular files are listed, hidden ones (".x") excepted, sorted by name.
func (l Layout) ListRelease(vcs, version, publicURL string) (*Release, error) {
	if !ValidName(vcs) {
		return nil, ErrNotFound
	}
	version, err := l.resolveVersion(vcs, version)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(publicURL, "/")

	dir, _ := l.VersionDir(vcs, version)
	fileURL := func(name string) string {
		return releaseFilesURL(publicURL, vcs, version) + "/" + url.PathEscape(name)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		dir = filepath.Join(l.root, "assets", "binaries", vcs, version)
		fileURL = func(name string) string {
			return base + "/assets/binary/" + url.PathEscape(path.Join("assets", "binaries", vcs, version, name))
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
			ID:         uuid.NewSHA1(assetNamespace, []byte(vcs+"/"+version+"/"+e.Name())).String(),
			Name:       e.Name(),
			URL:        fileURL(e.Name()),
			IsChecksum: isChecksumName(e.Name()),
		})
	}
	return rel, nil
}

// ReadMetadata returns the metadata.json of the published version <vcs>/<version>
// ("latest" or "" for the latest one), decoded.
func (l Layout) ReadMetadata(vcs, version string) (any, error) {
	if !ValidName(vcs) {
		return nil, ErrNotFound
	}
	version, err := l.resolveVersion(vcs, version)
	if err != nil {
		return nil, err
	}
	dir, err := l.VersionDir(vcs, version)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// AddReleaseRoutes registers the read-only release routes:
//
//	POST /releases/{vcs}/{version}            files of a version, with their download URL
//	POST /releases/{version}                  deprecated: the same for vcs "gitlab"
//	GET  /artifacts/metadatas/{vcs}?version=  metadata.json of a version (default: latest)
//
// publicURL, when set, makes the download URLs absolute (see UploadConfig.PublicURL).
func (s *LocalRegistryServer) AddReleaseRoutes(publicURL string) {
	l := NewLayout(s.absoluteAssetPath)
	s.AddRouteE("/releases/{vcs}/{version}", "POST", func(pvs ...string) (any, error) {
		return l.ListRelease(pvs[0], pvs[1], publicURL)
	})
	s.serverMux.HandleFunc("POST /releases/{version}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Link", `</releases/gitlab/`+url.PathEscape(r.PathValue("version"))+`>; rel="successor-version"`)
		rel, err := l.ListRelease("gitlab", r.PathValue("version"), publicURL)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, rel)
	})
	s.serverMux.HandleFunc("GET /artifacts/metadatas/{vcs}", func(w http.ResponseWriter, r *http.Request) {
		meta, err := l.ReadMetadata(r.PathValue("vcs"), r.URL.Query().Get("version"))
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, meta)
	})
}
