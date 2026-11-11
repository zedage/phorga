package scheduler

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

// parser is the standard 5-field cron parser used for all jobs.
var parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// Scheduler wraps robfig/cron with overlap prevention.
type Scheduler struct {
	cron    *cron.Cron
	locks   map[string]*sync.Mutex
	entries map[string]cron.EntryID
	running map[string]*atomic.Bool
	mu      sync.Mutex
}

// New creates a new Scheduler with standard 5-field cron parsing.
func New() *Scheduler {
	return &Scheduler{
		cron:    cron.New(cron.WithParser(parser)),
		locks:   make(map[string]*sync.Mutex),
		entries: make(map[string]cron.EntryID),
		running: make(map[string]*atomic.Bool),
	}
}

// Parse parses a 5-field cron expression with the same rules the scheduler uses.
func Parse(expr string) (cron.Schedule, error) {
	return parser.Parse(expr)
}

// NextTimes returns the next n activation times of expr after from.
func NextTimes(expr string, from time.Time, n int) ([]time.Time, error) {
	sched, err := Parse(expr)
	if err != nil {
		return nil, err
	}
	times := make([]time.Time, 0, n)
	t := from
	for i := 0; i < n; i++ {
		t = sched.Next(t)
		if t.IsZero() {
			break
		}
		times = append(times, t)
	}
	return times, nil
}

// Register adds a named job with a cron expression. The job will not run
// concurrently with itself — overlapping invocations are skipped and logged.
func (s *Scheduler) Register(name, schedule string, fn func()) (cron.EntryID, error) {
	s.mu.Lock()
	if _, ok := s.locks[name]; !ok {
		s.locks[name] = &sync.Mutex{}
		s.running[name] = &atomic.Bool{}
	}
	lock := s.locks[name]
	running := s.running[name]
	s.mu.Unlock()

	wrappedFn := func() {
		if !lock.TryLock() {
			slog.Warn("skipping overlapping run", "job", name)
			return
		}
		defer lock.Unlock()
		running.Store(true)
		defer running.Store(false)

		slog.Info("job starting", "job", name)
		fn()
		slog.Info("job finished", "job", name)
	}

	id, err := s.cron.AddFunc(schedule, wrappedFn)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	s.entries[name] = id
	s.mu.Unlock()

	slog.Info("job registered", "job", name, "schedule", schedule)
	return id, nil
}

// Next returns the next planned run of the named job, or the zero time if
// the job is unknown or the scheduler has not been started.
func (s *Scheduler) Next(name string) time.Time {
	s.mu.Lock()
	id, ok := s.entries[name]
	s.mu.Unlock()
	if !ok {
		return time.Time{}
	}
	return s.cron.Entry(id).Next
}

// Running reports whether the named job is currently executing.
func (s *Scheduler) Running(name string) bool {
	s.mu.Lock()
	r, ok := s.running[name]
	s.mu.Unlock()
	return ok && r.Load()
}

// Start begins the scheduler. Non-blocking.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop stops the scheduler and waits for running jobs to finish.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}
