package local_registry

import (
	"net/http"
	"strings"
	"testing"
)

func putStatus(t *testing.T, h http.Handler, target, body string, chunked bool) (int, string) {
	t.Helper()
	rec, res := do(t, h, call{method: "PUT", target: target, body: body, token: testToken, chunked: chunked})
	msg, _ := res["error"].(string)
	return rec.Code, msg
}

func TestVersionSizeQuota(t *testing.T) {
	_, h := newUploadServer(t, UploadConfig{Token: testToken, MaxVersionSize: 10, MaxStagingSize: -1, MinFreeSpace: -1})
	v1 := "/gitlab/artifacts/v1/files/"

	if code, _ := putStatus(t, h, v1+"a", "123456", false); code != http.StatusCreated {
		t.Fatalf("first file: %d", code)
	}
	if code, msg := putStatus(t, h, v1+"b", "123456", false); code != http.StatusRequestEntityTooLarge || !strings.Contains(msg, "staging quota") {
		t.Fatalf("over version quota: %d %q", code, msg)
	}
	if code, msg := putStatus(t, h, v1+"b", "123456", true); code != http.StatusRequestEntityTooLarge || !strings.Contains(msg, "staging quota") {
		t.Fatalf("over version quota, chunked: %d %q", code, msg)
	}
	// Replacing a file frees its old size first.
	if code, msg := putStatus(t, h, v1+"a", "1234567890", false); code != http.StatusCreated {
		t.Fatalf("replace within quota: %d %q", code, msg)
	}
	// Another version has its own quota.
	if code, _ := putStatus(t, h, "/gitlab/artifacts/v2/files/a", "123456", false); code != http.StatusCreated {
		t.Fatalf("other version: %d", code)
	}
}

func TestVersionFilesQuota(t *testing.T) {
	_, h := newUploadServer(t, UploadConfig{Token: testToken, MaxVersionFiles: 2})
	v1 := "/gitlab/artifacts/v1/files/"
	for _, name := range []string{"a", "b"} {
		if code, _ := putStatus(t, h, v1+name, "x", false); code != http.StatusCreated {
			t.Fatalf("%s: %d", name, code)
		}
	}
	if code, msg := putStatus(t, h, v1+"c", "x", false); code != http.StatusUnprocessableEntity || !strings.Contains(msg, "limit 2") {
		t.Fatalf("third file: %d %q", code, msg)
	}
	if code, _ := putStatus(t, h, v1+"a", "y", false); code != http.StatusCreated {
		t.Fatalf("replacing an existing file: %d", code)
	}
}

func TestStagingSizeQuota(t *testing.T) {
	_, h := newUploadServer(t, UploadConfig{Token: testToken, MaxStagingSize: 10, MinFreeSpace: -1})
	if code, _ := putStatus(t, h, "/gitlab/artifacts/v1/files/a", "123456", false); code != http.StatusCreated {
		t.Fatalf("first version: %d", code)
	}
	if code, msg := putStatus(t, h, "/github/artifacts/v9/files/a", "123456", false); code != http.StatusInsufficientStorage || !strings.Contains(msg, "staging area is full") {
		t.Fatalf("over total staging: %d %q", code, msg)
	}
}

func TestFreeSpace(t *testing.T) {
	_, h := newUploadServer(t, UploadConfig{Token: testToken, MaxFileSize: 80, MinFreeSpace: 50})
	stubFreeSpace(t, 100, true)
	v1 := "/gitlab/artifacts/v1/files/"

	if code, msg := putStatus(t, h, v1+"a", strings.Repeat("x", 60), false); code != http.StatusInsufficientStorage || !strings.Contains(msg, "free disk space") {
		t.Fatalf("would leave 40 of 50: %d %q", code, msg)
	}
	if code, _ := putStatus(t, h, v1+"a", strings.Repeat("x", 40), false); code != http.StatusCreated {
		t.Fatalf("leaves 60 of 50: %d", code)
	}
	// Unknown length: the server must assume the largest file it would accept (80).
	if code, _ := putStatus(t, h, v1+"b", "x", true); code != http.StatusInsufficientStorage {
		t.Fatalf("chunked: %d", code)
	}

	stubFreeSpace(t, 0, false) // platform cannot tell: no check
	if code, _ := putStatus(t, h, v1+"c", "x", true); code != http.StatusCreated {
		t.Fatalf("unknown free space: %d", code)
	}
}

func TestLimitsCanBeDisabled(t *testing.T) {
	_, h := newUploadServer(t, UploadConfig{Token: testToken, MaxVersionSize: -1, MaxVersionFiles: -1, MaxStagingSize: -1, MinFreeSpace: -1})
	stubFreeSpace(t, 0, true) // a full disk is ignored when the check is off
	for _, name := range []string{"a", "b", "c"} {
		if code, msg := putStatus(t, h, "/gitlab/artifacts/v1/files/"+name, "123456", false); code != http.StatusCreated {
			t.Fatalf("%s: %d %q", name, code, msg)
		}
	}
}

func TestUploadConfigDefaults(t *testing.T) {
	got := UploadConfig{}.withDefaults()
	want := UploadConfig{
		MaxFileSize: DefaultMaxFileSize, MaxVersionSize: DefaultMaxVersionSize, MaxVersionFiles: DefaultMaxVersionFiles,
		MaxStagingSize: DefaultMaxStagingSize, MinFreeSpace: DefaultMinFreeSpace, StagingTTL: DefaultStagingTTL,
	}
	if got != want {
		t.Fatalf("defaults = %+v", got)
	}
	off := UploadConfig{MaxVersionSize: -1, MaxVersionFiles: -1, MaxStagingSize: -1, MinFreeSpace: -1, StagingTTL: -1}.withDefaults()
	if off.MaxVersionSize != -1 || off.MaxVersionFiles != -1 || off.MaxStagingSize != -1 || off.MinFreeSpace != -1 || off.StagingTTL != -1 {
		t.Fatalf("disabled limits were changed: %+v", off)
	}
}
