package scheduler

import (
	"context"
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
