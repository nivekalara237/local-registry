package audit_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nivekalara237/local-registry/internal/audit"
	"github.com/nivekalara237/local-registry/internal/core"
	"github.com/nivekalara237/local-registry/internal/endpoints"
	"github.com/nivekalara237/local-registry/internal/testutil"
)

func TestRecordWritesOneLinePerEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "audit.log")
	a, err := audit.OpenAuditLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	r := httptest.NewRequest("PUT", "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	a.Record(r, "upload", map[string]any{"name": "x"})
	a.Record(nil, "tmp_purged", map[string]any{"files": 1})

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var events []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e map[string]any
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("audit line %q: %v", sc.Text(), err)
		}
		events = append(events, e)
	}
	if len(events) != 2 {
		t.Fatalf("events = %v", events)
	}
	if e := events[0]; e["event"] != "upload" || e["name"] != "x" || e["time"] == nil ||
		e["remote"] != "192.0.2.1" || e["forwarded_for"] != "203.0.113.7" {
		t.Errorf("first event = %v", e)
	}
	if e := events[1]; e["event"] != "tmp_purged" || e["remote"] != nil {
		t.Errorf("event without request = %v", e)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("audit log mode = %v", info.Mode().Perm())
	}
}

func TestRecordWithoutLogGoesToSlog(t *testing.T) {
	var a *audit.AuditLog
	// Must not panic without an audit file.
	a.Record(httptest.NewRequest("GET", "/", nil), "probe", map[string]any{"k": "v"})
}

func TestTokenID(t *testing.T) {
	// printf %s s3cret | shasum -a 256 | cut -c1-12
	if got := audit.TokenID("s3cret"); got != "1ec1c26b50d5" {
		t.Fatalf("TokenID = %s", got)
	}
}

func TestAuditTrail(t *testing.T) {
	root := t.TempDir()
	testutil.StubFreeSpace(t, 1<<40, true)
	s := core.NewRegistry(root)
	a, path := testutil.TempAudit(t)
	s.Audit = a
	endpoints.AddUploadRoutes(s, endpoints.UploadConfig{Token: testutil.TestToken})
	h := s.Mux

	// 1. failed authentication
	testutil.Do(t, h, testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v0.1.0/files/x", Body: "x", Token: "wrong-token"})
	// 2. refused upload (checksum header mismatch), through a proxy
	testutil.Do(t, h, testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v0.1.0/files/x", Body: "x", Token: testutil.TestToken,
		Header: map[string]string{"X-Checksum-Sha256": testutil.Sha("y"), "X-Forwarded-For": "203.0.113.7"}})
	// 3. accepted uploads, 4. refused publish (nothing valid yet) — then a complete release
	testutil.Do(t, h, testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v0.1.0/files/stray.bin", Body: "x", Token: testutil.TestToken})
	testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish", Token: testutil.TestToken})
	testutil.PutAll(t, h, "v0.1.0", testutil.Release())
	testutil.Do(t, h, testutil.Call{Method: "PUT", Target: "/gitlab/artifacts/v0.1.0/files/stray.bin", Body: "x", Token: testutil.TestToken})
	rec, _ := testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish", Token: testutil.TestToken})
	if rec.Code != http.StatusUnprocessableEntity { // stray.bin is neither in the checksums nor in artifacts.json
		t.Fatalf("publish with stray file: %d", rec.Code)
	}
	staging, _ := core.NewLayout(root).StagingDir("gitlab", "v0.1.0")
	os.Remove(filepath.Join(staging, "stray.bin"))
	if rec, _ := testutil.Do(t, h, testutil.Call{Method: "POST", Target: "/gitlab/artifacts/v0.1.0/publish?latest=true", Token: testutil.TestToken}); rec.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", rec.Code, rec.Body)
	}

	ev := testutil.ReadAudit(t, path)
	byEvent := map[string][]map[string]any{}
	for _, e := range ev {
		byEvent[e["event"].(string)] = append(byEvent[e["event"].(string)], e)
		if e["time"] == nil || e["remote"] != "192.0.2.1" {
			t.Errorf("event without time or remote: %v", e)
		}
	}

	if f := byEvent["auth_failed"]; len(f) != 1 || f[0]["status"] != float64(401) || f[0]["authorization_sent"] != true || f[0]["path"] != "/gitlab/artifacts/v0.1.0/files/x" {
		t.Errorf("auth_failed = %v", f)
	}
	if r := byEvent["upload_refused"]; len(r) != 1 || r[0]["status"] != float64(422) || r[0]["forwarded_for"] != "203.0.113.7" || r[0]["name"] != "x" {
		t.Errorf("upload_refused = %v", r)
	}
	ups := byEvent["upload"]
	if len(ups) != 2+len(testutil.Release()) {
		t.Errorf("upload events = %d", len(ups))
	}
	if last := ups[len(ups)-1]; last["name"] != "stray.bin" || last["replaced"] != true || last["token_id"] != audit.TokenID(testutil.TestToken) || last["sha256"] != testutil.Sha("x") {
		t.Errorf("last upload = %v", last)
	}
	if r := byEvent["publish_refused"]; len(r) != 2 || r[0]["status"] != float64(422) {
		t.Errorf("publish_refused = %v", r)
	}
	pub := byEvent["publish"]
	if len(pub) != 1 || pub[0]["latest"] != true || pub[0]["version"] != "v0.1.0" {
		t.Fatalf("publish = %v", pub)
	}
	files := pub[0]["files"].([]any)
	if len(files) != len(testutil.Release())+2 { // + artifacts.goreleaser.json, release.json
		t.Fatalf("publish files = %v", files)
	}
	final, _ := core.NewLayout(root).VersionDir("gitlab", "v0.1.0")
	for _, f := range files {
		rec := f.(map[string]any)
		content, err := os.ReadFile(filepath.Join(final, rec["name"].(string)))
		if err != nil || rec["sha256"] != testutil.Sha(string(content)) || rec["size"] != float64(len(content)) {
			t.Errorf("publish record %v does not match the file", rec)
		}
	}

	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), testutil.TestToken) || strings.Contains(string(raw), "wrong-token") {
		t.Error("a token value leaked into the audit log")
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("audit log mode = %v", info.Mode().Perm())
	}
}
