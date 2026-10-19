package endpoints

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ReleaseDocument is release.json: one published version, as the server describes it.
// Optional values are null rather than missing, so every key is always there.
type ReleaseDocument struct {
	APIVersion  string  `json:"apiVersion"`
	Product     string  `json:"product"`
	DisplayName string  `json:"displayName"`
	Version     string  `json:"version"`
	Tag         *string `json:"tag"`
	PreviousTag *string `json:"previousTag"`
	Prerelease  bool    `json:"prerelease"`
	Commit      *string `json:"commit"`
	BuiltAt     *string `json:"builtAt"`
	PublishedAt string  `json:"publishedAt"`
	// PublishedBy is the token_id of the upload token, never the token.
	PublishedBy string `json:"publishedBy"`
	// Deprecated is false, true or a message, from registry.yaml.
	Deprecated      any               `json:"deprecated"`
	SecurityUpdates bool              `json:"securityUpdates"`
	Changes         []Change          `json:"changes"`
	Artifacts       []ReleaseArtifact `json:"artifacts"`
	Checksums       *ReleaseChecksums `json:"checksums"`
}

// ReleaseArtifact is one downloadable file of a ReleaseDocument.
type ReleaseArtifact struct {
	Name string `json:"name"`
	// Kind is archive, binary, package, file or source.
	Kind      string  `json:"kind"`
	Format    *string `json:"format"`
	MediaType string  `json:"mediaType"`
	OS        *string `json:"os"`
	Arch      *string `json:"arch"`
	Variant   *string `json:"variant"`
	Size      int64   `json:"size"`
	Sha256    string  `json:"sha256"`
	URL       string  `json:"url"`
	// SBOM, Signature and Certificate are the URLs of the files that cover this one.
	SBOM        *string `json:"sbom"`
	Signature   *string `json:"signature"`
	Certificate *string `json:"certificate"`
}

// ReleaseChecksums is the checksums file of a ReleaseDocument, with its signature.
type ReleaseChecksums struct {
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	Signature   *string `json:"signature"`
	Certificate *string `json:"certificate"`
}

// artifactKinds maps the goreleaser types that are files of their own to their kind.
// "Binary" counts only with extra.Format (see artifact.published).
var artifactKinds = map[string]string{
	"Archive": "archive", "Binary": "binary", "Linux Package": "package", "File": "file", "Source": "source",
}

// attachmentSuffixes name the file an SBOM, signature or certificate covers: <name><suffix>.
var attachmentSuffixes = map[string][]string{
	"SBOM":        {".sbom.json", ".spdx.json", ".cdx.json", ".sbom"},
	"Signature":   {".sigstore.json", ".bundle", ".sig"},
	"Certificate": {".pem", ".crt", ".cert"},
}

// mediaTypes are tried in order against the end of a file name, case ignored.
var mediaTypes = []struct{ suffix, mediaType string }{
	{".tar.gz", "application/gzip"},
	{".tgz", "application/gzip"},
	{".gz", "application/gzip"},
	{".tar.xz", "application/x-xz"},
	{".xz", "application/x-xz"},
	{".tar.zst", "application/zstd"},
	{".zst", "application/zstd"},
	{".tar", "application/x-tar"},
	{".zip", "application/zip"},
	{".deb", "application/vnd.debian.binary-package"},
	{".rpm", "application/x-rpm"},
	{".exe", "application/vnd.microsoft.portable-executable"}, {".msi", "application/x-msi"},
	{".dmg", "application/x-apple-diskimage"},
	{".pdf", "application/pdf"},
	{".sql", "application/sql"},
	{".yaml", "application/yaml"},
	{".yml", "application/yaml"},
	{".json", "application/json"},
	{".sh", "application/x-sh"},
	{".md", "text/markdown"},
	{".txt", "text/plain"},
}

// multiExtensions are formats whose extension has two parts.
var multiExtensions = []string{".tar.gz", ".tar.xz", ".tar.zst", ".tar.bz2"}

func mediaType(name string) string {
	n := strings.ToLower(name)
	for _, m := range mediaTypes {
		if strings.HasSuffix(n, m.suffix) {
			return m.mediaType
		}
	}
	return "application/octet-stream"
}

// formatOf is the entry's extra.Format, else its file extension without the dot, else null.
func formatOf(a artifact) *string {
	if f, ok := a.Extra.Format.(string); ok && f != "" {
		return &f
	}
	n := strings.ToLower(a.Name)
	for _, ext := range multiExtensions {
		if strings.HasSuffix(n, ext) {
			return nullable(ext[1:])
		}
	}
	return nullable(strings.TrimPrefix(filepath.Ext(n), "."))
}

// variantKeys are the artifacts.json fields that refine goarch.
var variantKeys = []string{"goamd64", "goarm", "goarm64", "go386", "gomips", "goppc64", "goriscv64"}

func (a artifact) field(key string) string {
	var s string
	_ = json.Unmarshal(a.fields[key], &s)
	return s
}

func (a artifact) variant() *string {
	for _, k := range variantKeys {
		if v := a.field(k); v != "" {
			return &v
		}
	}
	return nil
}

// inferredTypes are the artifacts.json types inferArtifacts gives a file, by name suffix;
// any other file is a "File".
var inferredTypes = []struct{ suffix, typ string }{
	{".tar.gz", "Archive"},
	{".tgz", "Archive"},
	{".tar.xz", "Archive"},
	{".tar.zst", "Archive"},
	{".tar.bz2", "Archive"},
	{".tar", "Archive"},
	{".zip", "Archive"},
	{".deb", "Linux Package"},
	{".rpm", "Linux Package"},
	{".apk", "Linux Package"},
}

// inferArtifacts stands in for artifacts.json when none was uploaded: one entry per staged
// file of the release, typed from its name. checksums names the checksums file. A file
// named <other file><suffix of attachmentSuffixes> covers that file. The documents of the
// standard and metadata.json are not entries. No entry has goos nor goarch: who needs
// them uploads an artifacts.json.
func inferArtifacts(files []string, checksums string) []artifact {
	var arts []artifact
	for _, f := range files {
		switch {
		case f == productFile, f == changelogFile, f == metadataFile, reservedFile(f):
			continue
		case f == checksums:
			arts = append(arts, artifact{Name: f, Type: "Checksum"})
			continue
		}
		typ := "File"
		if t, ok := attachmentType(f, files); ok {
			typ = t
		} else {
			n := strings.ToLower(f)
			for _, it := range inferredTypes {
				if strings.HasSuffix(n, it.suffix) {
					typ = it.typ
					break
				}
			}
		}
		arts = append(arts, artifact{Name: f, Type: typ})
	}
	return arts
}

// attachmentType is SBOM, Signature or Certificate when name is another of files with one
// of their suffixes.
func attachmentType(name string, files []string) (string, bool) {
	for _, typ := range []string{"SBOM", "Signature", "Certificate"} {
		for _, suffix := range attachmentSuffixes[typ] {
			if target, ok := strings.CutSuffix(name, suffix); ok && target != "" && slices.Contains(files, target) {
				return typ, true
			}
		}
	}
	return "", false
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// goreleaserMetadata is the part of metadata.json release.json uses.
type goreleaserMetadata struct {
	ProjectName string `json:"project_name"`
	Tag         string `json:"tag"`
	PreviousTag string `json:"previous_tag"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	Date        string `json:"date"`
}

// releaseInput is what buildRelease needs from a publish.
type releaseInput struct {
	dir, product, version string
	filesURL              string // download URL of the files, without the trailing /name
	tokenID               string
	now                   time.Time
	published             []artifact
	sums                  map[string]string
}

// buildRelease describes the staged version in in.dir: registry.yaml, the published entries
// (of artifacts.json, or inferred from the files), and metadata.json and CHANGELOG.md when
// they are there.
func buildRelease(in releaseInput) (*ReleaseDocument, error) {
	var meta goreleaserMetadata
	if raw, err := os.ReadFile(filepath.Join(in.dir, metadataFile)); err == nil {
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, unprocessable("metadata.json is not valid JSON")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	card, err := readProductFile(filepath.Join(in.dir, productFile))
	if err != nil {
		return nil, err
	}
	changes := []Change{}
	if text, err := os.ReadFile(filepath.Join(in.dir, changelogFile)); err == nil {
		changes = parseChangelog(string(text), in.version)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	doc := &ReleaseDocument{
		APIVersion:  APIVersion,
		Product:     in.product,
		DisplayName: meta.ProjectName,
		Version:     in.version,
		Tag:         nullable(meta.Tag),
		PreviousTag: nullable(meta.PreviousTag),
		Prerelease:  isPrerelease(in.version),
		Commit:      nullable(meta.Commit),
		BuiltAt:     nullable(meta.Date),
		PublishedAt: in.now.UTC().Format(time.RFC3339),
		PublishedBy: in.tokenID,
		Deprecated:  false,
		Changes:     changes,
		Artifacts:   []ReleaseArtifact{},
	}
	if card != nil {
		if card.DisplayName != "" {
			doc.DisplayName = card.DisplayName
		}
		doc.Deprecated = card.Deprecated
	}
	if doc.DisplayName == "" {
		doc.DisplayName = in.product
	}
	for _, c := range changes {
		if c.Kind == "security" {
			doc.SecurityUpdates = true
		}
	}

	fileURL := func(name string) string { return in.filesURL + "/" + url.PathEscape(name) }
	index := map[string]int{}
	for _, a := range in.published {
		switch {
		case a.Type == "Checksum" && doc.Checksums == nil:
			doc.Checksums = &ReleaseChecksums{Name: a.Name, URL: fileURL(a.Name)}
		case artifactKinds[a.Type] != "":
			info, err := os.Stat(filepath.Join(in.dir, a.Name))
			if err != nil {
				return nil, err
			}
			index[a.Name] = len(doc.Artifacts)
			doc.Artifacts = append(doc.Artifacts, ReleaseArtifact{
				Name:      a.Name,
				Kind:      artifactKinds[a.Type],
				Format:    formatOf(a),
				MediaType: mediaType(a.Name),
				OS:        nullable(a.Goos),
				Arch:      nullable(a.Goarch),
				Variant:   a.variant(),
				Size:      info.Size(),
				Sha256:    in.sums[a.Name],
				URL:       fileURL(a.Name),
			})
		}
	}
	// Attachments come after every file, whatever their order in artifacts.json.
	for _, a := range in.published {
		for _, suffix := range attachmentSuffixes[a.Type] {
			target, ok := strings.CutSuffix(a.Name, suffix)
			if !ok {
				continue
			}
			u := fileURL(a.Name)
			if i, ok := index[target]; ok {
				attach(&doc.Artifacts[i], a.Type, u)
			} else if doc.Checksums != nil && doc.Checksums.Name == target {
				switch a.Type {
				case "Signature":
					setOnce(&doc.Checksums.Signature, u)
				case "Certificate":
					setOnce(&doc.Checksums.Certificate, u)
				}
			}
			break
		}
	}
	return doc, nil
}

func attach(r *ReleaseArtifact, typ, u string) {
	switch typ {
	case "SBOM":
		setOnce(&r.SBOM, u)
	case "Signature":
		setOnce(&r.Signature, u)
	case "Certificate":
		setOnce(&r.Certificate, u)
	}
}

// setOnce keeps the first attachment of a kind (an archive signed twice keeps its first signature).
func setOnce(p **string, u string) {
	if *p == nil {
		*p = &u
	}
}

// encodeDocument is the JSON form of a generated document: indented, "&" and "<" left as is.
func encodeDocument(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
