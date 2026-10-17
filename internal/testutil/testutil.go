// Package testutil holds the helpers shared by the tests of several packages: a registry on a
// temp asset path, HTTP calls against it, a goreleaser-like release, and audit log readers.
// Only tests import it.
package testutil

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nivekalara237/local-registry/internal/audit"
	"github.com/nivekalara237/local-registry/internal/core"
	"github.com/nivekalara237/local-registry/internal/endpoints"
	regfs "github.com/nivekalara237/local-registry/internal/fs"
)

// TestToken is the upload token of the test servers.
const TestToken = "s3cret"

// NewRegistry returns a registry with no route on a temp asset path, with plenty of free disk
// space: results must not depend on the test machine's disk.
func NewRegistry(t *testing.T) (string, *core.Registry) {
	t.Helper()
	root := t.TempDir()
	StubFreeSpace(t, 1<<40, true)
	return root, core.NewRegistry(root)
}

// NewUploadServer serves the upload routes with cfg on a NewRegistry.
func NewUploadServer(t *testing.T, cfg endpoints.UploadConfig) (string, http.Handler) {
	t.Helper()
	root, reg := NewRegistry(t)
	endpoints.AddUploadRoutes(reg, cfg)
	return root, reg.Mux
}

// StubFreeSpace makes the free-space check see free bytes (known: false, an unknown amount).
func StubFreeSpace(t *testing.T, free int64, known bool) {
	t.Helper()
	old := regfs.FreeSpace
	regfs.FreeSpace = func(string) (int64, bool) { return free, known }
	t.Cleanup(func() { regfs.FreeSpace = old })
}

// Call is one request for Do.
type Call struct {
	Method, Target, Body string
	Token                string
	Header               map[string]string
	Chunked              bool // send without Content-Length
}

// Do sends c to h and returns the response, with its body decoded when it is a JSON object.
func Do(t *testing.T, h http.Handler, c Call) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var body io.Reader = strings.NewReader(c.Body)
	if c.Chunked {
		body = io.MultiReader(strings.NewReader(c.Body)) // hides the length from NewRequest
	}
	req := httptest.NewRequest(c.Method, c.Target, body)
	if c.Chunked {
		req.ContentLength = -1
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	for k, v := range c.Header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var res map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	return rec, res
}

// Sha is the hex sha256 of s.
func Sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ReleaseArtifacts is the artifacts.json of Release(). The build output entry ("Binary"
// without extra.Format) names no uploaded file: publishing drops it.
func ReleaseArtifacts() []map[string]any {
	return []map[string]any{
		{"name": "metadata.json", "path": "dist/metadata.json", "internal_type": 36, "type": "Metadata"},
		{"name": "tool", "path": "dist/tool_linux_amd64_v1/tool", "goos": "linux", "goarch": "amd64", "internal_type": 4, "type": "Binary",
			"extra": map[string]any{"Binary": "tool", "Builder": "go", "ID": "tool"}},
		{"name": "tool_0.1.0_linux_amd64.tar.gz", "path": "dist/tool_0.1.0_linux_amd64.tar.gz", "goos": "linux", "goarch": "amd64",
			"internal_type": 1, "type": "Archive", "extra": map[string]any{"Format": "tar.gz", "ID": "tool"}},
		{"name": "tool_0.1.0_darwin_arm64.tar.gz", "path": "dist/tool_0.1.0_darwin_arm64.tar.gz", "goos": "darwin", "goarch": "arm64",
			"internal_type": 1, "type": "Archive", "extra": map[string]any{"Checksum": "sha256:" + Sha("DARWIN"), "Format": "tar.gz", "ID": "tool"}},
		{"name": "tool_0.1.0_checksums.txt", "path": "dist/tool_0.1.0_checksums.txt", "internal_type": 12, "type": "Checksum", "extra": map[string]any{}},
	}
}

// EncodeArtifacts encodes entries as an artifacts.json.
func EncodeArtifacts(t *testing.T, entries []map[string]any) string {
	t.Helper()
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Release is a goreleaser-like dist for gitlab v0.1.0, keyed by file name.
func Release() map[string]string {
	b, _ := json.MarshalIndent(ReleaseArtifacts(), "", "  ")
	files := map[string]string{
		"tool_0.1.0_linux_amd64.tar.gz":  "LINUX",
		"tool_0.1.0_darwin_arm64.tar.gz": "DARWIN",
		"artifacts.json":                 string(b),
		"metadata.json":                  `{"project_name":"tool","tag":"v0.1.0","version":"0.1.0"}`,
	}
	files["tool_0.1.0_checksums.txt"] = Sha("DARWIN") + "  tool_0.1.0_darwin_arm64.tar.gz\n" +
		Sha("LINUX") + "  tool_0.1.0_linux_amd64.tar.gz\n"
	return files
}

// PutAll uploads files to gitlab/<version>, artifacts.json last as the server requires.
func PutAll(t *testing.T, h http.Handler, version string, files map[string]string) {
	t.Helper()
	names := slices.Sorted(maps.Keys(files))
	if i := slices.Index(names, "artifacts.json"); i >= 0 {
		names = append(slices.Delete(names, i, i+1), "artifacts.json")
	}
	for _, name := range names {
		content := files[name]
		rec, res := Do(t, h, Call{Method: "PUT", Target: "/gitlab/artifacts/" + version + "/files/" + name, Body: content, Token: TestToken})
		if rec.Code != http.StatusCreated {
			t.Fatalf("PUT %s: status = %d, body = %s", name, rec.Code, rec.Body)
		}
		if res["name"] != name || res["size"] != float64(len(content)) || res["sha256"] != Sha(content) {
			t.Fatalf("PUT %s: response = %v", name, res)
		}
	}
}

// PublishOK posts target with the test token and fails unless it answers 200.
func PublishOK(t *testing.T, h http.Handler, target string) {
	t.Helper()
	if rec, _ := Do(t, h, Call{Method: "POST", Target: target, Token: TestToken}); rec.Code != http.StatusOK {
		t.Fatalf("POST %s: status = %d, body = %s", target, rec.Code, rec.Body)
	}
}

// ReadArtifacts decodes the artifacts.json at path.
func ReadArtifacts(t *testing.T, path string) []map[string]any {
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

// WriteFile writes content to path, creating its directories.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Age sets the modification time of path to ago in the past.
func Age(t *testing.T, path string, ago time.Duration) {
	t.Helper()
	old := time.Now().Add(-ago)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

// TempAudit opens an audit log in a temp directory and returns it with its path.
func TempAudit(t *testing.T) (*audit.AuditLog, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sub", "audit.log")
	a, err := audit.OpenAuditLog(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	return a, path
}

// ReadAudit parses every line of the audit log at path.
func ReadAudit(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e map[string]any
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("audit line %q: %v", sc.Text(), err)
		}
		out = append(out, e)
	}
	return out
}
