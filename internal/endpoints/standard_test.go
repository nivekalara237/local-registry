package endpoints_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nivekalara237/local-registry/internal/core"
	"github.com/nivekalara237/local-registry/internal/endpoints"
	"github.com/nivekalara237/local-registry/internal/testutil"
	"github.com/nivekalara237/local-registry/schema"
)

const productYAML = `apiVersion: local-registry/v1
displayName: The Tool
description: Does things.
owners: [{name: Platform}]
`

// standardRelease is Release() with a product file, a changelog, a plain file, an SBOM of
// an archive and the signature of the checksums file, for version (tag) v<v>.
func standardRelease(t *testing.T, v string) map[string]string {
	t.Helper()
	files := testutil.Release()
	arts := testutil.ReleaseArtifacts()
	if v != "0.1.0" {
		files = map[string]string{"metadata.json": `{"project_name":"tool","tag":"v` + v + `","version":"` + v + `"}`}
		arts = []map[string]any{{"name": "metadata.json", "path": "dist/metadata.json", "type": "Metadata"}}
	}
	files["policy.yaml"] = "rules: []\n"
	files["tool.sbom.json"] = "{}"
	if v == "0.1.0" {
		files["tool_0.1.0_linux_amd64.tar.gz.sbom.json"] = "{}"
		files["tool_0.1.0_checksums.txt.sigstore.json"] = "{}"
		arts = append(arts,
			map[string]any{"name": "tool_0.1.0_linux_amd64.tar.gz.sbom.json", "path": "dist/x", "type": "SBOM"},
			map[string]any{"name": "tool_0.1.0_checksums.txt.sigstore.json", "path": "dist/y", "type": "Signature"})
	}
	arts = append(arts,
		map[string]any{"name": "policy.yaml", "path": "policy.yaml", "type": "File"},
		map[string]any{"name": "tool.sbom.json", "path": "dist/z", "type": "SBOM"}) // covers no file
	sums := ""
	for _, n := range []string{"policy.yaml", "tool.sbom.json", "tool_0.1.0_darwin_arm64.tar.gz", "tool_0.1.0_linux_amd64.tar.gz", "tool_0.1.0_linux_amd64.tar.gz.sbom.json"} {
		if c, ok := files[n]; ok {
			sums += testutil.Sha(c) + "  " + n + "\n"
		}
	}
	checksums := "tool_" + v + "_checksums.txt"
	delete(files, "tool_0.1.0_checksums.txt")
	files[checksums] = sums
	if v != "0.1.0" {
		arts = append(arts, map[string]any{"name": checksums, "path": "dist/" + checksums, "type": "Checksum"})
	}
	files["registry.yaml"] = productYAML
	files["CHANGELOG.md"] = "## Changelog\n* abc1234: feat: first\n* abc1235: fix(security): CVE\n"
	files["artifacts.json"] = testutil.EncodeArtifacts(t, arts)
	return files
}

func TestPublishWritesReleaseAndIndex(t *testing.T) {
	root, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken, PublicURL: "https://r.example.com"})
	reg := core.NewRegistry(root)
	endpoints.AddProductRoutes(reg)
	l := core.NewLayout(root)

	testutil.PutAll(t, h, "v0.1.0", standardRelease(t, "0.1.0"))
	testutil.PublishOK(t, h, "/gitlab/artifacts/v0.1.0/publish?latest=true")

	final, _ := l.VersionDir("gitlab", "v0.1.0")
	raw, err := os.ReadFile(filepath.Join(final, "release.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc endpoints.ReleaseDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	base := "https://r.example.com/gitlab/artifacts/v0.1.0/files/"
	if doc.APIVersion != "local-registry/v1" || doc.Product != "gitlab" || doc.DisplayName != "The Tool" ||
		doc.Version != "v0.1.0" || *doc.Tag != "v0.1.0" || doc.Prerelease || doc.PublishedBy != "1ec1c26b50d5" ||
		doc.Deprecated != false || !doc.SecurityUpdates || len(doc.Changes) != 2 || doc.Changes[0].Kind != "added" {
		t.Fatalf("release.json = %s", raw)
	}
	if c := doc.Checksums; c == nil || c.Name != "tool_0.1.0_checksums.txt" || c.Signature == nil ||
		*c.Signature != base+"tool_0.1.0_checksums.txt.sigstore.json" {
		t.Fatalf("checksums = %+v", c)
	}
	byName := map[string]endpoints.ReleaseArtifact{}
	for _, a := range doc.Artifacts {
		byName[a.Name] = a
	}
	if len(doc.Artifacts) != 3 { // the two archives and policy.yaml; SBOMs and signatures are attached
		t.Fatalf("artifacts = %+v", doc.Artifacts)
	}
	linux := byName["tool_0.1.0_linux_amd64.tar.gz"]
	if linux.Kind != "archive" || *linux.Format != "tar.gz" || linux.MediaType != "application/gzip" || *linux.OS != "linux" ||
		*linux.Arch != "amd64" || linux.Size != 5 || linux.Sha256 != testutil.Sha("LINUX") || linux.URL != base+"tool_0.1.0_linux_amd64.tar.gz" ||
		linux.SBOM == nil || *linux.SBOM != base+"tool_0.1.0_linux_amd64.tar.gz.sbom.json" || linux.Signature != nil {
		t.Fatalf("linux archive = %+v", linux)
	}
	if p := byName["policy.yaml"]; p.Kind != "file" || *p.Format != "yaml" || p.MediaType != "application/yaml" || p.OS != nil || p.Arch != nil {
		t.Fatalf("policy.yaml = %+v", p)
	}
	for _, f := range []string{"registry.yaml", "CHANGELOG.md", "release.json"} {
		if _, err := os.Stat(filepath.Join(final, f)); err != nil {
			t.Errorf("%s not published: %v", f, err)
		}
	}

	// A second version, not latest, with another card: the index keeps the latest's.
	files := standardRelease(t, "0.2.0-rc.1")
	files["registry.yaml"] = "apiVersion: local-registry/v1\ndisplayName: Next Tool\n"
	testutil.PutAll(t, h, "v0.2.0-rc.1", files)
	testutil.PublishOK(t, h, "/gitlab/artifacts/v0.2.0-rc.1/publish")

	productDir, _ := l.ProductDir("gitlab")
	var onDisk endpoints.ProductIndex
	raw, _ = os.ReadFile(filepath.Join(productDir, "index.json"))
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("index.json: %v", err)
	}
	rec, _ := testutil.Do(t, reg.Mux, testutil.Call{Method: "GET", Target: "/gitlab/index"})
	var idx endpoints.ProductIndex
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &idx) != nil {
		t.Fatalf("GET index: %d %s", rec.Code, rec.Body)
	}
	if idx.DisplayName != "The Tool" || idx.Description != "Does things." || idx.Latest == nil || *idx.Latest != "v0.1.0" ||
		len(idx.Versions) != 2 || idx.Versions[0].Version != "v0.2.0-rc.1" || !idx.Versions[0].Prerelease ||
		idx.Versions[1].Version != "v0.1.0" || idx.UpdatedAt == "" || len(idx.Owners) != 1 || onDisk.UpdatedAt != idx.UpdatedAt {
		t.Fatalf("index = %+v\non disk = %+v", idx, onDisk)
	}

	rec, _ = testutil.Do(t, reg.Mux, testutil.Call{Method: "GET", Target: "/gitlab/releases/latest"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":"v0.1.0"`) {
		t.Fatalf("GET releases/latest: %d %s", rec.Code, rec.Body)
	}
	rec, _ = testutil.Do(t, reg.Mux, testutil.Call{Method: "GET", Target: "/gitlab/releases/v0.2.0-rc.1"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"displayName":"Next Tool"`) {
		t.Fatalf("GET releases/v0.2.0-rc.1: %d %s", rec.Code, rec.Body)
	}
	rec, _ = testutil.Do(t, reg.Mux, testutil.Call{Method: "HEAD", Target: "/gitlab/releases/v0.1.0"})
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD releases: %d", rec.Code)
	}

	var products []endpoints.ProductSummary
	rec, _ = testutil.Do(t, reg.Mux, testutil.Call{Method: "GET", Target: "/products"})
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &products) != nil || len(products) != 1 ||
		products[0].Product != "gitlab" || *products[0].Latest != "v0.1.0" || products[0].DisplayName != "The Tool" {
		t.Fatalf("GET /products: %d %s", rec.Code, rec.Body)
	}
}

func TestProductRoutesNotFound(t *testing.T) {
	root, reg := testutil.NewRegistry(t)
	endpoints.AddProductRoutes(reg)

	rec, _ := testutil.Do(t, reg.Mux, testutil.Call{Method: "GET", Target: "/products"})
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("GET /products on an empty registry: %d %s", rec.Code, rec.Body)
	}
	// A version published before release.json existed is not in the index.
	dir, _ := core.NewLayout(root).VersionDir("old", "v1.0.0")
	testutil.WriteFile(t, filepath.Join(dir, "metadata.json"), `{"tag":"v1.0.0"}`)
	for _, target := range []string{"/old/index", "/old/releases/v1.0.0", "/old/releases/latest", "/missing/index", "/products/index", "/old/releases/..%2Fx"} {
		if rec, _ := testutil.Do(t, reg.Mux, testutil.Call{Method: "GET", Target: target}); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d %s", target, rec.Code, rec.Body)
		}
	}
}

func TestStandardUploadRules(t *testing.T) {
	_, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken})
	put := func(target, body string) (int, string) {
		rec, _ := testutil.Do(t, h, testutil.Call{Method: "PUT", Target: target, Body: body, Token: testutil.TestToken})
		return rec.Code, rec.Body.String()
	}
	for _, tt := range []struct {
		name, target, body string
		status             int
		want               string
	}{
		{"reserved product", "/products/artifacts/v1/files/x", "x", http.StatusBadRequest, "product name products is reserved"},
		{"release.json is written by publish", "/p/artifacts/v1/files/release.json", "{}", http.StatusBadRequest, "release.json is reserved"},
		{"index.json is written by publish", "/p/artifacts/v1/files/index.json", "{}", http.StatusBadRequest, "index.json is reserved"},
		{"reserved version", "/p/artifacts/release.json/files/x", "x", http.StatusBadRequest, "version name release.json is reserved"},
		{"invalid product file", "/p/artifacts/v1/files/registry.yaml", "apiVersion: x\n", http.StatusUnprocessableEntity, "registry.yaml: apiVersion"},
		{"binary changelog", "/p/artifacts/v1/files/CHANGELOG.md", "\xff\xfe", http.StatusUnprocessableEntity, "not UTF-8"},
		{"valid product file", "/p/artifacts/v1/files/registry.yaml", productYAML, http.StatusCreated, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if code, body := put(tt.target, tt.body); code != tt.status || !strings.Contains(body, tt.want) {
				t.Fatalf("%d %s", code, body)
			}
		})
	}
}

func TestProductFileRequired(t *testing.T) {
	_, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken})
	files := standardRelease(t, "0.1.0")
	delete(files, "registry.yaml")
	testutil.PutAll(t, h, "v0.1.0", files)
	rec, _ := testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish", Token: testutil.TestToken})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "registry.yaml not uploaded") {
		t.Fatalf("publish without registry.yaml: %d %s", rec.Code, rec.Body)
	}
	testutil.PutAll(t, h, "v0.1.0", map[string]string{"registry.yaml": productYAML})
	testutil.PublishOK(t, h, "/gitlab/artifacts/v0.1.0/publish")
}

// A version needs no goreleaser: registry.yaml and the files are enough.
func TestPublishWithoutGoreleaser(t *testing.T) {
	root, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken})
	reg := core.NewRegistry(root)
	endpoints.AddProductRoutes(reg)
	l := core.NewLayout(root)
	productDir, _ := l.ProductDir("gitlab")
	publish := func(target string) (int, string) {
		rec, _ := testutil.Do(t, h, testutil.Call{Method: "POST", Target: target, Token: testutil.TestToken})
		return rec.Code, rec.Body.String()
	}

	// A goreleaser version first, latest: <product>/artifacts.json is its artifacts.json.
	testutil.PutAll(t, h, "v0.1.0", testutil.Release())
	testutil.PublishOK(t, h, "/gitlab/artifacts/v0.1.0/publish?latest=true")
	if _, err := os.Stat(filepath.Join(productDir, "artifacts.json")); err != nil {
		t.Fatal(err)
	}

	// registry.yaml alone is not a release.
	testutil.PutAll(t, h, "v1.0.0", map[string]string{"registry.yaml": productYAML})
	if code, body := publish("/gitlab/artifacts/v1.0.0/publish"); code != http.StatusUnprocessableEntity || !strings.Contains(body, "no artifact to publish") {
		t.Fatalf("registry.yaml alone: %d %s", code, body)
	}
	testutil.PutAll(t, h, "v1.0.0", map[string]string{
		"db.tar.gz":          "DB",
		"db.tar.gz.sig":      "SIG",
		"README.txt":         "read me",
		"CHANGELOG.md":       "## [1.0.0] - 2026-10-18\n### Added\n- orders table\n### Security\n- no default password\n## [0.1.0]\n### Added\n- users\n",
		"metadata.json.orig": "x",
	})
	// A metadata.json, optional, must still describe the version.
	testutil.PutAll(t, h, "v1.0.0", map[string]string{"metadata.json": `{"tag":"v0.9.0"}`})
	if code, body := publish("/gitlab/artifacts/v1.0.0/publish"); code != http.StatusUnprocessableEntity || !strings.Contains(body, "not v1.0.0") {
		t.Fatalf("wrong metadata.json: %d %s", code, body)
	}
	testutil.PutAll(t, h, "v1.0.0", map[string]string{"metadata.json": `{"tag":"v1.0.0","commit":"abc"}`})

	if code, body := publish("/gitlab/artifacts/v1.0.0/publish?latest=true"); code != http.StatusOK ||
		!strings.Contains(body, `"files":["CHANGELOG.md","README.txt","checksums.txt","db.tar.gz","db.tar.gz.sig","metadata.json","metadata.json.orig","registry.yaml","release.json"]`) {
		t.Fatalf("publish: %d %s", code, body)
	}
	final, _ := l.VersionDir("gitlab", "v1.0.0")
	sums, _ := os.ReadFile(filepath.Join(final, "checksums.txt"))
	if want := testutil.Sha("read me") + "  README.txt\n" + testutil.Sha("DB") + "  db.tar.gz\n" + testutil.Sha("x") + "  metadata.json.orig\n"; string(sums) != want {
		t.Fatalf("generated checksums.txt =\n%s\nwant\n%s", sums, want)
	}
	var doc endpoints.ReleaseDocument
	raw, _ := os.ReadFile(filepath.Join(final, "release.json"))
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	base := "/gitlab/artifacts/v1.0.0/files/"
	if doc.DisplayName != "The Tool" || *doc.Tag != "v1.0.0" || *doc.Commit != "abc" || !doc.SecurityUpdates || len(doc.Changes) != 2 ||
		doc.Checksums == nil || doc.Checksums.Name != "checksums.txt" || doc.Checksums.URL != base+"checksums.txt" || len(doc.Artifacts) != 3 {
		t.Fatalf("release.json = %s", raw)
	}
	byName := map[string]endpoints.ReleaseArtifact{}
	for _, a := range doc.Artifacts {
		byName[a.Name] = a
	}
	if db := byName["db.tar.gz"]; db.Kind != "archive" || *db.Format != "tar.gz" || db.OS != nil || db.Arch != nil ||
		db.Sha256 != testutil.Sha("DB") || db.Signature == nil || *db.Signature != base+"db.tar.gz.sig" {
		t.Fatalf("db.tar.gz = %+v", db)
	}
	if r := byName["README.txt"]; r.Kind != "file" || r.MediaType != "text/plain" {
		t.Fatalf("README.txt = %+v", r)
	}

	// The latest is now v1.0.0, which has no artifacts.json: neither has the product.
	if v, err := endpoints.LatestVersion(l, "gitlab"); err != nil || v != "v1.0.0" {
		t.Fatalf("LatestVersion = %q, %v", v, err)
	}
	if _, err := os.Stat(filepath.Join(productDir, "artifacts.json")); !os.IsNotExist(err) {
		t.Fatalf("<product>/artifacts.json of an older latest left: %v", err)
	}
	rec, _ := testutil.Do(t, reg.Mux, testutil.Call{Method: "GET", Target: "/gitlab/releases/latest"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":"v1.0.0"`) {
		t.Fatalf("GET releases/latest: %d %s", rec.Code, rec.Body)
	}

	// An uploaded checksums file is checked, and kept.
	testutil.PutAll(t, h, "v1.1.0", map[string]string{"registry.yaml": productYAML, "db.zip": "ZIP", "sums_checksums.txt": testutil.Sha("other") + "  db.zip\n"})
	if code, body := publish("/gitlab/artifacts/v1.1.0/publish"); code != http.StatusUnprocessableEntity || !strings.Contains(body, "db.zip: sha256 is") {
		t.Fatalf("wrong checksums: %d %s", code, body)
	}
	testutil.PutAll(t, h, "v1.1.0", map[string]string{"sums_checksums.txt": testutil.Sha("ZIP") + "  db.zip\n"})
	if code, body := publish("/gitlab/artifacts/v1.1.0/publish"); code != http.StatusOK || strings.Contains(body, `"checksums.txt"`) {
		t.Fatalf("publish with checksums: %d %s", code, body)
	}
}

// artifacts.json without a checksums file: the written checksums.txt is the release's.
func TestPublishWritesChecksums(t *testing.T) {
	root, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken})
	files := testutil.Release()
	delete(files, "tool_0.1.0_checksums.txt")
	var arts []map[string]any
	for _, a := range testutil.ReleaseArtifacts() {
		if a["type"] != "Checksum" {
			arts = append(arts, a)
		}
	}
	files["artifacts.json"] = testutil.EncodeArtifacts(t, arts)
	testutil.PutAll(t, h, "v0.1.0", files)
	testutil.PublishOK(t, h, "/gitlab/artifacts/v0.1.0/publish")

	final, _ := core.NewLayout(root).VersionDir("gitlab", "v0.1.0")
	sums, _ := os.ReadFile(filepath.Join(final, "checksums.txt"))
	if want := testutil.Sha("DARWIN") + "  tool_0.1.0_darwin_arm64.tar.gz\n" + testutil.Sha("LINUX") + "  tool_0.1.0_linux_amd64.tar.gz\n"; string(sums) != want {
		t.Fatalf("checksums.txt =\n%s", sums)
	}
	var doc endpoints.ReleaseDocument
	raw, _ := os.ReadFile(filepath.Join(final, "release.json"))
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Checksums == nil || doc.Checksums.Name != "checksums.txt" || len(doc.Artifacts) != 2 {
		t.Fatalf("release.json = %s, %v", raw, err)
	}
}

func TestAddProductSchemaRoutes(t *testing.T) {
	_, registry := testutil.NewRegistry(t)
	endpoints.AddProductSchemaRoutes(registry)

	resp, _ := testutil.Do(t, registry.Mux, testutil.Call{Method: "GET", Target: "/registry/json-schema"})

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /registry/json-schema: %d %s", resp.Code, resp.Body)
	}
}

func TestAddProductSchemaInvalidation(t *testing.T) {
	orig := schema.RegistrySchemaJson
	t.Cleanup(func() {
		schema.RegistrySchemaJson = orig
	})
	_, registry := testutil.NewRegistry(t)
	endpoints.AddProductSchemaRoutes(registry)

	for _, invalidJson := range [][]byte{
		make([]byte, 0),
		[]byte(`"wrong": "value"`),
	} {
		schema.RegistrySchemaJson = invalidJson
		resp, _ := testutil.Do(t, registry.Mux, testutil.Call{Method: "GET", Target: "/registry/json-schema"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("GET /registry/json-schema: %d %s", resp.Code, resp.Body)
		}
	}
}
