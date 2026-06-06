package webui

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/zedage/phorga/internal/config"
)

//go:embed templates/*.html
var templateFS embed.FS

type Options struct {
	Address    string
	ConfigPath string
	Config     *config.Config
}

type Server struct {
	address    string
	configPath string
	config     *config.Config
	templates  *template.Template
	httpServer *http.Server
}

func New(opts Options) (*Server, error) {
	if opts.Config == nil {
		return nil, fmt.Errorf("web UI config is required")
	}
	address := strings.TrimSpace(opts.Address)
	if address == "" {
		if opts.Config.WebUI != nil && opts.Config.WebUI.Address != "" {
			address = opts.Config.WebUI.Address
		} else {
			address = config.DefaultWebUIAddress
		}
	}

	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing web UI templates: %w", err)
	}

	s := &Server{
		address:    address,
		configPath: opts.ConfigPath,
		config:     opts.Config,
		templates:  tmpl,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/ui", s.handlePage)
	mux.HandleFunc("/ui/validate", s.handleValidate)
	mux.HandleFunc("/ui/save", s.handleSave)

	s.httpServer = &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s, nil
}

func (s *Server) Address() string {
	return s.address
}

func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

func (s *Server) Start() error {
	slog.Info("web UI starting", "address", s.address)
	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/ui", http.StatusFound)
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lang := s.resolveLanguage(w, r)
	data := pageData{
		Lang:       lang,
		ConfigPath: s.configPath,
		Form:       newFormFromConfig(s.config),
	}
	s.renderPage(w, data)
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	s.processForm(w, r, false)
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	s.processForm(w, r, true)
}

func (s *Server) processForm(w http.ResponseWriter, r *http.Request, persist bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	lang := s.resolveLanguage(w, r)
	form, err := parseForm(r)
	if err != nil {
		s.renderResult(w, r, pageData{
			Lang:       lang,
			ConfigPath: s.configPath,
			Form:       newFormFromConfig(s.config),
			Flash: flashMessage{
				Level:   "error",
				Title:   t(lang, "validation_failed"),
				Errors:  []string{err.Error()},
				Message: err.Error(),
			},
		})
		return
	}

	data := pageData{Lang: lang, ConfigPath: s.configPath, Form: form}
	cfg := form.toConfig()
	problems := config.ValidateProblems(cfg)
	if persist {
		problems = append(problems, s.validateSaveTarget(form)...)
	}
	if len(problems) > 0 {
		data.Flash = flashMessage{
			Level:   "error",
			Title:   t(lang, "validation_failed"),
			Errors:  problems,
			Message: t(lang, "validation_failed"),
		}
		s.renderResult(w, r, data)
		return
	}

	if !persist {
		data.Flash = flashMessage{
			Level:   "success",
			Title:   t(lang, "validate"),
			Message: "OK",
		}
		s.renderResult(w, r, data)
		return
	}

	targetPath := s.targetPath(form)
	if err := config.Save(targetPath, cfg); err != nil {
		data.Flash = flashMessage{
			Level:   "error",
			Title:   t(lang, "save_failed"),
			Errors:  []string{err.Error()},
			Message: err.Error(),
		}
		s.renderResult(w, r, data)
		return
	}

	data.Flash = flashMessage{
		Level:   "success",
		Title:   t(lang, "save"),
		Message: fmt.Sprintf("%s %s", t(lang, "success_prefix"), targetPath),
	}
	w.Header().Set("HX-Trigger", `{"configSaved":true}`)
	s.renderResult(w, r, data)
}

func (s *Server) validateSaveTarget(form configForm) []string {
	if form.SaveTarget != "separate" {
		return nil
	}
	if strings.TrimSpace(form.SeparatePath) == "" {
		return []string{"separatePath is required when saving to a separate file"}
	}
	return nil
}

func (s *Server) targetPath(form configForm) string {
	if form.SaveTarget == "separate" && strings.TrimSpace(form.SeparatePath) != "" {
		return strings.TrimSpace(form.SeparatePath)
	}
	return s.configPath
}

func (s *Server) resolveLanguage(w http.ResponseWriter, r *http.Request) string {
	cookieLang := ""
	if cookie, err := r.Cookie("phorga_lang"); err == nil {
		cookieLang = cookie.Value
	}
	lang := detectLanguage(r.FormValue("lang"), cookieLang, r.Header.Get("Accept-Language"))
	http.SetCookie(w, &http.Cookie{
		Name:     "phorga_lang",
		Value:    lang,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return lang
}

func (s *Server) renderResult(w http.ResponseWriter, r *http.Request, data pageData) {
	if r.Header.Get("HX-Request") == "true" {
		s.renderTemplate(w, "flash", data)
		return
	}
	s.renderPage(w, data)
}

func (s *Server) renderPage(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.renderTemplate(w, "page", data)
}

func (s *Server) renderTemplate(w http.ResponseWriter, name string, data pageData) {
	if err := s.templates.ExecuteTemplate(w, name, map[string]any{
		"Data": data,
		"T": func(key string) string {
			return t(data.Lang, key)
		},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
