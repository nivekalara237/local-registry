package local_registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func serve(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestAddRoutePassesPathValuesInOrder(t *testing.T) {
	s := NewLocalRegistryServer(0, "localhost", t.TempDir())
	var got []string
	s.AddRoute("/{vcs}/artifacts/{version}", "get", func(pvs ...string) any {
		got = pvs
		return map[string]string{"vcs": pvs[0], "version": pvs[1]}
	})

	rec := serve(t, s.Handler(), http.MethodGet, "/gitlab/artifacts/v0.1.0")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if !slices.Equal(got, []string{"gitlab", "v0.1.0"}) {
		t.Fatalf("path values = %v", got)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["vcs"] != "gitlab" || body["version"] != "v0.1.0" {
		t.Fatalf("body = %v", body)
	}
}

func TestAddRouteMethodIsEnforced(t *testing.T) {
	s := NewLocalRegistryServer(0, "localhost", t.TempDir())
	s.AddRoute("/releases/{version}", "POST", func(...string) any { return nil })

	if rec := serve(t, s.Handler(), http.MethodGet, "/releases/v1"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAddRouteEErrors(t *testing.T) {
	s := NewLocalRegistryServer(0, "localhost", t.TempDir())
	s.AddRouteE("/err/{kind}", "GET", func(pvs ...string) (any, error) {
		switch pvs[0] {
		case "notfound":
			return nil, fmt.Errorf("x: %w", ErrNotFound)
		case "notexist":
			return nil, fmt.Errorf("x: %w", fs.ErrNotExist)
		default:
			return nil, errors.New("boom /secret/path")
		}
	})

	tests := []struct {
		kind   string
		status int
		msg    string
	}{
		{"notfound", http.StatusNotFound, "not found"},
		{"notexist", http.StatusNotFound, "not found"},
		{"other", http.StatusInternalServerError, "internal error"},
	}
	for _, tt := range tests {
		rec := serve(t, s.Handler(), http.MethodGet, "/err/"+tt.kind)
		if rec.Code != tt.status {
			t.Fatalf("%s: status = %d, want %d", tt.kind, rec.Code, tt.status)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("%s: Content-Type = %q", tt.kind, ct)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["error"] != tt.msg {
			t.Fatalf("%s: body = %v", tt.kind, body)
		}
	}
}

func TestAddFileRouteHeadersAndRange(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bin.local/assets/sub/tool.txt"), "0123456789")
	s := NewLocalRegistryServer(0, "localhost", root)
	s.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")
	target := "/assets/binary/bin.local%2Fassets%2Fsub%2Ftool.txt"

	rec := serve(t, s.Handler(), http.MethodGet, target)
	if ct := rec.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename=tool.txt` {
		t.Fatalf("Content-Disposition = %q", cd)
	}

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Range", "bytes=2-4")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "234" {
		t.Fatalf("range: status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestAddFileRouteRefusesOutsideAndMissing(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "registry")
	writeFile(t, filepath.Join(parent, "secret.txt"), "SECRET")
	writeFile(t, filepath.Join(root, "bin.local/assets/sub/tool.bin"), "BIN")
	// Private files under the asset path itself.
	writeFile(t, filepath.Join(root, "main.go"), "SECRET")
	writeFile(t, filepath.Join(root, ".env"), "SECRET")
	writeFile(t, filepath.Join(root, "bin.local/.uploads/gitlab/v1/tool.bin"), "SECRET")
	writeFile(t, filepath.Join(root, "bin.local/.uploads-tmp/upload-1"), "SECRET")
	writeFile(t, filepath.Join(root, "bin.local/assets/.git/config"), "SECRET")
	writeFile(t, filepath.Join(root, "bin.local/assets/binaries/gitlab/.tmp-1"), "SECRET")
	// Symbolic links leading out of the asset path, to a file and to a directory.
	if err := os.Symlink(filepath.Join(parent, "secret.txt"), filepath.Join(root, "bin.local/assets/link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(parent, filepath.Join(root, "bin.local/assets/up")); err != nil {
		t.Fatal(err)
	}
	s := NewLocalRegistryServer(0, "localhost", root)
	s.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")

	for _, target := range []string{
		"/assets/binary/..%2Fsecret.txt",
		"/assets/binary/bin.local%2Fassets%2F..%2F..%2F..%2Fsecret.txt",
		"/assets/binary/%252E%252E%252Fsecret.txt", // double-encoded: stays a literal name
		"/assets/binary/bin.local%2Fassets%2Fmissing.bin",
		"/assets/binary/bin.local%2Fassets%2Fsub", // a directory
		"/assets/binary/main.go",
		"/assets/binary/.env",
		"/assets/binary/bin.local%2F.uploads%2Fgitlab%2Fv1%2Ftool.bin",
		"/assets/binary/bin.local%2F.uploads-tmp%2Fupload-1",
		"/assets/binary/bin.local%2Fassets%2F.git%2Fconfig",
		"/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2F.tmp-1",
		"/assets/binary/bin.local%2Fassets%2Flink.txt",
		"/assets/binary/bin.local%2Fassets%2Fup%2Fsecret.txt",
	} {
		rec := serve(t, s.Handler(), http.MethodGet, target)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "SECRET") {
			t.Fatalf("%s: leaked a private file", target)
		}
	}
}

func TestAddFileRouteDecodesNamesOnce(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "bin.local/assets/v1")
	writeFile(t, filepath.Join(dir, "tool+linux.bin"), "PLUS")
	writeFile(t, filepath.Join(dir, "tool linux.bin"), "SPACE")
	writeFile(t, filepath.Join(dir, "100%.bin"), "PERCENT")
	writeFile(t, filepath.Join(dir, "sub/a.bin"), "SUB")
	s := NewLocalRegistryServer(0, "localhost", root)
	s.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")
	prefix := "/assets/binary/bin.local%2Fassets%2Fv1%2F"

	tests := []struct {
		target string
		status int
		body   string
	}{
		{prefix + "tool+linux.bin", http.StatusOK, "PLUS"}, // "+" is a plus, no longer a space
		{prefix + "tool%2Blinux.bin", http.StatusOK, "PLUS"},
		{prefix + "tool%20linux.bin", http.StatusOK, "SPACE"},
		{prefix + "100%25.bin", http.StatusOK, "PERCENT"},
		{prefix + "sub%2Fa.bin", http.StatusOK, "SUB"},
		{prefix + "sub%252Fa.bin", http.StatusNotFound, ""},      // double-encoded slash is not a slash
		{prefix + "tool%252Blinux.bin", http.StatusNotFound, ""}, // the old way to write "+"
	}
	for _, tt := range tests {
		rec := serve(t, s.Handler(), http.MethodGet, tt.target)
		if rec.Code != tt.status || (tt.status == http.StatusOK && rec.Body.String() != tt.body) {
			t.Fatalf("%s: status = %d, body = %q; want %d %q", tt.target, rec.Code, rec.Body.String(), tt.status, tt.body)
		}
	}
}

func TestAddFileRouteServesFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bin.local/assets/binaries/gitlab/v1/tool.bin"), "BINARY")
	writeFile(t, filepath.Join(root, "assets/binaries/gitlab/v1/legacy.bin"), "LEGACY")
	// A symbolic link that stays inside the asset path still works.
	if err := os.Symlink("v1", filepath.Join(root, "bin.local/assets/binaries/gitlab/current")); err != nil {
		t.Fatal(err)
	}
	s := NewLocalRegistryServer(0, "localhost", root)
	s.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")

	tests := []struct{ target, want string }{
		// An encoded slash reaches files in subdirectories.
		{"/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Fv1%2Ftool.bin", "BINARY"},
		{"/assets/binary/%2Fbin.local%2Fassets%2Fbinaries%2Fgitlab%2Fv1%2Ftool.bin", "BINARY"},
		{"/assets/binary/assets%2Fbinaries%2Fgitlab%2Fv1%2Flegacy.bin", "LEGACY"},
		{"/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Fcurrent%2Ftool.bin", "BINARY"},
	}
	for _, tt := range tests {
		rec := serve(t, s.Handler(), http.MethodGet, tt.target)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", tt.target, rec.Code)
		}
		if rec.Body.String() != tt.want {
			t.Fatalf("%s: body = %q, want %q", tt.target, rec.Body.String(), tt.want)
		}
	}
}
