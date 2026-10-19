package endpoints

import (
	"cmp"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/nivekalara237/local-registry/internal/core"
)

// ProductIndex is index.json: the product card and the versions published with a release.json.
type ProductIndex struct {
	APIVersion  string            `json:"apiVersion"`
	Product     string            `json:"product"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	License     string            `json:"license"`
	Owners      []Owner           `json:"owners"`
	Links       []Link            `json:"links"`
	Keywords    []string          `json:"keywords"`
	Labels      map[string]string `json:"labels"`
	Install     string            `json:"install"`
	// Latest is the version publish?latest=true designated last, null if none.
	Latest    *string        `json:"latest"`
	UpdatedAt string         `json:"updatedAt"`
	Versions  []IndexVersion `json:"versions"`
}

// IndexVersion is one version of a ProductIndex.
type IndexVersion struct {
	Version         string  `json:"version"`
	Tag             *string `json:"tag"`
	PublishedAt     string  `json:"publishedAt"`
	Prerelease      bool    `json:"prerelease"`
	SecurityUpdates bool    `json:"securityUpdates"`
	Deprecated      any     `json:"deprecated"`
}

// ProductSummary is one product of GET /products.
type ProductSummary struct {
	Product     string  `json:"product"`
	DisplayName string  `json:"displayName"`
	Description string  `json:"description"`
	Latest      *string `json:"latest"`
	UpdatedAt   string  `json:"updatedAt"`
}

// BuildIndex describes product from the disk: every published version that has a release.json,
// and the card (registry.yaml) of the latest version, else of the last one published.
// It answers ErrNotFound for a product with no such version.
func BuildIndex(l core.Layout, product string) (*ProductIndex, error) {
	if product == reservedProduct {
		return nil, core.ErrNotFound
	}
	dir, err := l.ProductDir(product)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var docs []ReleaseDocument
	for _, e := range entries {
		if !e.IsDir() || !core.ValidName(e.Name()) || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name(), releaseFile))
		if errors.Is(err, os.ErrNotExist) {
			continue // published before release.json existed
		}
		if err != nil {
			return nil, err
		}
		var d ReleaseDocument
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, err
		}
		d.Version = e.Name() // the directory is the truth, as for every route
		docs = append(docs, d)
	}
	if len(docs) == 0 {
		return nil, core.ErrNotFound
	}
	slices.SortStableFunc(docs, func(a, b ReleaseDocument) int { return compareVersions(b, a) })

	idx := &ProductIndex{
		APIVersion: APIVersion,
		Product:    product,
		Owners:     []Owner{},
		Links:      []Link{},
		Keywords:   []string{},
		Labels:     map[string]string{},
		Versions:   make([]IndexVersion, 0, len(docs)),
	}
	if latest, err := LatestVersion(l, product); err == nil {
		idx.Latest = &latest
	}
	cardFrom := &docs[0]
	for i := range docs {
		d := &docs[i]
		idx.Versions = append(idx.Versions, IndexVersion{
			Version: d.Version, Tag: d.Tag, PublishedAt: d.PublishedAt,
			Prerelease: d.Prerelease, SecurityUpdates: d.SecurityUpdates, Deprecated: d.Deprecated,
		})
		idx.UpdatedAt = max(idx.UpdatedAt, d.PublishedAt)
		if d.PublishedAt > cardFrom.PublishedAt {
			cardFrom = d
		}
	}
	if idx.Latest != nil {
		for i := range docs {
			if docs[i].Version == *idx.Latest {
				cardFrom = &docs[i]
			}
		}
	}

	idx.DisplayName = cardFrom.DisplayName
	card, err := readProductFile(filepath.Join(dir, cardFrom.Version, productFile))
	if err == nil && card != nil {
		idx.Description, idx.License, idx.Install = card.Description, card.License, card.Install
		if card.Owners != nil {
			idx.Owners = card.Owners
		}
		if card.Links != nil {
			idx.Links = card.Links
		}
		if card.Keywords != nil {
			idx.Keywords = card.Keywords
		}
		if card.Labels != nil {
			idx.Labels = card.Labels
		}
	}
	return idx, nil
}

// writeIndex rewrites <product>/index.json from the disk, atomically.
func writeIndex(l core.Layout, product string) error {
	idx, err := BuildIndex(l, product)
	if err != nil {
		return err
	}
	data, err := encodeDocument(idx)
	if err != nil {
		return err
	}
	dir, _ := l.ProductDir(product)
	return writeAtomic(filepath.Join(dir, indexFile), data)
}

// ListProducts summarises every product that has a version with a release.json, by name.
func ListProducts(l core.Layout) ([]ProductSummary, error) {
	out := []ProductSummary{}
	entries, err := os.ReadDir(l.BinariesDir())
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || !core.ValidName(e.Name()) || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		idx, err := BuildIndex(l, e.Name())
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, ProductSummary{
			Product: idx.Product, DisplayName: idx.DisplayName, Description: idx.Description,
			Latest: idx.Latest, UpdatedAt: idx.UpdatedAt,
		})
	}
	return out, nil
}

// ReadReleaseDocument returns the release.json of <product>/<version> ("latest" allowed) as is.
func ReadReleaseDocument(l core.Layout, product, version string) (json.RawMessage, error) {
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
	raw, err := os.ReadFile(filepath.Join(dir, releaseFile))
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		return nil, core.Errorf(http.StatusInternalServerError, "%s/%s/%s is not valid JSON", product, version, releaseFile)
	}
	return raw, nil
}

// AddProductRoutes registers the read-only routes of the local-registry/v1 standard on reg:
//
//	GET /products                        every product, with its latest version
//	GET /{product}/index                 index.json of a product
//	GET /{product}/releases/{version}    release.json of a version ("latest" allowed)
func AddProductRoutes(reg *core.Registry) {
	l := reg.Layout
	reg.AddRouteE("/products", "GET", func(...string) (any, error) {
		return ListProducts(l)
	})
	reg.AddRouteE("/{product}/index", "GET", func(pvs ...string) (any, error) {
		return BuildIndex(l, pvs[0])
	})
	reg.AddRouteE("/{product}/releases/{version}", "GET", func(pvs ...string) (any, error) {
		return ReadReleaseDocument(l, pvs[0], pvs[1])
	})
}

var semverRe = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

type semver struct {
	core [3]uint64
	pre  []string
}

func parseSemver(v string) (semver, bool) {
	m := semverRe.FindStringSubmatch(v)
	if m == nil {
		return semver{}, false
	}
	var s semver
	for i := range 3 {
		n, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return semver{}, false
		}
		s.core[i] = n
	}
	if m[4] != "" {
		s.pre = strings.Split(m[4], ".")
	}
	return s, true
}

// isPrerelease reports a semver pre-release version, such as 1.5.0-rc.1.
func isPrerelease(version string) bool {
	s, ok := parseSemver(version)
	return ok && len(s.pre) > 0
}

// compareSemver orders by semver precedence: 1.0.0-alpha < 1.0.0-alpha.1 < 1.0.0-beta < 1.0.0.
func compareSemver(a, b semver) int {
	if c := slices.Compare(a.core[:], b.core[:]); c != 0 {
		return c
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := range min(len(a.pre), len(b.pre)) {
		x, y := a.pre[i], b.pre[i]
		nx, errx := strconv.ParseUint(x, 10, 64)
		ny, erry := strconv.ParseUint(y, 10, 64)
		var c int
		switch {
		case errx == nil && erry == nil:
			c = cmp.Compare(nx, ny)
		case errx == nil:
			c = -1 // numeric identifiers sort first
		case erry == nil:
			c = 1
		default:
			c = strings.Compare(x, y)
		}
		if c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a.pre), len(b.pre))
}

// compareVersions orders semver versions by precedence, after every other version, which
// are ordered by publication date.
func compareVersions(a, b ReleaseDocument) int {
	sa, oka := parseSemver(a.Version)
	sb, okb := parseSemver(b.Version)
	switch {
	case oka && okb:
		if c := compareSemver(sa, sb); c != 0 {
			return c
		}
	case oka:
		return 1
	case okb:
		return -1
	}
	return cmp.Compare(a.PublishedAt, b.PublishedAt)
}
