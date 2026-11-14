package fileops

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// copyData copies the file content. It is a variable so tests can simulate
// an incomplete copy.
var copyData = func(dst io.Writer, src io.Reader) (int64, error) { return io.Copy(dst, src) }

// AtomicCopy copies src to dst using a temp file + rename for atomicity.
// Creates destination directories as needed. The copy keeps the source's
// modification time (ORG-018). It is flushed to disk and its size is checked
// against the source before it replaces dst (ORG-019).
func AtomicCopy(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
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

	// Cleanup temp file on error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

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

	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("renaming temp to destination: %w", err)
	}
	success = true

	syncDir(filepath.Dir(dst))
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

// AtomicMove copies src to dst atomically, then removes src on success.
func AtomicMove(src, dst string) error {
	if err := AtomicCopy(src, dst); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("removing source after move: %w", err)
	}
	return nil
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
// and a rename, creating parent directories as needed.
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
