package endpoints

import (
	"regexp"
	"strings"
)

// Change is one release note of release.json.
type Change struct {
	// Kind is added, changed, deprecated, removed, fixed or security (Keep a Changelog).
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

var (
	// bulletRe is a list item: "* …" or "- …".
	bulletRe = regexp.MustCompile(`^\s*[*-]\s+(.+)$`)
	// shaRe is the commit goreleaser puts before the message: "9c9d766 " or "9c9d766: ".
	shaRe = regexp.MustCompile(`^[0-9a-f]{7,40}:?\s+`)
	// commitRe is a Conventional Commits subject: type(scope)!: description.
	commitRe = regexp.MustCompile(`^([A-Za-z]+)(?:\(([^)]*)\))?(!)?:\s*(.+)$`)
	// authorRe is the author goreleaser appends: " (@user)", " (Name <mail>)" or " by @user in <url>".
	authorRe = regexp.MustCompile(`\s+(?:\((?:@[\w-]+|[^()]*<[^<>]*@[^<>]*>)\)|by @[\w-]+ in \S+)$`)
	// headingRe is a Markdown heading: its level and its text.
	headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	// versionHeadingRe is a Keep a Changelog version heading text: "[1.2.0] - 2026-10-18",
	// "v1.2.0", "[Unreleased]". Its first group is the version.
	versionHeadingRe = regexp.MustCompile(`^\[?(v?\d[^\]\s]*|(?i:unreleased))\]?(?:\s|$)`)
)

// changeKinds maps a commit type to its release note kind; other types (docs, test, chore,
// ci, build, style…) are not release notes.
var changeKinds = map[string]string{
	"feat": "added", "add": "added", "feature": "added",
	"fix": "fixed", "bugfix": "fixed",
	"perf": "changed", "refactor": "changed", "change": "changed",
	"deprecate": "deprecated", "deprecated": "deprecated",
	"remove": "removed", "removed": "removed",
	"sec": "security", "security": "security",
}

// sectionKinds are the Keep a Changelog sections ("### Added"), by lower-case title.
var sectionKinds = map[string]string{
	"added": "added", "changed": "changed", "deprecated": "deprecated",
	"removed": "removed", "fixed": "fixed", "security": "security",
}

// parseChangelog turns a CHANGELOG.md into the release notes of version, in order. It reads
// two forms, which may mix:
//
//   - Keep a Changelog: each list item under an "### Added", "### Fixed"… heading is a note
//     of that kind. When the file has version headings ("## [1.2.0] - 2026-10-18"), only the
//     section of version is read (with or without its "v"), and none when it has no section.
//   - goreleaser: each other list item whose message is a Conventional Commit of a known type.
//     A security scope (fix(security): …) makes a security note; a breaking change
//     (feat!: …) of an unknown type makes a changed note.
func parseChangelog(text, version string) []Change {
	changes := []Change{}
	section := "" // kind of the current Keep a Changelog section, if any
	for line := range strings.Lines(versionSection(text, version)) {
		line = strings.TrimRight(line, "\r\n")
		if h := headingRe.FindStringSubmatch(line); h != nil {
			section = ""
			if len(h[1]) >= 3 {
				section = sectionKinds[strings.ToLower(h[2])]
			}
			continue
		}
		m := bulletRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if section != "" {
			if desc := strings.TrimSpace(m[1]); desc != "" {
				changes = append(changes, Change{Kind: section, Description: desc})
			}
			continue
		}
		if c, ok := parseCommit(m[1]); ok {
			changes = append(changes, c)
		}
	}
	return changes
}

// parseCommit reads one goreleaser changelog item: "<sha>: type(scope)!: description (author)".
func parseCommit(item string) (Change, bool) {
	msg := shaRe.ReplaceAllString(item, "")
	msg = authorRe.ReplaceAllString(msg, "")
	c := commitRe.FindStringSubmatch(msg)
	if c == nil {
		return Change{}, false
	}
	typ, scope, breaking, desc := strings.ToLower(c[1]), strings.ToLower(c[2]), c[3] != "", strings.TrimSpace(c[4])
	kind, ok := changeKinds[typ]
	switch {
	case scope == "security" || scope == "sec":
		kind, ok = "security", true
	case !ok && breaking:
		kind, ok = "changed", true
	}
	return Change{Kind: kind, Description: desc}, ok && desc != ""
}

// versionSection is the part of text under the "## <version>" heading of version: from it
// to the next level 1 or 2 heading. Text without such version headings is returned whole;
// text with them but none for version gives "".
func versionSection(text, version string) string {
	want := strings.TrimPrefix(version, "v")
	var out strings.Builder
	versioned, in := false, false
	for line := range strings.Lines(text) {
		if h := headingRe.FindStringSubmatch(strings.TrimRight(line, "\r\n")); h != nil && len(h[1]) <= 2 {
			in = false
			if v := versionHeadingRe.FindStringSubmatch(h[2]); v != nil && len(h[1]) == 2 {
				versioned = true
				in = strings.TrimPrefix(v[1], "v") == want
			}
		}
		if in {
			out.WriteString(line)
		}
	}
	if !versioned {
		return text
	}
	return out.String()
}
