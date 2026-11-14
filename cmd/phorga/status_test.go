package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/organizer"
	"github.com/zedage/phorga/internal/runstatus"
	"github.com/zedage/phorga/internal/scheduler"
)

// Spec: WEB-011
func TestStatusBoardReportsRecordedRun(t *testing.T) {
	cfg := testConfig(t)
	cfg.DuplicateDetection = []config.DuplicateDetection{{
		Enabled: true, DryRun: true, Schedule: "0 0 1 1 *", Sources: []string{t.TempDir()},
		DetectionMode: "metadata", Action: config.DuplicateAction{Method: "skip"},
	}}
	statusPath := runstatus.Path(cfg.StateDir)
	recorder := runstatus.Load(statusPath)
	sched := scheduler.New()
	jobs, err := registerJobs(sched, cfg, recorder)
	if err != nil {
		t.Fatal(err)
	}
	sched.Start()
	defer sched.Stop()
	board := &statusBoard{sched: sched, recorder: recorder, jobs: jobs}

	before := board.Jobs()
	if len(before) != 2 || before[0].HasLast || before[0].Next.IsZero() {
		t.Fatalf("unexpected initial status %+v", before)
	}

	profile, source := cfg.Organizer[0], cfg.Organizer[0].Sources[0]
	organizerJob(jobs[0].name, profile, source, organizer.StatePath(cfg.StateDir, profile.Target, source.Path), recorder)(context.Background())
	dedupJob(jobs[1].name, cfg.DuplicateDetection[0], recorder)(context.Background())

	after := board.Jobs()
	org, dd := after[0], after[1]
	if !org.HasLast || org.DryRun || len(org.Counters) != 3 || org.Counters[0].Key != "processed" {
		t.Fatalf("unexpected organizer status %+v", org)
	}
	if !dd.HasLast || !dd.DryRun || len(dd.Counters) != 4 || dd.Counters[0].Key != "files" {
		t.Fatalf("unexpected dedup status %+v", dd)
	}

	// Only the non-dry organizer run is persisted.
	reloaded := runstatus.Load(statusPath)
	if _, ok := reloaded.Last(jobs[0].name); !ok {
		t.Error("organizer run must be persisted")
	}
	if _, ok := reloaded.Last(jobs[1].name); ok {
		t.Error("dry-run dedup result must not be persisted")
	}
}

// Spec: WEB-011
func TestRun_StatusPageIsServed(t *testing.T) {
	addr := freeAddr(t)
	cfg := testConfig(t)
	cfg.WebUI = &config.WebUIConfig{Enabled: true, Address: addr}
	path := writeConfig(t, cfg)
	a := startRun(t, "--config", path)

	getUI(t, addr) // waits until the UI is up
	resp, err := http.Get("http://" + addr + "/ui/status/table")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, cfg.Organizer[0].Sources[0].Path) || !strings.Contains(body, "not run yet") {
		t.Errorf("status table does not list the organizer job:\n%s", body)
	}
	a.stop()

	if _, err := os.Stat(filepath.Join(cfg.StateDir, runstatus.FileName)); !os.IsNotExist(err) {
		t.Errorf("no run happened, so no status file may exist (err=%v)", err)
	}
}

// Spec: RUN-001
func TestCancelledRunIsNotRecorded(t *testing.T) {
	dir := t.TempDir()
	recorder := runstatus.Load(filepath.Join(dir, "run-status.json"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	profile := config.OrganizerProfile{Target: filepath.Join(dir, "target"), TargetStructure: "{year}/{month}"}
	source := config.OrganizerSource{Path: dir, Mode: "copy", Schedule: "0 2 * * *"}
	organizerJob("org", profile, source, filepath.Join(dir, "state.json"), recorder)(ctx)
	dedupJob("dd", config.DuplicateDetection{
		Sources: []string{dir}, DetectionMode: "content", Action: config.DuplicateAction{Method: "skip"},
	}, recorder)(ctx)

	for _, name := range []string{"org", "dd"} {
		if _, ok := recorder.Last(name); ok {
			t.Errorf("%s: a run stopped by shutdown must not be recorded", name)
		}
	}
}
