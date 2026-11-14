package organizer

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/runstatus"
	"github.com/zedage/phorga/internal/state"
)

// Spec: WEB-014
func TestOrganize_ReportsProgress(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	for i := 0; i < 3; i++ {
		createDatedFile(t, srcDir, fmt.Sprintf("photo%d.jpg", i), "data", july2019)
	}
	createTestFile(t, srcDir, "notes.txt", "not media")

	type report struct {
		phase       string
		done, total int
	}
	var got []report
	ctx := runstatus.WithReporter(context.Background(), func(phase string, done, total int) {
		got = append(got, report{phase, done, total})
	})

	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}"}
	source := config.OrganizerSource{Path: srcDir, Mode: "copy", Schedule: "0 2 * * *"}
	Run(ctx, profile, source, state.New(), filepath.Join(stateDir, "state.json"))

	want := []report{
		{runstatus.PhaseCounting, 0, 0},
		{runstatus.PhaseProcessing, 1, 3},
		{runstatus.PhaseProcessing, 2, 3},
		{runstatus.PhaseProcessing, 3, 3},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("reports = %v, want %v", got, want)
	}
}
