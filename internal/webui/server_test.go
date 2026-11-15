package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/zedage/phorga/internal/config"
)

// Spec: WEB-002
func TestMainPageRendersCurrentConfiguration(t *testing.T) {
	server := newTestServer(t)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui", nil)

	server.handlePage(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "Phorga Web Configuration") {
		t.Fatalf("expected english heading in page, got %s", body)
	}
	if !strings.Contains(body, sampleConfigPath()) {
		t.Fatalf("expected config path in page")
	}
	if !strings.Contains(body, sampleConfig().Organizer[0].Target) {
		t.Fatalf("expected organizer target to be rendered")
	}
}

// Spec: WEB-005
func TestLocalizationUsesRequestedLanguageAndFallback(t *testing.T) {
	server := newTestServer(t)

	tests := []struct {
		lang     string
		expected string
	}{
		{lang: "de", expected: "Phorga Web-Konfiguration"},
		{lang: "en", expected: "Phorga Web Configuration"},
		{lang: "it", expected: "Configurazione Web di Phorga"},
		{lang: "fr", expected: "Configuration Web Phorga"},
	}

	for _, tc := range tests {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ui?lang="+tc.lang, nil)
		server.handlePage(resp, req)
		if !strings.Contains(resp.Body.String(), tc.expected) {
			t.Fatalf("expected heading %q for lang %s, got %s", tc.expected, tc.lang, resp.Body.String())
		}
	}

	respFallback := httptest.NewRecorder()
	reqFallback := httptest.NewRequest(http.MethodGet, "/ui?lang=es", nil)
	server.handlePage(respFallback, reqFallback)
	if !strings.Contains(respFallback.Body.String(), "Phorga Web Configuration") {
		t.Fatalf("expected fallback english heading, got %s", respFallback.Body.String())
	}
}

// Spec: WEB-003
func TestValidateReturnsHTMXErrorFragment(t *testing.T) {
	server := newTestServer(t)
	values := validFormValues(sampleConfig(), "active", "")
	values.Set("organizer.0.target", "")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/validate", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	server.handleValidate(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "Validation failed") {
		t.Fatalf("expected validation title, got %s", body)
	}
	if !strings.Contains(body, "organizer[0].target is required") {
		t.Fatalf("expected field-level validation error, got %s", body)
	}
}

// Spec: WEB-004
func TestSaveWritesSeparateFile(t *testing.T) {
	server := newTestServer(t)
	target := filepath.Join(t.TempDir(), "saved-config.yaml")
	values := validFormValues(sampleConfig(), "separate", target)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/save", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	server.handleSave(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected saved config file: %v", err)
	}
	body := resp.Body.String()
	if !strings.Contains(body, target) {
		t.Fatalf("expected success message to include save target, got %s", body)
	}
}

// Spec: WEB-004
func TestSaveWritesActiveConfigFile(t *testing.T) {
	activePath := filepath.Join(t.TempDir(), "active-config.yaml")
	cfg := sampleConfig()
	if err := config.Save(activePath, cfg); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	server := newConfiguredServer(t, activePath, cfg)
	values := validFormValues(cfg, "active", "")
	values.Set("stateDir", "/changed/state")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/save", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	server.handleSave(resp, req)

	loaded, err := config.Load(activePath)
	if err != nil {
		t.Fatalf("expected saved config to load: %v", err)
	}
	if loaded.StateDir != "/changed/state" {
		t.Fatalf("expected active config file to be updated, got %s", loaded.StateDir)
	}
}

func validFormValues(cfg *config.Config, saveTarget string, separatePath string) url.Values {
	form := newFormFromConfig(cfg)
	form.SaveTarget = saveTarget
	form.SeparatePath = separatePath
	values := url.Values{}
	values.Set("lang", "en")
	values.Set("stateDir", form.StateDir)
	if form.WebUI.Enabled {
		values.Set("webUI.enabled", "on")
	}
	values.Set("webUI.address", form.WebUI.Address)
	values.Set("webUI.browseRoots", strings.Join(form.WebUI.BrowseRoots, "\n"))
	values.Set("organizerCount", strconv.Itoa(len(form.Organizer)))
	for i, org := range form.Organizer {
		prefix := "organizer." + strconv.Itoa(i)
		if org.DryRun {
			values.Set(prefix+".dryRun", "on")
		}
		values.Set(prefix+".ntfyURL", org.NtfyURL)
		values.Set(prefix+".target", org.Target)
		values.Set(prefix+".targetStructure", org.TargetStructure)
		values.Set(prefix+".fallbackFolder", org.FallbackFolder)
		values.Set(prefix+".sourceCount", strconv.Itoa(len(org.Sources)))
		for j, src := range org.Sources {
			srcPrefix := prefix + ".sources." + strconv.Itoa(j)
			values.Set(srcPrefix+".path", src.Path)
			values.Set(srcPrefix+".mode", src.Mode)
			values.Set(srcPrefix+".schedule", src.Schedule)
			if src.DuplicatesEnabled {
				values.Set(srcPrefix+".duplicatesEnabled", "on")
			}
			values.Set(srcPrefix+".detectionMode", src.DetectionMode)
			values.Set(srcPrefix+".actionMethod", src.ActionMethod)
			values.Set(srcPrefix+".duplicateFolder", src.DuplicateFolder)
			values.Set(srcPrefix+".exclude", strings.Join(src.Exclude, "\n"))
		}
	}
	values.Set("duplicateDetectionCount", strconv.Itoa(len(form.DuplicateDetection)))
	for i, dd := range form.DuplicateDetection {
		prefix := "duplicateDetection." + strconv.Itoa(i)
		if dd.Enabled {
			values.Set(prefix+".enabled", "on")
		}
		if dd.DryRun {
			values.Set(prefix+".dryRun", "on")
		}
		values.Set(prefix+".schedule", dd.Schedule)
		values.Set(prefix+".detectionMode", dd.DetectionMode)
		values.Set(prefix+".actionMethod", dd.ActionMethod)
		values.Set(prefix+".duplicateFolder", dd.DuplicateFolder)
		values.Set(prefix+".ntfyURL", dd.NtfyURL)
		values.Set(prefix+".exclude", strings.Join(dd.Exclude, "\n"))
		values.Set(prefix+".sourceCount", strconv.Itoa(len(dd.Sources)))
		for j, source := range dd.Sources {
			values.Set(prefix+".sources."+strconv.Itoa(j), source)
		}
	}
	values.Set("saveTarget", saveTarget)
	values.Set("separatePath", separatePath)
	return values
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return newConfiguredServer(t, sampleConfigPath(), sampleConfig())
}

func newConfiguredServer(t *testing.T, configPath string, cfg *config.Config) *Server {
	t.Helper()
	server, err := New(Options{Address: "127.0.0.1:0", ConfigPath: configPath, Config: cfg})
	if err != nil {
		t.Fatalf("failed to create web UI server: %v", err)
	}
	return server
}

func sampleConfigPath() string {
	return filepath.Join("C:", "config", "config.yaml")
}

func sampleConfig() *config.Config {
	return &config.Config{
		StateDir: "/data/state",
		WebUI:    &config.WebUIConfig{Enabled: true, Address: "127.0.0.1:8080"},
		Organizer: []config.OrganizerProfile{{
			DryRun:          true,
			Target:          "/photos/organized",
			TargetStructure: "{year}/{month}",
			FallbackFolder:  "/photos/fallback",
			Ntfy:            &config.NtfyConfig{URL: "https://ntfy.sh/phorga"},
			Sources: []config.OrganizerSource{{
				Path:     "/photos/incoming",
				Mode:     "copy",
				Schedule: "0 2 * * *",
				Duplicates: &config.OrganizerDuplicate{
					DetectionMode: "filename",
					Action:        config.DuplicateAction{Method: "skip"},
				},
			}},
		}},
		DuplicateDetection: []config.DuplicateDetection{{
			Enabled:       true,
			DryRun:        true,
			Schedule:      "0 4 * * *",
			Sources:       []string{"/photos/organized"},
			DetectionMode: "content",
			Action:        config.DuplicateAction{Method: "skip"},
			Ntfy:          &config.NtfyConfig{URL: "https://ntfy.sh/phorga-dedup"},
		}},
	}
}
