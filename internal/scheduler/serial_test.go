package scheduler

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// trigger runs a registered job the way cron would, in the background.
func trigger(id interface{ Run() }) chan struct{} {
	done := make(chan struct{})
	go func() { id.Run(); close(done) }()
	return done
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Spec: RUN-002
func TestJobsRunOneAtATime(t *testing.T) {
	logs := captureLogs(t)
	s := New()
	releaseA := make(chan struct{})
	var aRunning, bStarted atomic.Bool
	idA, _ := s.Register("organizer-0-source-0", "0 2 * * *", func(context.Context) {
		aRunning.Store(true)
		<-releaseA
		aRunning.Store(false)
	})
	idB, _ := s.Register("dedup-0", "0 2 * * *", func(context.Context) {
		if aRunning.Load() {
			t.Error("dedup-0 started while organizer-0-source-0 was running")
		}
		bStarted.Store(true)
	})
	s.Start()
	defer s.Stop()

	doneA := trigger(s.cron.Entry(idA).Job)
	waitFor(t, "job A to run", aRunning.Load)
	doneB := trigger(s.cron.Entry(idB).Job)
	waitFor(t, "job B to wait", func() bool { return s.Waiting("dedup-0") })

	if bStarted.Load() || s.Running("dedup-0") {
		t.Fatal("job B must wait while job A runs")
	}
	if !strings.Contains(logs.String(), `"msg":"waiting for another job to finish","job":"dedup-0","running":["organizer-0-source-0"]`) {
		t.Errorf("missing wait log line:\n%s", logs.String())
	}

	close(releaseA)
	<-doneA
	<-doneB
	if !bStarted.Load() {
		t.Error("job B must run after job A finished")
	}
	if s.Waiting("dedup-0") {
		t.Error("job B must not be waiting any more")
	}
}

// Spec: RUN-002, RUN-001
func TestWaitingJobDoesNotStartAfterStop(t *testing.T) {
	s := New()
	var bStarted atomic.Bool
	finishedA := startJob(t, s, "a", func(ctx context.Context) { <-ctx.Done() })
	idB, _ := s.Register("b", "0 2 * * *", func(context.Context) { bStarted.Store(true) })
	doneB := trigger(s.cron.Entry(idB).Job)
	waitFor(t, "job B to wait", func() bool { return s.Waiting("b") })

	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return")
	}
	<-finishedA
	<-doneB
	if bStarted.Load() {
		t.Error("a job that was waiting must not start after Stop")
	}
}
