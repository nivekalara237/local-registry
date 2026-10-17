package core

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLayoutPaths(t *testing.T) {
	l := NewLayout("/srv")
	bins := filepath.FromSlash("/srv/bin.local/assets/binaries")

	if got := l.AssetsDir(); got != filepath.FromSlash("/srv/bin.local/assets") {
		t.Fatalf("AssetsDir = %q", got)
	}
	tests := []struct {
		name string
		got  func() (string, error)
		want string
	}{
		{"vcs", func() (string, error) { return l.VCSDir("gitlab") }, bins + "/gitlab"},
		{"version", func() (string, error) { return l.VersionDir("gitlab", "v0.1.0") }, bins + "/gitlab/v0.1.0"},
		{"latest", func() (string, error) { return l.ArtifactsFile("gitlab", "latest") }, bins + "/gitlab/artifacts.json"},
		{"no version", func() (string, error) { return l.ArtifactsFile("gitlab", "") }, bins + "/gitlab/artifacts.json"},
		{"versioned", func() (string, error) { return l.ArtifactsFile("gitlab", "v0.1.0") }, bins + "/gitlab/v0.1.0/artifacts.json"},
	}
	for _, tt := range tests {
		got, err := tt.got()
		if err != nil || got != filepath.FromSlash(tt.want) {
			t.Fatalf("%s: got %q, %v; want %q", tt.name, got, err, tt.want)
		}
	}
}

func TestLayoutRejectsUnsafeNames(t *testing.T) {
	l := NewLayout("/srv")
	for _, bad := range []string{"", ".", "..", "../x", "a/b", `a\b`, "a b"} {
		if _, err := l.VCSDir(bad); !errors.Is(err, ErrNotFound) {
			t.Fatalf("VCSDir(%q) err = %v", bad, err)
		}
		if bad != "" {
			if _, err := l.ArtifactsFile("gitlab", bad); !errors.Is(err, ErrNotFound) {
				t.Fatalf("ArtifactsFile(gitlab, %q) err = %v", bad, err)
			}
		}
	}
}

func TestIsPublicFile(t *testing.T) {
	tests := map[string]bool{
		"bin.local/assets/binaries/gitlab/v1/tool.tar.gz": true,
		"bin.local/assets/binaries/gitlab/artifacts.json": true,
		"assets/binaries/gitlab/v1/tool":                  true,
		"bin.local/assets":                                false, // the directory itself
		"bin.local/assetsX/tool":                          false,
		"bin.local/.uploads/gitlab/v1/tool":               false,
		"bin.local/.uploads-tmp/upload-1":                 false,
		"bin.local/assets/.git/config":                    false,
		"bin.local/assets/binaries/gitlab/.tmp-1":         false,
		"assets/../bin.local/.uploads/x":                  false,
		"bin.local/assets//tool":                          false,
		"/bin.local/assets/tool":                          false,
		"main.go":                                         false,
		".env":                                            false,
		"":                                                false,
	}
	for name, want := range tests {
		if got := IsPublicFile(name); got != want {
			t.Errorf("IsPublicFile(%q) = %v, want %v", name, got, want)
		}
	}
}
