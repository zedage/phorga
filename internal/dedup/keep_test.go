package dedup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nfnt/resize"
)

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Spec: DUP-004
func TestDedup_KeepsCameraOriginalOverWhatsAppCopy(t *testing.T) {
	src, dupDir := t.TempDir(), t.TempDir()
	orig := scene(identity, false)
	// "IMG-…" sorts before "IMG2…", so the WhatsApp copy is found first.
	wa := writeJPEG(t, src, "IMG-20220920-WA0004.jpg", resize.Resize(256, 192, orig, resize.Lanczos3), 60)
	cam := writeJPEG(t, src, "IMG20220920182841.jpg", orig, 90)

	result := Run(context.Background(), contentJob("moveToFolder", dupDir, src))

	if result.DuplicatesPair != 1 || result.Errors != 0 {
		t.Fatalf("pairs = %d, errors = %d, want 1 and 0", result.DuplicatesPair, result.Errors)
	}
	if !exists(cam) {
		t.Error("camera original was moved, want it kept")
	}
	if exists(wa) || !exists(filepath.Join(dupDir, "IMG-20220920-WA0004.jpg")) {
		t.Error("WhatsApp copy must be moved to the duplicate folder")
	}
}

// Spec: DUP-004
func TestDedup_SameResolutionKeepsLargerFile(t *testing.T) {
	src, dupDir := t.TempDir(), t.TempDir()
	img := scene(identity, false)
	a := writeJPEG(t, src, "a.jpg", img, 50)
	b := writeJPEG(t, src, "b.jpg", img, 95)
	sa, _ := os.Stat(a)
	sb, _ := os.Stat(b)
	if sa.Size() >= sb.Size() {
		t.Fatalf("precondition: a.jpg (%d bytes) must be smaller than b.jpg (%d bytes)", sa.Size(), sb.Size())
	}

	result := Run(context.Background(), contentJob("moveToFolder", dupDir, src))

	if result.DuplicatesPair != 1 {
		t.Fatalf("pairs = %d, want 1", result.DuplicatesPair)
	}
	if !exists(b) || exists(a) {
		t.Error("want b.jpg (larger) kept and a.jpg moved")
	}
}

// Spec: DUP-004
func TestDedup_GroupOfThree_KeepsHighestResolution(t *testing.T) {
	src1, src2, src3, dupDir := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	orig := scene(identity, false)
	writeJPEG(t, src1, "small.jpg", resize.Resize(256, 192, orig, resize.Lanczos3), 90)
	big := writeJPEG(t, src2, "big.jpg", orig, 90)
	writeJPEG(t, src3, "medium.jpg", resize.Resize(384, 288, orig, resize.Lanczos3), 90)

	result := Run(context.Background(), contentJob("moveToFolder", dupDir, src1, src2, src3))

	if result.DuplicatesPair != 2 || result.ActionsApplied != 2 || result.Errors != 0 {
		t.Fatalf("pairs = %d, actions = %d, errors = %d, want 2, 2, 0", result.DuplicatesPair, result.ActionsApplied, result.Errors)
	}
	if !exists(big) {
		t.Error("big.jpg (most pixels) must stay in place")
	}
	if n := countFiles(t, dupDir); n != 2 {
		t.Errorf("duplicate folder has %d files, want 2", n)
	}
}

// Spec: DUP-004, DUP-012
func TestCache_StoresResolutionOfGroupFiles(t *testing.T) {
	src := t.TempDir()
	orig := scene(identity, false)
	writeJPEG(t, src, "a.jpg", orig, 90)
	writeJPEG(t, src, "b.jpg", resize.Resize(256, 192, orig, resize.Lanczos3), 60)
	path := filepath.Join(t.TempDir(), CacheFileName)

	RunWithCache(context.Background(), contentJob("skip", "", src), LoadCache(path))

	c := LoadCache(path)
	for name, want := range map[string][2]int{"a.jpg": {512, 384}, "b.jpg": {256, 192}} {
		e := c.entries[filepath.Join(src, name)]
		if e.Width != want[0] || e.Height != want[1] {
			t.Errorf("%s: cached resolution %d×%d, want %d×%d", name, e.Width, e.Height, want[0], want[1])
		}
	}
}
