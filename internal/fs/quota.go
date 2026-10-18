package fs

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nivekalara237/local-registry/internal/core"
)

// Defaults applied by the upload routes to the zero values of their configuration.
const (
	DefaultMaxVersionSize  int64 = 2 << 30  // 2 GiB staged per version
	DefaultMaxVersionFiles       = 100      // files staged per version
	DefaultMaxStagingSize  int64 = 10 << 30 // 10 GiB staged in total
	DefaultMinFreeSpace    int64 = 1 << 30  // 1 GiB left free on the disk
	DefaultStagingTTL            = 24 * time.Hour
)

// FreeSpace returns the bytes available to this process on the filesystem holding path,
// and false when the platform cannot tell. A variable so tests can simulate a full disk.
var FreeSpace = diskFree

// Limits are the staging quotas, in bytes but MaxVersionFiles. A value of 0 or less disables
// a limit, except MaxFileSize, which always applies.
type Limits struct {
	MaxFileSize     int64 // one file
	MaxVersionSize  int64 // the files staged for one version (413 beyond)
	MaxVersionFiles int   // files staged for one version (422 beyond)
	MaxStagingSize  int64 // the files staged for all versions (507 beyond)
}

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

// Budget is how many bytes the next file of a version may hold, and the error to answer beyond it.
type Budget struct {
	Limit    int64
	TooLarge error
}

// UploadBudget applies the per-file, per-version and total staging quotas to the next file
// named name in staging. A file replacing one of the same name does not count twice.
func UploadBudget(cfg Limits, l core.Layout, product, version, staging, name string) (Budget, error) {
	versionBytes, versionFiles, err := usage(staging)
	if err != nil {
		return Budget{}, err
	}
	allBytes, _, err := usage(l.UploadsDir())
	if err != nil {
		return Budget{}, err
	}
	var existing int64
	replacing := false
	if info, err := os.Stat(filepath.Join(staging, name)); err == nil && info.Mode().IsRegular() {
		existing, replacing = info.Size(), true
	}

	if !replacing && cfg.MaxVersionFiles > 0 && versionFiles >= cfg.MaxVersionFiles {
		return Budget{}, core.Errorf(http.StatusUnprocessableEntity,
			"version %s/%s already has %d staged files (limit %d)", product, version, versionFiles, cfg.MaxVersionFiles)
	}

	b := Budget{Limit: cfg.MaxFileSize, TooLarge: core.Errorf(http.StatusRequestEntityTooLarge, "file exceeds %d bytes", cfg.MaxFileSize)}
	if cfg.MaxVersionSize > 0 {
		if rest := cfg.MaxVersionSize - (versionBytes - existing); rest < b.Limit {
			b = Budget{rest, core.Errorf(http.StatusRequestEntityTooLarge,
				"version %s/%s would exceed its staging quota of %d bytes", product, version, cfg.MaxVersionSize)}
		}
	}
	if cfg.MaxStagingSize > 0 {
		if rest := cfg.MaxStagingSize - (allBytes - existing); rest < b.Limit {
			b = Budget{rest, core.Errorf(http.StatusInsufficientStorage,
				"upload staging area is full (limit %d bytes): publish or abandon pending versions", cfg.MaxStagingSize)}
		}
	}
	b.Limit = max(b.Limit, 0)
	return b, nil
}

// CheckFreeSpace refuses (507) an upload of need bytes that would leave less than minFree free.
// A negative minFree disables the check.
func CheckFreeSpace(minFree int64, dir string, need int64) error {
	if minFree < 0 {
		return nil
	}
	free, ok := FreeSpace(dir)
	if ok && free-need < minFree {
		return core.Errorf(http.StatusInsufficientStorage, "not enough free disk space on the registry")
	}
	return nil
}
