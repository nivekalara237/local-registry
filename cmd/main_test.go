package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	localregistry "sonarbridge-go/local-registry"
	"testing"
	"time"
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
	// A version only in the legacy tree, which /releases also reads.
	writeFile(t, filepath.Join(root, "assets/binaries/gitlab/v0.0.9/tool.checksum.txt"), "abc")
	writeFile(t, filepath.Join(root, "assets/binaries/gitlab/v0.0.9/tool_linux"), "BIN")

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
		{http.MethodGet, "/artifacts/metadatas/gitlab", http.StatusNotFound}, // latest is not v0.1.0
		{http.MethodGet, "/artifacts/metadatas/gitlab?version=v9.9.9", http.StatusNotFound},
		{http.MethodGet, "/artifacts/metadatas/..%2F..?version=v0.1.0", http.StatusNotFound},
		{http.MethodPost, "/releases/gitlab/v9.9.9", http.StatusNotFound},
		{http.MethodPost, "/releases/gitlab/..%2F..", http.StatusNotFound},
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

// The release routes are wired; their behaviour is tested in the library (release_test.go).
func TestReleaseRoutes(t *testing.T) {
	_, h := setup(t)

	var rel localregistry.Release
	get(t, h, http.MethodPost, "/releases/gitlab/v0.1.0", &rel)
	if rel.TagName != "v0.1.0" || len(rel.Assets) != 3 || rel.Assets[2].Name != "tool_0.1.0_checksums.txt" || !rel.Assets[2].IsChecksum {
		t.Fatalf("published release = %+v", rel)
	}
	get(t, h, http.MethodPost, "/releases/v0.0.9", &rel) // deprecated route, legacy tree of gitlab
	if rel.TagName != "v0.0.9" || len(rel.Assets) != 2 {
		t.Fatalf("legacy release = %+v", rel)
	}
	for _, a := range rel.Assets {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, a.URL, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d", a.URL, rec.Code)
		}
	}
	var meta map[string]any
	get(t, h, http.MethodGet, "/artifacts/metadatas/gitlab?version=v0.1.0", &meta)
	if meta["project_name"] != "tool" {
		t.Fatalf("metadata = %v", meta)
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
