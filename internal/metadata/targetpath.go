package metadata

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// validTokens lists all supported targetStructure tokens (case-insensitive).
var validTokens = map[string]bool{
	// English
	"year":          true,
	"month":         true,
	"monthwithname": true,
	"day":           true,
	"monthnumber":   true,
	"daynumber":     true,

	// German
	"jahr":  true,
	"monat": true,
	"tag":   true,

	// French
	"année": true,
	"mois":  true,
	"jour":  true,

	// Italian
	"anno":   true,
	"mese":   true,
	"giorno": true,
}

// Month name maps for all languages (lowercase for base form)
var (
	// English month names
	monthEN = map[time.Month]string{
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

	// German month names
	monthDE = map[time.Month]string{
		time.January:   "januar",
		time.February:  "februar",
		time.March:     "märz",
		time.April:     "april",
		time.May:       "mai",
		time.June:      "juni",
		time.July:      "juli",
		time.August:    "august",
		time.September: "september",
		time.October:   "oktober",
		time.November:  "november",
		time.December:  "dezember",
	}

	// French month names
	monthFR = map[time.Month]string{
		time.January:   "janvier",
		time.February:  "février",
		time.March:     "mars",
		time.April:     "avril",
		time.May:       "mai",
		time.June:      "juin",
		time.July:      "juillet",
		time.August:    "août",
		time.September: "septembre",
		time.October:   "octobre",
		time.November:  "novembre",
		time.December:  "décembre",
	}

	// Italian month names
	monthIT = map[time.Month]string{
		time.January:   "gennaio",
		time.February:  "febbraio",
		time.March:     "marzo",
		time.April:     "aprile",
		time.May:       "maggio",
		time.June:      "giugno",
		time.July:      "luglio",
		time.August:    "agosto",
		time.September: "settembre",
		time.October:   "ottobre",
		time.November:  "novembre",
		time.December:  "dicembre",
	}

	// Deprecated: kept for backwards compatibility
	monthToName = monthEN
)

// normalizeCase applies case transformation to a string based on the placeholder case.
// Examples:
//   - placeholder "{monat}" (all lowercase) → result "februar" (lowercase)
//   - placeholder "{Monat}" (first letter capitalized) → result "Februar" (title case)
//   - placeholder "{MONAT}" (all uppercase) → result "FEBRUAR"
func normalizeCase(placeholder, result string) string {
	if len(placeholder) == 0 {
		return result
	}

	// Extract token name (remove { and })
	token := strings.TrimPrefix(strings.TrimSuffix(placeholder, "}"), "{")
	if len(token) == 0 {
		return result
	}

	// Determine case pattern from token
	if token == strings.ToUpper(token) {
		// All uppercase: {MONAT} → FEBRUAR
		return strings.ToUpper(result)
	}

	if token == strings.ToLower(token) {
		// All lowercase: {monat} → februar
		return strings.ToLower(result)
	}

	// Title case (first letter uppercase): {Monat} → Februar
	if len(result) > 0 {
		return strings.ToUpper(string(result[0])) + strings.ToLower(result[1:])
	}

	return result
}

// isValidToken checks if a token name (without braces) is supported.
func isValidToken(tokenName string) bool {
	// Token names are case-insensitive for lookup
	return validTokens[strings.ToLower(tokenName)]
}

// resolveToken resolves a single token to its value for a given date.
// Returns (value, found) tuple.
func resolveToken(token string, t time.Time) (string, bool) {
	lowerToken := strings.ToLower(token)

	switch lowerToken {
	// English
	case "year":
		return fmt.Sprintf("%04d", t.Year()), true
	case "month":
		return fmt.Sprintf("%02d", int(t.Month())), true
	case "monthwithname":
		return fmt.Sprintf("%02d-%s", int(t.Month()), monthEN[t.Month()]), true
	case "day":
		return fmt.Sprintf("%02d", t.Day()), true
	case "monthnumber":
		return fmt.Sprintf("%02d", int(t.Month())), true
	case "daynumber":
		return fmt.Sprintf("%02d", t.Day()), true

	// German
	case "jahr":
		return fmt.Sprintf("%04d", t.Year()), true
	case "monat":
		return monthDE[t.Month()], true
	case "tag":
		return fmt.Sprintf("%02d", t.Day()), true

	// French
	case "année":
		return fmt.Sprintf("%04d", t.Year()), true
	case "mois":
		return monthFR[t.Month()], true
	case "jour":
		return fmt.Sprintf("%02d", t.Day()), true

	// Italian
	case "anno":
		return fmt.Sprintf("%04d", t.Year()), true
	case "mese":
		return monthIT[t.Month()], true
	case "giorno":
		return fmt.Sprintf("%02d", t.Day()), true

	default:
		return "", false
	}
}

// ResolveTargetPath resolves a targetStructure template with a given date.
// Supports English, German, French, and Italian month/day tokens with case-sensitive rendering.
//
// Example:
//
//	"{year}/{month}" with 2024-03-15 → "2024/03"
//	"{year}/{Monat}" with 2024-03-15 → "2024/März"
//	"{year}/{mois}" with 2024-03-15 → "2024/mars"
func ResolveTargetPath(structure string, t time.Time) (string, error) {
	// Find all tokens in structure: {tokenName}
	tokenPattern := regexp.MustCompile(`\{([^}]+)\}`)
	matches := tokenPattern.FindAllStringSubmatchIndex(structure, -1)

	if len(matches) == 0 {
		// No tokens found - return as is
		return structure, nil
	}

	// Process tokens in reverse order to preserve indices
	result := structure
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		fullToken := structure[match[0]:match[1]] // e.g., "{year}"
		tokenName := structure[match[2]:match[3]] // e.g., "year"

		// Validate token
		if !isValidToken(tokenName) {
			return "", fmt.Errorf("unknown token in targetStructure: %s (supported: year/jahr/année/anno, month/monat/mois/mese, day/tag/jour/giorno, monthNumber, dayNumber, monthWithName)", fullToken)
		}

		// Resolve token value
		value, ok := resolveToken(tokenName, t)
		if !ok {
			return "", fmt.Errorf("failed to resolve token: %s", fullToken)
		}

		// Apply case normalization
		if strings.ToLower(tokenName) != tokenName || strings.ToUpper(tokenName) != tokenName {
			value = normalizeCase(fullToken, value)
		}

		// Replace token with resolved value (use string slicing to preserve original indices)
		result = result[:match[0]] + value + result[match[1]:]
	}

	return result, nil
}
