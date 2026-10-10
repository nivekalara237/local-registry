package local_registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	// artifactsFile is goreleaser's list of a release's files. Publishing rewrites it so that
	// every "path" is the file's download URL on this server.
	artifactsFile = "artifacts.json"
	// originalArtifactsFile keeps artifacts.json as uploaded, beside the rewritten one.
	// It is reserved: no upload may use this name.
	originalArtifactsFile = "artifacts.goreleaser.json"
	// maxArtifactsSize caps artifacts.json, which is read into memory to be checked.
	maxArtifactsSize = 32 << 20
)

// publishedTypes are the artifacts.json types whose file is part of a release.
// "Binary" is published only with extra.Format: without it, the entry is a build output,
// the raw file an archive was made from, which is in no checksums file.
var publishedTypes = []string{"Archive", "File", "Source", "Linux Package", "Checksum",
	"Signature", "Certificate", "SBOM", "Metadata"}

// artifact is one artifacts.json entry: the fields it is checked on, and all of its fields
// as sent, to write them back unchanged but for "path".
type artifact struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"`
	Extra struct {
		Format   any `json:"Format"`
		Checksum any `json:"Checksum"`
	} `json:"extra"`

	fields map[string]json.RawMessage
}

// published reports whether the entry's file belongs in the release.
func (a artifact) published() bool {
	if a.Type == "Binary" {
		f, _ := a.Extra.Format.(string)
		return f != ""
	}
	return slices.Contains(publishedTypes, a.Type)
}

// sha256 is the entry's extra.Checksum when it is a sha256 one, else "".
func (a artifact) sha256() string {
	c, _ := a.Extra.Checksum.(string)
	sum, _ := strings.CutPrefix(c, "sha256:")
	if sum == c {
		return ""
	}
	return sum
}

func unprocessable(format string, a ...any) error {
	return httpError(http.StatusUnprocessableEntity, format, a...)
}

func parseArtifacts(data []byte) ([]artifact, error) {
	var fields []map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, unprocessable("artifacts.json is not a JSON array of objects")
	}
	var arts []artifact
	if err := json.Unmarshal(data, &arts); err != nil {
		return nil, unprocessable("artifacts.json: name, path and type must be strings: %v", err)
	}
	for i := range arts {
		if fields[i] == nil {
			return nil, unprocessable("artifacts.json: entry %d is null", i)
		}
		arts[i].fields = fields[i]
	}
	return arts, nil
}

// checkArtifacts parses artifacts.json and checks that every file it publishes is staged in dir,
// with the sha256 its extra.Checksum gives. sums holds the sha256 of staged files already
// hashed; the others are hashed here. It returns the published entries.
func checkArtifacts(dir string, data []byte, sums map[string]string) ([]artifact, error) {
	arts, err := parseArtifacts(data)
	if err != nil {
		return nil, err
	}
	var out []artifact
	seen := map[string]bool{}
	for _, a := range arts {
		if !a.published() {
			continue
		}
		switch {
		case a.Name == "":
			return nil, unprocessable("artifacts.json: a %s entry has no name", a.Type)
		case seen[a.Name]:
			return nil, unprocessable("artifacts.json lists %s twice", a.Name)
		case !ValidName(a.Name) || a.Name == artifactsFile || a.Name == originalArtifactsFile:
			return nil, unprocessable("artifacts.json lists %q, a name that cannot be uploaded", a.Name)
		}
		seen[a.Name] = true

		info, err := os.Lstat(filepath.Join(dir, a.Name))
		if errors.Is(err, os.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
			return nil, unprocessable("artifacts.json references %s, which is not uploaded: upload every file first, artifacts.json last", a.Name)
		}
		if err != nil {
			return nil, err
		}
		if want := a.sha256(); want != "" {
			got, ok := sums[a.Name]
			if !ok {
				if got, err = fileSha256(filepath.Join(dir, a.Name)); err != nil {
					return nil, err
				}
			}
			if !strings.EqualFold(got, want) {
				return nil, unprocessable("%s: sha256 is %s but artifacts.json says %s", a.Name, got, want)
			}
		}
		out = append(out, a)
	}
	return out, nil
}

// rewriteArtifacts returns artifacts.json holding only the published entries, each "path"
// replaced by baseURL + "/" + its escaped name. Every other field is kept as uploaded.
func rewriteArtifacts(published []artifact, baseURL string) ([]byte, error) {
	out := make([]map[string]json.RawMessage, 0, len(published))
	for _, a := range published {
		fields := make(map[string]json.RawMessage, len(a.fields))
		for k, v := range a.fields {
			fields[k] = v
		}
		u, err := json.Marshal(baseURL + "/" + url.PathEscape(a.Name))
		if err != nil {
			return nil, err
		}
		fields["path"] = u
		out = append(out, fields)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// releaseFilesURL is where GET /{vcs}/artifacts/{version}/files/{name} serves a published
// version's files: an absolute URL under publicURL, or a path when publicURL is empty.
func releaseFilesURL(publicURL, vcs, version string) string {
	return fmt.Sprintf("%s/%s/artifacts/%s/files", strings.TrimRight(publicURL, "/"), url.PathEscape(vcs), url.PathEscape(version))
}

// serveReleaseFile answers GET /{vcs}/artifacts/{version}/files/{name} with a file of a
// published version. Staged files are not served; anything else answers 404.
func (s *LocalRegistryServer) serveReleaseFile(w http.ResponseWriter, r *http.Request) {
	vcs, version, name := r.PathValue("vcs"), r.PathValue("version"), r.PathValue("name")
	if version == "latest" || !ValidName(name) || strings.HasPrefix(name, ".") {
		http.NotFound(w, r)
		return
	}
	dir, err := NewLayout(s.absoluteAssetPath).VersionDir(vcs, version)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	serveFile(w, r, dir, name)
}
