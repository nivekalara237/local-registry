package endpoints_test

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nivekalara237/local-registry/internal/core"
	"github.com/nivekalara237/local-registry/internal/endpoints"
	"github.com/nivekalara237/local-registry/internal/testutil"
)

func TestUploadAndPublish(t *testing.T) {
	root, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken})
	l := core.NewLayout(root)
	files := testutil.Release()

	testutil.PutAll(t, h, "v0.1.0", files)

	// Staged files are invisible to the catalogue until publish.
	if _, err := os.Stat(l.AssetsDir()); !os.IsNotExist(err) {
		t.Fatalf("assets dir exists before publish: %v", err)
	}

	rec, res := testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish?latest=true", Token: testutil.TestToken})
	if rec.Code != http.StatusOK {
		t.Fatalf("publish: status = %d, body = %s", rec.Code, rec.Body)
	}
	if res["latest"] != true || res["product"] != "gitlab" || res["version"] != "v0.1.0" || len(res["files"].([]any)) != len(files)+2 {
		t.Fatalf("publish: response = %v", res)
	}

	final, _ := l.VersionDir("gitlab", "v0.1.0")
	for name, content := range files {
		if name == "artifacts.json" {
			name = "artifacts.goreleaser.json" // kept as uploaded
		}
		got, err := os.ReadFile(filepath.Join(final, name))
		if err != nil || string(got) != content {
			t.Fatalf("published %s = %q, %v", name, got, err)
		}
	}
	rewritten, _ := os.ReadFile(filepath.Join(final, "artifacts.json"))
	latest, _ := l.ArtifactsFile("gitlab", "latest")
	if got, _ := os.ReadFile(latest); string(got) != string(rewritten) {
		t.Fatalf("latest artifacts.json = %q, want the rewritten %q", got, rewritten)
	}
	staging, _ := l.StagingDir("gitlab", "v0.1.0")
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging still exists: %v", err)
	}

	paths, err := (&endpoints.Asset{}).FindAssetPaths(l.AssetsDir())
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
	root, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken})
	testutil.PutAll(t, h, "v0.1.0", testutil.Release())

	if rec, _ := testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish", Token: testutil.TestToken}); rec.Code != http.StatusOK {
		t.Fatalf("publish: status = %d, body = %s", rec.Code, rec.Body)
	}
	latest, _ := core.NewLayout(root).ArtifactsFile("gitlab", "latest")
	if _, err := os.Stat(latest); !os.IsNotExist(err) {
		t.Fatalf("latest artifacts.json written without ?latest=true: %v", err)
	}
}

func TestPublishedVersionIsImmutable(t *testing.T) {
	_, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken})
	testutil.PutAll(t, h, "v0.1.0", testutil.Release())
	testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish", Token: testutil.TestToken})

	if rec, _ := testutil.Do(t, h, testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v0.1.0/files/tool_0.1.0_linux_amd64.tar.gz", Body: "EVIL", Token: testutil.TestToken}); rec.Code != http.StatusConflict {
		t.Fatalf("PUT after publish: status = %d", rec.Code)
	}
	if rec, _ := testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish", Token: testutil.TestToken}); rec.Code != http.StatusConflict {
		t.Fatalf("second publish: status = %d", rec.Code)
	}
}

func TestUploadAuth(t *testing.T) {
	_, disabled := testutil.NewUploadServer(t, endpoints.UploadConfig{})
	_, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken})
	put := "/gitlab/artifacts/v0.1.0/files/a.bin"

	tests := []struct {
		name   string
		h      http.Handler
		c      testutil.Call
		status int
	}{
		{"disabled", disabled, testutil.Call{Method: "PUT", Target: put, Body: "x", Token: "anything"}, http.StatusForbidden},
		{"disabled publish", disabled, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish"}, http.StatusForbidden},
		{"no token", h, testutil.Call{Method: "PUT", Target: put, Body: "x"}, http.StatusUnauthorized},
		{"wrong token", h, testutil.Call{Method: "PUT", Target: put, Body: "x", Token: "s3cre"}, http.StatusUnauthorized},
		{"basic auth", h, testutil.Call{Method: "PUT", Target: put, Body: "x", Header: map[string]string{"Authorization": "Basic " + testutil.TestToken}}, http.StatusUnauthorized},
		{"publish no token", h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish"}, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		rec, res := testutil.Do(t, tt.h, tt.c)
		if rec.Code != tt.status || res["error"] == nil {
			t.Fatalf("%s: status = %d, body = %s", tt.name, rec.Code, rec.Body)
		}
		if tt.status == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("%s: no WWW-Authenticate header", tt.name)
		}
	}
}

func TestPutRejections(t *testing.T) {
	root, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken, MaxFileSize: 4})

	tests := []struct {
		name   string
		c      testutil.Call
		status int
	}{
		{"too large, known length", testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v1/files/a", Body: "12345"}, http.StatusRequestEntityTooLarge},
		{"too large, chunked", testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v1/files/a", Body: "12345", Chunked: true}, http.StatusRequestEntityTooLarge},
		{"checksum header mismatch", testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v1/files/a", Body: "1234", Header: map[string]string{"X-Checksum-Sha256": testutil.Sha("4321")}}, http.StatusUnprocessableEntity},
		{"traversal in name", testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v1/files/..%2Fa", Body: "1"}, http.StatusBadRequest},
		{"space in name", testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v1/files/a%20b", Body: "1"}, http.StatusBadRequest},
		{"traversal in product", testutil.Call{Method: "PUT", Target: "/..%2F..%2Fx/artifacts/v1/files/a", Body: "1"}, http.StatusBadRequest},
		{"latest as version", testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/latest/files/a", Body: "1"}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		tt.c.Token = testutil.TestToken
		rec, res := testutil.Do(t, h, tt.c)
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
	rec, _ := testutil.Do(t, h, testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v1/files/a", Body: "1234", Token: testutil.TestToken,
		Header: map[string]string{"X-Checksum-Sha256": strings.ToUpper(testutil.Sha("1234"))}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("matching checksum Header: status = %d, body = %s", rec.Code, rec.Body)
	}
}

// drop removes name from files and from their artifacts.json.
func drop(t *testing.T, files map[string]string, name string) {
	delete(files, name)
	entries := slices.DeleteFunc(testutil.ReleaseArtifacts(), func(e map[string]any) bool { return e["name"] == name })
	files["artifacts.json"] = testutil.EncodeArtifacts(t, entries)
}

func TestPublishChecks(t *testing.T) {
	tests := []struct {
		name   string
		edit   func(files map[string]string)
		status int
	}{
		{"no registry.yaml", func(f map[string]string) { delete(f, "registry.yaml") }, http.StatusUnprocessableEntity},
		{"no checksums file: publishing writes one", func(f map[string]string) { drop(t, f, "tool_0.1.0_checksums.txt") }, http.StatusOK},
		{"no metadata.json", func(f map[string]string) { drop(t, f, "metadata.json") }, http.StatusOK},
		{"no artifacts.json", func(f map[string]string) { delete(f, "artifacts.json") }, http.StatusOK},
		{"several checksums files", func(f map[string]string) { f["other_checksums.txt"] = f["tool_0.1.0_checksums.txt"] }, http.StatusUnprocessableEntity},
		{"binary differs from checksums", func(f map[string]string) { f["tool_0.1.0_linux_amd64.tar.gz"] = "TAMPERED" }, http.StatusUnprocessableEntity},
		{"binary not in checksums", func(f map[string]string) { f["extra.bin"] = "EXTRA" }, http.StatusUnprocessableEntity},
		{"malformed checksums", func(f map[string]string) { f["tool_0.1.0_checksums.txt"] = "not a checksum line\n" }, http.StatusUnprocessableEntity},
		{"metadata for another version", func(f map[string]string) {
			f["metadata.json"] = `{"tag":"v0.2.0","version":"0.2.0"}`
		}, http.StatusUnprocessableEntity},
		{"signature needs no checksum line", func(f map[string]string) {
			f["tool_0.1.0_checksums.txt.sig"] = "SIG"
			f["artifacts.json"] = testutil.EncodeArtifacts(t, append(testutil.ReleaseArtifacts(),
				map[string]any{"name": "tool_0.1.0_checksums.txt.sig", "path": "dist/tool_0.1.0_checksums.txt.sig", "type": "Signature"}))
		}, http.StatusOK},
		{"file in checksums but not in artifacts.json", func(f map[string]string) {
			f["extra.bin"] = "EXTRA"
			f["tool_0.1.0_checksums.txt"] += testutil.Sha("EXTRA") + "  extra.bin\n"
		}, http.StatusUnprocessableEntity},
		{"version without v prefix in metadata", func(f map[string]string) { f["metadata.json"] = `{"version":"0.1.0"}` }, http.StatusOK},
	}
	for _, tt := range tests {
		root, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken})
		files := testutil.Release()
		tt.edit(files)
		testutil.PutAll(t, h, "v0.1.0", files)

		rec, res := testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish", Token: testutil.TestToken})
		if rec.Code != tt.status {
			t.Fatalf("%s: status = %d, body = %s", tt.name, rec.Code, rec.Body)
		}
		final, _ := core.NewLayout(root).VersionDir("gitlab", "v0.1.0")
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
	_, h := testutil.NewUploadServer(t, endpoints.UploadConfig{Token: testutil.TestToken})
	rec, _ := testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish", Token: testutil.TestToken})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	rec, _ = testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish?latest=maybe", Token: testutil.TestToken})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("latest=maybe: status = %d", rec.Code)
	}
}
