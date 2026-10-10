package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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

// setup points the global args at a temp asset root and returns the server's routes.
func setup(t *testing.T) (string, http.Handler) {
	t.Helper()
	root := t.TempDir()
	bins := filepath.Join(root, "bin.local/assets/binaries/gitlab")
	writeFile(t, filepath.Join(bins, "artifacts.json"), `{"version":"latest"}`)
	writeFile(t, filepath.Join(bins, "v0.1.0/artifacts.json"), `{"version":"v0.1.0"}`)
	writeFile(t, filepath.Join(bins, "v0.1.0/metadata.json"), `{"project_name":"tool"}`)
	writeFile(t, filepath.Join(bins, "v0.1.0/tool_0.1.0_checksums.txt"), "abc  tool_linux\n")
	// /releases reads a different root: <asset-path>/assets/binaries/gitlab/<version>.
	writeFile(t, filepath.Join(root, "assets/binaries/gitlab/v0.1.0/tool.checksum.txt"), "abc")
	writeFile(t, filepath.Join(root, "assets/binaries/gitlab/v0.1.0/tool_linux"), "BIN")

	old := args
	args = arguments{port: 0, addr: "localhost", absAsset: root}
	t.Cleanup(func() { args = old })
	s, err := newServer()
	if err != nil {
		t.Fatal(err)
	}
	return root, s.Handler()
}

func get(t *testing.T, h http.Handler, method, target string, into any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s: status = %d", method, target, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("%s %s: Content-Type = %q", method, target, ct)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		t.Fatalf("%s %s: %v (body %q)", method, target, err, rec.Body.String())
	}
}

func TestVersionedArtifacts(t *testing.T) {
	_, h := setup(t)
	tests := []struct{ target, want string }{
		{"/gitlab/artifacts/v0.1.0", "v0.1.0"},
		{"/gitlab/artifacts/latest", "latest"},
	}
	for _, tt := range tests {
		var body map[string]string
		get(t, h, http.MethodGet, tt.target, &body)
		if body["version"] != tt.want {
			t.Fatalf("%s: body = %v", tt.target, body)
		}
	}
}

func TestArtifactsWithoutVersionIsLatest(t *testing.T) {
	_, h := setup(t)

	var body map[string]string
	get(t, h, http.MethodGet, "/gitlab/artifacts", &body)
	if body["version"] != "latest" {
		t.Fatalf("body = %v", body)
	}
}

func TestErrorsAreJSON(t *testing.T) {
	root, h := setup(t)
	writeFile(t, filepath.Join(root, "bin.local/assets/binaries/gitlab/v0.2.0/artifacts.json"), "{not json")

	tests := []struct {
		method, target string
		status         int
	}{
		{http.MethodGet, "/gitlab/artifacts/v9.9.9", http.StatusNotFound},
		{http.MethodGet, "/github/artifacts", http.StatusNotFound},
		{http.MethodGet, "/..%2F..%2F..%2Fx/artifacts/v0.1.0", http.StatusNotFound},
		{http.MethodGet, "/gitlab/artifacts/..%2F..", http.StatusNotFound},
		{http.MethodGet, "/gitlab/artifacts/v0.2.0", http.StatusInternalServerError},
		{http.MethodGet, "/artifacts/metadatas", http.StatusNotFound},
		{http.MethodPost, "/releases/v9.9.9", http.StatusNotFound},
		{http.MethodPost, "/releases/..%2F..", http.StatusNotFound},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))
		if rec.Code != tt.status {
			t.Fatalf("%s %s: status = %d, want %d", tt.method, tt.target, rec.Code, tt.status)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] == "" {
			t.Fatalf("%s %s: body = %q", tt.method, tt.target, rec.Body.String())
		}
	}
}

func TestAllArtifactsMissingRoot(t *testing.T) {
	root, h := setup(t)
	if err := os.RemoveAll(filepath.Join(root, "bin.local")); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/artifacts/all", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAllArtifacts(t *testing.T) {
	_, h := setup(t)

	var body []map[string]string
	get(t, h, http.MethodGet, "/artifacts/all", &body)

	byProvider := map[string]map[string]string{}
	for _, e := range body {
		byProvider[e["provider"]] = e
	}
	if len(body) != 2 || len(byProvider) != 2 {
		t.Fatalf("body = %v", body)
	}
	if got := byProvider["gitlab"]["artifacts_json"]; got != `{"version":"latest"}` {
		t.Fatalf("gitlab artifacts_json = %q", got)
	}
	v := byProvider["v0.1.0"]
	if v["artifacts_json"] != `{"version":"v0.1.0"}` || v["metadata_json"] != `{"project_name":"tool"}` || v["checksums_text"] != "abc  tool_linux\n" {
		t.Fatalf("v0.1.0 entry = %v", v)
	}
}

func TestReleases(t *testing.T) {
	root, h := setup(t)

	var body ReleaseEntry
	get(t, h, http.MethodPost, "/releases/v0.1.0", &body)

	if body.TagName != "v0.1.0" || len(body.Assets) != 2 {
		t.Fatalf("body = %+v", body)
	}
	wantNames := []string{"tool.checksum.txt", "tool_linux"}
	for i, a := range body.Assets {
		if a.Name != wantNames[i] {
			t.Fatalf("asset %d name = %q, want %q", i, a.Name, wantNames[i])
		}
		if want := "/assets/binary/assets%2Fbinaries%2Fgitlab%2Fv0.1.0%2F" + a.Name; a.Url != want {
			t.Fatalf("asset %d url = %q, want %q", i, a.Url, want)
		}
		if strings.Contains(a.Url, root) {
			t.Fatalf("asset %d url reveals the disk path: %q", i, a.Url)
		}
		// The url downloads the file it names.
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, a.Url, nil))
		want, _ := os.ReadFile(filepath.Join(root, "assets/binaries/gitlab/v0.1.0", a.Name))
		if rec.Code != http.StatusOK || rec.Body.String() != string(want) {
			t.Fatalf("GET %s: status = %d, body = %q, want %q", a.Url, rec.Code, rec.Body.String(), want)
		}
		if _, err := uuid.Parse(a.Id); err != nil {
			t.Fatalf("asset %d id = %q: %v", i, a.Id, err)
		}
	}
	if !body.Assets[0].IsChecksum || body.Assets[1].IsChecksum {
		t.Fatalf("is_checksum = %v, %v", body.Assets[0].IsChecksum, body.Assets[1].IsChecksum)
	}
}

func TestReleaseAssetURLEscapesNames(t *testing.T) {
	if got := releaseAssetURL("v1", "my tool+x%.bin"); got != "/assets/binary/assets%2Fbinaries%2Fgitlab%2Fv1%2Fmy%20tool+x%25.bin" {
		t.Fatalf("url = %q", got)
	}
}

func TestIsLoopback(t *testing.T) {
	for addr, want := range map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true, "0.0.0.0": false, "192.168.1.10": false, "": false} {
		if got := isLoopback(addr); got != want {
			t.Errorf("isLoopback(%q) = %v", addr, got)
		}
	}
}

func TestLimitZeroMeansNoLimit(t *testing.T) {
	if limit(int64(0)) != -1 || limit(0) != -1 || limit(time.Duration(0)) != -1 {
		t.Fatal("0 must map to -1 (no limit)")
	}
	if limit(int64(5)) != 5 || limit(time.Hour) != time.Hour {
		t.Fatal("non-zero values must pass through")
	}
}
