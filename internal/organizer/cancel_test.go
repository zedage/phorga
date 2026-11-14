package organizer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/state"
)

// Spec: ORG-021
func TestOrganize_StopsWhenCancelled(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	for i := 0; i < 5; i++ {
		createDatedFile(t, srcDir, fmt.Sprintf("photo%d.jpg", i), "data", july2019)
	}

	// Cancel right after the first file was processed and saved.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	origSave, origInterval := saveState, stateSaveInterval
	saveState = func(s *state.State, path string) error {
		cancel()
		return origSave(s, path)
	}
	stateSaveInterval = 1
	t.Cleanup(func() { saveState, stateSaveInterval = origSave, origInterval })

	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}"}
	source := config.OrganizerSource{Path: srcDir, Mode: "move", Schedule: "0 2 * * *"}
	sp := filepath.Join(stateDir, "state.json")
	result := Run(ctx, profile, source, state.New(), sp)

	if !result.Canceled || result.Processed != 1 || result.Errors != 0 {
		t.Fatalf("result = %+v, want canceled after 1 processed file", result)
	}
	left, _ := os.ReadDir(srcDir)
	if len(left) != 4 {
		t.Errorf("source has %d files, want the 4 untouched files", len(left))
	}
	if got := len(state.Load(sp).ProcessedFiles); got != 1 {
		t.Errorf("saved state has %d files, want 1", got)
	}
}

// Spec: ORG-021
func TestOrganize_CancelledBeforeStartChangesNothing(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	createDatedFile(t, srcDir, "photo.jpg", "data", july2019)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}"}
	source := config.OrganizerSource{Path: srcDir, Mode: "move", Schedule: "0 2 * * *"}
	result := Run(ctx, profile, source, state.New(), filepath.Join(stateDir, "state.json"))

	if !result.Canceled || result.Processed != 0 {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "photo.jpg")); err != nil {
		t.Errorf("source file must stay in place: %v", err)
	}
}
