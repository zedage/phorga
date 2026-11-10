package organizer

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/state"
	"github.com/zedage/phorga/internal/testutil"
)

var grey = color.RGBA{R: 120, G: 120, B: 120, A: 255}

func exifProfile(tgtDir string) (config.OrganizerProfile, config.OrganizerSource) {
	return config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}"},
		config.OrganizerSource{
			Mode:     "copy",
			Schedule: "0 2 * * *",
			Duplicates: &config.OrganizerDuplicate{
				DetectionMode: "exif",
				Action:        config.DuplicateAction{Method: "skip"},
			},
		}
}

func filesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// Spec: ORG-011
func TestOrganize_ExifDetection_SameEXIFDateIsDuplicate(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	profile, source := exifProfile(tgtDir)
	source.Path = srcDir

	date := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	existingDir := filepath.Join(tgtDir, "2024", "03")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatal(err)
	}
	testutil.WriteJPEGWithEXIF(t, existingDir, "photo.jpg", grey, date)
	testutil.WriteJPEGWithEXIF(t, srcDir, "photo.jpg", color.RGBA{R: 10, G: 200, B: 10, A: 255}, date)

	result := Run(profile, source, state.New(), filepath.Join(stateDir, "state.json"))

	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (same EXIF date → duplicate → skip)", result.Skipped)
	}
	if names := filesIn(t, existingDir); len(names) != 1 {
		t.Errorf("target dir = %v, want only the existing photo.jpg", names)
	}
}

// Spec: ORG-011, ORG-013
func TestOrganize_ExifDetection_NoEXIFNeverMatches(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	profile, source := exifProfile(tgtDir)
	source.Path = srcDir

	// Both files carry the same date in their name but have no EXIF data.
	// Before ORG-011 required real EXIF, the filename date made them "duplicates".
	name := "IMG_20240315_143000.jpg"
	existingDir := filepath.Join(tgtDir, "2024", "03")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatal(err)
	}
	testutil.WriteJPEG(t, existingDir, name, grey)
	testutil.WriteJPEG(t, srcDir, name, grey)

	result := Run(profile, source, state.New(), filepath.Join(stateDir, "state.json"))

	if result.Skipped != 0 {
		t.Errorf("skipped = %d, want 0 (no EXIF → not a duplicate)", result.Skipped)
	}
	if names := filesIn(t, existingDir); len(names) != 2 {
		t.Errorf("target dir = %v, want existing file plus a suffixed copy", names)
	}
}
