package exclude

import (
	"path/filepath"
	"testing"
)

// Spec: ORG-022
func TestSkip_FixedEntries(t *testing.T) {
	root := filepath.FromSlash("/photos")
	tests := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"@eaDir", true, true},
		{"2024/@eaDir", true, true},
		{"#recycle", true, true},
		{"#snapshot", true, true},
		{".thumbnails", true, true},
		{"._IMG_1234.jpg", false, true},
		{".hidden.jpg", false, true},
		{"IMG_1234.jpg", false, false},
		{"2024/07", true, false},
		{"@EADIR", true, false},  // exact names, as DSM writes them
		{"@eaDir", false, false}, // a file with that name is not a system folder
		{"my#recycle", true, false},
	}
	for _, tt := range tests {
		got := (*Matcher)(nil).Skip(root, filepath.Join(root, filepath.FromSlash(tt.rel)), tt.isDir)
		if got != tt.want {
			t.Errorf("Skip(%q, dir=%v) = %v, want %v", tt.rel, tt.isDir, got, tt.want)
		}
	}
}

// Spec: ORG-022
func TestSkip_RootIsNeverSkipped(t *testing.T) {
	root := filepath.FromSlash("/photos/.import")
	m := New([]string{".import", "*"})
	if m.Skip(root, root, true) {
		t.Error("the source root itself must not be skipped")
	}
	if !m.Skip(root, filepath.Join(root, "a.jpg"), false) {
		t.Error("pattern * should still skip entries below the root")
	}
}

// Spec: ORG-023
func TestSkip_Patterns(t *testing.T) {
	root := filepath.FromSlash("/photos")
	tests := []struct {
		pattern string
		rel     string
		isDir   bool
		want    bool
	}{
		// name patterns match at any depth
		{"Screenshots", "Screenshots", true, true},
		{"Screenshots", "2024/Screenshots", true, true},
		{"Screenshots", "2024/b.jpg", false, false},
		{"*_edited.jpg", "a_edited.jpg", false, true},
		{"*_edited.jpg", "2024/07/a_edited.jpg", false, true},
		{"*_edited.jpg", "a.jpg", false, false},
		// path patterns match the path relative to the root
		{"2019/Scans", "2019/Scans", true, true},
		{"2019/Scans", "2020/Scans", true, false},
		{"2019/Scans", "x/2019/Scans", true, false},
		{"*/WhatsApp", "2024/WhatsApp", true, true},
		{"*/WhatsApp", "WhatsApp", true, false},
		{"*/WhatsApp", "a/b/WhatsApp", true, false},
		// case is ignored
		{"screenshots", "Screenshots", true, true},
		{"2019/SCANS", "2019/scans", true, true},
		{"*.TMP", "x.tmp", false, true},
		// trailing slash: folders only
		{"Export/", "Export", true, true},
		{"Export/", "Export", false, false},
		// wildcards
		{"IMG_00?.jpg", "IMG_001.jpg", false, true},
		{"[ab].jpg", "b.jpg", false, true},
	}
	for _, tt := range tests {
		m := New([]string{tt.pattern})
		got := m.Skip(root, filepath.Join(root, filepath.FromSlash(tt.rel)), tt.isDir)
		if got != tt.want {
			t.Errorf("pattern %q: Skip(%q, dir=%v) = %v, want %v", tt.pattern, tt.rel, tt.isDir, got, tt.want)
		}
	}
}

// Spec: CFG-007
func TestValidate(t *testing.T) {
	tests := []struct {
		pattern string
		want    string
	}{
		{"Screenshots", ""},
		{"*_edited.jpg", ""},
		{"2019/Scans", ""},
		{"Export/", ""},
		{"", "must not be empty"},
		{"  ", "must not be empty"},
		{"/photos/tmp", "must be relative to the source"},
		{"/", "must be relative to the source"},
		{"../other", "must not contain '..'"},
		{"a/../b", "must not contain '..'"},
		{"[abc", "is not a valid pattern"},
	}
	for _, tt := range tests {
		err := Validate(tt.pattern)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != tt.want {
			t.Errorf("Validate(%q) = %q, want %q", tt.pattern, got, tt.want)
		}
	}
}
