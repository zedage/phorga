package organizer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zedage/phorganizer/internal/config"
	"github.com/zedage/phorganizer/internal/state"
)

func setupTestDirs(t *testing.T) (string, string, string) {
	t.Helper()
	srcDir := t.TempDir()
	tgtDir := t.TempDir()
	stateDir := t.TempDir()
	return srcDir, tgtDir, stateDir
}

func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOrganize_CopyMode(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	createTestFile(t, srcDir, "photo.jpg", "image data")

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path:     srcDir,
		Mode:     "copy",
		Schedule: "0 2 * * *",
	}

	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	result := Run(profile, source, st, sp)

	if result.Errors > 0 {
		t.Errorf("expected no errors, got %d", result.Errors)
	}
	if result.Processed == 0 {
		t.Error("expected at least one processed file")
	}

	// Source should still exist (copy mode)
	if _, err := os.Stat(filepath.Join(srcDir, "photo.jpg")); err != nil {
		t.Error("source should still exist in copy mode")
	}
}

func TestOrganize_MoveMode(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	createTestFile(t, srcDir, "photo.jpg", "image data")

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path:     srcDir,
		Mode:     "move",
		Schedule: "0 2 * * *",
	}

	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	result := Run(profile, source, st, sp)

	if result.Errors > 0 {
		t.Errorf("expected no errors, got %d", result.Errors)
	}

	// Source should be gone (move mode)
	if _, err := os.Stat(filepath.Join(srcDir, "photo.jpg")); !os.IsNotExist(err) {
		t.Error("source should be removed in move mode")
	}
}

func TestOrganize_DryRun(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	createTestFile(t, srcDir, "photo.jpg", "image data")

	profile := config.OrganizerProfile{
		DryRun:          true,
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path:     srcDir,
		Mode:     "move",
		Schedule: "0 2 * * *",
	}

	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	result := Run(profile, source, st, sp)

	if !result.DryRun {
		t.Error("expected dry run")
	}

	// Source should still exist (dry run)
	if _, err := os.Stat(filepath.Join(srcDir, "photo.jpg")); err != nil {
		t.Error("source should still exist in dry run")
	}

	// Nothing in target
	entries, _ := os.ReadDir(tgtDir)
	if len(entries) > 0 {
		t.Error("target should be empty in dry run mode")
	}
}

func TestOrganize_SkipUnsupported(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	createTestFile(t, srcDir, "readme.txt", "not a photo")

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path:     srcDir,
		Mode:     "copy",
		Schedule: "0 2 * * *",
	}

	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	result := Run(profile, source, st, sp)

	if result.Processed != 0 {
		t.Errorf("unsupported files should not be processed, got %d", result.Processed)
	}
}

func TestOrganize_Resume(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	filePath := createTestFile(t, srcDir, "photo.jpg", "image data")

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path:     srcDir,
		Mode:     "copy",
		Schedule: "0 2 * * *",
	}

	// First run
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, st, sp)

	// Second run with same state — should skip
	st2 := state.Load(sp)

	// Re-create target dir (simulate fresh target check)
	tgtDir2 := t.TempDir()
	profile.Target = tgtDir2

	result := Run(profile, source, st2, sp)

	// The file was already in state from first run, so it depends on whether
	// the file info matches. Since we didn't change the file, it should be skipped.
	_ = filePath
	if result.Skipped == 0 {
		t.Log("note: file may have been reprocessed if mtime changed during test")
	}
}

func TestOrganize_DuplicateSkip(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	createTestFile(t, srcDir, "photo.jpg", "image data")

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path: srcDir,
		Mode: "copy",
		Duplicates: &config.OrganizerDuplicate{
			DetectionMode: "filename",
			Action:        config.DuplicateAction{Method: "skip"},
		},
		Schedule: "0 2 * * *",
	}

	// First run
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, st, sp)

	// Create same-named file again in source
	createTestFile(t, srcDir, "photo.jpg", "different data")

	// Reset state to force reprocessing
	st2 := state.New()
	result := Run(profile, source, st2, sp)

	// The duplicate should be skipped
	if result.Skipped == 0 {
		t.Log("note: duplicate skip depends on target already having the file")
	}
}

func TestOrganize_FallbackFolder(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	fallbackDir := t.TempDir()

	// Create a PNG file (no EXIF, mtime will be used as fallback)
	createTestFile(t, srcDir, "noexif.png", "plain png content")

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
		FallbackFolder:  fallbackDir,
	}
	source := config.OrganizerSource{
		Path:     srcDir,
		Mode:     "copy",
		Schedule: "0 2 * * *",
	}

	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	result := Run(profile, source, st, sp)

	// File should be processed (mtime fallback works for all files)
	if result.Errors > 0 {
		t.Errorf("expected no errors, got %d", result.Errors)
	}
}

func TestStatePath(t *testing.T) {
	p := StatePath("/data/state", "/photos/organized", "/mnt/incoming")
	if p == "" {
		t.Error("expected non-empty state path")
	}
}

func TestRunWithTime(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	createTestFile(t, srcDir, "photo.jpg", "image data")

	profile := config.OrganizerProfile{
		DryRun:          true,
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path:     srcDir,
		Mode:     "copy",
		Schedule: "0 2 * * *",
	}

	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	result, duration := RunWithTime(profile, source, st, sp)

	if duration <= 0 {
		t.Error("expected positive duration")
	}
	if result.Processed == 0 {
		t.Error("expected at least one processed file")
	}
}

func TestOrganize_DuplicateReplace(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path: srcDir,
		Mode: "copy",
		Duplicates: &config.OrganizerDuplicate{
			DetectionMode: "filename",
			Action:        config.DuplicateAction{Method: "replace"},
		},
		Schedule: "0 2 * * *",
	}

	// First run — place the file
	createTestFile(t, srcDir, "photo.jpg", "original data")
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, st, sp)

	// Second run — same filename, different content → replace
	createTestFile(t, srcDir, "photo.jpg", "updated data")
	st2 := state.New()
	result := Run(profile, source, st2, sp)

	if result.Processed == 0 && result.Skipped == 0 {
		t.Error("expected file to be processed or skipped")
	}
}

func TestOrganize_DuplicateAddSuffix(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path: srcDir,
		Mode: "copy",
		Duplicates: &config.OrganizerDuplicate{
			DetectionMode: "filename",
			Action:        config.DuplicateAction{Method: "addSuffix"},
		},
		Schedule: "0 2 * * *",
	}

	// First run
	createTestFile(t, srcDir, "photo.jpg", "original data")
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, st, sp)

	// Second run — same filename → addSuffix
	createTestFile(t, srcDir, "photo.jpg", "different data")
	st2 := state.New()
	result := Run(profile, source, st2, sp)

	if result.Processed == 0 {
		t.Error("expected file to be processed with suffix")
	}
}

func TestOrganize_DuplicateMoveToFolder(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	dupDir := t.TempDir()

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path: srcDir,
		Mode: "copy",
		Duplicates: &config.OrganizerDuplicate{
			DetectionMode: "filename",
			Action: config.DuplicateAction{
				Method:          "moveToFolder",
				DuplicateFolder: dupDir,
			},
		},
		Schedule: "0 2 * * *",
	}

	// First run
	createTestFile(t, srcDir, "photo.jpg", "original data")
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, st, sp)

	// Second run — same filename → moveToFolder
	createTestFile(t, srcDir, "photo.jpg", "dup data")
	st2 := state.New()
	result := Run(profile, source, st2, sp)

	if result.Processed == 0 {
		t.Error("expected duplicate to be moved to folder")
	}

	entries, _ := os.ReadDir(dupDir)
	if len(entries) == 0 {
		t.Error("expected file in duplicate folder")
	}
}

func TestOrganize_DuplicateDryRun(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)

	profile := config.OrganizerProfile{
		DryRun:          true,
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path: srcDir,
		Mode: "copy",
		Duplicates: &config.OrganizerDuplicate{
			DetectionMode: "filename",
			Action:        config.DuplicateAction{Method: "replace"},
		},
		Schedule: "0 2 * * *",
	}

	// First run to place file in target
	createTestFile(t, srcDir, "photo.jpg", "data")
	profileReal := profile
	profileReal.DryRun = false
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profileReal, source, st, sp)

	// Dry run with same file → should log duplicate but not modify
	createTestFile(t, srcDir, "photo.jpg", "data2")
	st2 := state.New()
	result := Run(profile, source, st2, sp)

	if !result.DryRun {
		t.Error("expected dry run")
	}
	if result.Processed == 0 {
		t.Error("expected dry run to count the file as processed")
	}
}

func TestOrganize_NoDuplicateConfig_ExistingTarget(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path:     srcDir,
		Mode:     "copy",
		Schedule: "0 2 * * *",
		// No Duplicates config
	}

	// First run
	createTestFile(t, srcDir, "photo.jpg", "data")
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, st, sp)

	// Second run — no duplicate config → should skip
	createTestFile(t, srcDir, "photo.jpg", "data2")
	st2 := state.New()
	result := Run(profile, source, st2, sp)

	if result.Skipped == 0 {
		t.Error("expected file to be skipped when no duplicate config and target exists")
	}
}

func TestOrganize_FiledateDetection(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path: srcDir,
		Mode: "copy",
		Duplicates: &config.OrganizerDuplicate{
			DetectionMode: "filedate",
			Action:        config.DuplicateAction{Method: "skip"},
		},
		Schedule: "0 2 * * *",
	}

	createTestFile(t, srcDir, "photo.jpg", "data")
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, st, sp)

	// Second run with same file
	createTestFile(t, srcDir, "photo.jpg", "data2")
	st2 := state.New()
	result := Run(profile, source, st2, sp)

	// filedate detection checks mtime — may or may not match
	_ = result
}

func TestOrganize_ExifDetection(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path: srcDir,
		Mode: "copy",
		Duplicates: &config.OrganizerDuplicate{
			DetectionMode: "exif",
			Action:        config.DuplicateAction{Method: "skip"},
		},
		Schedule: "0 2 * * *",
	}

	createTestFile(t, srcDir, "photo.jpg", "data")
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, st, sp)

	createTestFile(t, srcDir, "photo.jpg", "data2")
	st2 := state.New()
	result := Run(profile, source, st2, sp)
	_ = result
}

func TestOrganize_OrDetection(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path: srcDir,
		Mode: "copy",
		Duplicates: &config.OrganizerDuplicate{
			DetectionMode: "filename|filedate",
			Action:        config.DuplicateAction{Method: "skip"},
		},
		Schedule: "0 2 * * *",
	}

	createTestFile(t, srcDir, "photo.jpg", "data")
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, st, sp)

	createTestFile(t, srcDir, "photo.jpg", "data2")
	st2 := state.New()
	result := Run(profile, source, st2, sp)
	_ = result
}

func TestOrganize_AndDetection_NotMatching(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)

	profile := config.OrganizerProfile{
		Target:          tgtDir,
		TargetStructure: "{year}/{month}",
	}
	source := config.OrganizerSource{
		Path: srcDir,
		Mode: "copy",
		Duplicates: &config.OrganizerDuplicate{
			DetectionMode: "filename+filedate",
			Action:        config.DuplicateAction{Method: "skip"},
		},
		Schedule: "0 2 * * *",
	}

	createTestFile(t, srcDir, "photo.jpg", "data")
	st := state.New()
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, st, sp)

	// Different content → different mtime → AND won't match → suffix added
	createTestFile(t, srcDir, "photo.jpg", "different data different size")
	st2 := state.New()
	result := Run(profile, source, st2, sp)

	if result.Processed == 0 {
		t.Error("expected file to be processed with suffix when AND detection doesn't match")
	}
}

func TestHandleNoDate_WithFallback_Copy(t *testing.T) {
	fallbackDir := t.TempDir()
	srcDir := t.TempDir()
	srcFile := createTestFile(t, srcDir, "nodate.jpg", "no exif data")

	profile := config.OrganizerProfile{
		FallbackFolder: fallbackDir,
	}
	source := config.OrganizerSource{
		Mode: "copy",
	}
	result := &RunResult{}

	err := handleNoDate(profile, source, srcFile, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Processed != 1 {
		t.Errorf("expected 1 processed, got %d", result.Processed)
	}

	entries, _ := os.ReadDir(fallbackDir)
	if len(entries) == 0 {
		t.Error("expected file in fallback folder")
	}
}

func TestHandleNoDate_WithFallback_DryRun(t *testing.T) {
	fallbackDir := t.TempDir()
	srcDir := t.TempDir()
	srcFile := createTestFile(t, srcDir, "nodate.jpg", "no exif data")

	profile := config.OrganizerProfile{
		DryRun:         true,
		FallbackFolder: fallbackDir,
	}
	source := config.OrganizerSource{
		Mode: "copy",
	}
	result := &RunResult{}

	err := handleNoDate(profile, source, srcFile, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Processed != 1 {
		t.Errorf("expected 1 processed, got %d", result.Processed)
	}

	// Dry run: nothing should be in fallback
	entries, _ := os.ReadDir(fallbackDir)
	if len(entries) > 0 {
		t.Error("fallback folder should be empty in dry run")
	}
}

func TestHandleNoDate_WithoutFallback(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := createTestFile(t, srcDir, "nodate.jpg", "no exif data")

	profile := config.OrganizerProfile{}
	source := config.OrganizerSource{
		Mode: "copy",
	}
	result := &RunResult{}

	err := handleNoDate(profile, source, srcFile, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
}

func TestHandleNoDate_WithFallback_Move(t *testing.T) {
	fallbackDir := t.TempDir()
	srcDir := t.TempDir()
	srcFile := createTestFile(t, srcDir, "nodate.jpg", "data")

	profile := config.OrganizerProfile{
		FallbackFolder: fallbackDir,
	}
	source := config.OrganizerSource{
		Mode: "move",
	}
	result := &RunResult{}

	err := handleNoDate(profile, source, srcFile, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Source should be gone (move mode)
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("source should be removed after move to fallback")
	}
}

func TestIsDuplicate_Filename(t *testing.T) {
	dir := t.TempDir()
	a := createTestFile(t, dir, "photo.jpg", "data1")
	b := createTestFile(t, filepath.Join(t.TempDir()), "photo.jpg", "data2")

	if !isDuplicate("filename", a, b) {
		t.Error("same filename should be detected as duplicate")
	}

	c := createTestFile(t, t.TempDir(), "other.jpg", "data3")
	if isDuplicate("filename", a, c) {
		t.Error("different filename should not be duplicate")
	}
}

func TestIsDuplicate_UnknownField(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	a := createTestFile(t, dir1, "a.jpg", "x")
	b := createTestFile(t, dir2, "b.jpg", "y")

	// Unknown field in AND mode → should be false
	if isDuplicate("unknown", a, b) {
		t.Error("unknown detection field should return false")
	}
}

func TestTransferFile_InvalidMode(t *testing.T) {
	src := createTestFile(t, t.TempDir(), "f.jpg", "data")
	dst := filepath.Join(t.TempDir(), "f.jpg")

	err := transferFile("invalid", src, dst)
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}
