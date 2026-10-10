package local_registry

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"
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

// purgeStaleUploads removes the staged versions nobody touched for longer than ttl, and the
// temporary files an interrupted server left behind. Each removal is audited.
func (s *LocalRegistryServer) purgeStaleUploads(ttl time.Duration, now time.Time) {
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()

	l := NewLayout(s.absoluteAssetPath)
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
			s.audit(nil, "staging_purged", map[string]any{
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
		s.audit(nil, "tmp_purged", map[string]any{"files": removed})
	}
}

// runPurger purges at once, then every min(ttl/2, 1h), until ctx is done.
func (s *LocalRegistryServer) runPurger(ctx context.Context, ttl time.Duration) {
	s.purgeStaleUploads(ttl, time.Now())
	t := time.NewTicker(min(ttl/2, time.Hour))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.purgeStaleUploads(ttl, now)
		}
	}
}
