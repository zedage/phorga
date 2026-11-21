package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Spec: CFG-001, ORG-005, WEB-006
func TestLoadAppliesDefaults(t *testing.T) {
	path := writeTemp(t, "config.yaml", `
webUI:
  enabled: true
organizer:
  - target: /photos/organized
    sources:
      - path: /photos/incoming
        mode: copy
        schedule: "0 2 * * *"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StateDir != "/data/state" {
		t.Errorf("stateDir = %q, want /data/state", cfg.StateDir)
	}
	if got := cfg.Organizer[0].TargetStructure; got != "{year}/{month}" {
		t.Errorf("targetStructure = %q, want {year}/{month}", got)
	}
	if cfg.WebUI.Address != "127.0.0.1:8080" {
		t.Errorf("webUI.address = %q, want loopback default 127.0.0.1:8080", cfg.WebUI.Address)
	}
}

// Spec: CFG-003
func TestResolveConfigPath_DefaultFileInWorkingDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("organizer: []"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv(envConfigPath, "")

	if path := ResolveConfigPath(""); path != "config.yaml" {
		t.Errorf("path = %q, want config.yaml", path)
	}
}
