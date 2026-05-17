package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewState(t *testing.T) {
	s := New()
	if s.ProcessedFiles == nil {
		t.Fatal("ProcessedFiles should be initialized")
	}
	if len(s.ProcessedFiles) != 0 {
		t.Error("ProcessedFiles should be empty")
	}
}

func TestMarkAndCheck(t *testing.T) {
	s := New()
	mtime := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)

	s.MarkProcessed("/photos/a.jpg", 1024, mtime)

	if !s.IsProcessed("/photos/a.jpg", 1024, mtime) {
		t.Error("file should be marked as processed")
	}

	// Different size
	if s.IsProcessed("/photos/a.jpg", 2048, mtime) {
		t.Error("different size should not match")
	}

	// Different mtime
	if s.IsProcessed("/photos/a.jpg", 1024, mtime.Add(time.Second)) {
		t.Error("different mtime should not match")
	}

	// Different path
	if s.IsProcessed("/photos/b.jpg", 1024, mtime) {
		t.Error("different path should not match")
	}
}

func TestSaveAndLoad(t *testing.T) {
	s := New()
	mtime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	s.MarkProcessed("/photos/test.jpg", 512, mtime)

	path := filepath.Join(t.TempDir(), "state.json")

	if err := Save(s, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := Load(path)
	if !loaded.IsProcessed("/photos/test.jpg", 512, mtime) {
		t.Error("loaded state should contain the processed file")
	}
	if loaded.LastRun.IsZero() {
		t.Error("LastRun should be set after save")
	}
}

func TestLoadMissing(t *testing.T) {
	s := Load("/nonexistent/state.json")
	if len(s.ProcessedFiles) != 0 {
		t.Error("missing file should return empty state")
	}
}

func TestLoadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	os.WriteFile(path, []byte("not json{{{"), 0644)

	s := Load(path)
	if len(s.ProcessedFiles) != 0 {
		t.Error("corrupt file should return empty state")
	}
}

func TestSaveAtomicity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "state.json")

	s := New()
	if err := Save(s, path); err != nil {
		t.Fatalf("Save to nested path failed: %v", err)
	}

	// No temp file should remain
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if e.Name() == "state.json.tmp" {
			t.Error("temp file left behind")
		}
	}
}

