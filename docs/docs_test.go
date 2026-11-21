// Package docs holds Phorga's documentation. This test keeps the specs,
// tests and Markdown links mechanically consistent (see docs/specs/README.md).
package docs

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const repoRoot = ".."

var (
	reqHeading   = regexp.MustCompile(`^###\s+([A-Z]{3}-\d{3})\b`)
	statusLine   = regexp.MustCompile(`^\*\*Status:\*\*\s+(\S+)`)
	codeLine     = regexp.MustCompile(`^\*\*Code:\*\*\s+(.*)$`)
	testsLine    = regexp.MustCompile(`^\*\*Tests:\*\*\s+(.*)$`)
	gapLine      = regexp.MustCompile(`^\*\*Gap:\*\*`)
	backticked   = regexp.MustCompile("`([^`]+)`")
	specComment  = regexp.MustCompile(`^//\s*Spec:\s*(.+)$`)
	specID       = regexp.MustCompile(`[A-Z]{3}-\d{3}`)
	mdLink       = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	adocLink     = regexp.MustCompile(`link:([^\[\s]+)\[`)
	testFuncDecl = regexp.MustCompile(`^func (Test\w+)\(`)
)

var specPrefixes = map[string]string{
	"organizer.md":           "ORG",
	"duplicate-detection.md": "DUP",
	"configuration.md":       "CFG",
	"notifications.md":       "NTF",
	"web-ui.md":              "WEB",
	"runtime.md":             "RUN",
	"synology.md":            "SYN",
}

var validStatuses = map[string]bool{"proposed": true, "implemented": true, "removed": true}

type requirement struct {
	id, file, status string
	code, tests      string // raw values of the Code/Tests lines ("" if missing)
	hasGap           bool
}

func loadRequirements(t *testing.T) map[string]*requirement {
	t.Helper()
	reqs := map[string]*requirement{}
	for name, prefix := range specPrefixes {
		path := filepath.Join(repoRoot, "docs", "specs", name)
		lines := readLines(t, path)
		var cur *requirement
		for _, line := range lines {
			if m := reqHeading.FindStringSubmatch(line); m != nil {
				id := m[1]
				if !strings.HasPrefix(id, prefix+"-") {
					t.Errorf("%s: requirement %s does not use prefix %s", name, id, prefix)
				}
				if prev, dup := reqs[id]; dup {
					t.Errorf("duplicate requirement ID %s in %s and %s", id, prev.file, name)
				}
				cur = &requirement{id: id, file: name}
				reqs[id] = cur
				continue
			}
			if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
				cur = nil
			}
			if cur == nil {
				continue
			}
			if m := statusLine.FindStringSubmatch(line); m != nil {
				cur.status = m[1]
			} else if m := codeLine.FindStringSubmatch(line); m != nil {
				cur.code = m[1]
			} else if m := testsLine.FindStringSubmatch(line); m != nil {
				cur.tests = m[1]
			} else if gapLine.MatchString(line) {
				cur.hasGap = true
			}
		}
	}
	if len(reqs) == 0 {
		t.Fatal("no requirements found in docs/specs")
	}
	return reqs
}

func TestSpecRequirementsAreWellFormed(t *testing.T) {
	for _, r := range sortedReqs(loadRequirements(t)) {
		if !validStatuses[r.status] {
			t.Errorf("%s (%s): invalid or missing **Status:** %q", r.id, r.file, r.status)
			continue
		}
		if r.status != "implemented" {
			continue
		}
		if r.code == "" || strings.TrimSpace(r.code) == "none" {
			t.Errorf("%s: implemented requirement needs a **Code:** line with paths", r.id)
		}
		if r.tests == "" {
			t.Errorf("%s: implemented requirement needs a **Tests:** line (use `none` + **Gap:** if untested)", r.id)
		}
		if strings.TrimSpace(r.tests) == "none" && !r.hasGap {
			t.Errorf("%s: **Tests:** none requires a **Gap:** line", r.id)
		}
	}
}

func TestSpecCodeAndTestReferencesExist(t *testing.T) {
	reqs := loadRequirements(t)
	for _, r := range sortedReqs(reqs) {
		for _, m := range backticked.FindAllStringSubmatch(r.code, -1) {
			if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(m[1]))); err != nil {
				t.Errorf("%s: **Code:** path %q does not exist", r.id, m[1])
			}
		}
		for _, m := range backticked.FindAllStringSubmatch(r.tests, -1) {
			file, name, ok := strings.Cut(m[1], "#")
			if !ok {
				t.Errorf("%s: **Tests:** entry %q must be path#TestName", r.id, m[1])
				continue
			}
			ids, found := testSpecIDs(t, filepath.Join(repoRoot, filepath.FromSlash(file)), name)
			if !found {
				t.Errorf("%s: test %s not found in %s", r.id, name, file)
				continue
			}
			if !contains(ids, r.id) {
				t.Errorf("%s: test %s in %s lacks a `// Spec: %s` comment", r.id, name, file, r.id)
			}
		}
	}
}

func TestSpecCommentsInTestsReferenceKnownIDs(t *testing.T) {
	reqs := loadRequirements(t)
	walkFiles(t, func(path string) bool { return strings.HasSuffix(path, "_test.go") }, func(path string) {
		for i, line := range readLines(t, path) {
			m := specComment.FindStringSubmatch(strings.TrimSpace(line))
			if m == nil {
				continue
			}
			for _, id := range specID.FindAllString(m[1], -1) {
				if _, ok := reqs[id]; !ok {
					t.Errorf("%s:%d references unknown spec ID %s", rel(path), i+1, id)
				}
			}
		}
	})
}

func TestMarkdownLinksResolve(t *testing.T) {
	isDoc := func(path string) bool {
		r := filepath.ToSlash(rel(path))
		return (strings.HasSuffix(r, ".md") || strings.HasSuffix(r, ".adoc")) &&
			(strings.HasPrefix(r, "docs/") || strings.HasPrefix(r, ".claude/") || !strings.Contains(r, "/"))
	}
	walkFiles(t, isDoc, func(path string) {
		linkRe := mdLink
		if strings.HasSuffix(path, ".adoc") {
			linkRe = adocLink
		}
		inFence := false
		for i, line := range readLines(t, path) {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, m := range linkRe.FindAllStringSubmatch(line, -1) {
				target := m[1]
				if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
					continue
				}
				target, _, _ = strings.Cut(target, "#")
				if _, err := os.Stat(filepath.Join(filepath.Dir(path), filepath.FromSlash(target))); err != nil {
					t.Errorf("%s:%d: broken link %q", rel(path), i+1, m[1])
				}
			}
		}
	})
}

// testSpecIDs returns the spec IDs from the `// Spec:` comment block directly
// above `func <name>(` and whether the function exists.
func testSpecIDs(t *testing.T, path, name string) ([]string, bool) {
	t.Helper()
	lines := readLines(t, path)
	for i, line := range lines {
		m := testFuncDecl.FindStringSubmatch(line)
		if m == nil || m[1] != name {
			continue
		}
		var ids []string
		for j := i - 1; j >= 0 && strings.HasPrefix(strings.TrimSpace(lines[j]), "//"); j-- {
			if sm := specComment.FindStringSubmatch(strings.TrimSpace(lines[j])); sm != nil {
				ids = append(ids, specID.FindAllString(sm[1], -1)...)
			}
		}
		return ids, true
	}
	return nil, false
}

func walkFiles(t *testing.T, match func(string) bool, fn func(string)) {
	t.Helper()
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "data":
				if path != repoRoot {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if match(path) {
			fn(path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repo: %v", err)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return lines
}

func rel(path string) string {
	r, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

func sortedReqs(m map[string]*requirement) []*requirement {
	out := make([]*requirement, 0, len(m))
	for _, r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
