package webui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zedage/phorga/internal/config"
)

// Spec: WEB-004, CFG-008
func TestSaveAppliesThroughApplyFunc(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	var got *config.Config
	s, err := New(Options{
		Address: "127.0.0.1:0", ConfigPath: configPath, Config: sampleConfig(),
		Apply: func(cfg *config.Config) (*config.Config, []string, error) {
			got = cfg
			return cfg, []string{"stateDir"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if code := get(t, s, "/ui/fs/list").Code; code != http.StatusNotFound {
		t.Fatalf("picker must be off before the save, got %d", code)
	}

	next := sampleConfig()
	next.Organizer[0].Target = "/photos/applied"
	next.WebUI.BrowseRoots = []string{root}
	resp := postForm(t, s, "/ui/save", validFormValues(next, "active", ""), true)
	body := resp.Body.String()

	if got == nil || got.Organizer[0].Target != "/photos/applied" {
		t.Fatalf("Apply was not called with the form values: %+v", got)
	}
	if !strings.Contains(body, "Saved and applied") || !strings.Contains(body, "These changes take effect after a restart: stateDir") {
		t.Fatalf("expected applied message with restart note, got %s", body)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("with Apply set, the UI must leave writing the file to Apply")
	}
	page := httptest.NewRecorder()
	s.handlePage(page, httptest.NewRequest(http.MethodGet, "/ui", nil))
	if !strings.Contains(page.Body.String(), "/photos/applied") {
		t.Error("the page must show the applied configuration")
	}
	if code := get(t, s, "/ui/fs/list").Code; code != http.StatusOK {
		t.Errorf("picker roots must follow the applied config, got %d", code)
	}
}

// Spec: WEB-004
func TestSaveShowsApplyError(t *testing.T) {
	s, err := New(Options{
		Address: "127.0.0.1:0", ConfigPath: sampleConfigPath(), Config: sampleConfig(),
		Apply: func(*config.Config) (*config.Config, []string, error) {
			return nil, nil, errors.New("disk full")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	next := sampleConfig()
	next.Organizer[0].Target = "/photos/never"
	body := postForm(t, s, "/ui/save", validFormValues(next, "active", ""), true).Body.String()
	if !strings.Contains(body, "disk full") {
		t.Fatalf("expected the error, got %s", body)
	}
	if s.currentConfig().Organizer[0].Target == "/photos/never" {
		t.Fatal("a failed apply must not change the shown configuration")
	}
}

// Spec: WEB-004
func TestSaveToSeparateFileDoesNotApply(t *testing.T) {
	separate := filepath.Join(t.TempDir(), "try.yaml")
	called := false
	s, err := New(Options{
		Address: "127.0.0.1:0", ConfigPath: sampleConfigPath(), Config: sampleConfig(),
		Apply: func(cfg *config.Config) (*config.Config, []string, error) {
			called = true
			return cfg, nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := postForm(t, s, "/ui/save", validFormValues(sampleConfig(), "separate", separate), true).Body.String()
	if called {
		t.Fatal("saving to a separate file must not apply")
	}
	if _, err := os.Stat(separate); err != nil {
		t.Fatalf("separate file not written: %v", err)
	}
	if !strings.Contains(body, "The running configuration is unchanged") {
		t.Errorf("expected the not-applied note, got %s", body)
	}
}

// Spec: CFG-009
func TestSetupCalloutOnlyWithoutJobs(t *testing.T) {
	empty := newConfiguredServer(t, sampleConfigPath(), config.Default())
	page := httptest.NewRecorder()
	empty.handlePage(page, httptest.NewRequest(http.MethodGet, "/ui", nil))
	if !strings.Contains(page.Body.String(), "Phorga has nothing to do yet") {
		t.Error("a config without jobs must show the setup callout")
	}

	full := newTestServer(t)
	page = httptest.NewRecorder()
	full.handlePage(page, httptest.NewRequest(http.MethodGet, "/ui", nil))
	if strings.Contains(page.Body.String(), "Phorga has nothing to do yet") {
		t.Error("a config with jobs must not show the setup callout")
	}
}
