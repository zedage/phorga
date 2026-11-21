package organizer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/state"
	"github.com/zedage/phorga/internal/testutil"
)

func moveSource(path string, dup *config.OrganizerDuplicate) config.OrganizerSource {
	return config.OrganizerSource{Path: path, Mode: "move", Schedule: "0 2 * * *", Duplicates: dup}
}

func listFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

// Spec: ORG-024
func TestOrganize_FileAlreadyInPlaceStaysUntouched(t *testing.T) {
	for _, method := range []string{"replace", "addSuffix", "moveToFolder", "skip"} {
		t.Run(method, func(t *testing.T) {
			root := t.TempDir() // target inside (here: equal to) the source
			dupFolder := t.TempDir()
			sub := filepath.Join(root, "2019", "07")
			os.MkdirAll(sub, 0755)
			path := createTestFile(t, sub, "IMG_0001.jpg", "the only copy")
			taken := time.Date(2019, 7, 15, 14, 30, 0, 0, time.UTC)
			os.Chtimes(path, taken, taken)

			profile := config.OrganizerProfile{Target: root, TargetStructure: "{year}/{month}"}
			source := moveSource(root, &config.OrganizerDuplicate{
				DetectionMode: "filename+filedate",
				Action:        config.DuplicateAction{Method: method, DuplicateFolder: dupFolder},
			})
			sp := filepath.Join(t.TempDir(), "state.json")
			for run := 1; run <= 2; run++ {
				result := Run(context.Background(), profile, source, state.New(), sp)
				if result.Errors != 0 || result.Processed != 0 || result.Skipped != 1 {
					t.Errorf("run %d: %+v, want 1 skipped", run, result)
				}
			}
			if files := listFiles(t, root); len(files) != 1 || files[0] != "2019/07/IMG_0001.jpg" {
				t.Errorf("files = %v, want only 2019/07/IMG_0001.jpg", files)
			}
			if got, _ := os.ReadFile(path); string(got) != "the only copy" {
				t.Errorf("content = %q", got)
			}
			if files := listFiles(t, dupFolder); len(files) != 0 {
				t.Errorf("duplicate folder must stay empty, has %v", files)
			}
		})
	}
}

// Spec: ORG-002, ORG-003
func TestOrganize_IgnoreModTimeSendsUndatedFilesToFallback(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	fallback := t.TempDir()
	createTestFile(t, srcDir, "scan.png", "no date anywhere")
	createTestFile(t, srcDir, "IMG_20240715_143000.jpg", "dated by name")

	profile := config.OrganizerProfile{
		Target: tgtDir, TargetStructure: "{year}/{month}",
		FallbackFolder: fallback, IgnoreModTime: true,
	}
	source := moveSource(srcDir, nil)
	result := Run(context.Background(), profile, source, state.New(), filepath.Join(stateDir, "s.json"))

	if result.Processed != 2 || result.Errors != 0 {
		t.Fatalf("result = %+v", result)
	}
	if files := listFiles(t, fallback); len(files) != 1 || files[0] != "scan.png" {
		t.Errorf("fallback = %v, want scan.png", files)
	}
	if files := listFiles(t, tgtDir); len(files) != 1 || files[0] != "2024/07/IMG_20240715_143000.jpg" {
		t.Errorf("target = %v", files)
	}
}

// Spec: ORG-003
func TestOrganize_IgnoreModTimeWithoutFallbackSkips(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	createTestFile(t, srcDir, "scan.png", "no date anywhere")

	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}", IgnoreModTime: true}
	result := Run(context.Background(), profile, moveSource(srcDir, nil), state.New(), filepath.Join(stateDir, "s.json"))

	if result.Skipped != 1 || result.Processed != 0 || result.Errors != 0 {
		t.Errorf("result = %+v, want 1 skipped", result)
	}
	if files := listFiles(t, srcDir); len(files) != 1 {
		t.Errorf("the file must stay in the source, source has %v", files)
	}
}

// Spec: ORG-001, ORG-002
func TestOrganize_HEICAndVideoUseEmbeddedDates(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	testutil.WriteHEIC(t, srcDir, "IMG_4711.HEIC", time.Date(2023, 8, 19, 17, 42, 10, 0, time.UTC), false)
	testutil.WriteMP4(t, srcDir, "IMG_4712.MOV", time.Date(2021, 12, 24, 12, 0, 0, 0, time.UTC), 0, true)
	// The mtime is the day of the copy to the NAS and must not be used.
	now := time.Now()
	for _, n := range []string{"IMG_4711.HEIC", "IMG_4712.MOV"} {
		os.Chtimes(filepath.Join(srcDir, n), now, now)
	}

	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}"}
	result := Run(context.Background(), profile, moveSource(srcDir, nil), state.New(), filepath.Join(stateDir, "s.json"))
	if result.Processed != 2 || result.Errors != 0 {
		t.Fatalf("result = %+v", result)
	}
	want := map[string]bool{"2023/08/IMG_4711.HEIC": true, "2021/12/IMG_4712.MOV": true}
	for _, f := range listFiles(t, tgtDir) {
		if !want[f] {
			t.Errorf("unexpected file %s", f)
		}
		delete(want, f)
	}
	for f := range want {
		t.Errorf("missing %s", f)
	}
}

// Spec: ORG-011
func TestOrganize_ExifDetectionComparesVideoCreationTime(t *testing.T) {
	srcDir, tgtDir, stateDir := setupTestDirs(t)
	dupFolder := t.TempDir()
	created := time.Date(2021, 12, 24, 12, 0, 0, 0, time.UTC)
	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}/{month}"}
	source := moveSource(srcDir, &config.OrganizerDuplicate{
		DetectionMode: "exif",
		Action:        config.DuplicateAction{Method: "moveToFolder", DuplicateFolder: dupFolder},
	})
	sp := filepath.Join(stateDir, "s.json")

	testutil.WriteMP4(t, srcDir, "clip.mp4", created, 0, false)
	Run(context.Background(), profile, source, state.New(), sp)

	// Same name, same creation time: a duplicate.
	testutil.WriteMP4(t, srcDir, "clip.mp4", created, 1, true)
	Run(context.Background(), profile, source, state.New(), sp)
	if files := listFiles(t, dupFolder); len(files) != 1 {
		t.Errorf("duplicate folder = %v, want the second clip", files)
	}
}
