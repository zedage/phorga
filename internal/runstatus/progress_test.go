package runstatus

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Spec: WEB-014
func TestProgressIsKeptInMemoryOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	r := Load(path)
	start := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)

	if _, ok := r.Progress("dedup-0"); ok {
		t.Fatal("no progress expected before a run")
	}
	r.SetProgress("dedup-0", Progress{Phase: PhaseScanning, Done: 1500, Total: 7402, Start: start})
	p, ok := r.Progress("dedup-0")
	if !ok || p.Phase != PhaseScanning || p.Done != 1500 || p.Total != 7402 || !p.Start.Equal(start) {
		t.Fatalf("unexpected progress %+v, %v", p, ok)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("progress must never be written to disk: %v", err)
	}

	r.ClearProgress("dedup-0")
	if _, ok := r.Progress("dedup-0"); ok {
		t.Error("progress must be gone after the run")
	}
}

// Spec: WEB-014
func TestReportReachesReporterInContext(t *testing.T) {
	// Without a reporter, Report does nothing.
	Report(context.Background(), PhaseScanning, 1, 2)

	var got []string
	ctx := WithReporter(context.Background(), func(phase string, done, total int) {
		got = append(got, phase)
	})
	Report(ctx, PhaseCounting, 0, 0)
	Report(ctx, PhaseScanning, 1, 2)
	if len(got) != 2 || got[0] != PhaseCounting || got[1] != PhaseScanning {
		t.Errorf("reports = %v", got)
	}
}
