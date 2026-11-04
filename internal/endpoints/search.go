package endpoints

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nivekalara237/local-registry/internal/core"
)

type ProductQuery struct {
	ApiVersion string             `json:"apiVersion"`
	Cursor     *string            `json:"next"`
	Total      int64              `json:"total"`
	Limit      int                `json:"limit"`
	Items      []ProductQueryItem `json:"items"`
}

type ProductQueryItem struct {
	Product     string            `json:"product"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	Latest      *string           `json:"latest"`
	UpdatedAt   string            `json:"updatedAt"`
	Keywords    []string          `json:"keywords"`
	Labels      map[string]string `json:"labels"`
	License     string            `json:"license"`
	Deprecated  bool              `json:"deprecated"`
}
type rawProductItem struct {
	item     ProductQueryItem
	versions []IndexVersion
	owners   []Owner
}

type searchProductEntrypoint struct {
	req      *http.Request
	registry *core.Registry
	layout   core.Layout
}

func newSearchProduct(req *http.Request, reg *core.Registry) *searchProductEntrypoint {
	return &searchProductEntrypoint{req: req, registry: reg, layout: reg.Layout}
}

type querySearchParams struct {
	q, limit, sort, cursor, deprecated,
	prerelease, updateSince,
	license, owner string
	keywords, labels []string
}

type searchCursor struct {
	Key     string `json:"k"`
	Product string `json:"p"`
	Query   string `json:"q"`
}

const (
	defaultLimit = 25
	maxLimit     = 100
)

var sortFields = []string{"product", "displayName", "updatedAt"}

var searchParams = []string{"q", "keyword", "label", "owner", "license", "deprecated", "prerelease", "updatedSince", "sort", "limit", "cursor"}

func newQuerySearchParams(s *searchProductEntrypoint) *querySearchParams {
	q := s.getSingleQueryParamByName("q")
	keywords := s.getArrayQueryParamByName("keyword")
	labels := s.getArrayQueryParamByName("label")
	limitStr := s.getSingleQueryParamByName("limit")
	sort := s.getSingleQueryParamByName("sort")
	cursor := s.getSingleQueryParamByName("cursor")
	deprecatedStr := s.getSingleQueryParamByName("deprecated")
	updatedSinceStr := s.getSingleQueryParamByName("updatedSince")
	license := s.getSingleQueryParamByName("license")
	owner := s.getSingleQueryParamByName("owner")
	prereleaseStr := s.getSingleQueryParamByName("prerelease")

	return &querySearchParams{
		q:           q,
		limit:       limitStr,
		sort:        sort,
		cursor:      cursor,
		deprecated:  deprecatedStr,
		prerelease:  prereleaseStr,
		updateSince: updatedSinceStr,
		license:     license,
		owner:       owner,
		keywords:    keywords,
		labels:      labels,
	}
}

func (q *querySearchParams) isStructBooleanStr(str string) bool {
	return slices.Contains([]string{"true", "false"}, strings.ToLower(str))
}

// validate checks every parameter and reports all the invalid ones in a single 400.
func (q *querySearchParams) validate() error {

	var errs []error

	if q.deprecated != "" && !q.isStructBooleanStr(q.deprecated) {
		errs = append(errs, errors.New("deprecated must be true or false"))
	}

	if q.prerelease != "" && !q.isStructBooleanStr(q.prerelease) {
		errs = append(errs, errors.New("prerelease must be true or false"))
	}

	for _, l := range q.labels {
		if k, _, ok := strings.Cut(l, "="); !ok || k == "" {
			errs = append(errs, fmt.Errorf("label %q must be key=value, e.g. label=team=data", l))
		}
	}

	if q.sort != "" && !slices.Contains(sortFields, strings.TrimPrefix(q.sort, "-")) {
		errs = append(errs, fmt.Errorf("sort must be one of %v, with a leading - for descending order", sortFields))
	}

	if q.updateSince != "" && !q.isValideDateYYYYMMDD(q.updateSince) {
		errs = append(errs, errors.New("updatedSince must be a date: YYYY-MM-DD"))
	}

	if q.limit != "" {
		if n, err := strconv.Atoi(q.limit); err != nil || n < 1 || n > maxLimit {
			errs = append(errs, fmt.Errorf("limit must be between 1 and %d", maxLimit))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, len(errs))
	for i, err := range errs {
		msgs[i] = err.Error()
	}
	return core.Errorf(http.StatusBadRequest, "%s", strings.Join(msgs, "; "))
}

// limitValue is the page size: limit, else defaultLimit. validate has checked limit.
func (q *querySearchParams) limitValue() int {
	if n, err := strconv.Atoi(q.limit); err == nil {
		return n
	}
	return defaultLimit
}

// sortValue is the sort of the results: sort, else by product name.
func (q *querySearchParams) sortValue() string {
	if q.sort == "" {
		return "product"
	}
	return q.sort
}

func (q *querySearchParams) isValideDateYYYYMMDD(dateStr string) bool {
	t, err := time.Parse(time.DateOnly, dateStr)
	return err == nil && t.Format(time.DateOnly) == dateStr
}

// parseQuery runs the search of the request: it validates the parameters, keeps the
// products that match every criterion, then returns the page that follows the cursor.
func (s *searchProductEntrypoint) parseQuery() (*ProductQuery, error) {
	if err := s.checkUnknownParams(); err != nil {
		return nil, err
	}
	params := newQuerySearchParams(s)
	if err := params.validate(); err != nil {
		return nil, err
	}
	fingerprint := queryFingerprint(s.query())
	var after *searchCursor
	if params.cursor != "" {
		c, err := decodeCursor(params.cursor, fingerprint)
		if err != nil {
			return nil, err
		}
		after = c
	}

	out := &ProductQuery{ApiVersion: APIVersion, Limit: params.limitValue(), Items: []ProductQueryItem{}}
	raws := []rawProductItem{}
	entries, err := os.ReadDir(s.layout.BinariesDir())
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}

	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() || !core.ValidName(entry.Name()) || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		index, err := ReadIndex(s.layout, entry.Name())
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		item := ProductQueryItem{
			Product:     index.Product,
			DisplayName: index.DisplayName,
			Description: index.Description,
			Latest:      index.Latest,
			UpdatedAt:   index.UpdatedAt,
			Keywords:    index.Keywords,
			Labels:      index.Labels,
			License:     index.License,
		}
		raw := rawProductItem{
			item:     item,
			versions: index.Versions,
			owners:   index.Owners,
		}
		if v, ok := s.currentVersion(raw); ok {
			raw.item.Deprecated = isDeprecated(v.Deprecated)
		}
		raws = append(raws, raw)
	}

	for _, raw := range raws {
		if params.q != "" && !s.matchQ(raw, params.q) {
			continue
		}

		if params.license != "" && !s.eqLicense(raw, params.license) {
			continue
		}

		if params.owner != "" && !s.hasOwner(raw, params.owner) {
			continue
		}

		if params.deprecated != "" && !s.hasDeprecatedVersion(raw, params.deprecated) {
			continue
		}

		if params.prerelease != "" && !s.hasPrereleaseVersion(raw, params.prerelease) {
			continue
		}

		if len(params.labels) > 0 && !s.hasExactLabelAndOperator(raw, params.labels) {
			continue
		}

		if len(params.keywords) > 0 && !s.hasExactKeywordAndOperator(raw, params.keywords) {
			continue
		}

		if len(params.updateSince) > 0 && !s.isUpdateSince(raw, params.updateSince) {
			continue
		}

		out.Items = append(out.Items, raw.item)
	}

	out.Total = int64(len(out.Items))
	out.Items, out.Cursor = paginate(out.Items, params.sortValue(), out.Limit, after, fingerprint)
	return out, nil
}

// checkUnknownParams refuses a parameter the search does not know, so that a typo does
// not silently return the whole catalog.
func (s *searchProductEntrypoint) checkUnknownParams() error {
	for _, name := range slices.Sorted(maps.Keys(s.query())) {
		if !slices.Contains(searchParams, name) {
			return core.Errorf(http.StatusBadRequest, "unknown parameter %q: use %s", name, strings.Join(searchParams, ", "))
		}
	}
	return nil
}

// hasExactLabelAndOperator reports whether the product has every requested key=value label.
func (s *searchProductEntrypoint) hasExactLabelAndOperator(raw rawProductItem, labels []string) bool {
	rawLabels := make([]string, 0, len(raw.item.Labels))
	for k, v := range raw.item.Labels {
		rawLabels = append(rawLabels, fmt.Sprintf("%s=%s", k, v))
	}
	for _, l := range labels {
		if !slices.Contains(rawLabels, l) {
			return false
		}
	}
	return true
}

// hasExactKeywordAndOperator reports whether the product has every requested keyword,
// regardless of case.
func (s *searchProductEntrypoint) hasExactKeywordAndOperator(raw rawProductItem, keywords []string) bool {
	rawKeywords := make([]string, 0, len(raw.item.Keywords))
	for _, k := range raw.item.Keywords {
		rawKeywords = append(rawKeywords, strings.ToLower(k))
	}
	for _, k := range keywords {
		if !slices.Contains(rawKeywords, strings.ToLower(k)) {
			return false
		}
	}
	return true
}

// hasPrereleaseVersion reports whether the current version of the product (see
// currentVersion) is, or is not, a pre-release.
func (s *searchProductEntrypoint) hasPrereleaseVersion(raw rawProductItem, prereleaseStr string) bool {
	boolean, _ := strconv.ParseBool(prereleaseStr)
	v, ok := s.currentVersion(raw)
	return ok && v.Prerelease == boolean
}

// hasDeprecatedVersion reports whether the current version of the product is, or is not,
// deprecated.
func (s *searchProductEntrypoint) hasDeprecatedVersion(raw rawProductItem, deprecatedStr string) bool {
	boolean, _ := strconv.ParseBool(deprecatedStr)
	return raw.item.Deprecated == boolean
}

// currentVersion is the latest version of the product, else its highest version.
func (s *searchProductEntrypoint) currentVersion(raw rawProductItem) (IndexVersion, bool) {
	for _, v := range raw.versions {
		if raw.item.Latest != nil && v.Version == *raw.item.Latest {
			return v, true
		}
	}
	if len(raw.versions) == 0 {
		return IndexVersion{}, false
	}
	return raw.versions[0], true
}

// isDeprecated reads the deprecated field of the standard: true, false or a message.
func isDeprecated(d any) bool {
	switch v := d.(type) {
	case bool:
		return v
	case string:
		return v != ""
	}
	return false
}

// hasOwner reports whether an owner of the product has this name or email, regardless of case.
func (s *searchProductEntrypoint) hasOwner(raw rawProductItem, owner string) bool {
	return slices.ContainsFunc(raw.owners, func(o Owner) bool {
		return strings.EqualFold(o.Name, owner) || (o.Email != "" && strings.EqualFold(o.Email, owner))
	})
}

func (s *searchProductEntrypoint) eqLicense(raw rawProductItem, l string) bool {
	return strings.EqualFold(l, raw.item.License)
}

// matchQ reports whether q appears, regardless of case, in the name, display name,
// description or a keyword of the product.
func (s *searchProductEntrypoint) matchQ(raw rawProductItem, q string) bool {
	it := raw.item
	q = strings.ToLower(q)
	if strings.Contains(strings.ToLower(it.Product), q) || strings.Contains(strings.ToLower(it.DisplayName), q) || strings.Contains(strings.ToLower(it.Description), q) {
		return true
	}
	return slices.ContainsFunc(it.Keywords, func(k string) bool { return strings.Contains(strings.ToLower(k), q) })
}

func (s *searchProductEntrypoint) query() url.Values {
	return s.req.URL.Query()
}

func (s *searchProductEntrypoint) getArrayQueryParamByName(name string) []string {
	query := s.query()
	if v, ok := query[name]; ok {
		return v
	}
	return []string{}
}
func (s *searchProductEntrypoint) getSingleQueryParamByName(name string) string {
	query := s.query()
	if query.Has(name) {
		return query.Get(name)
	}
	return ""
}

// isUpdateSince reports whether the product was published on the day since or later.
func (s *searchProductEntrypoint) isUpdateSince(raw rawProductItem, since string) bool {
	updatedSince, _ := time.Parse(time.DateOnly, since)
	rawDate, err := time.Parse(time.RFC3339, raw.item.UpdatedAt)
	return err == nil && !rawDate.Before(updatedSince)
}

// encodeCursor turns c into the opaque, URL-safe token of the next page.
func encodeCursor(c searchCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

// decodeCursor reads a cursor token, and refuses it when it was made for another query.
func decodeCursor(token, fingerprint string) (*searchCursor, error) {
	var c searchCursor
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || json.Unmarshal(b, &c) != nil || c.Product == "" {
		return nil, core.Errorf(http.StatusBadRequest, "invalid cursor")
	}
	if c.Query != fingerprint {
		return nil, core.Errorf(http.StatusBadRequest, "cursor does not match the query: keep the same filters and sort")
	}
	return &c, nil
}

// queryFingerprint identifies the filters and the sort of a search: every parameter but
// cursor and limit, with sorted values, so that the page size may change between pages.
func queryFingerprint(q url.Values) string {
	v := url.Values{}
	for k, vs := range q {
		if k != "cursor" && k != "limit" {
			v[k] = slices.Sorted(slices.Values(vs))
		}
	}
	sum := sha256.Sum256([]byte(v.Encode()))
	return hex.EncodeToString(sum[:8])
}

// sortKey is the value of it that field sorts on. updatedAt is RFC 3339 in UTC, so its
// text order is its time order.
func sortKey(it ProductQueryItem, field string) string {
	switch field {
	case "displayName":
		return strings.ToLower(it.DisplayName)
	case "updatedAt":
		return it.UpdatedAt
	default:
		return it.Product
	}
}

// paginate sorts items by sort ("updatedAt", "-updatedAt"…) then by product, a total order
// since product names are unique, and returns the limit items that follow the cursor, with
// the cursor of the next page (nil on the last page). The cursor holds the sort key and the
// product of the last item sent, not a position: a product published or removed between
// two pages neither repeats nor skips a result.
func paginate(items []ProductQueryItem, sort string, limit int, after *searchCursor, fingerprint string) ([]ProductQueryItem, *string) {
	field, desc := strings.CutPrefix(sort, "-")
	compare := func(aKey, aProduct, bKey, bProduct string) int {
		c := strings.Compare(aKey, bKey)
		if desc {
			c = -c
		}
		if c != 0 {
			return c
		}
		return strings.Compare(aProduct, bProduct)
	}
	slices.SortFunc(items, func(a, b ProductQueryItem) int {
		return compare(sortKey(a, field), a.Product, sortKey(b, field), b.Product)
	})

	start := 0
	if after != nil {
		i, found := slices.BinarySearchFunc(items, *after, func(it ProductQueryItem, c searchCursor) int {
			return compare(sortKey(it, field), it.Product, c.Key, c.Product)
		})
		if found {
			i++
		}
		start = i
	}

	end := min(start+limit, len(items))
	if end == len(items) {
		return items[start:end], nil
	}
	last := items[end-1]
	next := encodeCursor(searchCursor{Key: sortKey(last, field), Product: last.Product, Query: fingerprint})
	return items[start:end], &next
}
