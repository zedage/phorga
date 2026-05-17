package scheduler

import (
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"
)

// Scheduler wraps robfig/cron with overlap prevention.
type Scheduler struct {
	cron  *cron.Cron
	locks map[string]*sync.Mutex
	mu    sync.Mutex
}

// New creates a new Scheduler with standard 5-field cron parsing.
func New() *Scheduler {
	return &Scheduler{
		cron: cron.New(cron.WithParser(
			cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		)),
		locks: make(map[string]*sync.Mutex),
	}
}

// Register adds a named job with a cron expression. The job will not run
// concurrently with itself — overlapping invocations are skipped and logged.
func (s *Scheduler) Register(name, schedule string, fn func()) (cron.EntryID, error) {
	s.mu.Lock()
	if _, ok := s.locks[name]; !ok {
		s.locks[name] = &sync.Mutex{}
	}
	lock := s.locks[name]
	s.mu.Unlock()

	wrappedFn := func() {
		if !lock.TryLock() {
			slog.Warn("skipping overlapping run", "job", name)
			return
		}
		defer lock.Unlock()

		slog.Info("job starting", "job", name)
		fn()
		slog.Info("job finished", "job", name)
	}

	id, err := s.cron.AddFunc(schedule, wrappedFn)
	if err != nil {
		return 0, err
	}

	slog.Info("job registered", "job", name, "schedule", schedule)
	return id, nil
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

