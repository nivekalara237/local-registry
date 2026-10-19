package endpoints

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

const (
	// APIVersion versions every document of the local-registry/v1 standard.
	APIVersion = "local-registry/v1"
	// productFile is the product card a team writes by hand, uploaded with each version.
	productFile = "registry.yaml"
	// changelogFile is the release notes: goreleaser's, or a Keep a Changelog file.
	changelogFile = "CHANGELOG.md"
	// metadataFile is goreleaser's description of a build; optional.
	metadataFile = "metadata.json"
	// checksumsFile is the checksums file publishing writes when none was uploaded.
	checksumsFile = "checksums.txt"
	// releaseFile describes a published version; publishing writes it.
	releaseFile = "release.json"
	// indexFile describes a product and its versions; publishing rewrites it.
	indexFile = "index.json"
	// reservedProduct is the product name GET /products would make ambiguous.
	reservedProduct = "products"

	maxProductFileSize = 64 << 10
	maxChangelogSize   = 1 << 20
)

// ProductFile is registry.yaml: the product card, and whether this version is deprecated.
// Every field is optional but apiVersion.
type ProductFile struct {
	APIVersion  string            `yaml:"apiVersion" json:"apiVersion"`
	DisplayName string            `yaml:"displayName" json:"displayName"`
	Description string            `yaml:"description" json:"description"`
	License     string            `yaml:"license" json:"license"`
	Owners      []Owner           `yaml:"owners" json:"owners"`
	Links       []Link            `yaml:"links" json:"links"`
	Keywords    []string          `yaml:"keywords" json:"keywords"`
	Labels      map[string]string `yaml:"labels" json:"labels"`
	Install     string            `yaml:"install" json:"install"`
	// Deprecated is false, true or a message such as "replaced by sonarbridge2".
	Deprecated any `yaml:"deprecated" json:"deprecated"`
}

// reservedFile reports a name the server writes: no upload, nor artifacts.json entry, may use it.
func reservedFile(name string) bool {
	return name == artifactsFile || name == originalArtifactsFile || name == releaseFile || name == indexFile
}

type Owner struct {
	Name  string `yaml:"name" json:"name"`
	Email string `yaml:"email" json:"email,omitempty"`
}

type Link struct {
	Name string `yaml:"name" json:"name"`
	URL  string `yaml:"url" json:"url"`
}

// parseProductFile decodes and validates registry.yaml. Every problem is a 422 that names it.
func parseProductFile(data []byte) (*ProductFile, error) {
	invalid := func(format string, a ...any) error {
		return unprocessable(productFile+": "+format, a...)
	}
	if len(data) > maxProductFileSize {
		return nil, invalid("larger than %d KiB", maxProductFileSize>>10)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var p ProductFile
	if err := dec.Decode(&p); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, invalid("empty")
		}
		return nil, invalid("%v", strings.TrimPrefix(err.Error(), "yaml: "))
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, invalid("holds several YAML documents")
	}

	switch {
	case p.APIVersion != APIVersion:
		return nil, invalid("apiVersion must be %s", APIVersion)
	case utf8.RuneCountInString(p.DisplayName) > 100:
		return nil, invalid("displayName is longer than 100 characters")
	case utf8.RuneCountInString(p.Description) > 500:
		return nil, invalid("description is longer than 500 characters")
	case len(p.Keywords) > 20:
		return nil, invalid("more than 20 keywords")
	case len(p.Labels) > 50:
		return nil, invalid("more than 50 labels")
	case len(p.Install) > 10<<10:
		return nil, invalid("install is larger than 10 KiB")
	}
	for i, o := range p.Owners {
		if o.Name == "" {
			return nil, invalid("owners[%d] has no name", i)
		}
		if o.Email != "" && !strings.Contains(o.Email, "@") {
			return nil, invalid("owners[%d].email %q is not an email address", i, o.Email)
		}
	}
	for i, l := range p.Links {
		u, err := url.Parse(l.URL)
		if l.Name == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, invalid("links[%d] needs a name and an http(s) url", i)
		}
	}
	for i, k := range p.Keywords {
		if strings.TrimSpace(k) == "" {
			return nil, invalid("keywords[%d] is empty", i)
		}
	}
	switch d := p.Deprecated.(type) {
	case nil, bool:
	case string:
		if d == "" {
			p.Deprecated = false
		}
	default:
		return nil, invalid("deprecated must be true, false or a message")
	}
	if p.Deprecated == nil {
		p.Deprecated = false
	}
	return &p, nil
}

// readProductFile reads the registry.yaml at path; a missing file gives nil, nil.
func readProductFile(path string) (*ProductFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseProductFile(data)
}

// checkUploadedChangelog refuses a CHANGELOG.md that is too large or not text.
func checkUploadedChangelog(file string, size int64) error {
	if size > maxChangelogSize {
		return unprocessable("%s is larger than %d MiB", changelogFile, maxChangelogSize>>20)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if !utf8.Valid(data) {
		return unprocessable("%s is not UTF-8 text", changelogFile)
	}
	return nil
}

// checkUploadedProductFile validates a registry.yaml being uploaded.
func checkUploadedProductFile(file string, size int64) error {
	if size > maxProductFileSize {
		return unprocessable("%s is larger than %d KiB", productFile, maxProductFileSize>>10)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if _, err := parseProductFile(data); err != nil {
		return err
	}
	return nil
}
