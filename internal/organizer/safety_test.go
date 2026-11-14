package organizer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/state"
)

var july2019 = time.Date(2019, 7, 15, 14, 30, 0, 0, time.UTC)

// createDatedFile writes a file whose only date is its modification time.
func createDatedFile(t *testing.T, dir, name, content string, mtime time.Time) string {
	t.Helper()
	path := createTestFile(t, dir, name, content)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return path
}

// fileContents returns the sorted contents of all files directly in dir.
func fileContents(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(data))
	}
	sort.Strings(out)
	return out
}

// Spec: ORG-018
func TestOrganize_KeepsModTime(t *testing.T) {
	for _, mode := range []string{"copy", "move"} {
		t.Run(mode, func(t *testing.T) {
			srcDir, tgtDir, stateDir := setupTestDirs(t)
			createDatedFile(t, srcDir, "clip.mov", "video data", july2019)

			profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}"}
			source := config.OrganizerSource{Path: srcDir, Mode: mode, Schedule: "0 2 * * *"}
			result := Run(profile, source, state.New(), filepath.Join(stateDir, "state.json"))
			if result.Errors != 0 || result.Processed != 1 {
				t.Fatalf("processed = %d, errors = %d", result.Processed, result.Errors)
			}

			matches, _ := filepath.Glob(filepath.Join(tgtDir, "*", "*", "clip.mov"))
			if len(matches) != 1 {
				t.Fatalf("expected one organized file, got %v", matches)
			}
			info := mustStat(t, matches[0])
			if d := info.ModTime().Sub(july2019); d < -2*time.Second || d > 2*time.Second {
				t.Errorf("organized file mtime = %v, want %v", info.ModTime(), july2019)
			}
		})
	}
}

// Spec: ORG-012, ORG-020
func TestOrganize_MoveToFolderNeverOverwrites(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	dupDir := t.TempDir()
	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}"}
	source := config.OrganizerSource{
		Path: srcDir,
		Mode: "move",
		Duplicates: &config.OrganizerDuplicate{
			DetectionMode: "filename",
			Action:        config.DuplicateAction{Method: "moveToFolder", DuplicateFolder: dupDir},
		},
		Schedule: "0 2 * * *",
	}
	sp := filepath.Join(stateDir, "state.json")

	// Three different photos that share one name, e.g. from three cameras.
	for _, content := range []string{"first", "second", "third"} {
		createDatedFile(t, srcDir, "IMG_0001.jpg", content, july2019)
		if result := Run(profile, source, state.New(), sp); result.Errors != 0 {
			t.Fatalf("%s: errors = %d", content, result.Errors)
		}
	}

	got := fileContents(t, dupDir)
	if fmt.Sprint(got) != "[second third]" {
		t.Errorf("duplicate folder holds %v, want both later files", got)
	}
}

// Spec: ORG-003, ORG-020
func TestHandleNoDate_FallbackNeverOverwrites(t *testing.T) {
	fallbackDir := t.TempDir()
	profile := config.OrganizerProfile{FallbackFolder: fallbackDir}
	source := config.OrganizerSource{Mode: "move"}

	for _, content := range []string{"first", "second"} {
		src := createTestFile(t, t.TempDir(), "nodate.jpg", content)
		if err := handleNoDate(profile, source, src, mustStat(t, src), state.New(), &RunResult{}); err != nil {
			t.Fatal(err)
		}
	}

	got := fileContents(t, fallbackDir)
	if fmt.Sprint(got) != "[first second]" {
		t.Errorf("fallback folder holds %v, want both files", got)
	}
}

// Spec: ORG-003, ORG-015
func TestHandleNoDate_CopyIsRemembered(t *testing.T) {
	fallbackDir := t.TempDir()
	profile := config.OrganizerProfile{FallbackFolder: fallbackDir}
	source := config.OrganizerSource{Mode: "copy"}
	src := createTestFile(t, t.TempDir(), "nodate.jpg", "data")
	info := mustStat(t, src)
	st := state.New()

	if err := handleNoDate(profile, source, src, info, st, &RunResult{}); err != nil {
		t.Fatal(err)
	}
	if !st.IsProcessed(src, info.Size(), info.ModTime()) {
		t.Error("a file copied to the fallback folder must be recorded in the state")
	}
}

// countStateSaves replaces saveState with a counting wrapper for one test.
func countStateSaves(t *testing.T, interval int) *int {
	t.Helper()
	calls := 0
	origSave, origInterval := saveState, stateSaveInterval
	saveState = func(s *state.State, path string) error {
		calls++
		return origSave(s, path)
	}
	stateSaveInterval = interval
	t.Cleanup(func() { saveState, stateSaveInterval = origSave, origInterval })
	return &calls
}

// Spec: ORG-015
func TestOrganize_SavesStateDuringRun(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	for i := 0; i < 5; i++ {
		createDatedFile(t, srcDir, fmt.Sprintf("photo%d.jpg", i), "data", july2019)
	}
	calls := countStateSaves(t, 2)

	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}"}
	source := config.OrganizerSource{Path: srcDir, Mode: "copy", Schedule: "0 2 * * *"}
	sp := filepath.Join(stateDir, "state.json")
	result := Run(profile, source, state.New(), sp)

	if result.Processed != 5 {
		t.Fatalf("processed = %d, want 5", result.Processed)
	}
	// After the 2nd and 4th file, plus once at the end.
	if *calls != 3 {
		t.Errorf("state saved %d times, want 3", *calls)
	}
	if got := len(state.Load(sp).ProcessedFiles); got != 5 {
		t.Errorf("saved state has %d files, want 5", got)
	}
}

// Spec: ORG-014, ORG-015
func TestOrganize_DryRunNeverSavesState(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	for i := 0; i < 3; i++ {
		createDatedFile(t, srcDir, fmt.Sprintf("photo%d.jpg", i), "data", july2019)
	}
	calls := countStateSaves(t, 1)

	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}", DryRun: true}
	source := config.OrganizerSource{Path: srcDir, Mode: "move", Schedule: "0 2 * * *"}
	sp := filepath.Join(stateDir, "state.json")
	Run(profile, source, state.New(), sp)

	if *calls != 0 {
		t.Errorf("dry run saved the state %d times", *calls)
	}
	if _, err := os.Stat(sp); !os.IsNotExist(err) {
		t.Errorf("dry run must not create a state file: %v", err)
	}
}
