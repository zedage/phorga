package metadata

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zedage/phorga/internal/testutil"
)

// Spec: ORG-002
func TestExtractDate_HEICUsesEXIF(t *testing.T) {
	taken := time.Date(2023, 8, 19, 17, 42, 10, 0, time.UTC)
	for _, inIdat := range []bool{false, true} {
		dir := t.TempDir()
		// The filename date must lose against the embedded one.
		path := testutil.WriteHEIC(t, dir, "IMG_20240101_000000.HEIC", taken, inIdat)
		old := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
		os.Chtimes(path, old, old)

		got, method, err := ExtractDate(path)
		if err != nil {
			t.Fatalf("inIdat=%v: %v", inIdat, err)
		}
		if method != "exif" {
			t.Errorf("inIdat=%v: method = %q, want exif", inIdat, method)
		}
		if got.Year() != 2023 || got.Month() != time.August || got.Day() != 19 || got.Hour() != 17 {
			t.Errorf("inIdat=%v: date = %v, want 2023-08-19 17:42", inIdat, got)
		}
	}
}

// Spec: ORG-002
func TestExtractDate_VideoUsesQuickTimeCreationTime(t *testing.T) {
	created := time.Date(2019, 7, 15, 12, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		version  byte
		moovLast bool
	}{{0, false}, {0, true}, {1, true}} {
		dir := t.TempDir()
		path := testutil.WriteMP4(t, dir, "clip.mp4", created, tc.version, tc.moovLast)
		old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		os.Chtimes(path, old, old)

		got, method, err := ExtractDate(path)
		if err != nil {
			t.Fatalf("%+v: %v", tc, err)
		}
		if method != "quicktime" {
			t.Errorf("%+v: method = %q, want quicktime", tc, method)
		}
		if !got.Equal(created) {
			t.Errorf("%+v: date = %v, want %v", tc, got, created)
		}
		if got.Location() != time.Local {
			t.Errorf("%+v: date must be in the local zone, got %v", tc, got.Location())
		}
	}
}

// Spec: ORG-002
func TestExtractDate_VideoWithoutCreationTimeFallsBack(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "2018", "05")
	os.MkdirAll(sub, 0755)
	path := testutil.WriteMP4(t, sub, "clip.mov", time.Time{}, 0, false)

	got, method, err := ExtractDate(path)
	if err != nil {
		t.Fatal(err)
	}
	if method != "path" || got.Year() != 2018 || got.Month() != time.May {
		t.Errorf("got %v via %s, want May 2018 via path", got, method)
	}
}

// Spec: ORG-011, DUP-002
func TestExtractEmbeddedDate_HEICAndVideo(t *testing.T) {
	dir := t.TempDir()
	taken := time.Date(2022, 3, 4, 5, 6, 7, 0, time.UTC)

	heic := testutil.WriteHEIC(t, dir, "a.heic", taken, false)
	if got, err := ExtractEmbeddedDate(heic); err != nil || got.Day() != 4 || got.Hour() != 5 {
		t.Errorf("HEIC: got %v, %v", got, err)
	}
	video := testutil.WriteMP4(t, dir, "b.mov", taken, 0, true)
	if got, err := ExtractEmbeddedDate(video); err != nil || !got.Equal(taken) {
		t.Errorf("video: got %v, %v", got, err)
	}

	noExif := testutil.WriteHEIC(t, dir, "IMG_20220304_050607.heic", time.Time{}, false)
	if _, err := ExtractEmbeddedDate(noExif); !errors.Is(err, ErrNoEmbeddedDate) {
		t.Errorf("HEIC without Exif item: expected ErrNoEmbeddedDate, got %v", err)
	}
	zero := testutil.WriteMP4(t, dir, "VID_20220304_050607.mp4", time.Time{}, 1, false)
	if _, err := ExtractEmbeddedDate(zero); !errors.Is(err, ErrNoEmbeddedDate) {
		t.Errorf("video with creation time 0: expected ErrNoEmbeddedDate, got %v", err)
	}
}

// Spec: ORG-002
func TestExtractEmbeddedDate_BrokenFilesAreNoDate(t *testing.T) {
	dir := t.TempDir()
	good := testutil.WriteHEIC(t, dir, "good.heic", time.Date(2022, 3, 4, 5, 6, 7, 0, time.UTC), false)
	data, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	// Every truncation of a valid file must fail cleanly, never panic.
	for n := 0; n < len(data); n += 7 {
		p := filepath.Join(dir, "cut.heic")
		os.WriteFile(p, data[:n], 0644)
		if _, err := ExtractEmbeddedDate(p); err == nil && n < len(data)/2 {
			t.Errorf("truncated to %d bytes: expected an error", n)
		}
	}
	// A box that claims to be larger than the file.
	p := filepath.Join(dir, "huge.mp4")
	os.WriteFile(p, []byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0xFF, 0xFF, 0xFF, 0xFF, 'm', 'o', 'o', 'v'}, 0644)
	if _, err := ExtractEmbeddedDate(p); err == nil {
		t.Error("expected an error for a box larger than the file")
	}
}

// Spec: ORG-002, ORG-003
func TestResolveDate_WithoutModTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scan.png")
	os.WriteFile(path, []byte("no date anywhere"), 0644)

	if _, method, err := ResolveDate(path, true); err != nil || method != "mtime" {
		t.Errorf("with mtime: method %q, err %v", method, err)
	}
	if _, _, err := ResolveDate(path, false); !errors.Is(err, ErrNoDate) {
		t.Errorf("without mtime: expected ErrNoDate, got %v", err)
	}

	named := filepath.Join(dir, "IMG_20240715_143000.png")
	os.WriteFile(named, []byte("x"), 0644)
	if got, method, err := ResolveDate(named, false); err != nil || method != "filename" || got.Year() != 2024 {
		t.Errorf("filename date must still be used: %v %q %v", got, method, err)
	}
}
