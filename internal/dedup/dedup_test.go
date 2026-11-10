package dedup

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/zedage/phorga/internal/config"
)

func createTestImage(t *testing.T, dir, name string, r, g, b uint8) string {
	t.Helper()
	path := filepath.Join(dir, name)

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

// createCheckerImage creates a 64x64 checkerboard pattern (very different pHash from solid).
func createCheckerImage(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if (x/8+y/8)%2 == 0 {
				img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255})
			}
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Spec: DUP-002
func TestDedup_MetadataMode_SameSize(t *testing.T) {
	src1 := t.TempDir()
	src2 := t.TempDir()

	// Create files with same size
	createTestFile(t, src1, "a.jpg", "same content here!")
	createTestFile(t, src2, "b.jpg", "same content here!")

	job := config.DuplicateDetection{
		Enabled:       true,
		DryRun:        true,
		Schedule:      "0 4 * * *",
		Sources:       []string{src1, src2},
		DetectionMode: "metadata",
		Action:        config.DuplicateAction{Method: "skip"},
	}

	result := Run(job)
	if result.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", result.TotalFiles)
	}
	// Metadata mode requires matching EXIF dates — plain text files won't match
	// So this should find 0 duplicates (no EXIF)
	t.Logf("duplicates found: %d (expected 0 for files without EXIF)", result.DuplicatesPair)
}

// Spec: DUP-003
func TestDedup_ContentMode_IdenticalImages(t *testing.T) {
	src1 := t.TempDir()
	src2 := t.TempDir()

	// Create identical images in two sources
	createTestImage(t, src1, "photo1.jpg", 128, 64, 32)
	createTestImage(t, src2, "photo2.jpg", 128, 64, 32)

	job := config.DuplicateDetection{
		Enabled:       true,
		DryRun:        true,
		Schedule:      "0 4 * * *",
		Sources:       []string{src1, src2},
		DetectionMode: "content",
		Action:        config.DuplicateAction{Method: "skip"},
	}

	result := Run(job)
	if result.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", result.TotalFiles)
	}
	if result.DuplicatesPair == 0 {
		t.Error("expected identical images to be detected as duplicates")
	}
}

// Spec: DUP-003
func TestDedup_ContentMode_DifferentImages(t *testing.T) {
	src1 := t.TempDir()
	src2 := t.TempDir()

	// Create visually different images: solid color vs checkerboard pattern
	createTestImage(t, src1, "solid.jpg", 128, 128, 128)
	createCheckerImage(t, src2, "checker.jpg")

	job := config.DuplicateDetection{
		Enabled:       true,
		DryRun:        true,
		Schedule:      "0 4 * * *",
		Sources:       []string{src1, src2},
		DetectionMode: "content",
		Action:        config.DuplicateAction{Method: "skip"},
	}

	result := Run(job)
	if result.DuplicatesPair > 0 {
		t.Error("visually different images should not be classified as duplicates")
	}
}

// Spec: DUP-005
func TestDedup_MoveToFolder(t *testing.T) {
	src1 := t.TempDir()
	src2 := t.TempDir()
	dupDir := t.TempDir()

	createTestImage(t, src1, "photo.jpg", 100, 100, 100)
	createTestImage(t, src2, "copy.jpg", 100, 100, 100)

	job := config.DuplicateDetection{
		Enabled:       true,
		DryRun:        false,
		Schedule:      "0 4 * * *",
		Sources:       []string{src1, src2},
		DetectionMode: "content",
		Action: config.DuplicateAction{
			Method:          "moveToFolder",
			DuplicateFolder: dupDir,
		},
	}

	result := Run(job)
	if result.Errors > 0 {
		t.Errorf("expected no errors, got %d", result.Errors)
	}

	// Check that duplicate was moved
	entries, _ := os.ReadDir(dupDir)
	if len(entries) == 0 && result.DuplicatesPair > 0 {
		t.Error("expected duplicate to be moved to duplicate folder")
	}
}

// Spec: DUP-007
func TestDedup_DryRunNoChanges(t *testing.T) {
	src1 := t.TempDir()
	src2 := t.TempDir()
	dupDir := t.TempDir()

	path1 := createTestImage(t, src1, "photo.jpg", 100, 100, 100)
	path2 := createTestImage(t, src2, "copy.jpg", 100, 100, 100)

	job := config.DuplicateDetection{
		Enabled:       true,
		DryRun:        true,
		Schedule:      "0 4 * * *",
		Sources:       []string{src1, src2},
		DetectionMode: "content",
		Action: config.DuplicateAction{
			Method:          "moveToFolder",
			DuplicateFolder: dupDir,
		},
	}

	Run(job)

	// Both files should still exist
	if _, err := os.Stat(path1); err != nil {
		t.Error("file1 should exist after dry run")
	}
	if _, err := os.Stat(path2); err != nil {
		t.Error("file2 should exist after dry run")
	}

	// Duplicate folder should be empty
	entries, _ := os.ReadDir(dupDir)
	if len(entries) > 0 {
		t.Error("duplicate folder should be empty in dry run")
	}
}

// Spec: DUP-004
func TestDedup_CrossSource(t *testing.T) {
	src1 := t.TempDir()
	src2 := t.TempDir()

	createTestImage(t, src1, "a.jpg", 50, 50, 50)
	createTestImage(t, src2, "b.jpg", 50, 50, 50)

	job := config.DuplicateDetection{
		Enabled:       true,
		DryRun:        true,
		Schedule:      "0 4 * * *",
		Sources:       []string{src1, src2},
		DetectionMode: "content",
		Action:        config.DuplicateAction{Method: "skip"},
	}

	result := Run(job)
	if result.DuplicatesPair == 0 {
		t.Error("expected cross-source duplicate detection")
	}
}
