package dedup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zedage/phorga/internal/config"
)

func contentJob(method, dupFolder string, sources ...string) config.DuplicateDetection {
	return config.DuplicateDetection{
		Enabled:       true,
		Schedule:      "0 4 * * *",
		Sources:       sources,
		DetectionMode: "content",
		Action:        config.DuplicateAction{Method: method, DuplicateFolder: dupFolder},
	}
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

// Spec: DUP-004, DUP-005
func TestDedup_GroupOfThree_MovesTwoWithoutErrors(t *testing.T) {
	src1, src2, src3 := t.TempDir(), t.TempDir(), t.TempDir()
	dupDir := t.TempDir()
	createTestImage(t, src1, "original.jpg", 100, 100, 100)
	createTestImage(t, src2, "copy1.jpg", 100, 100, 100)
	createTestImage(t, src3, "copy2.jpg", 100, 100, 100)

	result := Run(contentJob("moveToFolder", dupDir, src1, src2, src3))

	if result.Errors != 0 {
		t.Errorf("errors = %d, want 0", result.Errors)
	}
	if result.DuplicatesPair != 2 || result.ActionsApplied != 2 {
		t.Errorf("pairs = %d, actions = %d, want 2 and 2", result.DuplicatesPair, result.ActionsApplied)
	}
	if _, err := os.Stat(filepath.Join(src1, "original.jpg")); err != nil {
		t.Errorf("first file (canonical) must stay in place: %v", err)
	}
	if n := countFiles(t, dupDir); n != 2 {
		t.Errorf("duplicate folder has %d files, want 2", n)
	}
}

// Spec: DUP-006
func TestDedup_SkipLeavesFilesUntouched(t *testing.T) {
	src1, src2 := t.TempDir(), t.TempDir()
	createTestImage(t, src1, "a.jpg", 100, 100, 100)
	createTestImage(t, src2, "b.jpg", 100, 100, 100)

	result := Run(contentJob("skip", "", src1, src2))

	if result.DuplicatesPair != 1 {
		t.Errorf("pairs = %d, want 1", result.DuplicatesPair)
	}
	if countFiles(t, src1) != 1 || countFiles(t, src2) != 1 {
		t.Error("skip must not move or delete any file")
	}
}

// Spec: DUP-009
func TestDedup_ActionErrorDoesNotAbortRun(t *testing.T) {
	src1, src2, src3 := t.TempDir(), t.TempDir(), t.TempDir()
	createTestImage(t, src1, "a.jpg", 100, 100, 100)
	createTestImage(t, src2, "b.jpg", 100, 100, 100)
	createTestImage(t, src3, "c.jpg", 100, 100, 100)

	// duplicateFolder is a regular file, so every move fails.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	result := Run(contentJob("moveToFolder", blocker, src1, src2, src3))

	if result.DuplicatesPair != 2 {
		t.Fatalf("pairs = %d, want 2", result.DuplicatesPair)
	}
	if result.Errors != 2 {
		t.Errorf("errors = %d, want 2 (one per failed pair, run continues after the first)", result.Errors)
	}
	for _, dir := range []string{src1, src2, src3} {
		if countFiles(t, dir) != 1 {
			t.Errorf("%s: file must remain after a failed move", dir)
		}
	}
}

// Spec: DUP-005
func TestDedup_MoveToFolderNeverOverwrites(t *testing.T) {
	src1, src2 := t.TempDir(), t.TempDir()
	dupDir := t.TempDir()
	createTestImage(t, src1, "a.jpg", 100, 100, 100)
	createTestImage(t, src2, "photo.jpg", 100, 100, 100)
	// A file with the same name is already in the duplicate folder.
	if err := os.WriteFile(filepath.Join(dupDir, "photo.jpg"), []byte("earlier duplicate"), 0644); err != nil {
		t.Fatal(err)
	}

	result := Run(contentJob("moveToFolder", dupDir, src1, src2))

	if result.Errors != 0 || result.ActionsApplied != 1 {
		t.Fatalf("errors = %d, actions = %d", result.Errors, result.ActionsApplied)
	}
	if n := countFiles(t, dupDir); n != 2 {
		t.Errorf("duplicate folder has %d files, want 2", n)
	}
	kept, err := os.ReadFile(filepath.Join(dupDir, "photo.jpg"))
	if err != nil || string(kept) != "earlier duplicate" {
		t.Errorf("earlier file was changed: %q, %v", kept, err)
	}
}
