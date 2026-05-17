package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	content := `
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
	if cfg.Organizer[0].TargetStructure != "{year}/{month}" {
		t.Errorf("expected default targetStructure, got: %s", cfg.Organizer[0].TargetStructure)
	}
}

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

func TestLoadUnsupportedExtension(t *testing.T) {
	tmp := writeTemp(t, "config.txt", "foo")
	_, err := Load(tmp)
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

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

func TestValidateEmptyConfig(t *testing.T) {
	cfg := &Config{}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for empty config")
	}
}

func TestResolveConfigPath_CLI(t *testing.T) {
	tmp := writeTemp(t, "config.yaml", "organizer: []")
	path, err := ResolveConfigPath(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != tmp {
		t.Errorf("expected %s, got %s", tmp, path)
	}
}

func TestResolveConfigPath_Env(t *testing.T) {
	tmp := writeTemp(t, "config.yaml", "organizer: []")
	t.Setenv(envConfigPath, tmp)
	path, err := ResolveConfigPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != tmp {
		t.Errorf("expected %s, got %s", tmp, path)
	}
}

func TestResolveConfigPath_NotFound(t *testing.T) {
	t.Setenv(envConfigPath, "")
	// Change to a temp dir without config.yaml
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer os.Chdir(oldWd)

	_, err := ResolveConfigPath("")
	if err == nil {
		t.Fatal("expected error when no config file found")
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

