package fileops

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AtomicCopy copies src to dst using a temp file + rename for atomicity.
// Creates destination directories as needed.
func AtomicCopy(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer srcFile.Close()

	// Write to temp file in the same directory as dst
	tmpFile, err := os.CreateTemp(filepath.Dir(dst), ".phorganizer-tmp-*")
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

	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		tmpFile.Close()
		return fmt.Errorf("copying data: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Preserve original file permissions
	srcInfo, err := os.Stat(src)
	if err == nil {
		os.Chmod(tmpPath, srcInfo.Mode())
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("renaming temp to destination: %w", err)
	}

	success = true
	return nil
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

// EnsureDir creates a directory and all parents if they don't exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

