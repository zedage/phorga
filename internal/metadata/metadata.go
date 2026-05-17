package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// filenameDatePatterns matches common date patterns in filenames.
var filenameDatePatterns = []struct {
	re     *regexp.Regexp
	layout string
}{
	// IMG20240802161602 or VID20240802161602
	{regexp.MustCompile(`(?:IMG|VID|PXL|MVIMG)(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})`), "20060102150405"},
	// IMG_20240802_161602 or IMG-20240802-WA0001 (WhatsApp)
	{regexp.MustCompile(`(?:IMG|VID|PXL|MVIMG)[_-](\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`), "20060102150405"},
	// IMG-20230706-WA0001 (WhatsApp, date only — no time in WA suffix)
	{regexp.MustCompile(`(?:IMG|VID)[_-](\d{4})(\d{2})(\d{2})[_-](?:WA|wa)\d+`), "20060102"},
	// 2024-08-02_16-15-02 or 2024-08-02 16-15-02
	{regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})[_ ](\d{2})-(\d{2})-(\d{2})`), "20060102150405"},
	// 20240802_161602
	{regexp.MustCompile(`(\d{4})(\d{2})(\d{2})_(\d{2})(\d{2})(\d{2})`), "20060102150405"},
	// 2024-08-02 (date only)
	{regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`), "20060102"},
	// 20240802 (date only, 8 digits not preceded/followed by more digits)
	{regexp.MustCompile(`(?:^|[^\d])(\d{4})(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])(?:[^\d]|$)`), "20060102"},
}

// supportedExtensions lists file extensions that Phorganizer will process.
var supportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".heic": true,
	".raw":  true,
	".cr2":  true,
	".nef":  true,
	".arw":  true,
	".mp4":  true,
	".mov":  true,
}

// IsSupported returns true if the file extension is a supported media type.
func IsSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return supportedExtensions[ext]
}

// ExtractDate attempts to extract the capture date from a media file.
// Fallback order: EXIF DateTimeOriginal → filename date → source path date → file modification time.
// Returns the resolved date and the method used ("exif", "filename", "path", "mtime").
func ExtractDate(path string) (time.Time, string, error) {
	// Try EXIF first
	if t, err := extractEXIFDate(path); err == nil {
		return t, "exif", nil
	}

	// Try filename-based date extraction
	if t, err := extractFilenameDate(path); err == nil {
		return t, "filename", nil
	}

	// Try source path-based date extraction (e.g. /photos/2019/07/video.mp4)
	if t, err := extractPathDate(path); err == nil {
		return t, "path", nil
	}

	// Fallback to file modification time
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("unable to stat file: %w", err)
	}
	return info.ModTime(), "mtime", nil
}

// extractFilenameDate attempts to parse a date from the filename.
func extractFilenameDate(path string) (time.Time, error) {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	for _, p := range filenameDatePatterns {
		matches := p.re.FindStringSubmatch(name)
		if matches == nil {
			continue
		}

		// Reconstruct the digits from capture groups (skip full match)
		var digits string
		for _, m := range matches[1:] {
			digits += m
		}

		t, err := time.Parse(p.layout, digits)
		if err != nil {
			continue
		}

		// Sanity check: year between 1970 and 2100
		if t.Year() < 1970 || t.Year() > 2100 {
			continue
		}

		return t, nil
	}

	return time.Time{}, fmt.Errorf("no date found in filename: %s", base)
}

// monthNames maps English and German month names (lowercase) to month numbers.
var monthNames = map[string]time.Month{
	// English
	"january": time.January, "february": time.February, "march": time.March,
	"april": time.April, "may": time.May, "june": time.June,
	"july": time.July, "august": time.August, "september": time.September,
	"october": time.October, "november": time.November, "december": time.December,
	// English short
	"jan": time.January, "feb": time.February, "mar": time.March,
	"apr": time.April, "jun": time.June,
	"jul": time.July, "aug": time.August, "sep": time.September,
	"oct": time.October, "nov": time.November, "dec": time.December,
	// German
	"januar": time.January, "februar": time.February, "märz": time.March, "maerz": time.March,
	"mai": time.May, "juni": time.June,
	"juli": time.July, "oktober": time.October, "dezember": time.December,
}

// pathDatePatterns matches date patterns in directory paths (most specific first).
var pathDatePatterns = []struct {
	re      *regexp.Regexp
	handler func(matches []string) (time.Time, bool)
}{
	// /2019-05-15/ or /2019_05_15/ (full date as folder name)
	{regexp.MustCompile(`[/\\](\d{4})[-_](0[1-9]|1[0-2])[-_](0[1-9]|[12]\d|3[01])(?:[/\\]|$)`),
		func(m []string) (time.Time, bool) {
			return parseYMD(m[1], m[2], m[3])
		}},
	// /2019/07/15/ (year/month/day as separate folders)
	{regexp.MustCompile(`[/\\](\d{4})[/\\](0[1-9]|1[0-2])[/\\](0[1-9]|[12]\d|3[01])(?:[/\\]|$)`),
		func(m []string) (time.Time, bool) {
			return parseYMD(m[1], m[2], m[3])
		}},
	// /2019/07/ or \2019\07\ (year + month as separate folders)
	{regexp.MustCompile(`[/\\](\d{4})[/\\](0[1-9]|1[0-2])(?:[/\\]|$)`),
		func(m []string) (time.Time, bool) {
			return parseYMD(m[1], m[2], "01")
		}},
	// /2019-07/ or /2019_07/ (year-month in single folder)
	{regexp.MustCompile(`[/\\](\d{4})[-_](0[1-9]|1[0-2])(?:[/\\]|$)`),
		func(m []string) (time.Time, bool) {
			return parseYMD(m[1], m[2], "01")
		}},
	// /2019/January/ or /2019/januar/ or /2019/Mai/ (year + month name)
	{regexp.MustCompile(`(?i)[/\\](\d{4})[/\\]([a-zäöü]+)(?:[/\\]|$)`),
		func(m []string) (time.Time, bool) {
			return parseYearMonthName(m[1], m[2])
		}},
	// /2019-January/ or /2019_Mai/ (year-monthname in single folder)
	{regexp.MustCompile(`(?i)[/\\](\d{4})[-_]([a-zäöü]+)(?:[/\\]|$)`),
		func(m []string) (time.Time, bool) {
			return parseYearMonthName(m[1], m[2])
		}},
	// /01-january/ or /05-Mai/ preceded by year folder (MM-monthname)
	// Handled via month name patterns above with year context
	// /2019/ or \2019\ (year only — least specific)
	{regexp.MustCompile(`[/\\](\d{4})(?:[/\\]|$)`),
		func(m []string) (time.Time, bool) {
			return parseYMD(m[1], "01", "01")
		}},
}

func parseYMD(yearStr, monthStr, dayStr string) (time.Time, bool) {
	dateStr := yearStr + monthStr + dayStr
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return time.Time{}, false
	}
	if t.Year() < 1970 || t.Year() > 2100 {
		return time.Time{}, false
	}
	return t, true
}

func parseYearMonthName(yearStr, nameStr string) (time.Time, bool) {
	month, ok := monthNames[strings.ToLower(nameStr)]
	if !ok {
		return time.Time{}, false
	}
	dateStr := yearStr + fmt.Sprintf("%02d", int(month)) + "01"
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return time.Time{}, false
	}
	if t.Year() < 1970 || t.Year() > 2100 {
		return time.Time{}, false
	}
	return t, true
}

// extractPathDate attempts to derive a date from directory components of the source path.
// Supports: YYYY/MM, YYYY-MM, YYYY-MM-DD, YYYY/MonthName (EN/DE), and more.
// Uses the deepest (rightmost) match for best accuracy.
func extractPathDate(path string) (time.Time, error) {
	dir := filepath.Dir(path)
	normalized := filepath.ToSlash(dir)

	for _, p := range pathDatePatterns {
		allMatches := p.re.FindAllStringSubmatch(normalized, -1)
		if len(allMatches) == 0 {
			continue
		}

		// Use last match (deepest directory)
		matches := allMatches[len(allMatches)-1]
		if t, ok := p.handler(matches); ok {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("no date found in path: %s", dir)
}

// extractEXIFDate attempts to read DateTimeOriginal from EXIF data.
func extractEXIFDate(path string) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return time.Time{}, err
	}

	t, err := x.DateTime()
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

