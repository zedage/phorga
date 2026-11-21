package webui

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/zedage/phorga/internal/config"
)

// rawData is the page that edits the config file as text (WEB-018).
type rawData struct {
	Lang       string
	ConfigPath string
	Content    string
	// Exists is false in setup mode, before the file was first saved; the
	// text is then the configuration in effect, as it would be written.
	Exists bool
	Flash  flashMessage
}

// handleRaw shows the config file as text (GET) and validates or saves the
// edited text (POST, action=validate|save). Saving writes the text as it is,
// so comments and formatting stay, and applies it like a form save
// (WEB-018, CFG-008).
func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request) {
	lang := s.resolveLanguage(w, r)
	data := rawData{Lang: lang, ConfigPath: s.configPath}
	switch r.Method {
	case http.MethodGet:
		content, exists, err := s.configText()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data.Content, data.Exists = content, exists
	case http.MethodPost:
		// Browsers send textarea lines with CRLF; the file keeps LF.
		data.Content = strings.ReplaceAll(r.FormValue("content"), "\r\n", "\n")
		_, statErr := os.Stat(s.configPath)
		data.Exists = statErr == nil
		data.Flash = s.processRaw(lang, []byte(data.Content), r.FormValue("action") == "save")
		if data.Flash.Level != "error" && r.FormValue("action") == "save" {
			data.Exists = true
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.execute(w, "rawedit", lang, data)
}

// configText returns the text of the config file, or the configuration in
// effect as it would be written when there is no file yet.
func (s *Server) configText() (string, bool, error) {
	content, err := os.ReadFile(s.configPath)
	if err == nil {
		return string(content), true, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", false, err
	}
	generated, err := config.Marshal(s.configPath, s.currentConfig())
	return string(generated), false, err
}

// processRaw validates the text with the startup rules and, if save is set
// and it is valid, writes and applies it.
func (s *Server) processRaw(lang string, content []byte, save bool) flashMessage {
	fail := func(title string, problems ...string) flashMessage {
		return flashMessage{Level: "error", Title: title, Message: title, Errors: problems}
	}
	cfg, err := config.Parse(s.configPath, content)
	if err != nil {
		return fail(t(lang, "validation_failed"), err.Error())
	}
	if problems := config.ValidateProblems(cfg); len(problems) > 0 {
		return fail(t(lang, "validation_failed"), problems...)
	}
	if !save {
		return flashMessage{Level: "success", Title: t(lang, "validate"), Message: "OK"}
	}

	var restartNeeded []string
	if s.applyRaw != nil {
		applied, restart, err := s.applyRaw(content)
		if err != nil {
			return fail(t(lang, "save_failed"), err.Error())
		}
		s.setConfig(applied)
		restartNeeded = restart
	} else if err := config.WriteFile(s.configPath, content); err != nil {
		return fail(t(lang, "save_failed"), err.Error())
	}

	flash := flashMessage{
		Level:   "success",
		Title:   t(lang, "applied_title"),
		Message: t(lang, "success_prefix") + " " + s.configPath + ". " + t(lang, "applied_message"),
	}
	if s.applyRaw == nil {
		flash.Title, flash.Message = t(lang, "save"), t(lang, "success_prefix")+" "+s.configPath
	}
	if len(restartNeeded) > 0 {
		flash.Level = "warning"
		flash.Errors = []string{t(lang, "restart_needed") + " " + strings.Join(restartNeeded, ", ")}
	}
	return flash
}
