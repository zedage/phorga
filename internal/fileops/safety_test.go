package fileops

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sameTime allows for coarse filesystem timestamps.
func sameTime(a, b time.Time) bool {
	d := a.Sub(b)
	return d > -2*time.Second && d < 2*time.Second
}

// Spec: ORG-018
func TestAtomicCopy_KeepsModTime(t *testing.T) {
	src := filepath.Join(t.TempDir(), "old.jpg")
	dst := filepath.Join(t.TempDir(), "old.jpg")
	os.WriteFile(src, []byte("data"), 0644)
	taken := time.Date(2019, 7, 15, 14, 30, 0, 0, time.UTC)
	if err := os.Chtimes(src, taken, taken); err != nil {
		t.Fatal(err)
	}

	if err := AtomicCopy(src, dst); err != nil {
		t.Fatalf("AtomicCopy: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !sameTime(info.ModTime(), taken) {
		t.Errorf("copy mtime = %v, want %v", info.ModTime(), taken)
	}
}

// Spec: ORG-018
func TestAtomicMove_KeepsModTime(t *testing.T) {
	src := filepath.Join(t.TempDir(), "old.mov")
	dst := filepath.Join(t.TempDir(), "sub", "old.mov")
	os.WriteFile(src, []byte("data"), 0644)
	taken := time.Date(2012, 12, 24, 18, 0, 0, 0, time.UTC)
	if err := os.Chtimes(src, taken, taken); err != nil {
		t.Fatal(err)
	}

	if err := AtomicMove(src, dst); err != nil {
		t.Fatalf("AtomicMove: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !sameTime(info.ModTime(), taken) {
		t.Errorf("moved file mtime = %v, want %v", info.ModTime(), taken)
	}
}

// Spec: ORG-019
func TestAtomicMove_IncompleteCopyKeepsSource(t *testing.T) {
	srcDir, dstDir := t.TempDir(), t.TempDir()
	src := filepath.Join(srcDir, "photo.jpg")
	dst := filepath.Join(dstDir, "photo.jpg")
	os.WriteFile(src, []byte("0123456789"), 0644)

	// Simulate a copy that silently stops halfway.
	orig := copyData
	copyData = func(w io.Writer, r io.Reader) (int64, error) { return io.CopyN(w, r, 5) }
	t.Cleanup(func() { copyData = orig })

	err := AtomicMove(src, dst)
	if err == nil || !strings.Contains(err.Error(), "incomplete copy") {
		t.Fatalf("expected an incomplete copy error, got %v", err)
	}

	got, readErr := os.ReadFile(src)
	if readErr != nil || string(got) != "0123456789" {
		t.Errorf("source must be untouched, got %q, %v", got, readErr)
	}
	entries, _ := os.ReadDir(dstDir)
	if len(entries) != 0 {
		t.Errorf("target directory must stay empty, has %d entries", len(entries))
	}
}

// Spec: ORG-020
func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	free := filepath.Join(dir, "photo.jpg")
	if got := UniquePath(free); got != free {
		t.Errorf("free path changed: %q", got)
	}

	os.WriteFile(free, []byte("x"), 0644)
	got := UniquePath(free)
	if got == free {
		t.Fatal("existing path must not be returned")
	}
	name := filepath.Base(got)
	if filepath.Dir(got) != dir || !strings.HasPrefix(name, "photo-") || !strings.HasSuffix(name, ".jpg") || len(name) != len("photo-xxxx.jpg") {
		t.Errorf("unexpected alternative path %q", got)
	}
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Errorf("alternative path must not exist yet: %v", err)
	}
}
