package organizer

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/state"
)

// runWithExclude creates files (slash paths) in a fresh source, organizes
// them by copy into {year} and returns the result and the organized names.
func runWithExclude(t *testing.T, srcDir string, exclude []string, files ...string) (RunResult, []string) {
	t.Helper()
	tgtDir, stateDir := t.TempDir(), t.TempDir()
	for _, f := range files {
		path := filepath.Join(srcDir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		createTestFile(t, filepath.Dir(path), filepath.Base(path), "data "+f)
	}
	profile := config.OrganizerProfile{Target: tgtDir, TargetStructure: "{year}"}
	source := config.OrganizerSource{Path: srcDir, Mode: "copy", Schedule: "0 2 * * *", Exclude: exclude}
	result := Run(context.Background(), profile, source, state.New(), filepath.Join(stateDir, "state.json"))

	var organized []string
	filepath.WalkDir(tgtDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			organized = append(organized, d.Name())
		}
		return nil
	})
	sort.Strings(organized)
	return result, organized
}

// Spec: ORG-022
func TestOrganize_SkipsSystemAndHiddenEntries(t *testing.T) {
	result, organized := runWithExclude(t, t.TempDir(), nil,
		"IMG_1234.JPG",
		"@eaDir/IMG_1234.JPG/SYNO_PHOTO_THUMB_SM.jpg",
		"#recycle/old.jpg",
		"._IMG_1234.JPG",
		".thumbnails/t.jpg",
	)
	if result.Errors != 0 || result.Processed != 1 {
		t.Errorf("processed = %d, errors = %d, want 1 and 0", result.Processed, result.Errors)
	}
	if len(organized) != 1 || organized[0] != "IMG_1234.JPG" {
		t.Errorf("organized = %v, want [IMG_1234.JPG]", organized)
	}
}

// Spec: ORG-022
func TestOrganize_HiddenSourceRootIsWalked(t *testing.T) {
	src := filepath.Join(t.TempDir(), ".import")
	_, organized := runWithExclude(t, src, nil, "a.jpg")
	if len(organized) != 1 || organized[0] != "a.jpg" {
		t.Errorf("organized = %v, want [a.jpg]", organized)
	}
}

// Spec: ORG-023
func TestOrganize_ExcludePatterns(t *testing.T) {
	tests := []struct {
		exclude []string
		files   []string
		want    []string
	}{
		{[]string{"Screenshots"}, []string{"2024/Screenshots/a.png", "2024/b.jpg"}, []string{"b.jpg"}},
		{[]string{"*_edited.jpg"}, []string{"a.jpg", "a_edited.jpg"}, []string{"a.jpg"}},
		{[]string{"2019/Scans"}, []string{"2019/Scans/x.jpg", "2020/Scans/y.jpg"}, []string{"y.jpg"}},
		{[]string{"screenshots"}, []string{"Screenshots/a.png", "c.jpg"}, []string{"c.jpg"}},
	}
	for _, tt := range tests {
		result, organized := runWithExclude(t, t.TempDir(), tt.exclude, tt.files...)
		if result.Errors != 0 {
			t.Errorf("exclude %v: errors = %d", tt.exclude, result.Errors)
		}
		if len(organized) != len(tt.want) || (len(organized) > 0 && organized[0] != tt.want[0]) {
			t.Errorf("exclude %v: organized = %v, want %v", tt.exclude, organized, tt.want)
		}
	}
}
