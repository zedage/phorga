package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func browseRootsConfig(roots ...string) *Config {
	return &Config{
		WebUI: &WebUIConfig{BrowseRoots: roots},
		Organizer: []OrganizerProfile{{
			Target:  "/target",
			Sources: []OrganizerSource{{Path: "/src", Mode: "copy", Schedule: "0 2 * * *"}},
		}},
	}
}

// Spec: CFG-006
func TestValidateBrowseRoots(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if problems := ValidateProblems(browseRootsConfig(dir)); len(problems) != 0 {
		t.Fatalf("expected existing directory to be valid, got %v", problems)
	}

	cases := map[string]string{
		"relative":  "photos",
		"missing":   filepath.Join(dir, "missing"),
		"not a dir": file,
	}
	want := map[string]string{
		"relative":  "must be an absolute path",
		"missing":   "does not exist",
		"not a dir": "is not a directory",
	}
	for name, root := range cases {
		problems := ValidateProblems(browseRootsConfig(root))
		if len(problems) != 1 || !strings.Contains(problems[0], "webUI.browseRoots[0]") || !strings.Contains(problems[0], want[name]) {
			t.Errorf("%s: unexpected problems %v", name, problems)
		}
	}
}

// Spec: CFG-006
func TestSaveAndLoadKeepsBrowseRoots(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := browseRootsConfig(dir)
	cfg.StateDir = dir
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.WebUI == nil || !reflect.DeepEqual(loaded.WebUI.BrowseRoots, []string{dir}) {
		t.Fatalf("browseRoots not kept: %+v", loaded.WebUI)
	}
}

// Spec: WEB-007
func TestEnumAccessorsMatchValidation(t *testing.T) {
	check := func(name string, got []string, m map[string]bool) {
		if len(got) != len(m) {
			t.Errorf("%s: got %v, want keys of %v", name, got, m)
		}
		for _, v := range got {
			if !m[v] {
				t.Errorf("%s: %q not accepted by validation", name, v)
			}
		}
	}
	check("SourceModes", SourceModes(), validModes)
	check("OrganizerDuplicateMethods", OrganizerDuplicateMethods(), validOrgDupMethods)
	check("DedupMethods", DedupMethods(), validDedupMethods)
	check("DedupModes", DedupModes(), validDedupModes)
}
