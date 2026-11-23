package fileops

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// forceCrossVolume makes every hard link and rename between files fail as
// if source and target were on different volumes, so moves must copy.
func forceCrossVolume(t *testing.T) {
	t.Helper()
	origLink, origRename := linkFile, renameFile
	linkFile = func(oldname, newname string) error {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: syscall.EXDEV}
	}
	renameFile = func(oldname, newname string) error {
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: syscall.EXDEV}
	}
	t.Cleanup(func() { linkFile, renameFile = origLink, origRename })
}

// noHardLinks makes hard links fail as on a filesystem without them
// (exFAT, some network mounts). Renames still work.
func noHardLinks(t *testing.T) {
	t.Helper()
	orig := linkFile
	linkFile = func(oldname, newname string) error {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: syscall.EPERM}
	}
	t.Cleanup(func() { linkFile = orig })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	tmps, _ := filepath.Glob(filepath.Join(dir, ".phorga-tmp-*"))
	if len(tmps) != 0 {
		t.Errorf("temp files left behind: %v", tmps)
	}
}

// Spec: ORG-020
func TestAtomicCopy_NeverReplacesExistingFile(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T)
	}{
		{"hard links", func(*testing.T) {}},
		{"no hard links", noHardLinks},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			dir := t.TempDir()
			src, dst := filepath.Join(dir, "src", "a.jpg"), filepath.Join(dir, "dst", "a.jpg")
			writeFile(t, src, "new photo")
			writeFile(t, dst, "existing photo")

			err := AtomicCopy(src, dst)
			if !errors.Is(err, ErrDestinationExists) {
				t.Fatalf("expected ErrDestinationExists, got %v", err)
			}
			if got := readFile(t, dst); got != "existing photo" {
				t.Errorf("existing file changed to %q", got)
			}
			assertNoTempFiles(t, filepath.Dir(dst))
		})
	}
}

// Spec: ORG-020
func TestAtomicCopy_FileThatAppearsDuringCopyIsKept(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src", "a.jpg"), filepath.Join(dir, "dst", "a.jpg")
	writeFile(t, src, "new photo")

	// Another writer creates dst after the first check, while the copy runs.
	orig := copyData
	copyData = func(w io.Writer, r io.Reader) (int64, error) {
		writeFile(t, dst, "photo written meanwhile")
		return io.Copy(w, r)
	}
	t.Cleanup(func() { copyData = orig })

	err := AtomicCopy(src, dst)
	if !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("expected ErrDestinationExists, got %v", err)
	}
	if got := readFile(t, dst); got != "photo written meanwhile" {
		t.Errorf("file written meanwhile was replaced, now %q", got)
	}
	assertNoTempFiles(t, filepath.Dir(dst))
}

// Spec: ORG-020
func TestAtomicMove_NeverReplacesExistingFile(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T)
	}{
		{"same volume", func(*testing.T) {}},
		{"other volume", forceCrossVolume},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			dir := t.TempDir()
			src, dst := filepath.Join(dir, "src", "a.jpg"), filepath.Join(dir, "dst", "a.jpg")
			writeFile(t, src, "new photo")
			writeFile(t, dst, "existing photo")

			err := AtomicMove(src, dst)
			if !errors.Is(err, ErrDestinationExists) {
				t.Fatalf("expected ErrDestinationExists, got %v", err)
			}
			if got := readFile(t, dst); got != "existing photo" {
				t.Errorf("existing file changed to %q", got)
			}
			if got := readFile(t, src); got != "new photo" {
				t.Errorf("source must stay, got %q", got)
			}
		})
	}
}

// Spec: ORG-025
func TestAtomicMove_SameVolumeKeepsTheFileWithoutCopying(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "in", "clip.mov"), filepath.Join(dir, "out", "2019", "clip.mov")
	writeFile(t, src, "video data")
	before, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}

	orig := copyData
	copyData = func(io.Writer, io.Reader) (int64, error) {
		t.Error("a move on the same volume must not copy the data")
		return 0, errors.New("unexpected copy")
	}
	t.Cleanup(func() { copyData = orig })

	if err := AtomicMove(src, dst); err != nil {
		t.Fatalf("AtomicMove: %v", err)
	}
	after, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Error("the moved file must be the same file (inode) as the source")
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source must be gone, stat error: %v", err)
	}
}

// Spec: ORG-025, ORG-004
func TestAtomicMove_OtherVolumeCopiesAndRemovesSource(t *testing.T) {
	forceCrossVolume(t)
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "in", "a.jpg"), filepath.Join(dir, "out", "a.jpg")
	writeFile(t, src, "photo")

	if err := AtomicMove(src, dst); err != nil {
		t.Fatalf("AtomicMove: %v", err)
	}
	if got := readFile(t, dst); got != "photo" {
		t.Errorf("target content = %q", got)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source must be gone, stat error: %v", err)
	}
	assertNoTempFiles(t, filepath.Dir(dst))
}

// Spec: ORG-020
func TestAtomicCopy_WithoutHardLinksStillWorks(t *testing.T) {
	noHardLinks(t)
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "in", "a.jpg"), filepath.Join(dir, "out", "a.jpg")
	writeFile(t, src, "photo")

	if err := AtomicCopy(src, dst); err != nil {
		t.Fatalf("AtomicCopy: %v", err)
	}
	if got := readFile(t, dst); got != "photo" {
		t.Errorf("target content = %q", got)
	}
	assertNoTempFiles(t, filepath.Dir(dst))
}

// Spec: ORG-012, ORG-025
func TestReplace_OverwritesExistingFile(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T)
		op    func(src, dst string) error
		move  bool
	}{
		{"copy", func(*testing.T) {}, AtomicCopyReplace, false},
		{"move same volume", func(*testing.T) {}, AtomicMoveReplace, true},
		{"move other volume", forceCrossVolume, AtomicMoveReplace, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			dir := t.TempDir()
			src, dst := filepath.Join(dir, "in", "a.jpg"), filepath.Join(dir, "out", "a.jpg")
			writeFile(t, src, "new photo")
			writeFile(t, dst, "old photo")

			if err := tc.op(src, dst); err != nil {
				t.Fatalf("replace: %v", err)
			}
			if got := readFile(t, dst); got != "new photo" {
				t.Errorf("target content = %q, want the new photo", got)
			}
			_, err := os.Stat(src)
			if tc.move && !os.IsNotExist(err) {
				t.Errorf("source must be gone after a move, stat error: %v", err)
			}
			if !tc.move && err != nil {
				t.Errorf("source must stay after a copy: %v", err)
			}
			assertNoTempFiles(t, filepath.Dir(dst))
		})
	}
}

// Spec: ORG-024
func TestTransferOntoItselfIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2024", "03", "a.jpg")
	writeFile(t, path, "only copy")
	alias := filepath.Join(dir, "alias.jpg")
	if err := os.Link(path, alias); err != nil {
		t.Skipf("hard links not available: %v", err)
	}

	ops := map[string]func(src, dst string) error{
		"AtomicCopy": AtomicCopy, "AtomicCopyReplace": AtomicCopyReplace,
		"AtomicMove": AtomicMove, "AtomicMoveReplace": AtomicMoveReplace,
	}
	for name, op := range ops {
		for _, dst := range []string{path, alias} {
			if err := op(path, dst); !errors.Is(err, ErrSameFile) {
				t.Errorf("%s(%s, %s): expected ErrSameFile, got %v", name, path, dst, err)
			}
		}
	}
	if got := readFile(t, path); got != "only copy" {
		t.Errorf("file changed to %q", got)
	}
}

// Spec: ORG-025, DUP-005
func TestAtomicMove_SymlinkIsMovedAsItsContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   func(src, dst string) error
	}{
		{"move", AtomicMove},
		{"move with replace", AtomicMoveReplace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			shared := filepath.Join(dir, "shared", "IMG_1.jpg")
			writeFile(t, shared, "photo data")
			src := filepath.Join(dir, "in", "IMG_1.jpg")
			if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
				t.Fatal(err)
			}
			// A relative link, which would point nowhere from the target.
			if err := os.Symlink(filepath.Join("..", "shared", "IMG_1.jpg"), src); err != nil {
				t.Skipf("cannot create symbolic links here: %v", err)
			}
			dst := filepath.Join(dir, "out", "2024", "05", "IMG_1.jpg")

			if err := tc.op(src, dst); err != nil {
				t.Fatalf("move: %v", err)
			}
			info, err := os.Lstat(dst)
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() {
				t.Fatalf("target must be a regular file, mode %v", info.Mode())
			}
			if got := readFile(t, dst); got != "photo data" {
				t.Errorf("target content = %q, want the photo the link pointed to", got)
			}
			if _, err := os.Lstat(src); !os.IsNotExist(err) {
				t.Errorf("the source link must be gone, lstat error: %v", err)
			}
			if got := readFile(t, shared); got != "photo data" {
				t.Errorf("the file the link pointed to must stay, content %q", got)
			}
			assertNoTempFiles(t, filepath.Dir(dst))
		})
	}
}
