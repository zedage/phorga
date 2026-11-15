// Package exclude decides which entries below a source folder are skipped:
// the fixed system and hidden entries (ORG-022) and the user's exclude
// patterns (ORG-023, DUP-013).
package exclude

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
)

// systemFolders are folders that NAS systems create inside shares. They are
// matched exactly, as DSM writes them (ORG-022).
var systemFolders = map[string]bool{
	"@eaDir":    true, // Synology thumbnails and index data
	"#recycle":  true, // share recycle bin
	"#snapshot": true, // Btrfs snapshots
}

type pattern struct {
	glob    string // lower case
	byPath  bool   // contains "/": matches the path relative to the root
	dirOnly bool   // had a trailing "/": matches folders only
}

// Matcher holds the exclude patterns of one source or job. A nil Matcher
// skips the fixed entries only.
type Matcher struct {
	patterns []pattern
}

// New builds a Matcher from patterns that passed Validate. Empty patterns
// are ignored; a pattern that is still invalid never matches.
func New(patterns []string) *Matcher {
	m := &Matcher{}
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		dirOnly := strings.HasSuffix(p, "/")
		p = strings.TrimRight(p, "/")
		if p == "" {
			continue
		}
		m.patterns = append(m.patterns, pattern{
			glob:    strings.ToLower(p),
			byPath:  strings.Contains(p, "/"),
			dirOnly: dirOnly,
		})
	}
	return m
}

// Validate reports why p cannot be used as an exclude pattern (CFG-007).
func Validate(p string) error {
	trimmed := strings.TrimRight(strings.TrimSpace(p), "/")
	if trimmed == "" {
		if strings.HasPrefix(strings.TrimSpace(p), "/") {
			return errors.New("must be relative to the source")
		}
		return errors.New("must not be empty")
	}
	if strings.HasPrefix(trimmed, "/") {
		return errors.New("must be relative to the source")
	}
	for _, seg := range strings.Split(trimmed, "/") {
		if seg == ".." {
			return errors.New("must not contain '..'")
		}
	}
	if _, err := path.Match(trimmed, ""); err != nil {
		return errors.New("is not a valid pattern")
	}
	return nil
}

// Skip reports whether the entry at p, found while walking root, is skipped.
// The root itself is never skipped (ORG-022).
func (m *Matcher) Skip(root, p string, isDir bool) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == "." {
		return false
	}
	rel = filepath.ToSlash(rel)
	name := path.Base(rel)

	if strings.HasPrefix(name, ".") || (isDir && systemFolders[name]) {
		return true
	}
	if m == nil {
		return false
	}

	lowerRel, lowerName := strings.ToLower(rel), strings.ToLower(name)
	for _, pt := range m.patterns {
		if pt.dirOnly && !isDir {
			continue
		}
		subject := lowerName
		if pt.byPath {
			subject = lowerRel
		}
		if ok, _ := path.Match(pt.glob, subject); ok {
			return true
		}
	}
	return false
}
