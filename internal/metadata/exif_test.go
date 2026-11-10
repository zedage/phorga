package metadata

import (
	"image/color"
	"testing"
	"time"

	"github.com/zedage/phorga/internal/testutil"
)

var grey = color.RGBA{R: 120, G: 120, B: 120, A: 255}

// Spec: ORG-002
func TestExtractDate_EXIFBeforeFilename(t *testing.T) {
	dir := t.TempDir()
	exifDate := time.Date(2021, 6, 3, 9, 15, 0, 0, time.UTC)
	path := testutil.WriteJPEGWithEXIF(t, dir, "IMG_20240715_143000.jpg", grey, exifDate)

	got, method, err := ExtractDate(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != "exif" {
		t.Errorf("method = %q, want exif", method)
	}
	if got.Year() != 2021 || got.Month() != time.June || got.Day() != 3 {
		t.Errorf("date = %v, want 2021-06-03 (EXIF must win over filename)", got)
	}
}

// Spec: ORG-011, DUP-002
func TestExtractEXIFDate_IgnoresFilenameAndMtime(t *testing.T) {
	dir := t.TempDir()
	path := testutil.WriteJPEG(t, dir, "IMG_20240715_143000.jpg", grey)

	if _, err := ExtractEXIFDate(path); err == nil {
		t.Fatal("expected error for JPEG without EXIF, even though the filename contains a date")
	}

	exifDate := time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC)
	withExif := testutil.WriteJPEGWithEXIF(t, dir, "photo.jpg", grey, exifDate)
	got, err := ExtractEXIFDate(withExif)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year() != 2022 || got.Month() != time.January || got.Day() != 2 || got.Hour() != 3 {
		t.Errorf("date = %v, want 2022-01-02 03:04:05", got)
	}
}
