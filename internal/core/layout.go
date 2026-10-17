package core

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidName reports whether s is a single path segment that stays inside its parent directory.
func ValidName(s string) bool {
	return nameRe.MatchString(s) && s != "." && s != ".."
}

// publicDirs are the only trees, relative to the asset path, that file routes serve:
// published releases, and the legacy tree read by POST /releases/{version}.
// Staging (bin.local/.uploads*) and everything else under the asset path stay private.
var publicDirs = []string{"bin.local/assets/", "assets/"}

// IsPublicFile reports whether name, a slash-separated path relative to the asset path,
// may be served by a file route: a clean path inside one of the public trees with no
// segment starting with "." (no .., no hidden files such as .git or a .tmp-* in progress).
func IsPublicFile(name string) bool {
	if !fs.ValidPath(name) {
		return false
	}
	for _, seg := range strings.Split(name, "/") {
		if strings.HasPrefix(seg, ".") {
			return false
		}
	}
	for _, dir := range publicDirs {
		if strings.HasPrefix(name, dir) {
			return true
		}
	}
	return false
}

// Layout is the asset tree under the asset path, shared by every route that reads or writes it:
//
//	bin.local/assets/binaries/<vcs>/artifacts.json   latest release of <vcs>
//	bin.local/assets/binaries/<vcs>/<version>/       one release (binaries, checksums, artifacts.json, metadata.json)
//
// Path methods validate their names and return an error wrapping ErrNotFound for a name
// that is not a single safe segment.
type Layout struct {
	root string
}

func NewLayout(assetPath string) Layout {
	return Layout{root: assetPath}
}

// Root is the asset path.
func (l Layout) Root() string {
	return l.root
}

// AssetsDir is the tree walked by the catalogue (/artifacts/all).
func (l Layout) AssetsDir() string {
	return filepath.Join(l.root, "bin.local", "assets")
}

func (l Layout) BinariesDir() string {
	return filepath.Join(l.AssetsDir(), "binaries")
}

func (l Layout) VCSDir(vcs string) (string, error) {
	if !ValidName(vcs) {
		return "", fmt.Errorf("vcs %q: %w", vcs, ErrNotFound)
	}
	return filepath.Join(l.BinariesDir(), vcs), nil
}

func (l Layout) VersionDir(vcs, version string) (string, error) {
	dir, err := l.VCSDir(vcs)
	if err != nil {
		return "", err
	}
	if !ValidName(version) {
		return "", fmt.Errorf("version %q: %w", version, ErrNotFound)
	}
	return filepath.Join(dir, version), nil
}

// ArtifactsFile is <vcs>/artifacts.json for version "" or "latest", else <vcs>/<version>/artifacts.json.
func (l Layout) ArtifactsFile(vcs, version string) (string, error) {
	if version == "" || version == "latest" {
		dir, err := l.VCSDir(vcs)
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "artifacts.json"), nil
	}
	dir, err := l.VersionDir(vcs, version)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "artifacts.json"), nil
}

// StagingDir holds the files PUT for a version until it is published. It lives in
// bin.local/.uploads, outside AssetsDir, so the catalogue never sees half an upload.
func (l Layout) StagingDir(vcs, version string) (string, error) {
	if !ValidName(vcs) || !ValidName(version) {
		return "", fmt.Errorf("staging %q/%q: %w", vcs, version, ErrNotFound)
	}
	return filepath.Join(l.UploadsDir(), vcs, version), nil
}

// UploadsDir holds every StagingDir; its total size is what UploadConfig.MaxStagingSize caps.
func (l Layout) UploadsDir() string {
	return filepath.Join(l.root, "bin.local", ".uploads")
}

// UploadTmpDir receives request bodies while they stream in, before they move to a StagingDir.
// It sits on the same filesystem as the staging and published trees, so moves are renames.
func (l Layout) UploadTmpDir() string {
	return filepath.Join(l.root, "bin.local", ".uploads-tmp")
}
