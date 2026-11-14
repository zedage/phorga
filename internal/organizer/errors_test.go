package organizer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/state"
)

// Spec: ORG-017
func TestOrganize_FileErrorDoesNotAbortRun(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)

	// The 2023 file cannot be written: "<target>/2023" is a regular file,
	// so its target directory cannot be created. The 2024 file must still go through.
	if err := os.WriteFile(filepath.Join(tgtDir, "2023"), []byte("blocker"), 0644); err != nil {
		t.Fatal(err)
	}
	createTestFile(t, srcDir, "IMG_20230115_100000.jpg", "a")
	createTestFile(t, srcDir, "IMG_20240115_100000.jpg", "b")

	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}"}
	source := config.OrganizerSource{Path: srcDir, Mode: "copy", Schedule: "0 2 * * *"}

	result := Run(context.Background(), profile, source, state.New(), filepath.Join(stateDir, "state.json"))

	if result.Errors < 1 {
		t.Errorf("errors = %d, want at least 1", result.Errors)
	}
	if result.Processed != 1 {
		t.Errorf("processed = %d, want 1", result.Processed)
	}
	if _, err := os.Stat(filepath.Join(tgtDir, "2024", "01", "IMG_20240115_100000.jpg")); err != nil {
		t.Errorf("2024 file was not organized after the 2023 failure: %v", err)
	}
}
