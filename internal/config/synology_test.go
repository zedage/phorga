package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Spec: CFG-010
func TestValidateSynologyHomes(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, nil, 0644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"homes":                       "must be an absolute path",
		filepath.Join(dir, "missing"): "does not exist",
		file:                          "is not a directory",
	}
	for homes, want := range cases {
		cfg := &Config{SynologyPhotos: &SynologyPhotos{Homes: homes}}
		problems := strings.Join(ValidateProblems(cfg), "\n")
		if !strings.Contains(problems, "synologyPhotos.homes") || !strings.Contains(problems, want) {
			t.Errorf("homes %q: expected %q, got %q", homes, want, problems)
		}
	}
	ok := &Config{SynologyPhotos: &SynologyPhotos{Homes: dir}}
	if problems := ValidateProblems(ok); len(problems) > 0 {
		t.Errorf("existing folder must be valid, got %v", problems)
	}
	if got := (&Config{}).SynologyHomes(); got != DefaultSynologyHomes {
		t.Errorf("default homes = %q", got)
	}
}

// Spec: CFG-010
func TestSaveAndLoadKeepsSynologyHomes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &Config{SynologyPhotos: &SynologyPhotos{Homes: "/homes"}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SynologyHomes() != "/homes" {
		t.Fatalf("homes lost: %+v", loaded.SynologyPhotos)
	}
}

// Spec: CFG-011
func TestSaveAndLoadKeepsNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &Config{
		Organizer:          []OrganizerProfile{{Name: "Robin's phone", Target: "/t", Sources: []OrganizerSource{{Path: "/s", Mode: "copy", Schedule: "0 2 * * *"}}}},
		DuplicateDetection: []DuplicateDetection{{Name: "All photos", Schedule: "0 4 * * *", Sources: []string{"/t"}, DetectionMode: "metadata", Action: DuplicateAction{Method: "skip"}}},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Organizer[0].Name != "Robin's phone" || loaded.DuplicateDetection[0].Name != "All photos" {
		t.Fatalf("names lost: %+v", loaded)
	}
	unnamed := &Config{Organizer: []OrganizerProfile{{Target: "/t"}}}
	data, err := Marshal(path, unnamed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "name:") {
		t.Error("an empty name must be left out of the file")
	}
}
