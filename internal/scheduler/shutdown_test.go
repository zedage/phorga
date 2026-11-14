package scheduler

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLogs sends slog output to a buffer for the duration of a test.
func captureLogs(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(orig) })
	return buf
}

// startJob runs a registered job the way cron would and waits until it runs.
func startJob(t *testing.T, s *Scheduler, name string, fn Job) (finished chan struct{}) {
	t.Helper()
	started := make(chan struct{})
	finished = make(chan struct{})
	id, err := s.Register(name, "0 2 * * *", func(ctx context.Context) {
		close(started)
		fn(ctx)
	})
	if err != nil {
		t.Fatal(err)
	}
	s.Start()
	go func() { s.cron.Entry(id).Job.Run(); close(finished) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("job did not start")
	}
	return finished
}

// Spec: RUN-001
func TestStopCancelsRunningJob(t *testing.T) {
	logs := captureLogs(t)
	s := New()
	sawCancel := make(chan bool, 1)
	finished := startJob(t, s, "long-job", func(ctx context.Context) {
		select {
		case <-ctx.Done():
			sawCancel <- true
		case <-time.After(10 * time.Second):
			sawCancel <- false
		}
	})

	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return within 2s although the job honors cancellation")
	}
	<-finished
	if !<-sawCancel {
		t.Error("the running job's context was not cancelled")
	}
	out := logs.String()
	if !strings.Contains(out, "waiting for running jobs to stop") || !strings.Contains(out, "long-job") {
		t.Errorf("Stop must log which job it waits for, got:\n%s", out)
	}
	if !strings.Contains(out, "job stopped early because of shutdown") {
		t.Errorf("expected the early stop to be logged, got:\n%s", out)
	}
}

// Spec: RUN-001
func TestStopKeepsLoggingWhileWaiting(t *testing.T) {
	logs := captureLogs(t)
	orig := waitLogInterval
	waitLogInterval = 20 * time.Millisecond
	t.Cleanup(func() { waitLogInterval = orig })

	s := New()
	finished := startJob(t, s, "stubborn-job", func(context.Context) {
		time.Sleep(150 * time.Millisecond) // ignores cancellation
	})
	s.Stop()
	<-finished

	if !strings.Contains(logs.String(), "still waiting for running jobs to stop") {
		t.Errorf("expected repeated waiting messages, got:\n%s", logs.String())
	}
}

// Spec: RUN-001
func TestNoJobStartsAfterStop(t *testing.T) {
	s := New()
	ran := false
	id, err := s.Register("late-job", "0 2 * * *", func(context.Context) { ran = true })
	if err != nil {
		t.Fatal(err)
	}
	s.Start()
	s.Stop()

	s.cron.Entry(id).Job.Run()
	if ran {
		t.Error("a job must not start after Stop")
	}
}

// Spec: RUN-001
func TestStopTwiceIsHarmless(t *testing.T) {
	s := New()
	s.Start()
	s.Stop()
	s.Stop()
}
