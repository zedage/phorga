package webui

import (
	"net/http"
	"time"
)

// JobStatus describes one scheduled job for the status page (WEB-011).
type JobStatus struct {
	Name     string
	Kind     string // "organizer" or "dedup"
	Label    string
	Schedule string
	Next     time.Time
	Running  bool
	// Waiting is true while the job is due but waits for another job to
	// finish (RUN-002).
	Waiting bool

	// Progress of the current run (WEB-014); Phase is empty when unknown.
	Phase string
	Done  int
	Total int
	Since time.Time

	HasLast      bool
	LastStart    time.Time
	LastDuration time.Duration
	Counters     []Counter
	DryRun       bool
}

// Counter is one named result value of a run, e.g. processed=7.
type Counter struct {
	Key   string
	Value int
}

// StatusSource provides the current status of all scheduled jobs.
type StatusSource interface {
	Jobs() []JobStatus
}

type statusData struct {
	Available bool
	Jobs      []JobStatus
}

func (s *Server) statusData() statusData {
	if s.status == nil {
		return statusData{}
	}
	return statusData{Available: true, Jobs: s.status.Jobs()}
}

func (s *Server) handleStatusPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lang := s.resolveLanguage(w, r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.execute(w, "status", lang, s.statusData())
}

func (s *Server) handleStatusTable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lang := s.resolveLanguage(w, r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.execute(w, "statusTable", lang, s.statusData())
}
