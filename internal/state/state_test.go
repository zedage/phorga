package state

import (
	"os"
	"path/filepath"
	"runtime"
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

// Spec: ORG-015
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

// Spec: ORG-015
func TestSaveAndLoad(t *testing.T) {
	s := New()
	mtime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	s.MarkProcessed("/photos/test.jpg", 512, mtime)

	path := filepath.Join(t.TempDir(), "state.json")

	if err := Save(s, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.IsProcessed("/photos/test.jpg", 512, mtime) {
		t.Error("loaded state should contain the processed file")
	}
	if loaded.LastRun.IsZero() {
		t.Error("LastRun should be set after save")
	}
}

// Spec: ORG-015
func TestLoadMissing(t *testing.T) {
	s, err := Load("/nonexistent/state.json")
	if err != nil {
		t.Fatalf("a missing file is no error: %v", err)
	}
	if len(s.ProcessedFiles) != 0 {
		t.Error("missing file should return empty state")
	}
}

// Spec: ORG-015
func TestLoadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	os.WriteFile(path, []byte("not json{{{"), 0644)

	s, err := Load(path)
	if err != nil {
		t.Fatalf("a corrupt file is no error: %v", err)
	}
	if len(s.ProcessedFiles) != 0 {
		t.Error("corrupt file should return empty state")
	}
}

// Spec: ORG-015
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


// Spec: ORG-015
func TestLoadUnreadableIsAnError(t *testing.T) {
	skipWithoutPermissions(t)
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(New(), path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0644) })

	s, err := Load(path)
	if err == nil {
		t.Fatal("a state file that exists but cannot be read must be an error, not a fresh state")
	}
	if s != nil {
		t.Error("no state must be returned with the error")
	}
}

// skipWithoutPermissions skips tests that need file permissions to deny
// access: on Windows they do not, and root ignores them.
func skipWithoutPermissions(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("file permissions do not deny reading on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
}
