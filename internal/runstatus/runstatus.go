// Package runstatus records the result of the last run of each scheduled job
// for the web UI status page (WEB-011). Non-dry-run results are persisted as
// JSON in the state directory; dry-run results are kept in memory only, so a
// dry run never changes a file (ADR-0014).
package runstatus

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zedage/phorga/internal/fileops"
)

// FileName is the name of the status file inside the state directory.
const FileName = "run-status.json"

// Record is the result of one finished run.
type Record struct {
	Start    time.Time      `json:"start"`
	Duration time.Duration  `json:"duration"`
	Counters map[string]int `json:"counters"`
	DryRun   bool           `json:"dryRun"`
}

// Recorder keeps the last record per job name. It is safe for concurrent use.
type Recorder struct {
	mu        sync.Mutex
	path      string
	last      map[string]Record
	persisted map[string]Record
	progress  map[string]Progress // running jobs only, never persisted
}

// Path returns the status file path for a state directory.
func Path(stateDir string) string {
	return filepath.Join(stateDir, FileName)
}

// Load creates a Recorder backed by path and reads existing records from it.
// A missing or unreadable file starts empty; an empty path disables persistence.
func Load(path string) *Recorder {
	r := &Recorder{path: path, last: map[string]Record{}, persisted: map[string]Record{}}
	if path == "" {
		return r
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("reading run status failed", "path", path, "error", err)
		}
		return r
	}
	if err := json.Unmarshal(data, &r.persisted); err != nil {
		slog.Warn("parsing run status failed", "path", path, "error", err)
		r.persisted = map[string]Record{}
		return r
	}
	for name, rec := range r.persisted {
		r.last[name] = rec
	}
	return r
}

// Finish stores the record of a finished run. Non-dry-run records are written
// to disk; a write error is logged and never returned to the job.
func (r *Recorder) Finish(name string, rec Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last[name] = rec
	if rec.DryRun || r.path == "" {
		return
	}
	r.persisted[name] = rec
	r.write()
}

// Forget drops the records of a job, for example because the configuration
// changed and the name now stands for another job (CFG-008).
func (r *Recorder) Forget(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.last, name)
	if _, ok := r.persisted[name]; !ok {
		return
	}
	delete(r.persisted, name)
	if r.path != "" {
		r.write()
	}
}

// write stores the persisted records atomically. The caller holds r.mu.
func (r *Recorder) write() {
	data, err := json.MarshalIndent(r.persisted, "", "  ")
	if err != nil {
		slog.Error("encoding run status failed", "error", err)
		return
	}
	if err := fileops.WriteFileAtomic(r.path, data, 0644); err != nil {
		slog.Error("writing run status failed", "path", r.path, "error", err)
	}
}

// Last returns the most recent record for a job.
func (r *Recorder) Last(name string) (Record, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.last[name]
	return rec, ok
}

// Progress phases reported by running jobs (WEB-014).
const (
	PhaseCounting   = "counting"   // counting the files to work on
	PhaseProcessing = "processing" // organizer: handling files
	PhaseScanning   = "scanning"   // duplicate detection: reading files
	PhaseComparing  = "comparing"  // duplicate detection: comparing files
	PhaseActing     = "acting"     // duplicate detection: applying actions
)

// Progress is what a running job is doing right now. Total is 0 when the
// amount of work is not known yet. It is kept in memory only.
type Progress struct {
	Phase string
	Done  int
	Total int
	Start time.Time // start of the run
}

// SetProgress stores the current progress of a running job.
func (r *Recorder) SetProgress(name string, p Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.progress == nil {
		r.progress = map[string]Progress{}
	}
	r.progress[name] = p
}

// ClearProgress removes the progress of a job once its run has ended.
func (r *Recorder) ClearProgress(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.progress, name)
}

// Progress returns the current progress of a running job.
func (r *Recorder) Progress(name string) (Progress, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.progress[name]
	return p, ok
}

// ReportFunc receives progress updates from a running job.
type ReportFunc func(phase string, done, total int)

type reporterKey struct{}

// WithReporter returns a context whose jobs send their progress to fn.
// Progress travels with the run's context, like its cancellation, so the
// organizer and duplicate detection need no extra parameter.
func WithReporter(ctx context.Context, fn ReportFunc) context.Context {
	return context.WithValue(ctx, reporterKey{}, fn)
}

// Report sends a progress update to the reporter in ctx, if there is one.
// It is cheap enough to call once per file.
func Report(ctx context.Context, phase string, done, total int) {
	if fn, ok := ctx.Value(reporterKey{}).(ReportFunc); ok {
		fn(phase, done, total)
	}
}
