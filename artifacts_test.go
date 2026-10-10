package local_registry

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func publishOK(t *testing.T, h http.Handler, target string) {
	t.Helper()
	if rec, _ := do(t, h, call{method: "POST", target: target, token: testToken}); rec.Code != http.StatusOK {
		t.Fatalf("POST %s: status = %d, body = %s", target, rec.Code, rec.Body)
	}
}

func readArtifacts(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return entries
}

func TestPublishRewritesArtifacts(t *testing.T) {
	root, h := newUploadServer(t, UploadConfig{Token: testToken})
	files := release()
	putAll(t, h, "v0.1.0", files)
	publishOK(t, h, "/gitlab/artifacts/v0.1.0/publish")

	final, _ := NewLayout(root).VersionDir("gitlab", "v0.1.0")
	got := readArtifacts(t, filepath.Join(final, "artifacts.json"))

	// The build output entry is dropped, the others keep their order and fields, but for path.
	var want []map[string]any
	_ = json.Unmarshal([]byte(files["artifacts.json"]), &want)
	want = slices.DeleteFunc(want, func(e map[string]any) bool { return e["name"] == "tool" })
	if len(got) != len(want) {
		t.Fatalf("rewritten artifacts.json has %d entries, want %d: %v", len(got), len(want), got)
	}
	for i, e := range got {
		name := want[i]["name"].(string)
		wantPath := "/gitlab/artifacts/v0.1.0/files/" + name
		if e["name"] != name || e["path"] != wantPath {
			t.Fatalf("entry %d = %v, want name %s path %s", i, e, name, wantPath)
		}
		e["path"], want[i]["path"] = nil, nil
		a, _ := json.Marshal(e)
		b, _ := json.Marshal(want[i])
		if string(a) != string(b) {
			t.Fatalf("entry %d changed beyond path:\n got %s\nwant %s", i, a, b)
		}
		// Each path downloads the file it names.
		rec, _ := do(t, h, call{method: "GET", target: wantPath})
		if rec.Code != http.StatusOK || rec.Body.String() != files[name] {
			t.Fatalf("GET %s: status = %d, body = %q", wantPath, rec.Code, rec.Body)
		}
		if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename=`+name && cd != `attachment; filename="`+name+`"` {
			t.Fatalf("GET %s: Content-Disposition = %q", wantPath, cd)
		}
	}
}

func TestPublishPublicURL(t *testing.T) {
	root, h := newUploadServer(t, UploadConfig{Token: testToken, PublicURL: "https://registry.example.com/sub/"})
	putAll(t, h, "v0.1.0", release())
	publishOK(t, h, "/gitlab/artifacts/v0.1.0/publish?latest=true")

	latest, _ := NewLayout(root).ArtifactsFile("gitlab", "latest")
	for _, e := range readArtifacts(t, latest) {
		if want := "https://registry.example.com/sub/gitlab/artifacts/v0.1.0/files/" + e["name"].(string); e["path"] != want {
			t.Fatalf("path = %v, want %s", e["path"], want)
		}
	}
}

func TestArtifactsUploadChecks(t *testing.T) {
	missing := release()
	delete(missing, "tool_0.1.0_linux_amd64.tar.gz")

	tamperedSum := releaseArtifacts()
	tamperedSum[3]["extra"] = map[string]any{"Checksum": "sha256:" + sha("OTHER"), "Format": "tar.gz"}

	twice := append(releaseArtifacts(), releaseArtifacts()[2])

	tests := []struct {
		name      string
		staged    map[string]string // uploaded before artifacts.json
		artifacts string
		want      string // in the error message
	}{
		{"first, before the files", map[string]string{}, release()["artifacts.json"], "which is not uploaded"},
		{"a file is missing", missing, release()["artifacts.json"], "tool_0.1.0_linux_amd64.tar.gz, which is not uploaded"},
		{"checksum differs", release(), encodeArtifacts(t, tamperedSum), "artifacts.json says"},
		{"listed twice", release(), encodeArtifacts(t, twice), "twice"},
		{"name not uploadable", release(), `[{"name":"../x","path":"x","type":"Archive"}]`, "cannot be uploaded"},
		{"not JSON", release(), "{", "not a JSON array"},
		{"an object", release(), `{"name":"x"}`, "not a JSON array"},
		{"name is a number", release(), `[{"name":1,"type":"Archive"}]`, "must be strings"},
		{"null entry", release(), `[null]`, "null"},
	}
	for _, tt := range tests {
		root, h := newUploadServer(t, UploadConfig{Token: testToken})
		delete(tt.staged, "artifacts.json")
		putAll(t, h, "v0.1.0", tt.staged)
		rec, res := do(t, h, call{method: "PUT", target: "/gitlab/artifacts/v0.1.0/files/artifacts.json", body: tt.artifacts, token: testToken})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(res["error"].(string), tt.want) {
			t.Fatalf("%s: status = %d, body = %s", tt.name, rec.Code, rec.Body)
		}
		staging, _ := NewLayout(root).StagingDir("gitlab", "v0.1.0")
		if _, err := os.Stat(filepath.Join(staging, "artifacts.json")); !os.IsNotExist(err) {
			t.Fatalf("%s: refused artifacts.json was staged: %v", tt.name, err)
		}
	}
}

func TestReservedOriginalName(t *testing.T) {
	_, h := newUploadServer(t, UploadConfig{Token: testToken})
	rec, _ := do(t, h, call{method: "PUT", target: "/gitlab/artifacts/v0.1.0/files/artifacts.goreleaser.json", body: "[]", token: testToken})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
}

// A file uploaded or replaced after artifacts.json is checked again at publish.
func TestPublishRechecksArtifacts(t *testing.T) {
	root, h := newUploadServer(t, UploadConfig{Token: testToken})
	putAll(t, h, "v0.1.0", release())
	staging, _ := NewLayout(root).StagingDir("gitlab", "v0.1.0")
	if err := os.Remove(filepath.Join(staging, "tool_0.1.0_darwin_arm64.tar.gz")); err != nil {
		t.Fatal(err)
	}
	rec, res := do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish", token: testToken})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(res["error"].(string), "not uploaded") {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(staging, "artifacts.goreleaser.json")); !os.IsNotExist(err) {
		t.Fatalf("a refused publish rewrote staging: %v", err)
	}
}

// A publish that failed after its rewrite leaves both files in staging: the next one must
// start again from the uploaded artifacts.json, and a new upload of it must win.
func TestPublishAfterInterruptedRewrite(t *testing.T) {
	root, h := newUploadServer(t, UploadConfig{Token: testToken})
	files := release()
	putAll(t, h, "v0.1.0", files)
	staging, _ := NewLayout(root).StagingDir("gitlab", "v0.1.0")
	names := slices.Sorted(func(yield func(string) bool) {
		for n := range files {
			if !yield(n) {
				return
			}
		}
	})
	sums := map[string]string{}
	for _, n := range names {
		sums[n] = sha(files[n])
	}
	if _, err := rewriteStagedArtifacts(UploadConfig{PublicURL: "http://old"}, staging, "gitlab", "v0.1.0", names, sums); err != nil {
		t.Fatal(err)
	}

	publishOK(t, h, "/gitlab/artifacts/v0.1.0/publish")
	final, _ := NewLayout(root).VersionDir("gitlab", "v0.1.0")
	if got, _ := os.ReadFile(filepath.Join(final, "artifacts.goreleaser.json")); string(got) != files["artifacts.json"] {
		t.Fatalf("artifacts.goreleaser.json = %q", got)
	}
	for _, e := range readArtifacts(t, filepath.Join(final, "artifacts.json")) {
		if !strings.HasPrefix(e["path"].(string), "/gitlab/") {
			t.Fatalf("path = %v: rewritten from the rewritten file", e["path"])
		}
	}

	// Uploading artifacts.json again drops the leftover original.
	putAll(t, h, "v0.2.0", map[string]string{"x": "x"})
	staging2, _ := NewLayout(root).StagingDir("gitlab", "v0.2.0")
	if err := os.WriteFile(filepath.Join(staging2, "artifacts.goreleaser.json"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	putAll(t, h, "v0.2.0", map[string]string{"artifacts.json": "[]"})
	if _, err := os.Stat(filepath.Join(staging2, "artifacts.goreleaser.json")); !os.IsNotExist(err) {
		t.Fatalf("stale artifacts.goreleaser.json kept: %v", err)
	}
}

func TestServeReleaseFile(t *testing.T) {
	root, h := newUploadServer(t, UploadConfig{Token: testToken})
	putAll(t, h, "v0.1.0", release())
	putAll(t, h, "v0.2.0", map[string]string{"staged.bin": "STAGED"})
	publishOK(t, h, "/gitlab/artifacts/v0.1.0/publish?latest=true")
	final, _ := NewLayout(root).VersionDir("gitlab", "v0.1.0")
	_ = os.WriteFile(filepath.Join(final, ".hidden"), []byte("x"), 0o644)
	_ = os.Mkdir(filepath.Join(final, "dir"), 0o755)
	_ = os.Symlink("/etc/hosts", filepath.Join(final, "link"))

	for _, target := range []string{
		"/gitlab/artifacts/v0.2.0/files/staged.bin", // staged, not published
		"/gitlab/artifacts/latest/files/artifacts.json",
		"/gitlab/artifacts/v0.1.0/files/.hidden",
		"/gitlab/artifacts/v0.1.0/files/dir",
		"/gitlab/artifacts/v0.1.0/files/link", // leads outside the version
		"/gitlab/artifacts/v0.1.0/files/missing",
		"/gitlab/artifacts/v0.1.0/files/..%2F..%2Faudit.log",
		"/..%2F..%2Fx/artifacts/v0.1.0/files/artifacts.json",
	} {
		if rec, _ := do(t, h, call{method: "GET", target: target}); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", target, rec.Code)
		}
	}
	// No token needed to download.
	if rec, _ := do(t, h, call{method: "GET", target: "/gitlab/artifacts/v0.1.0/files/tool_0.1.0_linux_amd64.tar.gz"}); rec.Code != http.StatusOK || rec.Body.String() != "LINUX" {
		t.Fatalf("GET published file: status = %d, body = %q", rec.Code, rec.Body)
	}
}

// The token must reach neither the standard output nor the standard log.
func TestTokenNeverPrinted(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	out := make(chan []byte)
	go func() { b, _ := io.ReadAll(r); out <- b }()

	_, h := newUploadServer(t, UploadConfig{Token: testToken})
	putAll(t, h, "v0.1.0", release())
	do(t, h, call{method: "PUT", target: "/gitlab/artifacts/v0.1.0/files/x", body: "x", token: "wrong-" + testToken})
	do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish", token: testToken})

	os.Stdout = stdout
	w.Close()
	printed := <-out
	for _, s := range [][]byte{printed, logs.Bytes()} {
		if bytes.Contains(s, []byte(testToken)) {
			t.Fatalf("token printed: %s", s)
		}
	}
}
