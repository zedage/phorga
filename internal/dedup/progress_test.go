package dedup

import (
	"context"
	"testing"

	"github.com/zedage/phorga/internal/runstatus"
)

type report struct {
	phase       string
	done, total int
}

func collectReports(ctx context.Context) (context.Context, *[]report) {
	var got []report
	return runstatus.WithReporter(ctx, func(phase string, done, total int) {
		got = append(got, report{phase, done, total})
	}), &got
}

// last returns the last report of a phase, and whether there was one.
func last(reports []report, phase string) (report, bool) {
	var r report
	found := false
	for _, x := range reports {
		if x.phase == phase {
			r, found = x, true
		}
	}
	return r, found
}

// Spec: WEB-014
func TestDedup_ReportsProgress(t *testing.T) {
	src1, src2 := t.TempDir(), t.TempDir()
	dupDir := t.TempDir()
	createTestImage(t, src1, "a.jpg", 100, 100, 100)
	createTestImage(t, src2, "b.jpg", 100, 100, 100)
	createCheckerImage(t, src2, "c.jpg")
	createTestFile(t, src1, "notes.txt", "not media")

	ctx, got := collectReports(context.Background())
	result := Run(ctx, contentJob("moveToFolder", dupDir, src1, src2))
	if result.ActionsApplied != 1 {
		t.Fatalf("actions = %d, want 1", result.ActionsApplied)
	}

	reports := *got
	if len(reports) == 0 || reports[0].phase != runstatus.PhaseCounting {
		t.Fatalf("a run must start by counting, got %v", reports)
	}
	if r, ok := last(reports, runstatus.PhaseScanning); !ok || r.done != 3 || r.total != 3 {
		t.Errorf("last scanning report = %+v, want 3 of 3", r)
	}
	if r, ok := last(reports, runstatus.PhaseComparing); !ok || r.total != 3 {
		t.Errorf("last comparing report = %+v, want a total of 3", r)
	}
	if r, ok := last(reports, runstatus.PhaseActing); !ok || r.total != 1 {
		t.Errorf("last acting report = %+v, want a total of 1", r)
	}
	order := map[string]int{}
	for i, r := range reports {
		if _, seen := order[r.phase]; !seen {
			order[r.phase] = i
		}
	}
	if !(order[runstatus.PhaseCounting] < order[runstatus.PhaseScanning] &&
		order[runstatus.PhaseScanning] < order[runstatus.PhaseComparing] &&
		order[runstatus.PhaseComparing] < order[runstatus.PhaseActing]) {
		t.Errorf("phases out of order: %v", reports)
	}
}
