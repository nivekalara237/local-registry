package endpoints_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nivekalara237/local-registry/internal/core"
	"github.com/nivekalara237/local-registry/internal/endpoints"
	"github.com/nivekalara237/local-registry/internal/testutil"
)

type searchVersion struct {
	version, publishedAt string
	prerelease           bool
	deprecated           any
}

// writeSearchProduct publishes product on disk as publish does: each version with its
// release.json and card, the latest version when given, then index.json.
func writeSearchProduct(t *testing.T, l core.Layout, product, displayName, card, latest string, versions ...searchVersion) {
	t.Helper()
	bin := l.BinariesDir()
	for _, v := range versions {
		dir := filepath.Join(bin, product, v.version)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		deprecated, _ := json.Marshal(v.deprecated)
		doc := fmt.Sprintf(`{"apiVersion":"local-registry/v1","product":%q,"displayName":%q,"version":%q,"publishedAt":%q,"prerelease":%v,"deprecated":%s}`,
			product, displayName, v.version, v.publishedAt, v.prerelease, deprecated)
		testutil.WriteFile(t, filepath.Join(dir, "release.json"), doc)
		testutil.WriteFile(t, filepath.Join(dir, "registry.yaml"), "apiVersion: local-registry/v1\n"+card)
	}
	if latest != "" {
		setLatest(t, bin, product, latest)
	}
	writeIndexFile(t, l, product)
}

// newSearchRegistry serves the product routes on a registry of 8 products:
//
//	alpha … golf   v1.0.0, keywords [db, k0|k1], label team=t0|t1|t2, owner Ops <ops@x.io>,
//	               updated on 2026-10-10, 11 or 12; display names "Tool Z" … "Tool T"
//	alpha          license MIT
//	hotel          v1.9.0, then v2.0.0-rc.1 (deprecated, the highest), no latest, no owner,
//	               "Liquibase migrations", updated on 2026-10-20, display name "Tool S"
func newSearchRegistry(t *testing.T) *core.Registry {
	t.Helper()
	_, reg := testutil.NewRegistry(t)
	endpoints.AddProductRoutes(reg)
	for i, name := range []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf"} {
		card := fmt.Sprintf("keywords: [db, k%d]\nlabels: {team: t%d}\nowners: [{name: Ops, email: ops@x.io}]\n", i%2, i%3)
		if name == "alpha" {
			card += "license: MIT\n"
		}
		writeSearchProduct(t, reg.Layout, name, fmt.Sprintf("Tool %c", 'Z'-i), card, "v1.0.0",
			searchVersion{"v1.0.0", fmt.Sprintf("2026-10-%02dT10:00:00Z", 10+i%3), false, false})
	}
	writeSearchProduct(t, reg.Layout, "hotel", "Tool S", "description: Liquibase migrations\n", "",
		searchVersion{"v1.9.0", "2026-10-19T10:00:00Z", false, false},
		searchVersion{"v2.0.0-rc.1", "2026-10-20T10:00:00Z", true, "use india"})
	return reg
}

// searchResponse is the answer of GET /products/search.
type searchResponse struct {
	code   int
	res    endpoints.ProductQuery
	header http.Header
	body   string
}

// searchGet sends GET target and decodes the search result.
func searchGet(t *testing.T, reg *core.Registry, target string) searchResponse {
	t.Helper()
	rec, _ := testutil.Do(t, reg.Mux, testutil.Call{Method: "GET", Target: target})
	r := searchResponse{code: rec.Code, header: rec.Header(), body: rec.Body.String()}
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &r.res); err != nil {
			t.Fatalf("GET %s: %v: %s", target, err, rec.Body)
		}
	}
	return r
}

// searchNames lists the products of a page, comma-separated.
func searchNames(res endpoints.ProductQuery) string {
	names := make([]string, 0, len(res.Items))
	for _, it := range res.Items {
		names = append(names, it.Product)
	}
	return strings.Join(names, ",")
}

// nextLink is the target of the Link rel="next" header, "" when there is none.
func nextLink(t *testing.T, h http.Header) string {
	t.Helper()
	link := h.Get("Link")
	if link == "" {
		return ""
	}
	target, ok := strings.CutSuffix(strings.TrimPrefix(link, "<"), `>; rel="next"`)
	if !ok || !strings.HasPrefix(target, "/products/search?") {
		t.Fatalf("Link = %q", link)
	}
	return target
}

func TestSearchProductsPagination(t *testing.T) {
	reg := newSearchRegistry(t)
	cases := []struct{ sort, want string }{
		{"product", "alpha,bravo,charlie,delta,echo,foxtrot,golf,hotel"},
		{"-product", "hotel,golf,foxtrot,echo,delta,charlie,bravo,alpha"},
		{"updatedAt", "alpha,delta,golf,bravo,echo,charlie,foxtrot,hotel"},
		{"-updatedAt", "hotel,charlie,foxtrot,bravo,echo,alpha,delta,golf"},
		{"displayName", "hotel,golf,foxtrot,echo,delta,charlie,bravo,alpha"},
		{"-displayName", "alpha,bravo,charlie,delta,echo,foxtrot,golf,hotel"},
	}
	for _, tc := range cases {
		t.Run(tc.sort, func(t *testing.T) {
			var pages []string
			target := "/products/search?limit=3&sort=" + url.QueryEscape(tc.sort)
			for target != "" {
				r := searchGet(t, reg, target)
				if r.code != http.StatusOK || r.res.ApiVersion != "local-registry/v1" || r.res.Total != 8 || r.res.Limit != 3 {
					t.Fatalf("GET %s: %d %s", target, r.code, r.body)
				}
				pages = append(pages, searchNames(r.res))
				target = nextLink(t, r.header)
				if (target == "") != (r.res.Cursor == nil) {
					t.Fatalf("next = %v but Link = %q", r.res.Cursor, r.header.Get("Link"))
				}
				if len(pages) > 3 {
					t.Fatalf("more than 3 pages: %v", pages)
				}
			}
			if got := strings.Join(pages, ","); got != tc.want || len(pages) != 3 {
				t.Fatalf("pages = %v, want %s in 3 pages", pages, tc.want)
			}
		})
	}
}

func TestSearchProductsDefaultLimit(t *testing.T) {
	_, reg := testutil.NewRegistry(t)
	endpoints.AddProductRoutes(reg)
	for i := range 27 {
		writeSearchProduct(t, reg.Layout, fmt.Sprintf("p%02d", i), "P", "", "v1.0.0",
			searchVersion{"v1.0.0", "2026-10-01T10:00:00Z", false, false})
	}
	r := searchGet(t, reg, "/products/search")
	if r.code != http.StatusOK || len(r.res.Items) != 25 || r.res.Limit != 25 || r.res.Total != 27 || r.res.Cursor == nil {
		t.Fatalf("GET /products/search: %d %s", r.code, r.body)
	}
	r = searchGet(t, reg, "/products/search?limit=100&cursor="+url.QueryEscape(*r.res.Cursor))
	if r.code != http.StatusOK || searchNames(r.res) != "p25,p26" || r.res.Cursor != nil {
		t.Fatalf("second page with a larger limit: %d %s", r.code, r.body)
	}
}

func TestSearchProductsEmptyRegistry(t *testing.T) {
	_, reg := testutil.NewRegistry(t)
	endpoints.AddProductRoutes(reg)
	r := searchGet(t, reg, "/products/search")
	if r.code != http.StatusOK || r.header.Get("Link") != "" ||
		!strings.Contains(r.body, `"items":[]`) || !strings.Contains(r.body, `"total":0`) || !strings.Contains(r.body, `"next":null`) {
		t.Fatalf("GET /products/search on an empty registry: %d %s", r.code, r.body)
	}
}

func TestSearchProductsChangesBetweenPages(t *testing.T) {
	reg := newSearchRegistry(t)
	first := searchGet(t, reg, "/products/search?limit=2").res
	if searchNames(first) != "alpha,bravo" || first.Cursor == nil {
		t.Fatalf("first page = %s", searchNames(first))
	}
	next := "/products/search?limit=3&cursor=" + url.QueryEscape(*first.Cursor)

	writeSearchProduct(t, reg.Layout, "aaa", "A", "", "v1.0.0", searchVersion{"v1.0.0", "2026-10-25T10:00:00Z", false, false})
	writeSearchProduct(t, reg.Layout, "bzz", "B", "", "v1.0.0", searchVersion{"v1.0.0", "2026-10-25T10:00:00Z", false, false})
	res := searchGet(t, reg, next).res
	if searchNames(res) != "bzz,charlie,delta" || res.Total != 10 {
		t.Fatalf("after inserts: page = %s, total = %d", searchNames(res), res.Total)
	}

	if err := os.RemoveAll(filepath.Join(reg.Layout.BinariesDir(), "bravo")); err != nil {
		t.Fatal(err)
	}
	res = searchGet(t, reg, next).res
	if searchNames(res) != "bzz,charlie,delta" {
		t.Fatalf("after removing the last product of the page: page = %s", searchNames(res))
	}
}

func TestSearchProductsFilters(t *testing.T) {
	reg := newSearchRegistry(t)
	const seven = "alpha,bravo,charlie,delta,echo,foxtrot,golf"
	cases := []struct{ query, want string }{
		{"q=LIQUI", "hotel"},
		{"q=k1", "bravo,delta,foxtrot"},
		{"q=tool+z", "alpha"},
		{"q=golf", "golf"},
		{"keyword=DB&keyword=k1", "bravo,delta,foxtrot"},
		{"keyword=nope", ""},
		{"label=team=t0", "alpha,delta,golf"},
		{"label=team=t0&keyword=k0", "alpha,golf"},
		{"label=team=t0&label=team=t1", ""},
		{"owner=OPS@x.io", seven},
		{"owner=ops", seven},
		{"owner=someone", ""},
		{"license=mit", "alpha"},
		{"deprecated=true", "hotel"},
		{"deprecated=FALSE", seven},
		{"prerelease=true", "hotel"},
		{"prerelease=false", seven},
		{"updatedSince=2026-10-12", "charlie,foxtrot,hotel"},
		{"updatedSince=2026-10-21", ""},
		{"q=tool&keyword=db&label=team=t2&owner=ops&deprecated=false&prerelease=false&updatedSince=2026-10-01", "charlie,foxtrot"},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			r := searchGet(t, reg, "/products/search?"+tc.query)
			if r.code != http.StatusOK || searchNames(r.res) != tc.want || r.res.Total != int64(len(r.res.Items)) {
				t.Fatalf("got %d [%s] total %d, want [%s]: %s", r.code, searchNames(r.res), r.res.Total, tc.want, r.body)
			}
		})
	}
}

func TestSearchProductsItem(t *testing.T) {
	reg := newSearchRegistry(t)
	res := searchGet(t, reg, "/products/search?q=alpha").res
	if len(res.Items) != 1 {
		t.Fatalf("items = %+v", res.Items)
	}
	it := res.Items[0]
	if it.DisplayName != "Tool Z" || it.Latest == nil || *it.Latest != "v1.0.0" || it.UpdatedAt != "2026-10-10T10:00:00Z" ||
		it.License != "MIT" || it.Labels["team"] != "t0" || strings.Join(it.Keywords, ",") != "db,k0" || it.Deprecated {
		t.Fatalf("alpha = %+v", it)
	}
	res = searchGet(t, reg, "/products/search?q=hotel").res
	if len(res.Items) != 1 || res.Items[0].Latest != nil || !res.Items[0].Deprecated {
		t.Fatalf("hotel = %+v", res.Items)
	}
}

func TestSearchProductsInvalid(t *testing.T) {
	reg := newSearchRegistry(t)
	noProduct := base64.RawURLEncoding.EncodeToString([]byte(`{"k":"alpha"}`))
	cases := []struct{ query, want string }{
		{"limit=0", "limit must be between 1 and 100"},
		{"limit=101", "limit must be between 1 and 100"},
		{"limit=abc", "limit must be between 1 and 100"},
		{"sort=size", "sort must be one of"},
		{"sort=--product", "sort must be one of"},
		{"deprecated=yes", "deprecated must be true or false"},
		{"prerelease=1", "prerelease must be true or false"},
		{"label=bad", `label \"bad\" must be key=value`},
		{"label==x", `label \"=x\" must be key=value`},
		{"updatedSince=2026-13-01", "updatedSince must be a date: YYYY-MM-DD"},
		{"updatedSince=2026-10-12T00:00:00Z", "updatedSince must be a date: YYYY-MM-DD"},
		{"typo=1", `unknown parameter \"typo\"`},
		{"cursor=%21%21", "invalid cursor"},
		{"cursor=" + noProduct, "invalid cursor"},
		{"limit=0&deprecated=yes", "deprecated must be true or false; limit must be between 1 and 100"},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			r := searchGet(t, reg, "/products/search?"+tc.query)
			if r.code != http.StatusBadRequest || !strings.Contains(r.body, tc.want) {
				t.Fatalf("got %d %s, want 400 with %q", r.code, r.body, tc.want)
			}
		})
	}
}

func TestSearchProductsCursorMismatch(t *testing.T) {
	reg := newSearchRegistry(t)
	first := searchGet(t, reg, "/products/search?limit=2&keyword=db").res
	cursor := url.QueryEscape(*first.Cursor)
	for _, query := range []string{"keyword=db&sort=-product", "keyword=k0", "", "keyword=db&q=a"} {
		r := searchGet(t, reg, "/products/search?limit=2&cursor="+cursor+"&"+query)
		if r.code != http.StatusBadRequest || !strings.Contains(r.body, "cursor does not match the query") {
			t.Errorf("cursor with %q: %d %s", query, r.code, r.body)
		}
	}
	r := searchGet(t, reg, "/products/search?limit=5&keyword=db&cursor="+cursor)
	if r.code != http.StatusOK || searchNames(r.res) != "charlie,delta,echo,foxtrot,golf" {
		t.Fatalf("same query, other limit: %d %s", r.code, r.body)
	}
}
