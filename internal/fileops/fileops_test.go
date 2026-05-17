package fileops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicCopy(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "original.jpg")
	dstPath := filepath.Join(dstDir, "sub", "copied.jpg")

	content := []byte("test image content")
	os.WriteFile(srcPath, content, 0644)

	if err := AtomicCopy(srcPath, dstPath); err != nil {
		t.Fatalf("AtomicCopy failed: %v", err)
	}

	// Source still exists
	if _, err := os.Stat(srcPath); err != nil {
		t.Error("source file should still exist after copy")
	}

	// Destination exists with correct content
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("reading destination: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("destination content mismatch: got %q", got)
	}
}

func TestAtomicMove(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "original.jpg")
	dstPath := filepath.Join(dstDir, "moved.jpg")

	content := []byte("test image content")
	os.WriteFile(srcPath, content, 0644)

	if err := AtomicMove(srcPath, dstPath); err != nil {
		t.Fatalf("AtomicMove failed: %v", err)
	}

	// Source should be gone
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("source file should not exist after move")
	}

	// Destination exists
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("reading destination: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("destination content mismatch: got %q", got)
	}
}

func TestGenerateSuffix(t *testing.T) {
	s := GenerateSuffix()
	if len(s) != 4 {
		t.Errorf("expected 4-char suffix, got %q (len=%d)", s, len(s))
	}
	// Should be alphanumeric
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			t.Errorf("unexpected character in suffix: %c", c)
		}
	}

	// Two calls should (almost certainly) produce different results
	s2 := GenerateSuffix()
	if s == s2 {
		t.Log("warning: two suffix calls returned same value (unlikely but possible)")
	}
}

func TestAddSuffix(t *testing.T) {
	tests := []struct {
		filename, suffix, want string
	}{
		{"photo.jpg", "a3f9", "photo-a3f9.jpg"},
		{"my.photo.png", "x1y2", "my.photo-x1y2.png"},
		{"noext", "abcd", "noext-abcd"},
	}
	for _, tt := range tests {
		got := AddSuffix(tt.filename, tt.suffix)
		if got != tt.want {
			t.Errorf("AddSuffix(%q, %q) = %q, want %q", tt.filename, tt.suffix, got, tt.want)
		}
	}
}

func TestEnsureDir(t *testing.T) {
	base := t.TempDir()
	deep := filepath.Join(base, "a", "b", "c")

	if err := EnsureDir(deep); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	info, err := os.Stat(deep)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestAtomicCopy_NoTempLeftOnError(t *testing.T) {
	dstDir := t.TempDir()
	dstPath := filepath.Join(dstDir, "fail.jpg")

	// Source doesn't exist → should fail
	err := AtomicCopy("/nonexistent/file.jpg", dstPath)
	if err == nil {
		t.Fatal("expected error")
	}

	// No temp files should be left
	entries, _ := os.ReadDir(dstDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".phorganizer-tmp-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

