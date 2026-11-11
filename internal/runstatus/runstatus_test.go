package runstatus

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Spec: WEB-011
func TestFinishPersistsAndReloads(t *testing.T) {
	path := Path(t.TempDir())
	r := Load(path)
	start := time.Date(2026, 11, 11, 2, 0, 0, 0, time.UTC)
	r.Finish("organizer-0-source-0", Record{Start: start, Duration: 3 * time.Second, Counters: map[string]int{"processed": 7}})

	reloaded := Load(path)
	rec, ok := reloaded.Last("organizer-0-source-0")
	if !ok {
		t.Fatal("expected record after reload")
	}
	if !rec.Start.Equal(start) || rec.Duration != 3*time.Second || rec.Counters["processed"] != 7 {
		t.Fatalf("unexpected record %+v", rec)
	}
}

// Spec: WEB-011
func TestDryRunIsNotWritten(t *testing.T) {
	path := Path(t.TempDir())
	r := Load(path)
	r.Finish("dedup-0", Record{Start: time.Now(), DryRun: true, Counters: map[string]int{"duplicates": 2}})

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry run must not create %s (err=%v)", path, err)
	}
	if rec, ok := r.Last("dedup-0"); !ok || !rec.DryRun {
		t.Fatalf("dry-run record must be kept in memory, got %+v ok=%v", rec, ok)
	}
}

// Spec: WEB-011
func TestWriteErrorDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The parent of the status file is a regular file, so writing must fail.
	r := Load(filepath.Join(blocker, FileName))
	r.Finish("job", Record{Start: time.Now()})
	if _, ok := r.Last("job"); !ok {
		t.Fatal("record must still be kept in memory")
	}
}
