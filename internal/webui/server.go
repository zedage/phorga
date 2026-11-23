package webui

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zedage/phorga/internal/config"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

type Options struct {
	Address    string
	ConfigPath string
	Config     *config.Config
	// Status provides job status for the status page (WEB-011). Optional.
	Status StatusSource
	// Apply saves a valid configuration to the active config file and puts
	// it into effect without a restart (CFG-008, WEB-004). It returns the
	// configuration as applied and the fields that still need a restart.
	// Optional: without it, saving only writes the file.
	Apply ApplyFunc
	// ApplyRaw does the same for the edited text of the config file, which
	// it writes as it is (WEB-018). Optional: without it, the text is only
	// written.
	ApplyRaw ApplyRawFunc
	// Password, if not empty, protects every page with HTTP Basic auth
	// (WEB-020). It comes from PHORGA_WEB_UI_PASSWORD, never from the config.
	Password string
	// Version is shown next to the name in the top bar (RUN-004). Optional.
	Version string
}

// ApplyRawFunc saves and applies the text of a config file, see Options.ApplyRaw.
type ApplyRawFunc func(data []byte) (applied *config.Config, restartNeeded []string, err error)

// ApplyFunc saves and applies a configuration, see Options.Apply.
type ApplyFunc func(cfg *config.Config) (applied *config.Config, restartNeeded []string, err error)

type Server struct {
	address    string
	configPath string
	status     StatusSource
	apply      ApplyFunc
	applyRaw   ApplyRawFunc
	// passwordSet is shown on the help page (WEB-022).
	passwordSet bool

	// mu guards the configuration shown in the UI and the picker roots,
	// which change when a save is applied (CFG-008).
	mu          sync.RWMutex
	config      *config.Config
	browseRoots []string

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

	tmpl, err := template.New("").Funcs(templateFuncs).
		Funcs(template.FuncMap{"version": func() string { return opts.Version }}).
		ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing web UI templates: %w", err)
	}

	s := &Server{
		address:     address,
		configPath:  opts.ConfigPath,
		status:      opts.Status,
		apply:       opts.Apply,
		applyRaw:    opts.ApplyRaw,
		templates:   tmpl,
		passwordSet: opts.Password != "",
	}
	s.setConfig(opts.Config)

	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("loading web UI assets: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/ui", s.handlePage)
	mux.HandleFunc("/ui/validate", s.handleValidate)
	mux.HandleFunc("/ui/save", s.handleSave)
	mux.HandleFunc("/ui/form", s.handleFormOp)
	mux.HandleFunc("/ui/yaml", s.handleRaw)
	mux.HandleFunc("/ui/preview/structure", s.handlePreviewStructure)
	mux.HandleFunc("/ui/preview/schedule", s.handlePreviewSchedule)
	mux.HandleFunc("/ui/fs/list", s.handleFSList)
	mux.HandleFunc("/ui/fs/suggest", s.handleFSSuggest)
	mux.HandleFunc("/ui/status", s.handleStatusPage)
	mux.HandleFunc("/ui/status/table", s.handleStatusTable)
	mux.HandleFunc("/ui/help", s.handleHelp)
	mux.Handle("/ui/static/", http.StripPrefix("/ui/static/", cacheStatic(http.FileServer(http.FS(static)))))

	s.httpServer = &http.Server{
		Addr:              address,
		Handler:           requirePassword(opts.Password, rejectCrossSite(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s, nil
}

// setConfig makes cfg the configuration shown in the UI and takes its
// folder picker roots (WEB-010).
func (s *Server) setConfig(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
	s.browseRoots = nil
	if cfg.WebUI != nil {
		s.browseRoots = append([]string(nil), cfg.WebUI.BrowseRoots...)
	}
}

// currentConfig returns the configuration in effect.
func (s *Server) currentConfig() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// roots returns the folder picker roots in effect.
func (s *Server) roots() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.browseRoots
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
	data := s.newPageData(lang, newFormFromConfig(s.currentConfig()))
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
		data := s.newPageData(lang, newFormFromConfig(s.currentConfig()))
		data.Flash = flashMessage{
			Level:   "error",
			Title:   t(lang, "validation_failed"),
			Errors:  []string{err.Error()},
			Message: err.Error(),
		}
		s.renderResult(w, r, data)
		return
	}

	data := s.newPageData(lang, form)
	cfg := form.toConfig()
	problems := config.ValidateProblems(cfg)
	if persist {
		problems = append(problems, s.validateSaveTarget(form)...)
	}
	if len(problems) > 0 {
		data.FieldErrors = fieldErrors(problems)
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
	applyNow := s.apply != nil && targetPath == s.configPath
	var restartNeeded []string
	if applyNow {
		var applied *config.Config
		applied, restartNeeded, err = s.apply(cfg)
		if err == nil {
			s.setConfig(applied)
			data = s.newPageData(lang, newFormFromConfig(applied))
		}
	} else {
		err = config.Save(targetPath, cfg)
	}
	if err != nil {
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
	switch {
	case applyNow:
		data.Flash.Title = t(lang, "applied_title")
		data.Flash.Message = fmt.Sprintf("%s %s. %s", t(lang, "success_prefix"), targetPath, t(lang, "applied_message"))
		if len(restartNeeded) > 0 {
			data.Flash.Level = "warning"
			data.Flash.Errors = []string{t(lang, "restart_needed") + " " + strings.Join(restartNeeded, ", ")}
		}
	case targetPath != s.configPath:
		data.Flash.Message += ". " + t(lang, "saved_not_applied")
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

func (s *Server) newPageData(lang string, form configForm) pageData {
	return pageData{
		Lang:          lang,
		ConfigPath:    s.configPath,
		Form:          form,
		BrowseEnabled: len(s.roots()) > 0,
		Setup:         !s.currentConfig().HasJobs(),
		Choices:       newChoices(),
		Synology:      discoverUsers(form),
	}
}

// renderResult answers HTMX requests with the flash fragment plus
// out-of-band field errors (WEB-003, WEB-008), and plain POSTs with the page.
func (s *Server) renderResult(w http.ResponseWriter, r *http.Request, data pageData) {
	if isHTMX(r) {
		s.renderTemplate(w, "flash", data)
		s.renderTemplate(w, "fieldErrorsOOB", data)
		return
	}
	s.renderPage(w, data)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// cacheStatic marks embedded assets as cacheable (WEB-009).
func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) renderPage(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.renderTemplate(w, "page", data)
}

func (s *Server) renderTemplate(w http.ResponseWriter, name string, data pageData) {
	s.execute(w, name, data.Lang, data)
}

// execute renders a template with the language helpers T (translate) and
// FErr (field errors of the current page, if any).
func (s *Server) execute(w http.ResponseWriter, name string, lang string, data any) {
	var errs map[string][]string
	if pd, ok := data.(pageData); ok {
		errs = pd.FieldErrors
	}
	if err := s.templates.ExecuteTemplate(w, name, map[string]any{
		"Data": data,
		"Lang": lang,
		"T": func(key string) string {
			return t(lang, key)
		},
		"FErr": func(field string) string {
			return strings.Join(errs[field], "; ")
		},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
