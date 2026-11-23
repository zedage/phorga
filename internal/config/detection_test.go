package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Spec: ORG-011, CFG-002
func TestParseDetectionMode(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want DetectionExpr
	}{
		{"filename", DetectionExpr{Fields: []string{"filename"}}},
		{"filename+exif", DetectionExpr{Fields: []string{"filename", "exif"}}},
		{"filedate | exif", DetectionExpr{Fields: []string{"filedate", "exif"}, Any: true}},
	} {
		got, err := ParseDetectionMode(tc.mode)
		if err != nil {
			t.Errorf("%q: %v", tc.mode, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: got %+v, want %+v", tc.mode, got, tc.want)
		}
	}
	for _, bad := range []string{"", "  ", "content", "filename+filedate|exif", "filename+"} {
		if _, err := ParseDetectionMode(bad); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func replaceConfig(mode string) *Config {
	return &Config{Organizer: []OrganizerProfile{{
		Target: "/target",
		Sources: []OrganizerSource{{Path: "/src", Mode: "move", Schedule: "0 2 * * *",
			Duplicates: &OrganizerDuplicate{DetectionMode: mode, Action: DuplicateAction{Method: "replace"}}}},
	}}}
}

// Spec: CFG-002, ORG-012
func TestValidateReplaceNeedsMoreThanTheFilename(t *testing.T) {
	for _, mode := range []string{"filename", "filename|exif", "filedate|filename"} {
		problems := ValidateProblems(replaceConfig(mode))
		if len(problems) != 1 || !strings.HasPrefix(problems[0], "organizer[0].sources[0].duplicates.action.method 'replace' needs") {
			t.Errorf("%q: problems = %v", mode, problems)
		}
	}
	for _, mode := range []string{"filename+filedate", "filename+exif", "exif", "filedate", "filedate|exif"} {
		if problems := ValidateProblems(replaceConfig(mode)); len(problems) != 0 {
			t.Errorf("%q: unexpected problems %v", mode, problems)
		}
	}
	// Other methods are fine with filename alone.
	cfg := replaceConfig("filename")
	cfg.Organizer[0].Sources[0].Duplicates.Action.Method = "addSuffix"
	if problems := ValidateProblems(cfg); len(problems) != 0 {
		t.Errorf("addSuffix with filename: %v", problems)
	}
}

// Spec: ORG-002
func TestSaveAndLoadKeepsIgnoreModTime(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"config.yaml", "config.json"} {
		path := filepath.Join(dir, name)
		cfg := replaceConfig("exif")
		cfg.StateDir = dir
		cfg.Organizer[0].IgnoreModTime = true
		if err := Save(path, cfg); err != nil {
			t.Fatalf("%s: save: %v", name, err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("%s: load: %v", name, err)
		}
		if !loaded.Organizer[0].IgnoreModTime {
			t.Errorf("%s: ignoreModTime lost", name)
		}
	}
}
