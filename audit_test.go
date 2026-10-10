package local_registry

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tempAudit opens an audit log in a temp directory and returns it with its path.
func tempAudit(t *testing.T) (*AuditLog, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sub", "audit.log")
	a, err := OpenAuditLog(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	return a, path
}

// readAudit parses every line of the audit log at path.
func readAudit(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e map[string]any
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("audit line %q: %v", sc.Text(), err)
		}
		out = append(out, e)
	}
	return out
}

func TestAuditTrail(t *testing.T) {
	root := t.TempDir()
	stubFreeSpace(t, 1<<40, true)
	s := NewLocalRegistryServer(0, "localhost", root)
	a, path := tempAudit(t)
	s.SetAuditLog(a)
	s.AddUploadRoutes(UploadConfig{Token: testToken})
	h := s.Handler()

	// 1. failed authentication
	do(t, h, call{method: "PUT", target: "/gitlab/artifacts/v0.1.0/files/x", body: "x", token: "wrong-token"})
	// 2. refused upload (checksum header mismatch), through a proxy
	do(t, h, call{method: "PUT", target: "/gitlab/artifacts/v0.1.0/files/x", body: "x", token: testToken,
		header: map[string]string{"X-Checksum-Sha256": sha("y"), "X-Forwarded-For": "203.0.113.7"}})
	// 3. accepted uploads, 4. refused publish (nothing valid yet) — then a complete release
	do(t, h, call{method: "PUT", target: "/gitlab/artifacts/v0.1.0/files/stray.bin", body: "x", token: testToken})
	do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish", token: testToken})
	putAll(t, h, "v0.1.0", release())
	do(t, h, call{method: "PUT", target: "/gitlab/artifacts/v0.1.0/files/stray.bin", body: "x", token: testToken})
	rec, _ := do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish", token: testToken})
	if rec.Code != http.StatusUnprocessableEntity { // stray.bin is not in the checksums
		t.Fatalf("publish with stray file: %d", rec.Code)
	}
	staging, _ := NewLayout(root).StagingDir("gitlab", "v0.1.0")
	os.Remove(filepath.Join(staging, "stray.bin"))
	if rec, _ := do(t, h, call{method: "POST", target: "/gitlab/artifacts/v0.1.0/publish?latest=true", token: testToken}); rec.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", rec.Code, rec.Body)
	}

	ev := readAudit(t, path)
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
	if len(ups) != 2+len(release()) {
		t.Errorf("upload events = %d", len(ups))
	}
	if last := ups[len(ups)-1]; last["name"] != "stray.bin" || last["replaced"] != true || last["token_id"] != TokenID(testToken) || last["sha256"] != sha("x") {
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
	if len(files) != len(release()) {
		t.Fatalf("publish files = %v", files)
	}
	for _, f := range files {
		rec := f.(map[string]any)
		content := release()[rec["name"].(string)]
		if rec["sha256"] != sha(content) || rec["size"] != float64(len(content)) {
			t.Errorf("publish record %v does not match the file", rec)
		}
	}

	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), testToken) || strings.Contains(string(raw), "wrong-token") {
		t.Error("a token value leaked into the audit log")
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("audit log mode = %v", info.Mode().Perm())
	}
}

func TestAuditWithoutLogGoesToSlog(t *testing.T) {
	s := NewLocalRegistryServer(0, "localhost", t.TempDir())
	// Must not panic without an audit file.
	s.audit(httptest.NewRequest("GET", "/", nil), "probe", map[string]any{"k": "v"})
}
