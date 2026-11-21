package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Spec: CFG-001
func TestLoadYAML(t *testing.T) {
	content := `
webUI:
  enabled: true
organizer:
  - target: /photos/organized
    sources:
      - path: /photos/incoming
        mode: copy
        schedule: "0 2 * * *"
`
	cfg := loadFromString(t, content, ".yaml")
	if len(cfg.Organizer) != 1 {
		t.Fatalf("expected 1 organizer profile, got %d", len(cfg.Organizer))
	}
	if cfg.Organizer[0].Target != "/photos/organized" {
		t.Errorf("unexpected target: %s", cfg.Organizer[0].Target)
	}
	if cfg.WebUI == nil || cfg.WebUI.Address != DefaultWebUIAddress {
		t.Fatalf("expected default web UI address %q, got %#v", DefaultWebUIAddress, cfg.WebUI)
	}
	if cfg.Organizer[0].TargetStructure != "{year}/{month}" {
		t.Errorf("expected default targetStructure, got: %s", cfg.Organizer[0].TargetStructure)
	}
}

// Spec: CFG-001
func TestLoadJSON(t *testing.T) {
	content := `{
		"organizer": [{
			"target": "/photos/organized",
			"sources": [{
				"path": "/photos/incoming",
				"mode": "move",
				"schedule": "0 3 * * *"
			}]
		}]
	}`
	cfg := loadFromString(t, content, ".json")
	if len(cfg.Organizer) != 1 {
		t.Fatalf("expected 1 organizer profile, got %d", len(cfg.Organizer))
	}
	if cfg.Organizer[0].Sources[0].Mode != "move" {
		t.Errorf("unexpected mode: %s", cfg.Organizer[0].Sources[0].Mode)
	}
}

// Spec: CFG-001
func TestLoadUnsupportedExtension(t *testing.T) {
	tmp := writeTemp(t, "config.txt", "foo")
	_, err := Load(tmp)
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

// Spec: CFG-002
func TestValidateValid(t *testing.T) {
	cfg := &Config{
		Organizer: []OrganizerProfile{
			{
				Target:          "/target",
				TargetStructure: "{year}/{month}",
				Sources: []OrganizerSource{
					{Path: "/src", Mode: "copy", Schedule: "0 2 * * *"},
				},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// Spec: CFG-002
func TestValidateMissingTarget(t *testing.T) {
	cfg := &Config{
		Organizer: []OrganizerProfile{
			{Sources: []OrganizerSource{{Path: "/src", Mode: "copy", Schedule: "0 2 * * *"}}},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for missing target")
	}
}

// Spec: CFG-002
func TestValidateInvalidMode(t *testing.T) {
	cfg := &Config{
		Organizer: []OrganizerProfile{
			{
				Target: "/target",
				Sources: []OrganizerSource{
					{Path: "/src", Mode: "invalid", Schedule: "0 2 * * *"},
				},
			},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for invalid mode")
	}
}

// Spec: CFG-002
func TestValidateInvalidCron(t *testing.T) {
	cfg := &Config{
		Organizer: []OrganizerProfile{
			{
				Target: "/target",
				Sources: []OrganizerSource{
					{Path: "/src", Mode: "copy", Schedule: "not-a-cron"},
				},
			},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for invalid cron")
	}
}

// Spec: CFG-002, ORG-009
func TestValidateInvalidTargetToken(t *testing.T) {
	cfg := &Config{
		Organizer: []OrganizerProfile{
			{
				Target:          "/target",
				TargetStructure: "{year}/{invalid}",
				Sources: []OrganizerSource{
					{Path: "/src", Mode: "copy", Schedule: "0 2 * * *"},
				},
			},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for invalid target token")
	}
}

// Spec: CFG-002, DUP-005
func TestValidateMoveToFolderRequiresDuplicateFolder(t *testing.T) {
	cfg := &Config{
		Organizer: []OrganizerProfile{
			{
				Target: "/target",
				Sources: []OrganizerSource{
					{
						Path: "/src", Mode: "copy", Schedule: "0 2 * * *",
						Duplicates: &OrganizerDuplicate{
							DetectionMode: "filename",
							Action:        DuplicateAction{Method: "moveToFolder"},
						},
					},
				},
			},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error when moveToFolder has no duplicateFolder")
	}
}

// Spec: CFG-002, ORG-011
func TestValidateMixedOperators(t *testing.T) {
	cfg := &Config{
		Organizer: []OrganizerProfile{
			{
				Target: "/target",
				Sources: []OrganizerSource{
					{
						Path: "/src", Mode: "copy", Schedule: "0 2 * * *",
						Duplicates: &OrganizerDuplicate{
							DetectionMode: "filename+exif|filedate",
							Action:        DuplicateAction{Method: "skip"},
						},
					},
				},
			},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for mixed operators")
	}
}

// Spec: CFG-002, DUP-001
func TestValidateDuplicateDetection(t *testing.T) {
	cfg := &Config{
		DuplicateDetection: []DuplicateDetection{
			{
				Enabled:       true,
				Schedule:      "0 4 * * *",
				Sources:       []string{"/photos1"},
				DetectionMode: "content",
				Action:        DuplicateAction{Method: "skip"},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// Spec: CFG-002, CFG-009
func TestValidateEmptyConfig(t *testing.T) {
	cfg := &Config{}
	if err := Validate(cfg); err != nil {
		t.Fatalf("a config without jobs must be valid (setup mode), got %v", err)
	}
	if cfg.HasJobs() {
		t.Fatal("an empty config has no jobs")
	}
}

// Spec: CFG-002
func TestValidateWebUIRequiresAddressWhenEnabled(t *testing.T) {
	cfg := &Config{
		WebUI: &WebUIConfig{Enabled: true, Address: "invalid"},
		Organizer: []OrganizerProfile{{
			Target:  "/target",
			Sources: []OrganizerSource{{Path: "/src", Mode: "copy", Schedule: "0 2 * * *"}},
		}},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for invalid web UI address")
	}
}

// Spec: CFG-002
func TestValidateProblemsReturnsIndividualMessages(t *testing.T) {
	cfg := &Config{
		WebUI: &WebUIConfig{Enabled: true, Address: "invalid"},
		Organizer: []OrganizerProfile{{
			Target:  "/target",
			Sources: []OrganizerSource{{Path: "/src", Mode: "copy", Schedule: "0 2 * * *"}},
		}},
	}
	problems := ValidateProblems(cfg)
	if len(problems) == 0 {
		t.Fatal("expected at least one validation problem")
	}
	if !strings.Contains(problems[0], "webUI.address") {
		t.Fatalf("expected problem to reference webUI.address, got %q", problems[0])
	}
}

// Spec: CFG-003
func TestResolveConfigPath_CLI(t *testing.T) {
	tmp := writeTemp(t, "config.yaml", "organizer: []")
	if path := ResolveConfigPath(tmp); path != tmp {
		t.Errorf("expected %s, got %s", tmp, path)
	}
}

// Spec: CFG-003
func TestResolveConfigPath_Env(t *testing.T) {
	tmp := writeTemp(t, "config.yaml", "organizer: []")
	t.Setenv(envConfigPath, tmp)
	if path := ResolveConfigPath(""); path != tmp {
		t.Errorf("expected %s, got %s", tmp, path)
	}
}

// Spec: CFG-003, CFG-009
func TestResolveConfigPath_NotFound(t *testing.T) {
	t.Setenv(envConfigPath, "")
	t.Chdir(t.TempDir()) // no config.yaml here

	path := ResolveConfigPath("")
	if path != defaultConfigFile {
		t.Fatalf("expected the default file %s, got %s", defaultConfigFile, path)
	}
	cfg, exists, err := LoadOrDefault(path)
	if err != nil || exists {
		t.Fatalf("missing file: exists=%v err=%v", exists, err)
	}
	if cfg.HasJobs() || cfg.WebUI == nil || !cfg.WebUI.Enabled || cfg.WebUI.Address != DefaultWebUIAddress || cfg.StateDir != "/data/state" {
		t.Fatalf("unexpected default config %+v / %+v", cfg, cfg.WebUI)
	}
}

// Spec: CFG-009
func TestLoadOrDefault(t *testing.T) {
	existing := writeTemp(t, "config.yaml", "stateDir: /tmp/x\n")
	cfg, exists, err := LoadOrDefault(existing)
	if err != nil || !exists || cfg.StateDir != "/tmp/x" {
		t.Fatalf("existing file: cfg=%+v exists=%v err=%v", cfg, exists, err)
	}
	if _, _, err := LoadOrDefault(filepath.Join(t.TempDir(), "config.toml")); err == nil {
		t.Fatal("a missing file with an unsupported extension must fail")
	}
	broken := writeTemp(t, "broken.yaml", "organizer: [")
	if _, _, err := LoadOrDefault(broken); err == nil {
		t.Fatal("an existing broken file must fail")
	}
}

// Spec: CFG-004
func TestSaveAndLoadYAMLRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := &Config{
		StateDir: "/tmp/state",
		WebUI:    &WebUIConfig{Enabled: true, Address: "127.0.0.1:9090"},
		Organizer: []OrganizerProfile{{
			DryRun:          true,
			Target:          "/target",
			TargetStructure: "{year}/{Monat}",
			FallbackFolder:  "/fallback",
			Sources: []OrganizerSource{{
				Path:     "/src",
				Mode:     "copy",
				Schedule: "0 2 * * *",
			}},
		}},
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.WebUI == nil || !loaded.WebUI.Enabled || loaded.WebUI.Address != "127.0.0.1:9090" {
		t.Fatalf("unexpected web UI config after roundtrip: %#v", loaded.WebUI)
	}
	if loaded.Organizer[0].Target != "/target" {
		t.Fatalf("unexpected organizer target after roundtrip: %s", loaded.Organizer[0].Target)
	}
}

// Spec: CFG-004
func TestSaveAndLoadJSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := &Config{
		Organizer: []OrganizerProfile{{
			Target:  "/target",
			Sources: []OrganizerSource{{Path: "/src", Mode: "move", Schedule: "0 3 * * *"}},
		}},
		DuplicateDetection: []DuplicateDetection{{
			Enabled:       true,
			Schedule:      "0 4 * * *",
			Sources:       []string{"/photos1"},
			DetectionMode: "content",
			Action:        DuplicateAction{Method: "skip"},
		}},
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.DuplicateDetection) != 1 || loaded.DuplicateDetection[0].DetectionMode != "content" {
		t.Fatalf("unexpected duplicate detection after roundtrip: %#v", loaded.DuplicateDetection)
	}
}

// --- helpers ---

func loadFromString(t *testing.T, content string, ext string) *Config {
	t.Helper()
	tmp := writeTemp(t, "config"+ext, content)
	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	return cfg
}

func writeTemp(t *testing.T, name string, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}
