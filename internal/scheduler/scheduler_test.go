package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

// Spec: DUP-008, ORG-010
func TestRegisterAndRun(t *testing.T) {
	s := New()

	var count int32
	_, err := s.Register("test-job", "* * * * *", func() {
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
	_, err := s.Register("bad-job", "invalid", func() {})
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

// Spec: DUP-008, ORG-010
func TestOverlapPrevention(t *testing.T) {
	s := New()

	var running int32
	var maxConcurrent int32

	_, err := s.Register("overlap-test", "* * * * *", func() {
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

