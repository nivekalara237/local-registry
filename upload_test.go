package local_registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const testToken = "s3cret"

func newUploadServer(t *testing.T, cfg UploadConfig) (string, http.Handler) {
	t.Helper()
	root := t.TempDir()
	stubFreeSpace(t, 1<<40, true) // results must not depend on the test machine's disk
	s := NewLocalRegistryServer(0, "localhost", root)
	s.AddUploadRoutes(cfg)
	return root, s.Handler()
}

func stubFreeSpace(t *testing.T, free int64, known bool) {
	t.Helper()
	old := freeSpace
	freeSpace = func(string) (int64, bool) { return free, known }
	t.Cleanup(func() { freeSpace = old })
}

type call struct {
	method, target, body string
	token                string
	header               map[string]string
	chunked              bool // send without Content-Length
}

func do(t *testing.T, h http.Handler, c call) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var body io.Reader = strings.NewReader(c.body)
	if c.chunked {
		body = io.MultiReader(strings.NewReader(c.body)) // hides the length from NewRequest
	}
	req := httptest.NewRequest(c.method, c.target, body)
	if c.chunked {
		req.ContentLength = -1
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range c.header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var res map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	return rec, res
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// release is a goreleaser-like dist for gitlab v0.1.0, keyed by file name.
func release() map[string]string {
	files := map[string]string{
		"tool_0.1.0_linux_amd64.tar.gz":  "LINUX",
		"tool_0.1.0_darwin_arm64.tar.gz": "DARWIN",
		"artifacts.json":                 `[{"name":"tool_0.1.0_linux_amd64.tar.gz"}]`,
		"metadata.json":                  `{"project_name":"tool","tag":"v0.1.0","version":"0.1.0"}`,
	}
	files["tool_0.1.0_checksums.txt"] = sha("DARWIN") + "  tool_0.1.0_darwin_arm64.tar.gz\n" +
		sha("LINUX") + "  tool_0.1.0_linux_amd64.tar.gz\n"
	return files
}

func putAll(t *testing.T, h http.Handler, version string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		rec, res := do(t, h, call{method: "PUT", target: "/gitlab/artifacts/" + version + "/files/" + name, body: content, token: testToken})
		if rec.Code != http.StatusCreated {
			t.Fatalf("PUT %s: status = %d, body = %s", name, rec.Code, rec.Body)
		}
		if res["name"] != name || res["size"] != float64(len(content)) || res["sha256"] != sha(content) {
			t.Fatalf("PUT %s: response = %v", name, res)
		}
	}
}

func TestUploadAndPublish(t *testing.T) {
	root, h := newUploadServer(t, UploadConfig{Token: testToken})
	l := NewLayout(root)
	files := release()

	putAll(t, h, "v0.1.0", files)

	// Staged files are invisible to the catalogue until publish.
	if _, err := os.Stat(l.AssetsDir()); !os.IsNotExist(err) {
		t.Fatalf("assets dir exists before publish: %v", err)
	}

	rec, res := do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish?latest=true", token: testToken})
	if rec.Code != http.StatusOK {
		t.Fatalf("publish: status = %d, body = %s", rec.Code, rec.Body)
	}
	if res["latest"] != true || res["vcs"] != "gitlab" || res["version"] != "v0.1.0" || len(res["files"].([]any)) != len(files) {
		t.Fatalf("publish: response = %v", res)
	}

	final, _ := l.VersionDir("gitlab", "v0.1.0")
	for name, content := range files {
		got, err := os.ReadFile(filepath.Join(final, name))
		if err != nil || string(got) != content {
			t.Fatalf("published %s = %q, %v", name, got, err)
		}
	}
	latest, _ := l.ArtifactsFile("gitlab", "latest")
	if got, _ := os.ReadFile(latest); string(got) != files["artifacts.json"] {
		t.Fatalf("latest artifacts.json = %q", got)
	}
	staging, _ := l.StagingDir("gitlab", "v0.1.0")
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging still exists: %v", err)
	}

	paths, err := (&Asset{}).FindAssetPaths(l.AssetsDir())
	if err != nil {
		t.Fatal(err)
	}
	var providers []string
	for _, p := range paths {
		providers = append(providers, p.Provider)
	}
	if !slices.Equal(providers, []string{"gitlab", "v0.1.0"}) {
		t.Fatalf("catalogue providers = %v", providers)
	}
}

func TestPublishWithoutLatestLeavesLatestAlone(t *testing.T) {
	root, h := newUploadServer(t, UploadConfig{Token: testToken})
	putAll(t, h, "v0.1.0", release())

	if rec, _ := do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish", token: testToken}); rec.Code != http.StatusOK {
		t.Fatalf("publish: status = %d, body = %s", rec.Code, rec.Body)
	}
	latest, _ := NewLayout(root).ArtifactsFile("gitlab", "latest")
	if _, err := os.Stat(latest); !os.IsNotExist(err) {
		t.Fatalf("latest artifacts.json written without ?latest=true: %v", err)
	}
}

func TestPublishedVersionIsImmutable(t *testing.T) {
	_, h := newUploadServer(t, UploadConfig{Token: testToken})
	putAll(t, h, "v0.1.0", release())
	do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish", token: testToken})

	if rec, _ := do(t, h, call{method: "PUT", target: "/gitlab/artifacts/v0.1.0/files/tool_0.1.0_linux_amd64.tar.gz", body: "EVIL", token: testToken}); rec.Code != http.StatusConflict {
		t.Fatalf("PUT after publish: status = %d", rec.Code)
	}
	if rec, _ := do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish", token: testToken}); rec.Code != http.StatusConflict {
		t.Fatalf("second publish: status = %d", rec.Code)
	}
}

func TestUploadAuth(t *testing.T) {
	_, disabled := newUploadServer(t, UploadConfig{})
	_, h := newUploadServer(t, UploadConfig{Token: testToken})
	put := "/gitlab/artifacts/v0.1.0/files/a.bin"

	tests := []struct {
		name   string
		h      http.Handler
		c      call
		status int
	}{
		{"disabled", disabled, call{method: "PUT", target: put, body: "x", token: "anything"}, http.StatusForbidden},
		{"disabled publish", disabled, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish"}, http.StatusForbidden},
		{"no token", h, call{method: "PUT", target: put, body: "x"}, http.StatusUnauthorized},
		{"wrong token", h, call{method: "PUT", target: put, body: "x", token: "s3cre"}, http.StatusUnauthorized},
		{"basic auth", h, call{method: "PUT", target: put, body: "x", header: map[string]string{"Authorization": "Basic " + testToken}}, http.StatusUnauthorized},
		{"publish no token", h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish"}, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		rec, res := do(t, tt.h, tt.c)
		if rec.Code != tt.status || res["error"] == nil {
			t.Fatalf("%s: status = %d, body = %s", tt.name, rec.Code, rec.Body)
		}
		if tt.status == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("%s: no WWW-Authenticate header", tt.name)
		}
	}
}

func TestPutRejections(t *testing.T) {
	root, h := newUploadServer(t, UploadConfig{Token: testToken, MaxFileSize: 4})

	tests := []struct {
		name   string
		c      call
		status int
	}{
		{"too large, known length", call{method: "PUT", target: "/gitlab/artifacts/v1/files/a", body: "12345"}, http.StatusRequestEntityTooLarge},
		{"too large, chunked", call{method: "PUT", target: "/gitlab/artifacts/v1/files/a", body: "12345", chunked: true}, http.StatusRequestEntityTooLarge},
		{"checksum header mismatch", call{method: "PUT", target: "/gitlab/artifacts/v1/files/a", body: "1234", header: map[string]string{"X-Checksum-Sha256": sha("4321")}}, http.StatusUnprocessableEntity},
		{"traversal in name", call{method: "PUT", target: "/gitlab/artifacts/v1/files/..%2Fa", body: "1"}, http.StatusBadRequest},
		{"space in name", call{method: "PUT", target: "/gitlab/artifacts/v1/files/a%20b", body: "1"}, http.StatusBadRequest},
		{"traversal in vcs", call{method: "PUT", target: "/..%2F..%2Fx/artifacts/v1/files/a", body: "1"}, http.StatusBadRequest},
		{"latest as version", call{method: "PUT", target: "/gitlab/artifacts/latest/files/a", body: "1"}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		tt.c.token = testToken
		rec, res := do(t, h, tt.c)
		if rec.Code != tt.status || res["error"] == nil {
			t.Fatalf("%s: status = %d, body = %s", tt.name, rec.Code, rec.Body)
		}
	}

	// None of the rejected uploads left a file behind.
	var left []string
	_ = filepath.WalkDir(filepath.Join(root, "bin.local"), func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			left = append(left, path)
		}
		return nil
	})
	if len(left) != 0 {
		t.Fatalf("files left behind: %v", left)
	}

	// A matching checksum header is accepted, in any case.
	rec, _ := do(t, h, call{method: "PUT", target: "/gitlab/artifacts/v1/files/a", body: "1234", token: testToken,
		header: map[string]string{"X-Checksum-Sha256": strings.ToUpper(sha("1234"))}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("matching checksum header: status = %d, body = %s", rec.Code, rec.Body)
	}
}

func TestPublishChecks(t *testing.T) {
	tests := []struct {
		name   string
		edit   func(files map[string]string)
		status int
	}{
		{"no checksums file", func(f map[string]string) { delete(f, "tool_0.1.0_checksums.txt") }, http.StatusUnprocessableEntity},
		{"no metadata.json", func(f map[string]string) { delete(f, "metadata.json") }, http.StatusUnprocessableEntity},
		{"no artifacts.json", func(f map[string]string) { delete(f, "artifacts.json") }, http.StatusUnprocessableEntity},
		{"binary differs from checksums", func(f map[string]string) { f["tool_0.1.0_linux_amd64.tar.gz"] = "TAMPERED" }, http.StatusUnprocessableEntity},
		{"binary not in checksums", func(f map[string]string) { f["extra.bin"] = "EXTRA" }, http.StatusUnprocessableEntity},
		{"malformed checksums", func(f map[string]string) { f["tool_0.1.0_checksums.txt"] = "not a checksum line\n" }, http.StatusUnprocessableEntity},
		{"invalid artifacts.json", func(f map[string]string) { f["artifacts.json"] = "{" }, http.StatusUnprocessableEntity},
		{"metadata for another version", func(f map[string]string) {
			f["metadata.json"] = `{"tag":"v0.2.0","version":"0.2.0"}`
		}, http.StatusUnprocessableEntity},
		{"signature needs no checksum line", func(f map[string]string) { f["tool_0.1.0_checksums.txt.sig"] = "SIG" }, http.StatusOK},
		{"version without v prefix in metadata", func(f map[string]string) { f["metadata.json"] = `{"version":"0.1.0"}` }, http.StatusOK},
	}
	for _, tt := range tests {
		root, h := newUploadServer(t, UploadConfig{Token: testToken})
		files := release()
		tt.edit(files)
		putAll(t, h, "v0.1.0", files)

		rec, res := do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish", token: testToken})
		if rec.Code != tt.status {
			t.Fatalf("%s: status = %d, body = %s", tt.name, rec.Code, rec.Body)
		}
		final, _ := NewLayout(root).VersionDir("gitlab", "v0.1.0")
		_, err := os.Stat(final)
		if tt.status != http.StatusOK {
			if res["error"] == nil || !os.IsNotExist(err) {
				t.Fatalf("%s: refused publish left %s (err %v), body = %s", tt.name, final, err, rec.Body)
			}
		} else if err != nil {
			t.Fatalf("%s: not published: %v", tt.name, err)
		}
	}
}

func TestPublishNothingStaged(t *testing.T) {
	_, h := newUploadServer(t, UploadConfig{Token: testToken})
	rec, _ := do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish", token: testToken})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	rec, _ = do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish?latest=maybe", token: testToken})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("latest=maybe: status = %d", rec.Code)
	}
}
