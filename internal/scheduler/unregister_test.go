package scheduler

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
)

// Spec: CFG-008
func TestUnregisterLetsTheRunningJobFinish(t *testing.T) {
	s := New()
	release := make(chan struct{})
	var started, finished, cancelled atomic.Bool
	id, err := s.Register("organizer-0-source-0", "0 2 * * *", func(ctx context.Context) {
		started.Store(true)
		<-release
		cancelled.Store(ctx.Err() != nil)
		finished.Store(true)
	})
	if err != nil {
		t.Fatal(err)
	}
	s.Start()
	defer s.Stop()
	oldJob := s.cron.Entry(id).Job

	done := trigger(oldJob)
	waitFor(t, "job to start", started.Load)

	s.Unregister("organizer-0-source-0")
	if !s.Next("organizer-0-source-0").IsZero() {
		t.Fatal("an unregistered job must have no next run")
	}
	if !s.Running("organizer-0-source-0") {
		t.Fatal("the running job must keep running after Unregister")
	}

	// Registered again under the same name: no overlap with the old run.
	var newRuns atomic.Int32
	newID, err := s.Register("organizer-0-source-0", "0 3 * * *", func(context.Context) { newRuns.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	<-trigger(s.cron.Entry(newID).Job)
	if newRuns.Load() != 0 {
		t.Fatal("the new job must not overlap with the old run of the same name")
	}

	close(release)
	<-done
	if !finished.Load() || cancelled.Load() {
		t.Fatalf("old run: finished=%v cancelled=%v", finished.Load(), cancelled.Load())
	}
	<-trigger(s.cron.Entry(newID).Job)
	if newRuns.Load() != 1 {
		t.Fatalf("the new job must run once the old run is done, ran %d times", newRuns.Load())
	}
	if s.Next("organizer-0-source-0").IsZero() {
		t.Fatal("the new schedule must be active")
	}
}

// Spec: CFG-008
func TestRegisterSameNameReplacesSchedule(t *testing.T) {
	s := New()
	if _, err := s.Register("dedup-0", "0 2 * * *", func(context.Context) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register("dedup-0", "0 3 * * *", func(context.Context) {}); err != nil {
		t.Fatal(err)
	}
	if n := len(s.cron.Entries()); n != 1 {
		t.Fatalf("expected one cron entry, got %d", n)
	}
}

// Spec: CFG-008, RUN-002
func TestWaitingRunOfUnregisteredJobDoesNotStart(t *testing.T) {
	logs := captureLogs(t)
	s := New()
	releaseA := make(chan struct{})
	var aRunning, oldRan atomic.Bool
	idA, _ := s.Register("dedup-0", "0 2 * * *", func(context.Context) {
		aRunning.Store(true)
		<-releaseA
	})
	idB, _ := s.Register("organizer-0-source-0", "0 2 * * *", func(context.Context) { oldRan.Store(true) })
	s.Start()
	defer s.Stop()

	doneA := trigger(s.cron.Entry(idA).Job)
	waitFor(t, "job A to run", aRunning.Load)
	doneB := trigger(s.cron.Entry(idB).Job)
	waitFor(t, "job B to wait", func() bool { return s.Waiting("organizer-0-source-0") })

	// The user removes the job (e.g. deletes the profile) while it waits.
	s.Unregister("organizer-0-source-0")

	close(releaseA)
	<-doneA
	<-doneB
	if oldRan.Load() {
		t.Fatal("a waiting run must not start after its job was unregistered")
	}
	if !strings.Contains(logs.String(), `"msg":"skipping run of a replaced job","job":"organizer-0-source-0"`) {
		t.Errorf("missing skip log line:\n%s", logs.String())
	}
	if s.Running("organizer-0-source-0") || s.Waiting("organizer-0-source-0") {
		t.Error("the skipped run must be neither running nor waiting")
	}
}

// Spec: CFG-008, RUN-002
func TestWaitingRunOfReplacedJobDoesNotStart(t *testing.T) {
	s := New()
	releaseA := make(chan struct{})
	var aRunning, oldRan atomic.Bool
	var newRuns atomic.Int32
	idA, _ := s.Register("dedup-0", "0 2 * * *", func(context.Context) {
		aRunning.Store(true)
		<-releaseA
	})
	idOld, _ := s.Register("organizer-0-source-0", "0 2 * * *", func(context.Context) { oldRan.Store(true) })
	s.Start()
	defer s.Stop()

	doneA := trigger(s.cron.Entry(idA).Job)
	waitFor(t, "job A to run", aRunning.Load)
	doneOld := trigger(s.cron.Entry(idOld).Job)
	waitFor(t, "old job to wait", func() bool { return s.Waiting("organizer-0-source-0") })

	// The user turns on dryRun and saves: same name, new settings.
	idNew, err := s.Register("organizer-0-source-0", "0 2 * * *", func(context.Context) { newRuns.Add(1) })
	if err != nil {
		t.Fatal(err)
	}

	close(releaseA)
	<-doneA
	<-doneOld
	if oldRan.Load() {
		t.Fatal("a waiting run must not start with the settings of a replaced registration")
	}

	<-trigger(s.cron.Entry(idNew).Job)
	if newRuns.Load() != 1 {
		t.Fatalf("the new registration must run normally, ran %d times", newRuns.Load())
	}
}
