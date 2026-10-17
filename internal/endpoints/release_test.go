package endpoints_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/nivekalara237/local-registry/internal/core"
	"github.com/nivekalara237/local-registry/internal/endpoints"
	"github.com/nivekalara237/local-registry/internal/testutil"
)

// newReleaseServer serves the upload and release routes on a temp root, with v0.1.0 of
// gitlab published and latest.
func newReleaseServer(t *testing.T, publicURL string) (string, http.Handler) {
	t.Helper()
	root := t.TempDir()
	testutil.StubFreeSpace(t, 1<<40, true)
	s := core.NewRegistry(root)
	endpoints.AddUploadRoutes(s, endpoints.UploadConfig{Token: testutil.TestToken, PublicURL: publicURL})
	endpoints.AddReleaseRoutes(s, publicURL)
	h := s.Mux
	testutil.PutAll(t, h, "v0.1.0", testutil.Release())
	testutil.PublishOK(t, h, "/gitlab/artifacts/v0.1.0/publish?latest=true")
	return root, h
}

func TestListReleasePublished(t *testing.T) {
	root, h := newReleaseServer(t, "")
	final, _ := core.NewLayout(root).VersionDir("gitlab", "v0.1.0")
	testutil.WriteFile(t, filepath.Join(final, ".hidden"), "x")
	_ = os.Mkdir(filepath.Join(final, "dir"), 0o755)
	_ = os.Symlink("/etc/hosts", filepath.Join(final, "link"))

	var rel endpoints.Release
	for _, version := range []string{"v0.1.0", "latest"} {
		rec, _ := testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/releases/gitlab/" + version})
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", version, rec.Code, rec.Body)
		}
		if err := jsonDecode(rec.Body.Bytes(), &rel); err != nil {
			t.Fatal(err)
		}
		if rel.TagName != "v0.1.0" {
			t.Fatalf("%s: tag_name = %q", version, rel.TagName)
		}
	}
	want := []string{"artifacts.goreleaser.json", "artifacts.json", "metadata.json", "tool_0.1.0_checksums.txt",
		"tool_0.1.0_darwin_arm64.tar.gz", "tool_0.1.0_linux_amd64.tar.gz"}
	if len(rel.Assets) != len(want) {
		t.Fatalf("assets = %+v", rel.Assets)
	}
	ids := map[string]bool{}
	for i, a := range rel.Assets {
		if a.Name != want[i] {
			t.Fatalf("asset %d = %q, want %q", i, a.Name, want[i])
		}
		if a.URL != "/gitlab/artifacts/v0.1.0/files/"+a.Name {
			t.Fatalf("asset %s url = %q", a.Name, a.URL)
		}
		if a.IsChecksum != (a.Name == "tool_0.1.0_checksums.txt") {
			t.Fatalf("asset %s is_checksum = %v", a.Name, a.IsChecksum)
		}
		if _, err := uuid.Parse(a.ID); err != nil || ids[a.ID] {
			t.Fatalf("asset %s id = %q (%v), duplicate %v", a.Name, a.ID, err, ids[a.ID])
		}
		ids[a.ID] = true
		if rec, _ := testutil.Do(t, h, testutil.Call{Method: "GET", Target: a.URL}); rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d", a.URL, rec.Code)
		}
	}
	// IDs do not change from one call to the next.
	again, err := endpoints.ListRelease(core.NewLayout(root), "gitlab", "v0.1.0", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := range again.Assets {
		if again.Assets[i].ID != rel.Assets[i].ID {
			t.Fatalf("id of %s changed: %s then %s", rel.Assets[i].Name, rel.Assets[i].ID, again.Assets[i].ID)
		}
	}
}

func TestListReleasePublicURL(t *testing.T) {
	root, _ := newReleaseServer(t, "https://registry.example.com/sub/")
	testutil.WriteFile(t, filepath.Join(root, "assets/binaries/github/v1/tool"), "BIN")
	l := core.NewLayout(root)

	rel, err := endpoints.ListRelease(l, "gitlab", "v0.1.0", "https://registry.example.com/sub/")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://registry.example.com/sub/gitlab/artifacts/v0.1.0/files/metadata.json"; rel.Assets[2].URL != want {
		t.Fatalf("url = %q, want %q", rel.Assets[2].URL, want)
	}
	rel, err = endpoints.ListRelease(l, "github", "v1", "https://registry.example.com/sub/")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://registry.example.com/sub/assets/binary/assets%2Fbinaries%2Fgithub%2Fv1%2Ftool"; rel.Assets[0].URL != want {
		t.Fatalf("legacy url = %q, want %q", rel.Assets[0].URL, want)
	}
}

// A version that is not published is read from the legacy tree, for any vcs.
func TestListReleaseLegacy(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "assets/binaries/github/v1.0.0")
	testutil.WriteFile(t, filepath.Join(legacy, "tool_1.0.0_checksums.txt"), "x")
	testutil.WriteFile(t, filepath.Join(legacy, "tool.checksum.txt"), "x")
	testutil.WriteFile(t, filepath.Join(legacy, "my tool+x%.bin"), "x")
	_ = os.Mkdir(filepath.Join(legacy, "empty"), 0o755)

	rel, err := endpoints.ListRelease(core.NewLayout(root), "github", "v1.0.0", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]endpoints.ReleaseAsset{}
	for _, a := range rel.Assets {
		got[a.Name] = a
	}
	if len(got) != 3 || !got["tool_1.0.0_checksums.txt"].IsChecksum || !got["tool.checksum.txt"].IsChecksum || got["my tool+x%.bin"].IsChecksum {
		t.Fatalf("assets = %+v", rel.Assets)
	}
	if u := got["my tool+x%.bin"].URL; u != "/assets/binary/assets%2Fbinaries%2Fgithub%2Fv1.0.0%2Fmy%20tool+x%25.bin" {
		t.Fatalf("url = %q", u)
	}

	// An empty version lists [], not null.
	testutil.WriteFile(t, filepath.Join(root, "assets/binaries/github/v2/.keep"), "")
	rel, err = endpoints.ListRelease(core.NewLayout(root), "github", "v2", "")
	if err != nil || rel.Assets == nil || len(rel.Assets) != 0 {
		t.Fatalf("empty release = %+v, %v", rel, err)
	}
}

func TestListReleaseNotFound(t *testing.T) {
	root, h := newReleaseServer(t, "")
	l := core.NewLayout(root)
	for _, c := range [][2]string{{"gitlab", "v9"}, {"github", "latest"}, {"..", "v0.1.0"}, {"gitlab", ".."}, {"", "v0.1.0"}} {
		if _, err := endpoints.ListRelease(l, c[0], c[1], ""); !errors.Is(err, os.ErrNotExist) && !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("endpoints.ListRelease(%q, %q) err = %v", c[0], c[1], err)
		}
	}
	for _, target := range []string{"/releases/gitlab/v9", "/releases/github/latest", "/releases/gitlab/..%2F..", "/releases/v9"} {
		if rec, res := testutil.Do(t, h, testutil.Call{Method: "POST", Target: target}); rec.Code != http.StatusNotFound || res["error"] == nil {
			t.Fatalf("POST %s: status = %d, body = %s", target, rec.Code, rec.Body)
		}
	}
}

func TestDeprecatedReleaseRoute(t *testing.T) {
	_, h := newReleaseServer(t, "")
	rec, _ := testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/releases/v0.1.0"})
	if rec.Code != http.StatusOK || rec.Header().Get("Deprecation") != "true" ||
		rec.Header().Get("Link") != `</releases/gitlab/v0.1.0>; rel="successor-version"` {
		t.Fatalf("status = %d, headers = %v", rec.Code, rec.Header())
	}
	var rel endpoints.Release
	if err := jsonDecode(rec.Body.Bytes(), &rel); err != nil || rel.TagName != "v0.1.0" || len(rel.Assets) != 6 {
		t.Fatalf("release = %+v, %v", rel, err)
	}
}

func TestLatestVersion(t *testing.T) {
	root, h := newReleaseServer(t, "")
	l := core.NewLayout(root)
	if v, err := endpoints.LatestVersion(l, "gitlab"); err != nil || v != "v0.1.0" {
		t.Fatalf("LatestVersion = %q, %v", v, err)
	}
	if _, err := endpoints.LatestVersion(l, "github"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("no latest: err = %v", err)
	}

	// v0.2.0 published without latest leaves v0.1.0 latest; with latest, it takes over.
	files := testutil.Release()
	files["metadata.json"] = `{"project_name":"tool","tag":"v0.2.0","version":"0.2.0"}`
	testutil.PutAll(t, h, "v0.2.0", files)
	testutil.PublishOK(t, h, "/gitlab/artifacts/v0.2.0/publish")
	if v, _ := endpoints.LatestVersion(l, "gitlab"); v != "v0.1.0" {
		t.Fatalf("latest after v0.2.0 without latest = %q", v)
	}

	// Two versions with the same artifacts.json (copied by hand): ambiguous.
	vcsDir, _ := l.VCSDir("gitlab")
	same, _ := os.ReadFile(filepath.Join(vcsDir, "v0.1.0", "artifacts.json"))
	testutil.WriteFile(t, filepath.Join(vcsDir, "v0.3.0", "artifacts.json"), string(same))
	_, err := endpoints.LatestVersion(l, "gitlab")
	if status, msg := core.ErrorStatus(err); status != http.StatusConflict || !strings.Contains(msg, "v0.1.0, v0.3.0") {
		t.Fatalf("ambiguous: %d %q", status, msg)
	}
}

func TestMetadataRoute(t *testing.T) {
	root, h := newReleaseServer(t, "")
	files := testutil.Release()
	files["metadata.json"] = `{"project_name":"tool","tag":"v0.2.0","version":"0.2.0"}`
	testutil.PutAll(t, h, "v0.2.0", files)
	testutil.PublishOK(t, h, "/gitlab/artifacts/v0.2.0/publish")

	for target, tag := range map[string]string{
		"/artifacts/metadatas/gitlab":                "v0.1.0", // latest
		"/artifacts/metadatas/gitlab?version=latest": "v0.1.0",
		"/artifacts/metadatas/gitlab?version=":       "v0.1.0",
		"/artifacts/metadatas/gitlab?version=v0.2.0": "v0.2.0",
		"/artifacts/metadatas/gitlab?version=v0.1.0": "v0.1.0",
	} {
		rec, res := testutil.Do(t, h, testutil.Call{Method: "GET", Target: target})
		if rec.Code != http.StatusOK || res["tag"] != tag || res["project_name"] != "tool" {
			t.Fatalf("GET %s: status = %d, body = %s", target, rec.Code, rec.Body)
		}
	}
	for target, status := range map[string]int{
		"/artifacts/metadatas/github":                 http.StatusNotFound, // no latest
		"/artifacts/metadatas/gitlab?version=v9":      http.StatusNotFound,
		"/artifacts/metadatas/gitlab?version=..%2F..": http.StatusNotFound,
		"/artifacts/metadatas/..%2F..":                http.StatusNotFound,
	} {
		if rec, res := testutil.Do(t, h, testutil.Call{Method: "GET", Target: target}); rec.Code != status || res["error"] == nil {
			t.Fatalf("GET %s: status = %d, body = %s", target, rec.Code, rec.Body)
		}
	}

	// A broken metadata.json is a server error, its content is not echoed.
	final, _ := core.NewLayout(root).VersionDir("gitlab", "v0.2.0")
	testutil.WriteFile(t, filepath.Join(final, "metadata.json"), "{broken")
	if rec, _ := testutil.Do(t, h, testutil.Call{Method: "GET", Target: "/artifacts/metadatas/gitlab?version=v0.2.0"}); rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "broken") {
		t.Fatalf("broken metadata: status = %d, body = %s", rec.Code, rec.Body)
	}
}

func jsonDecode(b []byte, v any) error { return json.Unmarshal(b, v) }
