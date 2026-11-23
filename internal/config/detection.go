package config

import (
	"fmt"
	"strings"
)

// organizerDetectionFields are the fields an organizer detectionMode may use
// (ORG-011).
var organizerDetectionFields = []string{"filename", "filedate", "exif"}

// DetectionFields returns the fields an organizer detectionMode may use, in
// their canonical order (WEB-017).
func DetectionFields() []string {
	return append([]string(nil), organizerDetectionFields...)
}

// DetectionExpr is a parsed organizer duplicates.detectionMode (ORG-011):
// one or more fields, joined by '+' (all must match) or '|' (one is enough).
type DetectionExpr struct {
	Fields []string
	// Any is true for '|' (OR) and false for '+' (AND) or a single field.
	Any bool
}

// ParseDetectionMode parses an organizer detectionMode such as
// "filename+exif" or "filedate|exif". It rejects unknown fields and the
// mixing of '+' and '|' (CFG-002).
func ParseDetectionMode(mode string) (DetectionExpr, error) {
	if strings.TrimSpace(mode) == "" {
		return DetectionExpr{}, fmt.Errorf("detectionMode is required")
	}
	hasPlus := strings.Contains(mode, "+")
	hasPipe := strings.Contains(mode, "|")
	if hasPlus && hasPipe {
		return DetectionExpr{}, fmt.Errorf("mixing '+' and '|' operators is not supported in '%s'", mode)
	}
	sep := "+"
	if hasPipe {
		sep = "|"
	}
	expr := DetectionExpr{Any: hasPipe}
	for _, field := range strings.Split(mode, sep) {
		field = strings.TrimSpace(field)
		if !contains(organizerDetectionFields, field) {
			return DetectionExpr{}, fmt.Errorf("unknown detection field '%s'; valid fields: %s", field, strings.Join(organizerDetectionFields, ", "))
		}
		expr.Fields = append(expr.Fields, field)
	}
	return expr, nil
}

// MatchesOnFilenameAlone reports whether the expression is true for any two
// files with the same name. The organizer checks for a duplicate only when a
// file with the same name is already at the target, so such an expression
// treats every name collision as a duplicate (CFG-002, ORG-012).
func (e DetectionExpr) MatchesOnFilenameAlone() bool {
	if e.Any {
		return contains(e.Fields, "filename")
	}
	for _, f := range e.Fields {
		if f != "filename" {
			return false
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
