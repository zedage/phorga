package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/zedage/phorga/internal/config"
)

// synologyHomes creates a homes folder with the given users, each with a
// Photos/MobileBackup folder.
func synologyHomes(t *testing.T, users ...string) string {
	t.Helper()
	homes := t.TempDir()
	for _, u := range users {
		if err := os.MkdirAll(filepath.Join(homes, u, "Photos", "MobileBackup"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return homes
}

// inputValue returns the value of the text input with the given name.
func inputValue(t *testing.T, body, name string) string {
	t.Helper()
	re := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("input %s not found", name)
	}
	return m[1]
}

// Spec: SYN-003
func TestSynologySectionListsUsers(t *testing.T) {
	homes := synologyHomes(t, "alice", "bob")
	cfg := sampleConfig()
	cfg.SynologyPhotos = &config.SynologyPhotos{Homes: homes}
	cfg.Organizer[0].Target = filepath.Join(homes, "bob", "Photos")
	s := newConfiguredServer(t, sampleConfigPath(), cfg)

	resp := httptest.NewRecorder()
	s.handlePage(resp, httptest.NewRequest(http.MethodGet, "/ui", nil))
	body := resp.Body.String()

	if !strings.Contains(body, `name="synologyUser" value="alice">`) {
		t.Error("alice must be offered as a selectable user")
	}
	if !strings.Contains(body, `name="synologyUser" value="bob" disabled checked>`) {
		t.Error("bob already has a profile and must be shown as set up")
	}
	if !strings.Contains(body, "already set up") || !strings.Contains(body, `value="addUsers"`) {
		t.Error("expected the configured badge and the add button")
	}
}

// Spec: SYN-003
func TestSynologySectionExplainsMissingHomes(t *testing.T) {
	cfg := sampleConfig()
	cfg.SynologyPhotos = &config.SynologyPhotos{Homes: filepath.Join(t.TempDir(), "missing")}
	s := newConfiguredServer(t, sampleConfigPath(), cfg)

	resp := httptest.NewRecorder()
	s.handlePage(resp, httptest.NewRequest(http.MethodGet, "/ui", nil))
	body := resp.Body.String()
	if !strings.Contains(body, "No homes folder found") || strings.Contains(body, `name="synologyUser"`) {
		t.Fatal("expected the hint for a missing homes folder and no user list")
	}
}

// Spec: SYN-002, SYN-003
func TestAddUsersCreatesProfiles(t *testing.T) {
	homes := synologyHomes(t, "alice", "bob")
	cfg := sampleConfig()
	cfg.SynologyPhotos = &config.SynologyPhotos{Homes: homes}
	cfg.Organizer[0].Target = filepath.Join(homes, "bob", "Photos")
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := newConfiguredServer(t, configPath, cfg)

	values := validFormValues(cfg, "active", "")
	values.Set("synologyPhotos.homes", homes)
	values.Set("op", "addUsers")
	values.Add("synologyUser", "alice")
	values.Add("synologyUser", "bob")     // already configured: ignored
	values.Add("synologyUser", "../etc")  // not discovered: ignored
	values.Add("synologyUser", "mallory") // not discovered: ignored
	resp := postForm(t, s, "/ui/form", values, false)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	body := resp.Body.String()

	if got := inputValue(t, body, "organizerCount"); got != "2" {
		t.Fatalf("expected exactly one added profile, organizerCount=%s", got)
	}
	if got := inputValue(t, body, "organizer.1.target"); got != filepath.Join(homes, "alice", "Photos") {
		t.Errorf("target = %q", got)
	}
	if got := inputValue(t, body, "organizer.1.sources.0.path"); got != filepath.Join(homes, "alice", "Photos", "MobileBackup") {
		t.Errorf("source = %q", got)
	}
	if !strings.Contains(body, `name="organizer.1.dryRun" checked`) {
		t.Error("the new profile must start as a dry run")
	}
	if !strings.Contains(body, `<option value="move" selected>move – remove the original after copying</option>`) {
		t.Error("the new source must use mode move")
	}
	if !strings.Contains(body, `data-focus="org-1"`) {
		t.Error("the page must scroll to the new profile")
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("adding users must not write the config file")
	}
}

// Spec: WEB-015
func TestFormOpsAddAndRemoveEntries(t *testing.T) {
	base := func() configForm { return newFormFromConfig(sampleConfig()) }
	cases := []struct {
		op    string
		check func(f configForm) bool
		focus string
	}{
		{"addProfile", func(f configForm) bool {
			p := f.Organizer[1]
			return len(f.Organizer) == 2 && p.DryRun && p.TargetStructure == "{year}/{month}" && len(p.Sources) == 1 && p.Sources[0].Mode == "copy" && p.Sources[0].Schedule == "0 2 * * *"
		}, "org-1"},
		{"removeProfile:0", func(f configForm) bool { return len(f.Organizer) == 0 }, "sec-organizer"},
		{"removeProfile:7", func(f configForm) bool { return len(f.Organizer) == 1 }, "sec-organizer"},
		{"addSource:0", func(f configForm) bool { return len(f.Organizer[0].Sources) == 2 }, "org-0-src-1"},
		{"removeSource:0:0", func(f configForm) bool { return len(f.Organizer[0].Sources) == 0 }, "org-0"},
		{"addDedup", func(f configForm) bool {
			d := f.DuplicateDetection[1]
			return len(f.DuplicateDetection) == 2 && d.Enabled && d.DryRun && d.DetectionMode == "metadata" && d.ActionMethod == "skip" && len(d.Sources) == 1
		}, "dd-1"},
		{"removeDedup:0", func(f configForm) bool { return len(f.DuplicateDetection) == 0 }, "sec-dedup"},
		{"addDedupSource:0", func(f configForm) bool { return len(f.DuplicateDetection[0].Sources) == 2 }, "dd-0"},
		{"removeDedupSource:0:0", func(f configForm) bool { return len(f.DuplicateDetection[0].Sources) == 0 }, "dd-0"},
		{"explode:0", func(f configForm) bool { return len(f.Organizer) == 1 && len(f.DuplicateDetection) == 1 }, ""},
		{"removeSource:0:-1", func(f configForm) bool { return len(f.Organizer[0].Sources) == 1 }, ""},
	}
	for _, tc := range cases {
		f := base()
		focus := applyFormOp(&f, tc.op, nil)
		if !tc.check(f) {
			t.Errorf("%s: unexpected form %+v", tc.op, f)
		}
		if focus != tc.focus {
			t.Errorf("%s: focus = %q, want %q", tc.op, focus, tc.focus)
		}
	}
}

// Spec: WEB-015
func TestFormOpKeepsTypedValuesAndNeverWrites(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := newConfiguredServer(t, configPath, sampleConfig())
	values := validFormValues(sampleConfig(), "active", "")
	values.Set("organizer.0.target", "/typed/but/not/saved")
	values.Set("op", "addSource:0")

	resp := postForm(t, s, "/ui/form", values, false)
	body := resp.Body.String()
	if got := inputValue(t, body, "organizer.0.target"); got != "/typed/but/not/saved" {
		t.Errorf("typed value lost: %q", got)
	}
	if got := inputValue(t, body, "organizer.0.sourceCount"); got != "2" {
		t.Errorf("sourceCount = %s", got)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("an edit must not write the config file")
	}

	get := httptest.NewRecorder()
	s.Handler().ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/ui/form", nil))
	if get.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /ui/form: expected 405, got %d", get.Code)
	}
}
