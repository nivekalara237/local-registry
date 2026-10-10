package local_registry

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Defaults applied by AddUploadRoutes to the zero values of UploadConfig.
const (
	DefaultMaxVersionSize  int64 = 2 << 30  // 2 GiB staged per version
	DefaultMaxVersionFiles       = 100      // files staged per version
	DefaultMaxStagingSize  int64 = 10 << 30 // 10 GiB staged in total
	DefaultMinFreeSpace    int64 = 1 << 30  // 1 GiB left free on the disk
	DefaultStagingTTL            = 24 * time.Hour
)

// freeSpace returns the bytes available to this process on the filesystem holding path,
// and false when the platform cannot tell. A variable so tests can simulate a full disk.
var freeSpace = diskFree

// usage returns the total size and count of regular files under dir; a missing dir is empty.
func usage(dir string) (bytes int64, files int, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		bytes += info.Size()
		files++
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return 0, 0, nil
	}
	return bytes, files, err
}

// budget is how many bytes the next file of a version may hold, and the error to answer beyond it.
type budget struct {
	limit    int64
	tooLarge error
}

// uploadBudget applies the per-file, per-version and total staging quotas to the next file
// named name in staging. A file replacing one of the same name does not count twice.
func (s *LocalRegistryServer) uploadBudget(cfg UploadConfig, vcs, version, staging, name string) (budget, error) {
	versionBytes, versionFiles, err := usage(staging)
	if err != nil {
		return budget{}, err
	}
	allBytes, _, err := usage(NewLayout(s.absoluteAssetPath).UploadsDir())
	if err != nil {
		return budget{}, err
	}
	var existing int64
	replacing := false
	if info, err := os.Stat(filepath.Join(staging, name)); err == nil && info.Mode().IsRegular() {
		existing, replacing = info.Size(), true
	}

	if !replacing && cfg.MaxVersionFiles > 0 && versionFiles >= cfg.MaxVersionFiles {
		return budget{}, httpError(http.StatusUnprocessableEntity,
			"version %s/%s already has %d staged files (limit %d)", vcs, version, versionFiles, cfg.MaxVersionFiles)
	}

	b := budget{limit: cfg.MaxFileSize, tooLarge: httpError(http.StatusRequestEntityTooLarge, "file exceeds %d bytes", cfg.MaxFileSize)}
	if cfg.MaxVersionSize > 0 {
		if rest := cfg.MaxVersionSize - (versionBytes - existing); rest < b.limit {
			b = budget{rest, httpError(http.StatusRequestEntityTooLarge,
				"version %s/%s would exceed its staging quota of %d bytes", vcs, version, cfg.MaxVersionSize)}
		}
	}
	if cfg.MaxStagingSize > 0 {
		if rest := cfg.MaxStagingSize - (allBytes - existing); rest < b.limit {
			b = budget{rest, httpError(http.StatusInsufficientStorage,
				"upload staging area is full (limit %d bytes): publish or abandon pending versions", cfg.MaxStagingSize)}
		}
	}
	b.limit = max(b.limit, 0)
	return b, nil
}

// checkFreeSpace refuses (507) an upload of need bytes that would leave less than cfg.MinFreeSpace free.
func checkFreeSpace(cfg UploadConfig, dir string, need int64) error {
	if cfg.MinFreeSpace < 0 {
		return nil
	}
	free, ok := freeSpace(dir)
	if ok && free-need < cfg.MinFreeSpace {
		return httpError(http.StatusInsufficientStorage, "not enough free disk space on the registry")
	}
	return nil
}
