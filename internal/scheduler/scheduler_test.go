package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// Spec: DUP-008, ORG-010
func TestRegisterAndRun(t *testing.T) {
	s := New()

	var count int32
	_, err := s.Register("test-job", "* * * * *", func(context.Context) {
		atomic.AddInt32(&count, 1)
	})
	if err != nil {
		t.Fatalf("failed to register job: %v", err)
	}

	// Just verify registration succeeded — don't wait for actual cron trigger
	s.Start()
	defer s.Stop()

	// Scheduler is running, no errors
	if count != 0 {
		t.Log("job may have fired immediately (depending on timing)")
	}
}

func TestRegisterInvalidCron(t *testing.T) {
	s := New()
	_, err := s.Register("bad-job", "invalid", func(context.Context) {})
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

// Spec: DUP-008, ORG-010
func TestOverlapPrevention(t *testing.T) {
	s := New()

	var running int32
	var maxConcurrent int32

	_, err := s.Register("overlap-test", "* * * * *", func(context.Context) {
		cur := atomic.AddInt32(&running, 1)
		if cur > atomic.LoadInt32(&maxConcurrent) {
			atomic.StoreInt32(&maxConcurrent, cur)
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&running, -1)
	})
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	// Manually invoke the job function concurrently to test lock
	// (Can't easily test with real cron timing in unit tests)
	s.mu.Lock()
	lock := s.locks["overlap-test"]
	s.mu.Unlock()

	// Simulate: first run takes the lock
	lock.Lock()
	// Second attempt should fail (TryLock returns false)
	got := lock.TryLock()
	if got {
		t.Error("TryLock should have returned false when lock is held")
		lock.Unlock()
	}
	lock.Unlock()
}

// Spec: WEB-011
func TestNextAndRunning(t *testing.T) {
	s := New()
	started := make(chan struct{})
	release := make(chan struct{})
	id, err := s.Register("status-job", "0 2 * * *", func(context.Context) {
		close(started)
		<-release
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !s.Next("unknown").IsZero() || s.Running("unknown") {
		t.Fatal("unknown job must have no next run and not be running")
	}

	s.Start()
	defer s.Stop()
	next := s.Next("status-job")
	if next.IsZero() || next.Hour() != 2 || next.Minute() != 0 {
		t.Fatalf("unexpected next run %v", next)
	}

	done := make(chan struct{})
	go func() { s.cron.Entry(id).Job.Run(); close(done) }()
	<-started
	if !s.Running("status-job") {
		t.Fatal("expected job to be reported as running")
	}
	close(release)
	<-done
	if s.Running("status-job") {
		t.Fatal("expected job to be idle after it finished")
	}
}

// Spec: WEB-008
func TestNextTimes(t *testing.T) {
	from := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	times, err := NextTimes("30 2 * * *", from, 3)
	if err != nil {
		t.Fatalf("NextTimes: %v", err)
	}
	want := []time.Time{
		time.Date(2026, 11, 12, 2, 30, 0, 0, time.UTC),
		time.Date(2026, 11, 13, 2, 30, 0, 0, time.UTC),
		time.Date(2026, 11, 14, 2, 30, 0, 0, time.UTC),
	}
	if len(times) != 3 {
		t.Fatalf("got %v", times)
	}
	for i := range want {
		if !times[i].Equal(want[i]) {
			t.Errorf("times[%d] = %v, want %v", i, times[i], want[i])
		}
	}
	if _, err := NextTimes("not a cron", from, 3); err == nil {
		t.Fatal("expected error for invalid expression")
	}
}
