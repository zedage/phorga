package metadata

import (
	"fmt"
	"strings"
	"time"
)

// validTokens lists all supported targetStructure tokens.
var validTokens = map[string]bool{
	"year":          true,
	"month":         true,
	"monthWithName": true,
	"day":           true,
}

// monthToName maps month numbers to lowercase English names (for target path rendering).
var monthToName = map[time.Month]string{
	time.January:   "january",
	time.February:  "february",
	time.March:     "march",
	time.April:     "april",
	time.May:       "may",
	time.June:      "june",
	time.July:      "july",
	time.August:    "august",
	time.September: "september",
	time.October:   "october",
	time.November:  "november",
	time.December:  "december",
}

// ResolveTargetPath resolves a targetStructure template with a given date.
// Example: "{year}/{month}" with 2024-03-15 → "2024/03"
func ResolveTargetPath(structure string, t time.Time) string {
	r := strings.NewReplacer(
		"{year}", fmt.Sprintf("%04d", t.Year()),
		"{month}", fmt.Sprintf("%02d", int(t.Month())),
		"{monthWithName}", fmt.Sprintf("%02d-%s", int(t.Month()), monthToName[t.Month()]),
		"{day}", fmt.Sprintf("%02d", t.Day()),
	)
	return r.Replace(structure)
}

