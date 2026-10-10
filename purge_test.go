package local_registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func age(t *testing.T, path string, ago time.Duration) {
	t.Helper()
	old := time.Now().Add(-ago)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func TestPurgeStaleUploads(t *testing.T) {
	root := t.TempDir()
	s := NewLocalRegistryServer(0, "localhost", root)
	a, auditPath := tempAudit(t)
	s.SetAuditLog(a)
	l := NewLayout(root)

	stale, _ := l.StagingDir("gitlab", "v1")
	fresh, _ := l.StagingDir("github", "v2")
	writeFile(t, filepath.Join(stale, "a.bin"), "12345")
	writeFile(t, filepath.Join(fresh, "a.bin"), "x")
	age(t, filepath.Join(stale, "a.bin"), 48*time.Hour)
	age(t, stale, 48*time.Hour)
	oldTmp := filepath.Join(l.UploadTmpDir(), "upload-old")
	newTmp := filepath.Join(l.UploadTmpDir(), "upload-new")
	writeFile(t, oldTmp, "x")
	writeFile(t, newTmp, "x")
	age(t, oldTmp, 48*time.Hour)
	published := filepath.Join(l.BinariesDir(), "gitlab", "v0", "a.bin")
	writeFile(t, published, "x")
	age(t, published, 48*time.Hour)

	s.purgeStaleUploads(24*time.Hour, time.Now())

	for path, wantExists := range map[string]bool{
		stale:                                   false,
		filepath.Join(l.UploadsDir(), "gitlab"): false, // emptied vcs dir is removed
		fresh:                                   true,
		oldTmp:                                  false,
		newTmp:                                  true,
		published:                               true, // published versions are never purged
	} {
		if _, err := os.Stat(path); (err == nil) != wantExists {
			t.Errorf("%s: exists = %v, want %v", path, err == nil, wantExists)
		}
	}
	events := readAudit(t, auditPath)
	if len(events) != 2 || events[0]["event"] != "staging_purged" || events[0]["vcs"] != "gitlab" ||
		events[0]["version"] != "v1" || events[0]["bytes"] != float64(5) || events[0]["files"] != float64(1) ||
		events[1]["event"] != "tmp_purged" || events[1]["files"] != float64(1) {
		t.Fatalf("audit events = %v", events)
	}
}

func TestServeContextRunsPurger(t *testing.T) {
	root := t.TempDir()
	s := NewLocalRegistryServer(freePort(t), "localhost", root)
	s.AddUploadRoutes(UploadConfig{Token: testToken, StagingTTL: time.Hour})
	stale, _ := NewLayout(root).StagingDir("gitlab", "v1")
	writeFile(t, filepath.Join(stale, "a"), "x")
	age(t, filepath.Join(stale, "a"), 2*time.Hour)
	age(t, stale, 2*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.ServeContext(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(stale); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stale staging was not purged at startup")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
