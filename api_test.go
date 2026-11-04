package local_registry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/nivekalara237/local-registry/internal/testutil"
)

// publishedLayout publishes testutil.Release as gitlab v0.1.0, the latest version, through the
// upload routes, and returns the layout of the asset path.
func publishedLayout(t *testing.T) Layout {
	t.Helper()
	root := t.TempDir()
	testutil.StubFreeSpace(t, 1<<40, true)
	s := NewLocalRegistryServer(0, "localhost", root)
	s.AddUploadRoutes(UploadConfig{Token: testutil.TestToken})
	testutil.PutAll(t, s.Handler(), "v0.1.0", testutil.Release())
	testutil.PublishOK(t, s.Handler(), "/gitlab/artifacts/v0.1.0/publish?latest=true")
	return NewLayout(root)
}

func TestAPINames(t *testing.T) {
	for name, want := range map[string]bool{"tool": true, "v1.0.0": true, "": false, ".": false, "..": false, "a/b": false} {
		if got := ValidName(name); got != want {
			t.Errorf("ValidName(%q) = %v, want %v", name, got, want)
		}
	}
	if IsPublicFile("../etc/passwd") {
		t.Error("IsPublicFile accepts a path outside the asset path")
	}
	id := TokenID(testutil.TestToken)
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(id) || id != TokenID(testutil.TestToken) || id == TokenID("other") {
		t.Errorf("TokenID = %q", id)
	}
}

func TestAPIOpenAuditLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "audit.log")
	a, err := OpenAuditLog(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("audit log: %v, mode %v", err, info.Mode())
	}
}

func TestAPIReadPublishedVersion(t *testing.T) {
	l := publishedLayout(t)

	latest, err := LatestVersion(l, "gitlab")
	if err != nil || latest != "v0.1.0" {
		t.Fatalf("LatestVersion = %q, %v", latest, err)
	}

	rel, err := ListRelease(l, "gitlab", "latest", "https://registry.example.com")
	if err != nil || rel.TagName != "v0.1.0" || len(rel.Assets) == 0 {
		t.Fatalf("ListRelease = %+v, %v", rel, err)
	}

	meta, err := ReadMetadata(l, "gitlab", "v0.1.0")
	if m, ok := meta.(map[string]any); err != nil || !ok || m["tag"] != "v0.1.0" {
		t.Fatalf("ReadMetadata = %v, %v", meta, err)
	}

	built, err := BuildIndex(l, "gitlab")
	if err != nil || built.APIVersion != APIVersion || built.Latest == nil || *built.Latest != "v0.1.0" {
		t.Fatalf("BuildIndex = %+v, %v", built, err)
	}
	read, err := ReadIndex(l, "gitlab")
	if err != nil || read.Product != built.Product || !slices.Equal(read.Keywords, built.Keywords) ||
		len(read.Versions) != 1 || read.Versions[0].Version != "v0.1.0" {
		t.Fatalf("ReadIndex = %+v, %v; BuildIndex = %+v", read, err, built)
	}

	products, err := ListProducts(l)
	if err != nil || len(products) != 1 || products[0].Product != "gitlab" || products[0].Latest == nil || *products[0].Latest != "v0.1.0" {
		t.Fatalf("ListProducts = %+v, %v", products, err)
	}

	raw, err := ReadReleaseDocument(l, "gitlab", "latest")
	var doc ReleaseDocument
	if err != nil || json.Unmarshal(raw, &doc) != nil || doc.APIVersion != APIVersion || doc.Version != "v0.1.0" {
		t.Fatalf("ReadReleaseDocument = %s, %v", raw, err)
	}
}

func TestAPINotFound(t *testing.T) {
	l := publishedLayout(t)
	notFound := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: error = %v, want not found", name, err)
		}
	}
	_, err := LatestVersion(l, "missing")
	notFound("LatestVersion", err)
	_, err = ListRelease(l, "gitlab", "v9.9.9", "")
	notFound("ListRelease", err)
	_, err = ReadMetadata(l, "gitlab", "v9.9.9")
	notFound("ReadMetadata", err)
	_, err = BuildIndex(l, "missing")
	notFound("BuildIndex", err)
	_, err = ReadIndex(l, "missing")
	notFound("ReadIndex", err)
	_, err = ReadReleaseDocument(l, "gitlab", "v9.9.9")
	notFound("ReadReleaseDocument", err)

	products, err := ListProducts(NewLayout(t.TempDir()))
	if err != nil || len(products) != 0 {
		t.Fatalf("ListProducts on an empty asset path = %+v, %v", products, err)
	}
}
