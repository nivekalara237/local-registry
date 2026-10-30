package endpoints

import (
	"encoding/json"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestParseProductFile(t *testing.T) {
	full := `apiVersion: local-registry/v1
displayName: SonarBridge
description: Bridge between SonarQube and internal tools.
license: Proprietary
owners:
  - name: Platform team
    email: platform@corp.example
links:
  - { name: source, url: "https://git.corp.example/platform/sonarbridge" }
keywords: [sonar, cli]
labels:
  cost-center: 4512
install: |
  curl -fsSL https://registry.example.com/… | tar xz
deprecated: replaced by sonarbridge2
`
	p, err := parseProductFile([]byte(full))
	if err != nil {
		t.Fatal(err)
	}
	if p.DisplayName != "SonarBridge" || p.Owners[0].Email != "platform@corp.example" || p.Links[0].Name != "source" ||
		!slices.Equal(p.Keywords, []string{"sonar", "cli"}) || p.Labels["cost-center"] != "4512" ||
		!strings.HasPrefix(p.Install, "curl") || p.Deprecated != "replaced by sonarbridge2" {
		t.Fatalf("parsed = %+v", p)
	}
	if p, err := parseProductFile([]byte("apiVersion: local-registry/v1\n")); err != nil || p.Deprecated != false {
		t.Fatalf("minimal: %+v, %v", p, err)
	}

	for _, tt := range []struct{ name, yaml, want string }{
		{"empty", "", "empty"},
		{"no apiVersion", "displayName: x\n", "apiVersion must be local-registry/v1"},
		{"wrong apiVersion", "apiVersion: v2\n", "apiVersion must be"},
		{"unknown key", "apiVersion: local-registry/v1\ndisplayname: x\n", "displayname"},
		{"changes are not written by hand", "apiVersion: local-registry/v1\nchanges: []\n", "changes"},
		{"two documents", "apiVersion: local-registry/v1\n---\napiVersion: local-registry/v1\n", "several YAML documents"},
		{"long display name", "apiVersion: local-registry/v1\ndisplayName: " + strings.Repeat("x", 101) + "\n", "displayName"},
		{"owner without name", "apiVersion: local-registry/v1\nowners: [{email: a@b}]\n", "owners[0] has no name"},
		{"bad email", "apiVersion: local-registry/v1\nowners: [{name: a, email: nope}]\n", "owners[0].email"},
		{"ftp link", "apiVersion: local-registry/v1\nlinks: [{name: a, url: \"ftp://x\"}]\n", "links[0]"},
		{"deprecated list", "apiVersion: local-registry/v1\ndeprecated: [1]\n", "deprecated must be"},
		{"too large", "apiVersion: local-registry/v1\ninstall: " + strings.Repeat("x", maxProductFileSize) + "\n", "larger than 64 KiB"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseProductFile([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.HasPrefix(err.Error(), "registry.yaml: ") {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseChangelog(t *testing.T) {
	text := "## Changelog\n\n### Features\n" +
		"* 9c9d7664ceefa1b2: feat(api): CSV export (@alice)\n" +
		"* 1a2b3c4: fix: projects beyond 500 (Bob <bob@corp.example>)\n" +
		"- 1a2b3c5 fix(security): bump golang.org/x/net\n" +
		"* sec: rotate keys by @carol in https://github.com/o/r/pull/9\n" +
		"* 1a2b3c6: docs: typo\n" +
		"* 1a2b3c7: chore!: drop Go 1.25\n" +
		"* 1a2b3c8: perf: faster hashing\n" +
		"* no conventional prefix\n" +
		"**Full Changelog**: https://github.com/o/r/compare/v1...v2\n"
	want := []Change{
		{"added", "CSV export"},
		{"fixed", "projects beyond 500"},
		{"security", "bump golang.org/x/net"},
		{"security", "rotate keys"},
		{"changed", "drop Go 1.25"},
		{"changed", "faster hashing"},
	}
	if got := parseChangelog(text, "v2.0.0"); !slices.Equal(got, want) {
		t.Fatalf("changes = %+v", got)
	}
	if got := parseChangelog("", "v1"); got == nil || len(got) != 0 {
		t.Fatalf("empty changelog = %#v, want an empty list", got)
	}
}

func TestParseKeepAChangelog(t *testing.T) {
	text := "# Changelog\n\nAll notable changes.\n\n" +
		"## [Unreleased]\n### Added\n- not yet\n\n" +
		"## [1.1.0] - 2026-10-18\n### Added\n- orders table\n* index on users.email\n" +
		"### Security\n- drop the default password\n" +
		"### Notes\n- not a kind\n- 1a2b3c4: fix: a goreleaser item still counts\n" +
		"### Fixed ###\n  - products.price precision\n\n" +
		"## [1.0.0] - 2026-09-01\n### Added\n- users table\n"
	want := []Change{
		{"added", "orders table"},
		{"added", "index on users.email"},
		{"security", "drop the default password"},
		{"fixed", "a goreleaser item still counts"},
		{"fixed", "products.price precision"},
	}
	for _, v := range []string{"v1.1.0", "1.1.0"} {
		if got := parseChangelog(text, v); !slices.Equal(got, want) {
			t.Fatalf("%s: changes = %+v", v, got)
		}
	}
	if got := parseChangelog(text, "v1.0.0"); !slices.Equal(got, []Change{{"added", "users table"}}) {
		t.Fatalf("v1.0.0: changes = %+v", got)
	}
	if got := parseChangelog(text, "v2.0.0"); len(got) != 0 {
		t.Fatalf("a version without a section has no notes: %+v", got)
	}
	// Without version headings, the whole file is this version's notes.
	if got := parseChangelog("### Removed\n- legacy table\n", "v3"); !slices.Equal(got, []Change{{"removed", "legacy table"}}) {
		t.Fatalf("unversioned: changes = %+v", got)
	}
}

func TestInferArtifacts(t *testing.T) {
	files := []string{"CHANGELOG.md", "checksums.txt", "checksums.txt.sig", "db.tar.gz", "db.tar.gz.sbom.json",
		"metadata.json", "notes.pdf", "orphan.sig", "registry.yaml", "release.json", "tool.deb", "tool.deb.pem"}
	var got []string
	for _, a := range inferArtifacts(files, "checksums.txt") {
		got = append(got, a.Name+"="+a.Type)
	}
	want := []string{"checksums.txt=Checksum", "checksums.txt.sig=Signature", "db.tar.gz=Archive", "db.tar.gz.sbom.json=SBOM",
		"notes.pdf=File", "orphan.sig=File", "tool.deb=Linux Package", "tool.deb.pem=Certificate"}
	if !slices.Equal(got, want) {
		t.Fatalf("inferred = %v\nwant       %v", got, want)
	}
}

func TestCompareVersions(t *testing.T) {
	docs := []ReleaseDocument{
		{Version: "v1.0.0-alpha"}, {Version: "1.0.0"}, {Version: "nightly", PublishedAt: "2026-01-02T00:00:00Z"},
		{Version: "v1.0.0-alpha.1"}, {Version: "v1.10.0"}, {Version: "v1.0.0-beta"}, {Version: "v1.2.0"},
		{Version: "old", PublishedAt: "2026-01-01T00:00:00Z"}, {Version: "v1.0.0-alpha.beta"}, {Version: "v1.0.0-2"},
	}
	slices.SortFunc(docs, func(a, b ReleaseDocument) int { return compareVersions(b, a) })
	var got []string
	for _, d := range docs {
		got = append(got, d.Version)
	}
	want := []string{"v1.10.0", "v1.2.0", "1.0.0", "v1.0.0-beta", "v1.0.0-alpha.beta", "v1.0.0-alpha.1", "v1.0.0-alpha", "v1.0.0-2", "nightly", "old"}
	if !slices.Equal(got, want) {
		t.Fatalf("order = %v\nwant    %v", got, want)
	}
	if !isPrerelease("v1.5.0-rc.1") || isPrerelease("v1.5.0") || isPrerelease("nightly") || isPrerelease("v1.5.0+build.1") {
		t.Fatal("isPrerelease")
	}
}

func TestMediaTypeAndFormat(t *testing.T) {
	for _, tt := range []struct{ name, mediaType, format string }{
		{"tool_1.0.0_linux_amd64.tar.gz", "application/gzip", "tar.gz"},
		{"tool.ZIP", "application/zip", "zip"},
		{"tool_1.0.0_amd64.deb", "application/vnd.debian.binary-package", "deb"},
		{"migrations.sql", "application/sql", "sql"},
		{"policy.yaml", "application/yaml", "yaml"},
		{"tool", "application/octet-stream", ""},
	} {
		if got := mediaType(tt.name); got != tt.mediaType {
			t.Errorf("mediaType(%s) = %s", tt.name, got)
		}
		f := formatOf(artifact{Name: tt.name})
		if (f == nil) != (tt.format == "") || (f != nil && *f != tt.format) {
			t.Errorf("formatOf(%s) = %v", tt.name, f)
		}
	}
	a := artifact{Name: "x.pkg"}
	a.Extra.Format = "binary"
	if f := formatOf(a); f == nil || *f != "binary" {
		t.Errorf("extra.Format ignored: %v", f)
	}
}

// The published JSON Schema and the server must accept the same keys.
func TestProductFileSchemaMatches(t *testing.T) {
	raw, err := os.ReadFile("../../schema/registry.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	var tags []string
	typ := reflect.TypeFor[ProductFile]()
	for field := range typ.Fields() {
		tags = append(tags, field.Tag.Get("yaml"))
	}
	if got := slices.Sorted(maps.Keys(schema.Properties)); !slices.Equal(got, slices.Sorted(slices.Values(tags))) {
		t.Fatalf("schema properties = %v, ProductFile keys = %v", got, tags)
	}
}
