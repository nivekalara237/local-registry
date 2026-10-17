package endpoints

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assetTree builds binaries/gitlab/{artifacts.json, v0.1.0/{artifacts.json, metadata.json, tool_0.1.0_checksums.txt, tool_linux}}.
func assetTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "binaries/gitlab/artifacts.json"), `{"v":"latest"}`)
	writeFile(t, filepath.Join(root, "binaries/gitlab/v0.1.0/artifacts.json"), `{"v":"0.1.0"}`)
	writeFile(t, filepath.Join(root, "binaries/gitlab/v0.1.0/metadata.json"), `{"m":1}`)
	writeFile(t, filepath.Join(root, "binaries/gitlab/v0.1.0/tool_0.1.0_checksums.txt"), "abc  tool_linux\n")
	writeFile(t, filepath.Join(root, "binaries/gitlab/v0.1.0/tool_linux"), "BIN")
	return root
}

func sortByProvider[T any](items []T, provider func(T) string) {
	slices.SortFunc(items, func(a, b T) int { return strings.Compare(provider(a), provider(b)) })
}

func TestFindAssetPaths(t *testing.T) {
	root := assetTree(t)

	got, err := (&Asset{}).FindAssetPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	sortByProvider(got, func(p AssetPath) string { return p.Provider })

	want := []AssetPath{
		{Provider: "gitlab", ArtifactPath: filepath.Join(root, "binaries/gitlab/artifacts.json")},
		{
			Provider:      "v0.1.0",
			ArtifactPath:  filepath.Join(root, "binaries/gitlab/v0.1.0/artifacts.json"),
			MetadataPath:  filepath.Join(root, "binaries/gitlab/v0.1.0/metadata.json"),
			ChecksumsPath: filepath.Join(root, "binaries/gitlab/v0.1.0/tool_0.1.0_checksums.txt"),
		},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestFindAssetPathsIsSortedAndIgnoresSignatures(t *testing.T) {
	root := t.TempDir()
	for _, v := range []string{"v0.3.0", "v0.1.0", "v0.2.0"} {
		writeFile(t, filepath.Join(root, "binaries/gitlab", v, "artifacts.json"), "{}")
	}
	dir := filepath.Join(root, "binaries/gitlab/v0.2.0")
	writeFile(t, filepath.Join(dir, "tool_0.2.0_Checksums.txt"), "abc")
	// Written after the checksums file in walk order, so it would win if it matched.
	writeFile(t, filepath.Join(dir, "tool_0.2.0_Checksums.txt.sig"), "sig")

	for range 5 { // map iteration order changes between calls
		got, err := (&Asset{}).FindAssetPaths(root)
		if err != nil {
			t.Fatal(err)
		}
		var providers []string
		for _, p := range got {
			providers = append(providers, p.Provider)
		}
		if !slices.Equal(providers, []string{"v0.1.0", "v0.2.0", "v0.3.0"}) {
			t.Fatalf("providers = %v", providers)
		}
		if want := filepath.Join(dir, "tool_0.2.0_Checksums.txt"); got[1].ChecksumsPath != want {
			t.Fatalf("checksums = %q, want %q", got[1].ChecksumsPath, want)
		}
	}
}

func TestFindAssetPathsMissingRoot(t *testing.T) {
	if _, err := (&Asset{}).FindAssetPaths(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected an error for a missing root")
	}
}

func TestFindAssetContents(t *testing.T) {
	root := assetTree(t)

	got, err := (&Asset{}).FindAssetContents(root)
	if err != nil {
		t.Fatal(err)
	}
	sortByProvider(got, func(c AssetContents) string { return c.Provider })

	want := []AssetContents{
		{Provider: "gitlab", ArtifactJson: `{"v":"latest"}`},
		{Provider: "v0.1.0", ArtifactJson: `{"v":"0.1.0"}`, MetadataJson: `{"m":1}`, ChecksumsText: "abc  tool_linux\n"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
