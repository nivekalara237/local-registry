package endpoints_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nivekalara237/local-registry/internal/core"
	"github.com/nivekalara237/local-registry/internal/endpoints"
	"github.com/nivekalara237/local-registry/internal/testutil"
)

// writeVersion writes a published version of product as publish leaves it: release.json and registry.yaml.
func writeVersion(t testing.TB, bin, product, version, publishedAt string) {
	t.Helper()
	dir := filepath.Join(bin, product, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := fmt.Sprintf(`{"apiVersion":"local-registry/v1","product":%q,"displayName":"Tool","version":%q,"publishedAt":%q,"prerelease":false,"deprecated":false}`, product, version, publishedAt)
	if err := os.WriteFile(filepath.Join(dir, "release.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte("apiVersion: local-registry/v1\ndescription: Does things.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setLatest points <product>/release.json at version, as publish?latest=true does.
func setLatest(t testing.TB, bin, product, version string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(bin, product, "release.json"), []byte(`{"version":"`+version+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeIndexFile writes <product>/index.json from the disk, as publish does.
func writeIndexFile(t testing.TB, l core.Layout, product string) {
	t.Helper()
	idx, err := endpoints.BuildIndex(l, product)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(idx)
	if err := os.WriteFile(filepath.Join(l.BinariesDir(), product, "index.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// editIndexDescription changes the description in index.json only, to tell an index read from
// the file from a rebuilt one.
func editIndexDescription(t *testing.T, bin, product, description string) {
	t.Helper()
	path := filepath.Join(bin, product, "index.json")
	var idx endpoints.ProductIndex
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &idx); err != nil {
		t.Fatal(err)
	}
	idx.Description = description
	data, _ := json.Marshal(idx)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadIndex(t *testing.T) {
	cases := []struct {
		name     string
		change   func(t *testing.T, bin string)
		fromFile bool // index.json is still read: the edited description comes back
	}{
		{"index.json up to date", func(*testing.T, string) {}, true},
		{"version without release.json", func(t *testing.T, bin string) {
			if err := os.MkdirAll(filepath.Join(bin, "tool", "v0.0.1"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, true},
		{"index.json missing", func(t *testing.T, bin string) {
			if err := os.Remove(filepath.Join(bin, "tool", "index.json")); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"index.json corrupted", func(t *testing.T, bin string) {
			if err := os.WriteFile(filepath.Join(bin, "tool", "index.json"), []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"version added", func(t *testing.T, bin string) {
			writeVersion(t, bin, "tool", "v1.2.0", "2026-10-03T10:00:00Z")
		}, false},
		{"version removed", func(t *testing.T, bin string) {
			if err := os.RemoveAll(filepath.Join(bin, "tool", "v1.0.0")); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"latest changed", func(t *testing.T, bin string) {
			setLatest(t, bin, "tool", "v1.0.0")
		}, false},
		{"latest removed", func(t *testing.T, bin string) {
			if err := os.Remove(filepath.Join(bin, "tool", "release.json")); err != nil {
				t.Fatal(err)
			}
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, reg := testutil.NewRegistry(t)
			bin := reg.Layout.BinariesDir()
			writeVersion(t, bin, "tool", "v1.0.0", "2026-10-01T10:00:00Z")
			writeVersion(t, bin, "tool", "v1.1.0", "2026-10-02T10:00:00Z")
			setLatest(t, bin, "tool", "v1.1.0")
			writeIndexFile(t, reg.Layout, "tool")
			editIndexDescription(t, bin, "tool", "from index.json")

			tc.change(t, bin)

			got, err := endpoints.ReadIndex(reg.Layout, "tool")
			if err != nil {
				t.Fatal(err)
			}
			if tc.fromFile {
				if got.Description != "from index.json" {
					t.Fatalf("index.json not read: description = %q", got.Description)
				}
				return
			}
			want, err := endpoints.BuildIndex(reg.Layout, "tool")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("stale index.json served:\ngot  %+v\nwant %+v", got, want)
			}
		})
	}
}

func TestReadIndexNotFound(t *testing.T) {
	_, reg := testutil.NewRegistry(t)
	bin := reg.Layout.BinariesDir()
	writeVersion(t, bin, "tool", "v1.0.0", "2026-10-01T10:00:00Z")
	writeIndexFile(t, reg.Layout, "tool")
	if err := os.RemoveAll(filepath.Join(bin, "tool", "v1.0.0")); err != nil {
		t.Fatal(err)
	}
	for _, product := range []string{"tool", "missing", "products", ".."} {
		if _, err := endpoints.ReadIndex(reg.Layout, product); !errors.Is(err, core.ErrNotFound) && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("ReadIndex(%q) error = %v, want not found", product, err)
		}
	}
}

// benchProduct writes a product of n versions, its latest version and its index.json.
func benchProduct(b *testing.B, n int) core.Layout {
	reg := core.NewRegistry(b.TempDir())
	bin := reg.Layout.BinariesDir()
	for i := range n {
		writeVersion(b, bin, "tool", fmt.Sprintf("v1.0.%d", i), fmt.Sprintf("2026-10-01T10:%02d:00Z", i%60))
	}
	setLatest(b, bin, "tool", fmt.Sprintf("v1.0.%d", n-1))
	writeIndexFile(b, reg.Layout, "tool")
	return reg.Layout
}

func BenchmarkBuildIndex50(b *testing.B) {
	l := benchProduct(b, 50)
	for b.Loop() {
		if _, err := endpoints.BuildIndex(l, "tool"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadIndex50(b *testing.B) {
	l := benchProduct(b, 50)
	for b.Loop() {
		if _, err := endpoints.ReadIndex(l, "tool"); err != nil {
			b.Fatal(err)
		}
	}
}
