package dedup

import (
	"context"
	"image/color"
	"testing"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/testutil"
)

var grey = color.RGBA{R: 120, G: 120, B: 120, A: 255}

func metadataJob(sources ...string) config.DuplicateDetection {
	return config.DuplicateDetection{
		Enabled:       true,
		DryRun:        true,
		Schedule:      "0 4 * * *",
		Sources:       sources,
		DetectionMode: "metadata",
		Action:        config.DuplicateAction{Method: "skip"},
	}
}

// Spec: DUP-002
func TestDedup_MetadataMode_SameSizeAndEXIFDate(t *testing.T) {
	src1, src2 := t.TempDir(), t.TempDir()
	date := time.Date(2024, 7, 15, 10, 0, 0, 0, time.UTC)
	testutil.WriteJPEGWithEXIF(t, src1, "a.jpg", grey, date)
	testutil.WriteJPEGWithEXIF(t, src2, "b.jpg", grey, date)

	result := Run(context.Background(), metadataJob(src1, src2))
	if result.DuplicatesPair != 1 {
		t.Errorf("duplicates = %d, want 1", result.DuplicatesPair)
	}
}

// Spec: DUP-002
func TestDedup_MetadataMode_DifferentEXIFDate(t *testing.T) {
	src1, src2 := t.TempDir(), t.TempDir()
	// Same image, same byte size, different capture date.
	testutil.WriteJPEGWithEXIF(t, src1, "a.jpg", grey, time.Date(2024, 7, 15, 10, 0, 0, 0, time.UTC))
	testutil.WriteJPEGWithEXIF(t, src2, "b.jpg", grey, time.Date(2023, 1, 1, 8, 0, 0, 0, time.UTC))

	result := Run(context.Background(), metadataJob(src1, src2))
	if result.DuplicatesPair != 0 {
		t.Errorf("duplicates = %d, want 0", result.DuplicatesPair)
	}
}

// Spec: DUP-002
func TestDedup_MetadataMode_NoEXIFNeverMatches(t *testing.T) {
	src1, src2 := t.TempDir(), t.TempDir()
	// Same name pattern date, same size, (almost certainly) same mtime, but no EXIF.
	testutil.WriteJPEG(t, src1, "IMG_20240715_100000.jpg", grey)
	testutil.WriteJPEG(t, src2, "IMG_20240715_100000.jpg", grey)

	result := Run(context.Background(), metadataJob(src1, src2))
	if result.DuplicatesPair != 0 {
		t.Errorf("duplicates = %d, want 0 (files without EXIF are never metadata duplicates)", result.DuplicatesPair)
	}
}
