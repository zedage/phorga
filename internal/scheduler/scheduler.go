package scheduler

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

// parser is the standard 5-field cron parser used for all jobs.
var parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// waitLogInterval is how often Stop repeats which jobs it is still waiting
// for (RUN-001). It is a variable so tests can shorten it.
var waitLogInterval = 10 * time.Second

// Job is the work of a scheduled job. ctx is cancelled when the scheduler
// stops; a job MUST check it between files and return early (RUN-001).
type Job func(ctx context.Context)

// Scheduler wraps robfig/cron with overlap prevention and cancellation.
// Jobs run one at a time: a job whose time comes while another job runs
// waits for it (RUN-002).
type Scheduler struct {
	cron    *cron.Cron
	locks   map[string]*sync.Mutex
	entries map[string]registration
	gen     uint64 // last registration number handed out
	running map[string]*atomic.Bool
	waiting map[string]*atomic.Bool
	mu      sync.Mutex

	// exclusive is held by the one job that runs (RUN-002).
	exclusive chan struct{}

	ctx    context.Context
	cancel context.CancelFunc
	active sync.WaitGroup // jobs that are executing right now
}

// New creates a new Scheduler with standard 5-field cron parsing.
func New() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		cron:    cron.New(cron.WithParser(parser)),
		locks:   make(map[string]*sync.Mutex),
		entries: make(map[string]registration),
		running: make(map[string]*atomic.Bool),
		waiting: make(map[string]*atomic.Bool),

		exclusive: make(chan struct{}, 1),
		ctx:       ctx,
		cancel:    cancel,
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

// registration is one Register call of a job. A run checks that its
// registration is still the one in effect before it starts, so a run that
// waited for another job does not start after its job was unregistered or
// registered again with new settings (CFG-008).
type registration struct {
	id  cron.EntryID
	gen uint64 // unique per Register call
}

// current reports whether gen is the registration of name that is in
// effect right now.
func (s *Scheduler) current(name string, gen uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.entries[name]
	return ok && r.gen == gen
}

// Register adds a named job with a cron expression. The job will not run
// concurrently with itself — overlapping invocations are skipped and logged
// (ORG-010). While another job runs, it waits for that job (RUN-002).
// After Stop was called, the job is not started any more.
func (s *Scheduler) Register(name, schedule string, fn Job) (cron.EntryID, error) {
	s.mu.Lock()
	if _, ok := s.locks[name]; !ok {
		s.locks[name] = &sync.Mutex{}
		s.running[name] = &atomic.Bool{}
		s.waiting[name] = &atomic.Bool{}
	}
	lock := s.locks[name]
	running := s.running[name]
	waiting := s.waiting[name]
	s.gen++
	gen := s.gen
	s.mu.Unlock()

	wrappedFn := func() {
		if s.ctx.Err() != nil {
			return // shutting down
		}
		if !lock.TryLock() {
			slog.Warn("skipping overlapping run", "job", name)
			return
		}
		defer lock.Unlock()
		s.active.Add(1)
		defer s.active.Done()

		if !s.acquire(name, waiting) {
			return // shutting down while waiting
		}
		defer func() { <-s.exclusive }()

		// The job may have been unregistered or replaced while this run
		// waited for the slot; its settings are outdated then (CFG-008).
		if !s.current(name, gen) {
			slog.Info("skipping run of a replaced job", "job", name)
			return
		}

		running.Store(true)
		defer running.Store(false)

		slog.Info("job starting", "job", name)
		fn(s.ctx)
		if s.ctx.Err() != nil {
			slog.Info("job stopped early because of shutdown", "job", name)
			return
		}
		slog.Info("job finished", "job", name)
	}

	id, err := s.cron.AddFunc(schedule, wrappedFn)
	if err != nil {
		return 0, err
	}

	// Registering a name again replaces its schedule (CFG-008).
	s.mu.Lock()
	old, replaced := s.entries[name]
	s.entries[name] = registration{id: id, gen: gen}
	s.mu.Unlock()
	if replaced {
		s.cron.Remove(old.id)
	}

	slog.Info("job registered", "job", name, "schedule", schedule)
	return id, nil
}

// Unregister removes a job's schedule, so it is not started any more. A run
// of that job that is busy right now is not interrupted and finishes with
// the settings it started with; while it runs, a job registered again
// under the same name does not overlap with it. A run that is still waiting
// for another job does not start any more (CFG-008).
func (s *Scheduler) Unregister(name string) {
	s.mu.Lock()
	r, ok := s.entries[name]
	delete(s.entries, name)
	s.mu.Unlock()
	if ok {
		s.cron.Remove(r.id)
		slog.Info("job unregistered", "job", name)
	}
}

// acquire waits until no other job runs (RUN-002). It returns false when
// the scheduler stops first.
func (s *Scheduler) acquire(name string, waiting *atomic.Bool) bool {
	select {
	case s.exclusive <- struct{}{}:
	default:
		slog.Info("waiting for another job to finish", "job", name, "running", s.runningJobs())
		waiting.Store(true)
		defer waiting.Store(false)
		select {
		case s.exclusive <- struct{}{}:
		case <-s.ctx.Done():
			return false
		}
	}
	if s.ctx.Err() != nil {
		<-s.exclusive
		return false
	}
	return true
}

// Next returns the next planned run of the named job, or the zero time if
// the job is unknown or the scheduler has not been started.
func (s *Scheduler) Next(name string) time.Time {
	s.mu.Lock()
	r, ok := s.entries[name]
	s.mu.Unlock()
	if !ok {
		return time.Time{}
	}
	return s.cron.Entry(r.id).Next
}

// Running reports whether the named job is currently executing.
func (s *Scheduler) Running(name string) bool {
	s.mu.Lock()
	r, ok := s.running[name]
	s.mu.Unlock()
	return ok && r.Load()
}

// Waiting reports whether the named job is due but waits for another job
// to finish (RUN-002).
func (s *Scheduler) Waiting(name string) bool {
	s.mu.Lock()
	w, ok := s.waiting[name]
	s.mu.Unlock()
	return ok && w.Load()
}

// runningJobs returns the names of all jobs that are executing right now.
func (s *Scheduler) runningJobs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var names []string
	for name, r := range s.running {
		if r.Load() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// Start begins the scheduler. Non-blocking.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop cancels running jobs, stops starting new ones and waits until the
// running jobs have returned. While it waits, it logs which jobs it is
// waiting for (RUN-001). Calling it again is harmless.
func (s *Scheduler) Stop() {
	// Name the running jobs before cancelling, while they are still running.
	if jobs := s.runningJobs(); len(jobs) > 0 {
		slog.Info("waiting for running jobs to stop", "jobs", jobs)
	}
	s.cancel()
	s.cron.Stop() // no new runs; running ones are awaited below
	done := make(chan struct{})
	go func() {
		s.active.Wait()
		close(done)
	}()
	ticker := time.NewTicker(waitLogInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			slog.Warn("still waiting for running jobs to stop; press Ctrl+C again to quit immediately", "jobs", s.runningJobs())
		}
	}
}
