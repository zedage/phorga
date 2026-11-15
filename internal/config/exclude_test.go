package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func excludeConfig(orgExclude, ddExclude []string) *Config {
	return &Config{
		Organizer: []OrganizerProfile{{
			Target:  "/target",
			Sources: []OrganizerSource{{Path: "/src", Mode: "copy", Schedule: "0 2 * * *", Exclude: orgExclude}},
		}},
		DuplicateDetection: []DuplicateDetection{{
			Schedule:      "0 4 * * *",
			Sources:       []string{"/src"},
			Exclude:       ddExclude,
			DetectionMode: "content",
			Action:        DuplicateAction{Method: "skip"},
		}},
	}
}

// Spec: CFG-007
func TestValidateExclude(t *testing.T) {
	if problems := ValidateProblems(excludeConfig([]string{"Screenshots", "2019/Scans", "*_edited.jpg"}, []string{"Export/"})); len(problems) != 0 {
		t.Fatalf("valid patterns rejected: %v", problems)
	}

	problems := ValidateProblems(excludeConfig([]string{"ok", "[abc"}, []string{"/photos/tmp"}))
	want := []string{
		"organizer[0].sources[0].exclude[1] is not a valid pattern",
		"duplicateDetection[0].exclude[0] must be relative to the source",
	}
	if len(problems) != len(want) {
		t.Fatalf("problems = %v, want %d", problems, len(want))
	}
	for i, w := range want {
		if !strings.HasPrefix(problems[i], w) {
			t.Errorf("problem %d = %q, want prefix %q", i, problems[i], w)
		}
	}
}

// Spec: CFG-007
func TestSaveAndLoadKeepsExclude(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"config.yaml", "config.json"} {
		path := filepath.Join(dir, name)
		cfg := excludeConfig([]string{"Screenshots", "2019/Scans"}, []string{"*_edited.jpg"})
		cfg.StateDir = dir
		if err := Save(path, cfg); err != nil {
			t.Fatalf("%s: save: %v", name, err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("%s: load: %v", name, err)
		}
		if got := loaded.Organizer[0].Sources[0].Exclude; !reflect.DeepEqual(got, []string{"Screenshots", "2019/Scans"}) {
			t.Errorf("%s: organizer exclude = %v", name, got)
		}
		if got := loaded.DuplicateDetection[0].Exclude; !reflect.DeepEqual(got, []string{"*_edited.jpg"}) {
			t.Errorf("%s: duplicateDetection exclude = %v", name, got)
		}
	}
}
