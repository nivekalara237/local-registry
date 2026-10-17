package fs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/nivekalara237/local-registry/internal/core"
)

// newestModTime is the latest modification time of dir and of its direct entries.
func newestModTime(dir string) (time.Time, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return time.Time{}, err
	}
	newest := info.ModTime()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, err
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest, nil
}

// PurgeStaleUploads removes the staged versions nobody touched for longer than ttl, and the
// temporary files an interrupted server left behind. Each removal is audited.
func PurgeStaleUploads(reg *core.Registry, ttl time.Duration, now time.Time) {
	reg.UploadMu.Lock()
	defer reg.UploadMu.Unlock()

	l := reg.Layout
	vcsDirs, _ := os.ReadDir(l.UploadsDir())
	for _, vcs := range vcsDirs {
		if !vcs.IsDir() {
			continue
		}
		vcsDir := filepath.Join(l.UploadsDir(), vcs.Name())
		versions, _ := os.ReadDir(vcsDir)
		for _, version := range versions {
			dir := filepath.Join(vcsDir, version.Name())
			newest, err := newestModTime(dir)
			if err != nil || now.Sub(newest) <= ttl {
				continue
			}
			bytes, files, _ := usage(dir)
			if err := os.RemoveAll(dir); err != nil {
				slog.Error("staging purge failed", "dir", dir, "error", err)
				continue
			}
			reg.Audit.Record(nil, "staging_purged", map[string]any{
				"vcs": vcs.Name(), "version": version.Name(), "files": files, "bytes": bytes,
				"idle": now.Sub(newest).Round(time.Second).String(),
			})
		}
		_ = os.Remove(vcsDir) // only succeeds once empty
	}

	tmps, _ := os.ReadDir(l.UploadTmpDir())
	removed := 0
	for _, e := range tmps {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() || now.Sub(info.ModTime()) <= ttl {
			continue
		}
		if os.Remove(filepath.Join(l.UploadTmpDir(), e.Name())) == nil {
			removed++
		}
	}
	if removed > 0 {
		reg.Audit.Record(nil, "tmp_purged", map[string]any{"files": removed})
	}
}

// RunPurger purges at once, then every min(ttl/2, 1h), until ctx is done.
func RunPurger(ctx context.Context, reg *core.Registry, ttl time.Duration) {
	PurgeStaleUploads(reg, ttl, time.Now())
	t := time.NewTicker(min(ttl/2, time.Hour))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			PurgeStaleUploads(reg, ttl, now)
		}
	}
}
