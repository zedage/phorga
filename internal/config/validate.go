package config

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

// validModes lists accepted source modes.
var validModes = map[string]bool{"move": true, "copy": true}

// validOrgDupMethods lists accepted organizer duplicate action methods.
var validOrgDupMethods = map[string]bool{
	"replace":      true,
	"addSuffix":    true,
	"moveToFolder": true,
	"skip":         true,
}

// validDedupMethods lists accepted duplicate detection action methods.
var validDedupMethods = map[string]bool{
	"moveToFolder": true,
	"skip":         true,
}

// validDedupModes lists accepted duplicate detection modes.
var validDedupModes = map[string]bool{
	"content":  true,
	"metadata": true,
}

// validTargetTokens lists the allowed tokens in targetStructure.
// These are case-insensitive - {year}, {Year}, {YEAR} are all valid.
var validTargetTokens = map[string]bool{
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

// Validate checks the entire configuration and returns all errors found.
func Validate(cfg *Config) error {
	var errs []string

	if len(cfg.Organizer) == 0 && len(cfg.DuplicateDetection) == 0 {
		return fmt.Errorf("config must define at least one organizer profile or duplicateDetection job")
	}

	for i, profile := range cfg.Organizer {
		prefix := fmt.Sprintf("organizer[%d]", i)
		errs = append(errs, validateOrganizerProfile(prefix, profile)...)
	}

	for i, dd := range cfg.DuplicateDetection {
		prefix := fmt.Sprintf("duplicateDetection[%d]", i)
		errs = append(errs, validateDuplicateDetection(prefix, dd)...)
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func validateOrganizerProfile(prefix string, p OrganizerProfile) []string {
	var errs []string

	if p.Target == "" {
		errs = append(errs, fmt.Sprintf("%s.target is required", prefix))
	}

	if err := validateTargetStructure(p.TargetStructure); err != nil {
		errs = append(errs, fmt.Sprintf("%s.targetStructure: %v", prefix, err))
	}

	if len(p.Sources) == 0 {
		errs = append(errs, fmt.Sprintf("%s.sources must have at least one entry", prefix))
	}

	for j, src := range p.Sources {
		srcPrefix := fmt.Sprintf("%s.sources[%d]", prefix, j)
		errs = append(errs, validateOrganizerSource(srcPrefix, src)...)
	}

	return errs
}

func validateOrganizerSource(prefix string, src OrganizerSource) []string {
	var errs []string

	if src.Path == "" {
		errs = append(errs, fmt.Sprintf("%s.path is required", prefix))
	}

	if !validModes[src.Mode] {
		errs = append(errs, fmt.Sprintf("%s.mode must be 'move' or 'copy', got '%s'", prefix, src.Mode))
	}

	if err := validateCron(src.Schedule); err != nil {
		errs = append(errs, fmt.Sprintf("%s.schedule: %v", prefix, err))
	}

	if src.Duplicates != nil {
		dupPrefix := prefix + ".duplicates"
		errs = append(errs, validateOrganizerDuplicate(dupPrefix, *src.Duplicates)...)
	}

	return errs
}

func validateOrganizerDuplicate(prefix string, dup OrganizerDuplicate) []string {
	var errs []string

	if err := validateDetectionMode(dup.DetectionMode, []string{"filename", "filedate", "exif"}); err != nil {
		errs = append(errs, fmt.Sprintf("%s.detectionMode: %v", prefix, err))
	}

	if !validOrgDupMethods[dup.Action.Method] {
		errs = append(errs, fmt.Sprintf("%s.action.method must be one of replace, addSuffix, moveToFolder, skip; got '%s'", prefix, dup.Action.Method))
	}

	if dup.Action.Method == "moveToFolder" && dup.Action.DuplicateFolder == "" {
		errs = append(errs, fmt.Sprintf("%s.action.duplicateFolder is required when method is 'moveToFolder'", prefix))
	}

	return errs
}

func validateDuplicateDetection(prefix string, dd DuplicateDetection) []string {
	var errs []string

	if dd.Schedule == "" {
		errs = append(errs, fmt.Sprintf("%s.schedule is required", prefix))
	} else if err := validateCron(dd.Schedule); err != nil {
		errs = append(errs, fmt.Sprintf("%s.schedule: %v", prefix, err))
	}

	if len(dd.Sources) == 0 {
		errs = append(errs, fmt.Sprintf("%s.sources must have at least one entry", prefix))
	}

	if !validDedupModes[dd.DetectionMode] {
		errs = append(errs, fmt.Sprintf("%s.detectionMode must be 'content' or 'metadata', got '%s'", prefix, dd.DetectionMode))
	}

	if !validDedupMethods[dd.Action.Method] {
		errs = append(errs, fmt.Sprintf("%s.action.method must be 'moveToFolder' or 'skip', got '%s'", prefix, dd.Action.Method))
	}

	if dd.Action.Method == "moveToFolder" && dd.Action.DuplicateFolder == "" {
		errs = append(errs, fmt.Sprintf("%s.action.duplicateFolder is required when method is 'moveToFolder'", prefix))
	}

	return errs
}

// validateCron checks if a cron expression is parsable.
func validateCron(expr string) error {
	if expr == "" {
		return fmt.Errorf("cron expression is required")
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(expr); err != nil {
		return fmt.Errorf("invalid cron expression '%s': %v", expr, err)
	}
	return nil
}

// validateTargetStructure checks that all tokens in the structure are valid.
// Tokens are case-insensitive: {year}, {Year}, {YEAR} are all valid.
func validateTargetStructure(structure string) error {
	if structure == "" {
		return nil // will be defaulted
	}

	// Find all {token} patterns
	rest := structure
	for {
		start := strings.Index(rest, "{")
		if start == -1 {
			break
		}
		end := strings.Index(rest[start:], "}")
		if end == -1 {
			return fmt.Errorf("unclosed token at position %d", start)
		}
		token := rest[start+1 : start+end]
		lowerToken := strings.ToLower(token)
		if !validTargetTokens[lowerToken] {
			return fmt.Errorf("unknown token '{%s}'; valid tokens: year/jahr/année/anno, month/monat/mois/mese, monthWithName, day/tag/jour/giorno, monthNumber, dayNumber", token)
		}
		rest = rest[start+end+1:]
	}
	return nil
}

// validateDetectionMode checks if a detection mode expression uses valid fields and operators.
// Ensures only + or | is used, not both mixed.
func validateDetectionMode(mode string, validFields []string) error {
	if mode == "" {
		return fmt.Errorf("detectionMode is required")
	}

	hasPlus := strings.Contains(mode, "+")
	hasPipe := strings.Contains(mode, "|")
	if hasPlus && hasPipe {
		return fmt.Errorf("mixing '+' and '|' operators is not supported in '%s'", mode)
	}

	sep := "+"
	if hasPipe {
		sep = "|"
	}

	fields := strings.Split(mode, sep)
	validSet := make(map[string]bool, len(validFields))
	for _, f := range validFields {
		validSet[f] = true
	}

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if !validSet[field] {
			return fmt.Errorf("unknown detection field '%s'; valid fields: %s", field, strings.Join(validFields, ", "))
		}
	}
	return nil
}
