package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// FileEntry represents a processed file's identity for resume tracking.
type FileEntry struct {
	Path  string    `json:"path"`
	Size  int64     `json:"size"`
	Mtime time.Time `json:"mtime"`
}

// State holds the processing state for a single source or job.
type State struct {
	ProcessedFiles map[string]FileEntry `json:"processedFiles"` // keyed by path
	LastRun        time.Time            `json:"lastRun"`
}

// New creates a new empty state.
func New() *State {
	return &State{
		ProcessedFiles: make(map[string]FileEntry),
	}
}

// IsProcessed checks if a file has already been processed (by path + size + mtime).
func (s *State) IsProcessed(path string, size int64, mtime time.Time) bool {
	entry, ok := s.ProcessedFiles[path]
	if !ok {
		return false
	}
	return entry.Size == size && entry.Mtime.Equal(mtime)
}

// MarkProcessed records a file as successfully processed.
func (s *State) MarkProcessed(path string, size int64, mtime time.Time) {
	s.ProcessedFiles[path] = FileEntry{
		Path:  path,
		Size:  size,
		Mtime: mtime,
	}
}

// Load reads state from a JSON file. A missing file gives a fresh state; a
// corrupt one too, with a warning (ORG-015). A file that exists but cannot
// be read (e.g. it belongs to another user) is an error: starting fresh
// would transfer every file of the source again, and in copy mode on
// every run (ORG-015).
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return New(), nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	s := New()
	if err := json.Unmarshal(data, s); err != nil {
		slog.Warn("state file corrupt, starting fresh scan", "path", path, "error", err)
		return New(), nil
	}

	if s.ProcessedFiles == nil {
		s.ProcessedFiles = make(map[string]FileEntry)
	}

	return s, nil
}

// Save writes state to a JSON file atomically (temp file + rename).
func Save(s *State, path string) error {
	s.LastRun = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming state file: %w", err)
	}

	return nil
}

