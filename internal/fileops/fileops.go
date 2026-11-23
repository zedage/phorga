package fileops

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// copyData copies the file content. It is a variable so tests can simulate
// an incomplete copy.
var copyData = func(dst io.Writer, src io.Reader) (int64, error) { return io.Copy(dst, src) }

// linkFile and renameFile are variables so tests can simulate a target on
// another volume or a filesystem without hard links.
var (
	linkFile   = os.Link
	renameFile = os.Rename
)

// ErrDestinationExists is returned when a transfer would replace a file
// that already exists (ORG-020, DUP-005).
var ErrDestinationExists = errors.New("destination already exists")

// ErrSameFile is returned when source and destination are the same file, so
// a transfer would replace or delete the file with itself (ORG-024).
var ErrSameFile = errors.New("source and destination are the same file")

// AtomicCopy copies src to dst. It never replaces an existing file: if dst
// exists, or appears while the copy is written, it returns an error wrapping
// ErrDestinationExists and leaves dst untouched (ORG-020). The copy is
// written to a temp file, flushed to disk and checked against the source
// size before it becomes visible (ORG-019), and keeps the source's
// modification time (ORG-018). Destination directories are created as
// needed.
func AtomicCopy(src, dst string) error {
	return copyFile(src, dst, false)
}

// AtomicCopyReplace is AtomicCopy, but replaces an existing dst. Only the
// organizer's duplicate action replace uses it (ORG-012).
func AtomicCopyReplace(src, dst string) error {
	return copyFile(src, dst, true)
}

// AtomicMove moves src to dst without ever replacing an existing file
// (ORG-020). On the same volume, the file gets its new name through a hard
// link and keeps its data, inode and modification time (ORG-025). Otherwise
// it is copied as in AtomicCopy, and src is removed only after the verified
// copy is in place (ORG-004, ORG-019).
func AtomicMove(src, dst string) error {
	return moveFile(src, dst, false)
}

// AtomicMoveReplace is AtomicMove, but replaces an existing dst (ORG-012).
func AtomicMoveReplace(src, dst string) error {
	return moveFile(src, dst, true)
}

// checkTarget creates dst's directory. It refuses a transfer onto the
// source itself (ORG-024), and onto an existing file unless replace is set.
func checkTarget(src, dst string, replace bool) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		return nil // nothing there yet
	}
	if srcInfo, err := os.Stat(src); err == nil && os.SameFile(srcInfo, dstInfo) {
		return fmt.Errorf("%w: %s", ErrSameFile, dst)
	}
	if !replace {
		return fmt.Errorf("%w: %s", ErrDestinationExists, dst)
	}
	return nil
}

func copyFile(src, dst string, replace bool) error {
	if err := checkTarget(src, dst, replace); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("reading source info: %w", err)
	}

	// Write to temp file in the same directory as dst
	tmpFile, err := os.CreateTemp(filepath.Dir(dst), ".phorga-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Remove the temp name whatever happens. After a successful publish it
	// is only a second name of dst; after a rename it is gone already.
	defer os.Remove(tmpPath)

	written, err := copyData(tmpFile, srcFile)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("copying data: %w", err)
	}

	// Flush to disk, so a power cut after the rename cannot leave an empty
	// or partial target.
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("flushing temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Verify the copy is complete before it becomes visible.
	tmpInfo, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("checking temp file: %w", err)
	}
	if written != srcInfo.Size() || tmpInfo.Size() != srcInfo.Size() {
		return fmt.Errorf("incomplete copy: source has %d bytes, copy has %d", srcInfo.Size(), tmpInfo.Size())
	}

	// Preserve original file permissions (best effort)
	os.Chmod(tmpPath, srcInfo.Mode())

	// Preserve the modification time. For files without EXIF it is often
	// the only date there is.
	if err := os.Chtimes(tmpPath, time.Now(), srcInfo.ModTime()); err != nil {
		return fmt.Errorf("preserving modification time: %w", err)
	}

	if replace {
		if err := os.Rename(tmpPath, dst); err != nil {
			return fmt.Errorf("renaming temp to destination: %w", err)
		}
	} else if err := publish(tmpPath, dst); err != nil {
		return err
	}

	syncDir(filepath.Dir(dst))
	return nil
}

// publish gives the finished temp file its final name without replacing a
// file that exists there. Creating a hard link fails atomically when the
// name is taken, so a file that appeared after the earlier check is never
// lost (ORG-020). Filesystems without hard links (e.g. exFAT, some network
// mounts) fall back to a check followed by a rename, which leaves a short
// window for a file written by another program (ADR-0019).
func publish(tmpPath, dst string) error {
	err := linkFile(tmpPath, dst)
	if err == nil {
		return nil // the caller removes the temp name
	}
	if errors.Is(err, fs.ErrExist) || exists(dst) {
		return fmt.Errorf("%w: %s", ErrDestinationExists, dst)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("renaming temp to destination: %w", err)
	}
	return nil
}

func moveFile(src, dst string, replace bool) error {
	if err := checkTarget(src, dst, replace); err != nil {
		return err
	}

	// Same volume: give the file its new name without copying (ORG-025).
	// Any failure here (another volume, no hard links) falls back to a copy.
	// A symbolic link always takes the copy: a hard link or rename would
	// move the link itself, which may point nowhere from its new folder.
	switch {
	case isSymlink(src):
	case replace:
		if err := renameFile(src, dst); err == nil {
			syncDir(filepath.Dir(dst))
			syncDir(filepath.Dir(src))
			return nil
		}
	default:
		err := linkFile(src, dst)
		if err == nil {
			syncDir(filepath.Dir(dst))
			if err := os.Remove(src); err != nil {
				return fmt.Errorf("removing source after move: %w", err)
			}
			syncDir(filepath.Dir(src))
			return nil
		}
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrDestinationExists, dst)
		}
	}

	if err := copyFile(src, dst, replace); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("removing source after move: %w", err)
	}
	return nil
}

// syncDir flushes a directory entry change (the rename) to disk. Best effort:
// not every platform or filesystem supports it.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()
	_ = d.Sync()
}

// isSymlink reports whether path itself is a symbolic link.
func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

// GenerateSuffix returns a random 4-character alphanumeric string.
func GenerateSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 4)
	rand.Read(b)
	for i := range b {
		b[i] = chars[b[i]%byte(len(chars))]
	}
	return string(b)
}

// AddSuffix inserts a suffix before the file extension.
// Example: "photo.jpg" with suffix "a3f9" → "photo-a3f9.jpg"
func AddSuffix(filename, suffix string) string {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	return fmt.Sprintf("%s-%s%s", name, suffix, ext)
}

// UniquePath returns dst if nothing exists there. Otherwise it returns dst
// with a random suffix before the extension, so an existing file is never
// overwritten (ORG-020).
func UniquePath(dst string) string {
	if !exists(dst) {
		return dst
	}
	return SuffixedPath(dst)
}

// SuffixedPath returns dst with a random suffix before the extension, choosing
// a name that does not exist yet.
func SuffixedPath(dst string) string {
	dir, base := filepath.Dir(dst), filepath.Base(dst)
	candidate := filepath.Join(dir, AddSuffix(base, GenerateSuffix()))
	for i := 0; i < 100 && exists(candidate); i++ {
		candidate = filepath.Join(dir, AddSuffix(base, GenerateSuffix()))
	}
	return candidate
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// EnsureDir creates a directory and all parents if they don't exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// WriteFileAtomic writes data to path via a temp file in the same directory
// and a rename, creating parent directories as needed. It is meant for
// Phorga's own files (state, cache, config) and replaces an existing file.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".phorga-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
