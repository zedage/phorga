package webui

import (
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zedage/phorga/internal/config"
)

// Spec: WEB-022
func TestHelpPageInEveryLanguage(t *testing.T) {
	s := newTestServer(t)
	headings := map[string]string{
		"en": "Install on a Synology NAS",
		"de": "Auf einem Synology-NAS installieren",
		"it": "Installare su un NAS Synology",
		"fr": "Installer sur un NAS Synology",
	}
	for lang, heading := range headings {
		resp := get(t, s, "/ui/help?lang="+lang)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s: status %d", lang, resp.Code)
		}
		body := html.UnescapeString(resp.Body.String())
		for _, want := range []string{heading, ComposeSynology, ComposeOther, "PHORGA_WEB_UI_PASSWORD", `id="synology"`, `id="other"`, `id="setup"`, `id="remote"`, `id="trouble"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: help page lacks %q", lang, want)
			}
		}
		if !strings.Contains(body, `<a href="/ui/help" class="active" aria-current="page">`) {
			t.Errorf("%s: the top bar must mark Help as the current page", lang)
		}
	}
	if !strings.Contains(get(t, s, "/ui/help?lang=de").Body.String(), "Hilfe") {
		t.Error("the German top bar must say Hilfe")
	}
	// A language without a guide of its own falls back to English.
	if body := get(t, s, "/ui/help?lang=es").Body.String(); !strings.Contains(body, headings["en"]) {
		t.Error("unknown languages must get the English guide")
	}
	if resp := postForm(t, s, "/ui/help", nil, false); resp.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /ui/help: status %d, want 405", resp.Code)
	}
}

// Spec: WEB-022, WEB-020
func TestHelpPageDescribesThisInstallation(t *testing.T) {
	homes := t.TempDir()
	for _, user := range []string{"alice", "bob"} {
		if err := os.MkdirAll(filepath.Join(homes, user, "Photos"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := sampleConfig()
	cfg.SynologyPhotos = &config.SynologyPhotos{Homes: homes}
	missing := filepath.Join(t.TempDir(), "config.yaml")

	body := get(t, newConfiguredServer(t, missing, cfg), "/ui/help").Body.String()
	for _, want := range []string{missing, "not created yet", homes, "2 users found", "off – anyone who can open this page", "127.0.0.1:0"} {
		if !strings.Contains(body, want) {
			t.Errorf("help page lacks %q", want)
		}
	}

	s, err := New(Options{Address: "127.0.0.1:0", ConfigPath: missing, Config: cfg, Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "/ui/help", nil)
	req.SetBasicAuth("x", "pw")
	if body := serve(s, req).Body.String(); !strings.Contains(body, `<span class="badge ok">on</span>`) {
		t.Error("with a password, the help page must say it is on")
	}
}

// Spec: WEB-022
func TestReadmeHasTheHelpPageComposeFiles(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	// In the README, the blocks sit in a numbered list, indented by three spaces.
	indent := func(text string) string {
		var b strings.Builder
		for _, line := range strings.SplitAfter(text, "\n") {
			if strings.TrimSpace(line) != "" {
				b.WriteString("   ")
			}
			b.WriteString(line)
		}
		return b.String()
	}
	for name, compose := range map[string]string{"ComposeSynology": ComposeSynology, "ComposeOther": ComposeOther} {
		if !strings.Contains(string(readme), indent("```yaml\n"+compose+"```")) {
			t.Errorf("README must contain %s unchanged in a yaml code block", name)
		}
	}
}

// Spec: WEB-022
func TestHelpLinkOnEveryPage(t *testing.T) {
	s := newConfiguredServer(t, filepath.Join(t.TempDir(), "config.yaml"), config.Default())
	for _, target := range []string{"/ui", "/ui/status", "/ui/yaml", "/ui/help"} {
		if body := get(t, s, target).Body.String(); !strings.Contains(body, `href="/ui/help"`) {
			t.Errorf("%s has no link to the help page", target)
		}
	}
	// The setup hint (no jobs yet) links to the guide.
	body := get(t, s, "/ui").Body.String()
	if !strings.Contains(body, `<a href="/ui/help">Step-by-step guide</a>`) {
		t.Error("the setup hint must link to the guide")
	}
}
