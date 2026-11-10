package metadata

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Spec: ORG-016
func TestIsSupported(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"photo.jpg", true},
		{"photo.JPEG", true},
		{"video.mp4", true},
		{"video.MOV", true},
		{"photo.heic", true},
		{"photo.cr2", true},
		{"photo.nef", true},
		{"photo.arw", true},
		{"document.txt", false},
		{"archive.zip", false},
		{"noext", false},
	}
	for _, tt := range tests {
		if got := IsSupported(tt.path); got != tt.want {
			t.Errorf("IsSupported(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// Spec: ORG-005, ORG-006, ORG-007, ORG-008
func TestResolveTargetPath(t *testing.T) {
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	febDate := time.Date(2024, 2, 5, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		structure string
		date      time.Time
		want      string
	}{
		// English
		{"{year}/{month}", date, "2024/01"},
		{"{year}/{monthWithName}", date, "2024/01-january"},
		{"{year}/{month}/{day}", date, "2024/01/15"},
		{"{year}", date, "2024"},
		{"photos/{year}/{monthWithName}/{day}", date, "photos/2024/01-january/15"},
		{"{monthNumber}/{dayNumber}", febDate, "02/05"},

		// German
		{"{jahr}/{monat}", date, "2024/januar"},
		{"{jahr}/{Monat}", date, "2024/Januar"},
		{"{jahr}/{tag}", date, "2024/15"},
		{"{year}/{monthNumber}-{Monat}/{dayNumber}", febDate, "2024/02-Februar/05"},

		// French
		{"{année}/{mois}", date, "2024/janvier"},
		{"{année}/{Mois}", date, "2024/Janvier"},
		{"{année}/{jour}", date, "2024/15"},

		// Italian
		{"{anno}/{mese}", date, "2024/gennaio"},
		{"{anno}/{Mese}", date, "2024/Gennaio"},
		{"{anno}/{giorno}", date, "2024/15"},

		// Case sensitivity for month names
		{"{year}/{monat}", febDate, "2024/februar"},
		{"{year}/{Monat}", febDate, "2024/Februar"},
		{"{year}/{MONAT}", febDate, "2024/FEBRUAR"},
	}
	for _, tt := range tests {
		got, err := ResolveTargetPath(tt.structure, tt.date)
		if err != nil {
			t.Errorf("ResolveTargetPath(%q, %v) returned error: %v", tt.structure, tt.date, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ResolveTargetPath(%q, %v) = %q, want %q", tt.structure, tt.date, got, tt.want)
		}
	}
}

// Spec: ORG-009
func TestResolveTargetPath_InvalidToken(t *testing.T) {
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		structure string
		wantError bool
	}{
		{"{invalidToken}", true},
		{"{year}/{invalid}", true},
		{"{year}/{monthh}", true},
		{"{year}/{monat}", false}, // valid
		{"{year}/{moisx}", true},  // invalid French
		{"{year}/{year}", false},  // valid - same token twice is ok
	}

	for _, tt := range tests {
		_, err := ResolveTargetPath(tt.structure, date)
		if tt.wantError && err == nil {
			t.Errorf("ResolveTargetPath(%q) expected error, but got none", tt.structure)
		}
		if !tt.wantError && err != nil {
			t.Errorf("ResolveTargetPath(%q) expected no error, got: %v", tt.structure, err)
		}
	}
}

// TestResolveTargetPath_EdgeDates tests edge cases like Feb 29, Dec 31, etc.
// Spec: ORG-008
func TestResolveTargetPath_EdgeDates(t *testing.T) {
	tests := []struct {
		name      string
		date      time.Time
		structure string
		want      string
	}{
		{"Feb 5", time.Date(2024, 2, 5, 0, 0, 0, 0, time.UTC), "{year}/{monthNumber}/{dayNumber}", "2024/02/05"},
		{"Feb 29 leap", time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), "{year}/{monthNumber}/{dayNumber}", "2024/02/29"},
		{"Dec 31", time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), "{year}/{monthNumber}/{dayNumber}", "2024/12/31"},
		{"Jan 1", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "{year}/{monthNumber}/{dayNumber}", "2024/01/01"},
		{"German March", time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC), "{Monat}", "März"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTargetPath(tt.structure, tt.date)
			if err != nil {
				t.Errorf("ResolveTargetPath(%q, %v) returned error: %v", tt.structure, tt.date, err)
				return
			}
			if got != tt.want {
				t.Errorf("ResolveTargetPath(%q, %v) = %q, want %q", tt.structure, tt.date, got, tt.want)
			}
		})
	}
}

// Spec: ORG-002
func TestExtractDate_FallbackToMtime(t *testing.T) {
	// Create a temp file (no EXIF, no date in filename) — should fall back to mtime
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	if err := os.WriteFile(path, []byte("not an image"), 0644); err != nil {
		t.Fatal(err)
	}

	date, method, err := ExtractDate(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != "mtime" {
		t.Errorf("expected method 'mtime', got %q", method)
	}
	if date.IsZero() {
		t.Error("expected non-zero date")
	}
}

// Spec: ORG-002
func TestExtractFilenameDate(t *testing.T) {
	tests := []struct {
		filename string
		wantDate string // "2006-01-02" or "2006-01-02T15:04:05"
		wantOk   bool
	}{
		// IMG pattern without separators
		{"IMG20240802161602.heic", "2024-08-02T16:16:02", true},
		{"VID20240802161602.mp4", "2024-08-02T16:16:02", true},
		{"PXL20240802161602.jpg", "2024-08-02T16:16:02", true},
		{"MVIMG20240802161602.jpg", "2024-08-02T16:16:02", true},

		// IMG pattern with underscores
		{"IMG_20240802_161602.jpg", "2024-08-02T16:16:02", true},
		{"VID_20240802_161602.mp4", "2024-08-02T16:16:02", true},

		// WhatsApp patterns (IMG-YYYYMMDD-WA####)
		{"IMG-20230706-WA0001.jpg", "2023-07-06", true},
		{"VID-20230706-WA0042.mp4", "2023-07-06", true},
		{"IMG-20241225-WA0000.jpg", "2024-12-25", true},

		// Date with dashes and time
		{"2024-08-02_16-15-02.jpg", "2024-08-02T16:15:02", true},
		{"2024-08-02 16-15-02.jpg", "2024-08-02T16:15:02", true},

		// Date with underscored time
		{"20240802_161602.jpg", "2024-08-02T16:16:02", true},

		// Date only
		{"2024-08-02.jpg", "2024-08-02", true},

		// No date in filename
		{"vacation-beach.jpg", "", false},
		{"notes.txt", "", false},
		{"photo.jpg", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.filename)
			os.WriteFile(path, []byte("dummy"), 0644)

			got, err := extractFilenameDate(path)
			if tt.wantOk {
				if err != nil {
					t.Fatalf("expected date from %q, got error: %v", tt.filename, err)
				}
				var want time.Time
				if len(tt.wantDate) > 10 {
					want, _ = time.Parse("2006-01-02T15:04:05", tt.wantDate)
				} else {
					want, _ = time.Parse("2006-01-02", tt.wantDate)
				}
				if !got.Equal(want) {
					t.Errorf("extractFilenameDate(%q) = %v, want %v", tt.filename, got, want)
				}
			} else {
				if err == nil {
					t.Errorf("expected no date from %q, but got %v", tt.filename, got)
				}
			}
		})
	}
}

// Spec: ORG-002
func TestExtractDate_FilenameBeforeMtime(t *testing.T) {
	// File with date in filename but no EXIF → should use "filename" method
	dir := t.TempDir()
	path := filepath.Join(dir, "IMG20240802161602.heic")
	os.WriteFile(path, []byte("not a real heic"), 0644)

	date, method, err := ExtractDate(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != "filename" {
		t.Errorf("expected method 'filename', got %q", method)
	}
	if date.Year() != 2024 || date.Month() != 8 || date.Day() != 2 {
		t.Errorf("expected 2024-08-02, got %v", date)
	}
}

// Spec: ORG-002
func TestExtractPathDate(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantYear  int
		wantMonth time.Month
		wantDay   int
		wantOk    bool
	}{
		// Year + month as separate folders
		{"year/month forward slash", "/photos/2019/07/video.mp4", 2019, 7, 1, true},
		{"year/month backslash", `D:\photos\2019\07\video.mp4`, 2019, 7, 1, true},
		{"year/month deep path", "/data/backup/photos/2023/12/subdir/clip.mov", 2023, 12, 1, true},

		// Year-month as single folder name
		{"year-month dash", "/photos/2019-07/video.mp4", 2019, 7, 1, true},
		{"year_month underscore", "/photos/2019_07/video.mp4", 2019, 7, 1, true},

		// Full date as folder name
		{"full date dash", "/photos/2023-05-15/video.mp4", 2023, 5, 15, true},
		{"full date underscore", "/photos/2023_05_15/video.mp4", 2023, 5, 15, true},

		// Year/month/day as separate folders
		{"year/month/day", "/photos/2023/05/15/video.mp4", 2023, 5, 15, true},

		// English month names
		{"english January", "/photos/2019/January/video.mp4", 2019, 1, 1, true},
		{"english december lower", "/photos/2020/december/video.mp4", 2020, 12, 1, true},
		{"english May", "/photos/2021/May/video.mp4", 2021, 5, 1, true},
		{"english short jan", "/photos/2022/jan/video.mp4", 2022, 1, 1, true},
		{"english short aug", "/photos/2022/aug/video.mp4", 2022, 8, 1, true},

		// German month names
		{"german Januar", "/photos/2019/Januar/video.mp4", 2019, 1, 1, true},
		{"german März", "/photos/2020/März/video.mp4", 2020, 3, 1, true},
		{"german maerz", "/photos/2020/maerz/video.mp4", 2020, 3, 1, true},
		{"german Mai", "/photos/2021/Mai/video.mp4", 2021, 5, 1, true},
		{"german Juni", "/photos/2023/Juni/video.mp4", 2023, 6, 1, true},
		{"german Oktober", "/photos/2023/Oktober/video.mp4", 2023, 10, 1, true},
		{"german Dezember", "/photos/2024/Dezember/video.mp4", 2024, 12, 1, true},

		// Year-monthname as single folder
		{"year-monthname dash", "/photos/2019-January/video.mp4", 2019, 1, 1, true},
		{"year_monthname underscore", "/photos/2019_Mai/video.mp4", 2019, 5, 1, true},

		// Year only
		{"year only", "/photos/2019/video.mp4", 2019, 1, 1, true},

		// Deepest match wins
		{"deepest match wins", "/photos/2020/03/archive/2023/11/file.mp4", 2023, 11, 1, true},

		// No date in path
		{"no date", "/photos/vacation/video.mp4", 0, 0, 0, false},

		// Invalid month falls back to year-only
		{"invalid month 13", "/photos/2019/13/video.mp4", 2019, 1, 1, true},
		{"invalid month 00", "/photos/2019/00/video.mp4", 2019, 1, 1, true},

		// Year out of range
		{"year too old", "/photos/1900/07/video.mp4", 0, 0, 0, false},

		// Random word that is not a month name — falls back to year only
		{"non-month word", "/photos/2019/vacation/video.mp4", 2019, 1, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractPathDate(tt.path)
			if tt.wantOk {
				if err != nil {
					t.Fatalf("expected date from path %q, got error: %v", tt.path, err)
				}
				if got.Year() != tt.wantYear || got.Month() != tt.wantMonth {
					t.Errorf("extractPathDate(%q) = %v, want year=%d month=%d", tt.path, got, tt.wantYear, tt.wantMonth)
				}
				if tt.wantDay > 1 && got.Day() != tt.wantDay {
					t.Errorf("extractPathDate(%q) day = %d, want %d", tt.path, got.Day(), tt.wantDay)
				}
			} else {
				if err == nil {
					t.Errorf("expected no date from path %q, but got %v", tt.path, got)
				}
			}
		})
	}
}

// Spec: ORG-002
func TestExtractDate_PathBeforeMtime(t *testing.T) {
	// File with date in path but no EXIF and no date in filename → should use "path" method
	dir := filepath.Join(t.TempDir(), "photos", "2019", "07")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "random-video.mp4")
	os.WriteFile(path, []byte("not a real video"), 0644)

	date, method, err := ExtractDate(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != "path" {
		t.Errorf("expected method 'path', got %q", method)
	}
	if date.Year() != 2019 || date.Month() != 7 {
		t.Errorf("expected 2019-07, got %v", date)
	}
}

func TestExtractDate_FileNotFound(t *testing.T) {
	_, _, err := ExtractDate("/nonexistent/file.jpg")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}
