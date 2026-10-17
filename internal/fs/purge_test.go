package fs_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nivekalara237/local-registry/internal/core"
	regfs "github.com/nivekalara237/local-registry/internal/fs"
	"github.com/nivekalara237/local-registry/internal/testutil"
)

func TestPurgeStaleUploads(t *testing.T) {
	root := t.TempDir()
	s := core.NewRegistry(root)
	a, auditPath := testutil.TempAudit(t)
	s.Audit = a
	l := core.NewLayout(root)

	stale, _ := l.StagingDir("gitlab", "v1")
	fresh, _ := l.StagingDir("github", "v2")
	testutil.WriteFile(t, filepath.Join(stale, "a.bin"), "12345")
	testutil.WriteFile(t, filepath.Join(fresh, "a.bin"), "x")
	testutil.Age(t, filepath.Join(stale, "a.bin"), 48*time.Hour)
	testutil.Age(t, stale, 48*time.Hour)
	oldTmp := filepath.Join(l.UploadTmpDir(), "upload-old")
	newTmp := filepath.Join(l.UploadTmpDir(), "upload-new")
	testutil.WriteFile(t, oldTmp, "x")
	testutil.WriteFile(t, newTmp, "x")
	testutil.Age(t, oldTmp, 48*time.Hour)
	published := filepath.Join(l.BinariesDir(), "gitlab", "v0", "a.bin")
	testutil.WriteFile(t, published, "x")
	testutil.Age(t, published, 48*time.Hour)

	regfs.PurgeStaleUploads(s, 24*time.Hour, time.Now())

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
	events := testutil.ReadAudit(t, auditPath)
	if len(events) != 2 || events[0]["event"] != "staging_purged" || events[0]["vcs"] != "gitlab" ||
		events[0]["version"] != "v1" || events[0]["bytes"] != float64(5) || events[0]["files"] != float64(1) ||
		events[1]["event"] != "tmp_purged" || events[1]["files"] != float64(1) {
		t.Fatalf("audit events = %v", events)
	}
}
