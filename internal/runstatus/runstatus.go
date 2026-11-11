// Package runstatus records the result of the last run of each scheduled job
// for the web UI status page (WEB-011). Non-dry-run results are persisted as
// JSON in the state directory; dry-run results are kept in memory only, so a
// dry run never changes a file (ADR-0014).
package runstatus

import (
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
