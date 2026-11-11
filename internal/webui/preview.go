package webui

import (
	"net/http"
	"strings"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/metadata"
	"github.com/zedage/phorga/internal/scheduler"
)

// previewSampleDate is the fixed date used for targetStructure previews.
var previewSampleDate = time.Date(2024, time.March, 15, 10, 30, 0, 0, time.UTC)

// defaultTargetStructure mirrors the loader default (CFG-001).
const defaultTargetStructure = "{year}/{month}"

// now is replaceable in tests.
var now = time.Now

type previewData struct {
	OK      bool
	Label   string
	Lines   []string
	Message string
}

// previewValue reads the value of the field named by the "field" query
// parameter, so one handler serves every input of that kind.
func previewValue(r *http.Request) string {
	field := r.URL.Query().Get("field")
	return strings.TrimSpace(r.FormValue(field))
}

// handlePreviewStructure renders an example target path (WEB-008).
func (s *Server) handlePreviewStructure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lang := s.resolveLanguage(w, r)
	structure := previewValue(r)
	if structure == "" {
		structure = defaultTargetStructure
	}
	data := previewData{Label: t(lang, "preview_example")}
	if err := config.ValidateTargetStructure(structure); err != nil {
		data.Label, data.Message = t(lang, "preview_invalid"), err.Error()
	} else if path, err := metadata.ResolveTargetPath(structure, previewSampleDate); err != nil {
		data.Label, data.Message = t(lang, "preview_invalid"), err.Error()
	} else {
		data.OK = true
		data.Lines = []string{path}
	}
	s.renderPreview(w, lang, data)
}

// handlePreviewSchedule renders the next 3 run times of a cron expression (WEB-008).
func (s *Server) handlePreviewSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lang := s.resolveLanguage(w, r)
	expr := previewValue(r)
	data := previewData{Label: t(lang, "preview_next_runs")}
	times, err := scheduler.NextTimes(expr, now(), 3)
	if expr == "" || err != nil {
		data.Label = t(lang, "preview_invalid")
		if err != nil {
			data.Message = err.Error()
		}
	} else {
		data.OK = true
		for _, ts := range times {
			data.Lines = append(data.Lines, ts.Format("Mon 2006-01-02 15:04"))
		}
	}
	s.renderPreview(w, lang, data)
}

func (s *Server) renderPreview(w http.ResponseWriter, lang string, data previewData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.execute(w, "preview", lang, data)
}

