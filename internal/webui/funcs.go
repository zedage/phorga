package webui

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

// templateFuncs are the static helpers available in all templates.
var templateFuncs = template.FuncMap{
	// dict builds a map from key/value pairs, for passing several values
	// to a sub-template.
	"dict": func(pairs ...any) (map[string]any, error) {
		if len(pairs)%2 != 0 {
			return nil, fmt.Errorf("dict needs key/value pairs")
		}
		m := make(map[string]any, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			key, ok := pairs[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict key %v is not a string", pairs[i])
			}
			m[key] = pairs[i+1]
		}
		return m, nil
	},
	"fid":   fieldID,
	"has":   contains,
	"lines": func(values []string) string { return strings.Join(values, "\n") },
	"fmtTime": func(ts time.Time) string {
		if ts.IsZero() {
			return "–"
		}
		return ts.Local().Format("2006-01-02 15:04:05")
	},
	"fmtDuration": func(d time.Duration) string {
		if d < time.Second {
			return d.Round(time.Millisecond).String()
		}
		return d.Round(time.Second).String()
	},
}

// fieldID turns a form field name like "organizer.0.target" into an
// element id usable in CSS selectors ("f-organizer-0-target").
func fieldID(name string) string {
	return "f-" + strings.NewReplacer(".", "-", "[", "-", "]", "").Replace(name)
}

func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
