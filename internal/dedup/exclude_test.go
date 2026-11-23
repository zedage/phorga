package dedup

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Spec: DUP-013
func TestDedup_SynologyThumbnailIsNotADuplicate(t *testing.T) {
	src, dupDir := t.TempDir(), t.TempDir()
	createTestImage(t, src, "IMG_1234.JPG", 100, 100, 100)
	thumbDir := filepath.Join(src, "@eaDir", "IMG_1234.JPG")
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createTestImage(t, thumbDir, "SYNO_PHOTO_THUMB_SM.jpg", 100, 100, 100)

	result := Run(context.Background(), contentJob("moveToFolder", dupDir, src))

	if result.TotalFiles != 1 || result.DuplicatesPair != 0 || result.Errors != 0 {
		t.Errorf("files = %d, pairs = %d, errors = %d, want 1, 0, 0", result.TotalFiles, result.DuplicatesPair, result.Errors)
	}
	if _, err := os.Stat(filepath.Join(src, "IMG_1234.JPG")); err != nil {
		t.Errorf("the original must stay in place: %v", err)
	}
	if n := countFiles(t, dupDir); n != 0 {
		t.Errorf("duplicate folder has %d files, want 0", n)
	}
}

// Spec: DUP-013
func TestDedup_ExcludePatternsSkipFiles(t *testing.T) {
	src := t.TempDir()
	createTestImage(t, src, "a.jpg", 100, 100, 100)
	exportDir := filepath.Join(src, "Export")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createTestImage(t, exportDir, "a-copy.jpg", 100, 100, 100)

	job := contentJob("skip", "", src)
	job.Exclude = []string{"export"}
	result := Run(context.Background(), job)

	if result.TotalFiles != 1 || result.DuplicatesPair != 0 {
		t.Errorf("files = %d, pairs = %d, want 1 and 0", result.TotalFiles, result.DuplicatesPair)
	}
}

// Spec: DUP-013, DUP-012
func TestDedup_ExcludedFilesLeaveTheCache(t *testing.T) {
	src := t.TempDir()
	createTestImage(t, src, "a.jpg", 100, 100, 100)
	exportDir := filepath.Join(src, "Export")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createTestImage(t, exportDir, "b.jpg", 100, 100, 100)
	cache := LoadCache("")

	job := contentJob("skip", "", src)
	RunWithCache(context.Background(), job, cache)
	if cache.Len() != 2 {
		t.Fatalf("cached = %d after first run, want 2", cache.Len())
	}

	job.Exclude = []string{"Export"}
	RunWithCache(context.Background(), job, cache)
	if cache.Len() != 1 {
		t.Errorf("cached = %d after excluding Export, want 1", cache.Len())
	}
}

// Spec: DUP-013
func TestDedup_SkippedFolderIsLoggedAtDebugOnly(t *testing.T) {
	for _, tc := range []struct {
		level slog.Level
		want  int
	}{{slog.LevelInfo, 0}, {slog.LevelDebug, 1}} {
		t.Run(tc.level.String(), func(t *testing.T) {
			var buf bytes.Buffer
			orig := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: tc.level})))
			t.Cleanup(func() { slog.SetDefault(orig) })

			src := t.TempDir()
			createTestImage(t, src, "IMG_1234.JPG", 100, 100, 100)
			if err := os.MkdirAll(filepath.Join(src, "@eaDir"), 0o755); err != nil {
				t.Fatal(err)
			}
			Run(context.Background(), contentJob("skip", "", src))

			if got := strings.Count(buf.String(), `"msg":"skipping folder"`); got != tc.want {
				t.Errorf("at level %s: %d skipping folder lines, want %d:\n%s", tc.level, got, tc.want, buf.String())
			}
		})
	}
}
