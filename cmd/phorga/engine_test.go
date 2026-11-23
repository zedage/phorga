package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/runstatus"
	"github.com/zedage/phorga/internal/scheduler"
)

// startEngine schedules cfg and saves nothing yet; the config file lives at
// a path that does not exist, as in setup mode.
func startEngine(t *testing.T, cfg *config.Config) (*engine, *scheduler.Scheduler, *runstatus.Recorder, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config", "config.yaml")
	sched := scheduler.New()
	recorder := runstatus.Load(runstatus.Path(cfg.StateDir))
	e, err := newEngine(path, cfg, sched, recorder)
	if err != nil {
		t.Fatal(err)
	}
	sched.Start()
	t.Cleanup(sched.Stop)
	return e, sched, recorder, path
}

func jobNames(jobs []jobInfo) []string {
	var names []string
	for _, j := range jobs {
		names = append(names, j.name)
	}
	return names
}

// Spec: CFG-008, CFG-009
func TestEngineApplySavesAndSchedulesJobs(t *testing.T) {
	empty := &config.Config{StateDir: filepath.Join(t.TempDir(), "state")}
	e, sched, _, path := startEngine(t, empty)
	if len(e.Jobs()) != 0 {
		t.Fatal("setup mode starts without jobs")
	}

	next := testConfig(t)
	next.StateDir = empty.StateDir
	next.DuplicateDetection = []config.DuplicateDetection{{
		Enabled: true, Schedule: "0 0 1 1 *", Sources: []string{t.TempDir()},
		DetectionMode: "metadata", Action: config.DuplicateAction{Method: "skip"},
	}}
	applied, restart, err := e.Apply(next)
	if err != nil {
		t.Fatal(err)
	}
	if len(restart) != 0 {
		t.Errorf("nothing needs a restart, got %v", restart)
	}
	if got := jobNames(e.Jobs()); !slices.Equal(got, []string{"organizer-0-source-0", "dedup-0"}) {
		t.Fatalf("jobs = %v", got)
	}
	for _, name := range jobNames(e.Jobs()) {
		if sched.Next(name).IsZero() {
			t.Errorf("%s is not scheduled", name)
		}
	}
	// The file was created (with its folder) and holds the new config.
	saved, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Organizer[0].Target != next.Organizer[0].Target || applied.Organizer[0].Target != next.Organizer[0].Target {
		t.Fatalf("saved %+v, applied %+v", saved.Organizer, applied.Organizer)
	}

	// Removing the dedup job unschedules it.
	next.DuplicateDetection = nil
	if _, _, err := e.Apply(next); err != nil {
		t.Fatal(err)
	}
	if got := jobNames(e.Jobs()); !slices.Equal(got, []string{"organizer-0-source-0"}) {
		t.Fatalf("jobs after removal = %v", got)
	}
	if !sched.Next("dedup-0").IsZero() {
		t.Fatal("the removed job must not be scheduled any more")
	}
}

// Spec: CFG-008
func TestEngineApplyRejectsInvalidConfig(t *testing.T) {
	cfg := testConfig(t)
	e, _, _, path := startEngine(t, cfg)
	before := e.Jobs()

	bad := testConfig(t)
	bad.Organizer[0].Target = ""
	if _, _, err := e.Apply(bad); err == nil {
		t.Fatal("expected a validation error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("an invalid config must not be written")
	}
	if got := e.Jobs(); !slices.Equal(jobNames(got), jobNames(before)) || got[0].label != before[0].label {
		t.Fatal("an invalid config must not change the jobs")
	}
}

// Spec: CFG-008
func TestEngineApplyForgetsResultsOfChangedJobs(t *testing.T) {
	cfg := testConfig(t)
	cfg.DuplicateDetection = []config.DuplicateDetection{{
		Enabled: true, Schedule: "0 0 1 1 *", Sources: []string{t.TempDir()},
		DetectionMode: "metadata", Action: config.DuplicateAction{Method: "skip"},
	}}
	e, _, recorder, _ := startEngine(t, cfg)
	rec := runstatus.Record{Start: time.Now(), Counters: map[string]int{}}
	recorder.Finish("organizer-0-source-0", rec)
	recorder.Finish("dedup-0", rec)

	// The organizer now reads another folder; the dedup job only gets a new
	// schedule and stays the same job.
	next := *cfg
	next.Organizer = []config.OrganizerProfile{cfg.Organizer[0]}
	next.Organizer[0].Sources = []config.OrganizerSource{cfg.Organizer[0].Sources[0]}
	next.Organizer[0].Sources[0].Path = t.TempDir()
	next.DuplicateDetection = []config.DuplicateDetection{cfg.DuplicateDetection[0]}
	next.DuplicateDetection[0].Schedule = "0 0 2 1 *"
	if _, _, err := e.Apply(&next); err != nil {
		t.Fatal(err)
	}

	if _, ok := recorder.Last("organizer-0-source-0"); ok {
		t.Error("the result of the old organizer folder must be dropped")
	}
	if _, ok := recorder.Last("dedup-0"); !ok {
		t.Error("the dedup job is unchanged apart from its schedule and keeps its result")
	}
}

// Spec: CFG-008
func TestEngineApplyReportsRestartFields(t *testing.T) {
	cfg := testConfig(t)
	cfg.WebUI = &config.WebUIConfig{Enabled: true, Address: "127.0.0.1:8080"}
	e, _, _, _ := startEngine(t, cfg)

	next := *cfg
	next.StateDir = filepath.Join(t.TempDir(), "other-state")
	next.WebUI = &config.WebUIConfig{Enabled: false, Address: "127.0.0.1:9090"}
	_, restart, err := e.Apply(&next)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(restart, []string{"stateDir", "webUI.enabled", "webUI.address"}) {
		t.Fatalf("restart fields = %v", restart)
	}
	// Until the restart, jobs keep the state directory they started with.
	if got := e.jobConfig(&next).StateDir; got != cfg.StateDir {
		t.Fatalf("jobs use state dir %q, want %q", got, cfg.StateDir)
	}
}

// Spec: WEB-018, CFG-008
func TestEngineApplyRawKeepsCommentsAndSchedules(t *testing.T) {
	cfg := testConfig(t)
	e, sched, _, path := startEngine(t, cfg)

	text := "# written by hand\nstateDir: " + cfg.StateDir + "\norganizer:\n  - name: Phone # nightly\n    target: " + cfg.Organizer[0].Target + "\n    sources:\n      - path: " + cfg.Organizer[0].Sources[0].Path + "\n        mode: copy\n        schedule: \"0 0 2 1 *\"\n"
	applied, _, err := e.ApplyRaw([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	written, _ := os.ReadFile(path)
	if string(written) != text {
		t.Fatalf("the file must be the text as edited, got %q", written)
	}
	if applied.Organizer[0].Name != "Phone" || e.Jobs()[0].title != "Phone" || e.Jobs()[0].schedule != "0 0 2 1 *" {
		t.Fatalf("not applied: %+v / %+v", applied.Organizer[0], e.Jobs()[0])
	}
	if sched.Next("organizer-0-source-0").IsZero() {
		t.Fatal("the job must be scheduled")
	}

	if _, _, err := e.ApplyRaw([]byte("organizer: [")); err == nil {
		t.Fatal("broken text must fail")
	}
	if written2, _ := os.ReadFile(path); string(written2) != text {
		t.Fatal("broken text must not be written")
	}
}
