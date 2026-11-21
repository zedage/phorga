package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zedage/phorga/internal/config"
)

func pageBody(t *testing.T, s *Server) string {
	t.Helper()
	resp := httptest.NewRecorder()
	s.handlePage(resp, httptest.NewRequest(http.MethodGet, "/ui?lang=en", nil))
	return resp.Body.String()
}

// Spec: WEB-016
func TestCardsStartCollapsedWithFlowSummary(t *testing.T) {
	body := pageBody(t, newTestServer(t))
	cfg := sampleConfig()

	for _, id := range []string{"org-0-details", "dd-0-details", "sec-general-details", "sec-webui-details", "sec-save-details", "org-0-more"} {
		if !strings.Contains(body, `id="`+id+`">`) {
			t.Errorf("%s must start collapsed", id)
		}
	}
	// The summary shows where files come from and where they go.
	summary := body[strings.Index(body, `id="org-0-details"`):strings.Index(body, `class="entity-body"`)]
	for _, want := range []string{cfg.Organizer[0].Sources[0].Path, cfg.Organizer[0].Target, "{year}/{month}", "0 2 * * *", ">copy<", "Organizer profile 1"} {
		if !strings.Contains(summary, want) {
			t.Errorf("flow summary is missing %q", want)
		}
	}
	// Collapsed fields are still part of the form, so saving keeps them.
	if !strings.Contains(body, `name="organizer.0.ntfyURL" value="https://ntfy.sh/phorga"`) {
		t.Error("fields in a collapsed card must still be submitted")
	}
}

// Spec: WEB-016
func TestCardOpensForErrorsAndNewEntries(t *testing.T) {
	s := newTestServer(t)
	values := validFormValues(sampleConfig(), "active", "")
	values.Set("organizer.0.ntfyURL", "")
	values.Set("organizer.0.sources.0.exclude", "[abc")
	body := postForm(t, s, "/ui/validate", values, false).Body.String()
	if !strings.Contains(body, `id="org-0-details" open>`) || !strings.Contains(body, `id="org-0-src-0-more" open>`) {
		t.Error("a card with a field error and its more-options block must be open")
	}
	if !strings.Contains(body, `id="dd-0-details">`) {
		t.Error("a card without errors stays collapsed")
	}

	values = validFormValues(sampleConfig(), "active", "")
	values.Set("op", "addDedup")
	body = postForm(t, s, "/ui/form", values, false).Body.String()
	if !strings.Contains(body, `id="dd-1-details" open>`) {
		t.Error("a newly added job must be open")
	}
}

// Spec: WEB-017
func TestDetectionModeAsCheckboxes(t *testing.T) {
	cfg := sampleConfig()
	cfg.Organizer[0].Sources[0].Duplicates.DetectionMode = "filedate|exif"
	body := pageBody(t, newConfiguredServer(t, sampleConfigPath(), cfg))

	prefix := `name="organizer.0.sources.0.detectionFields" value=`
	for field, checked := range map[string]bool{"filename": false, "filedate": true, "exif": true} {
		tag := prefix + `"` + field + `"`
		if checked {
			tag += " checked"
		}
		if !strings.Contains(body, tag+">") {
			t.Errorf("checkbox %s: want checked=%v", field, checked)
		}
	}
	if !strings.Contains(body, `name="organizer.0.sources.0.detectionJoin" value="any" checked>`) {
		t.Error("| must select 'one match is enough'")
	}
	for _, label := range []string{"Same file name", "Same capture date", "One match is enough"} {
		if !strings.Contains(body, label) {
			t.Errorf("missing label %q", label)
		}
	}
	if strings.Contains(body, `name="organizer.0.sources.0.detectionMode"`) {
		t.Error("the detection mode must no longer be a text field")
	}
}

// Spec: WEB-017
func TestDetectionCheckboxesBuildTheMode(t *testing.T) {
	cases := []struct {
		fields []string
		join   string
		want   string
	}{
		{[]string{"exif", "filename"}, "all", "filename+exif"},
		{[]string{"filedate", "exif"}, "any", "filedate|exif"},
		{[]string{"exif"}, "any", "exif"},
		{nil, "all", ""},
	}
	for _, tc := range cases {
		values := url.Values{
			"organizerCount":                        {"1"},
			"organizer.0.sourceCount":               {"1"},
			"organizer.0.sources.0.detectionUI":     {"1"},
			"organizer.0.sources.0.detectionFields": tc.fields,
			"organizer.0.sources.0.detectionJoin":   {tc.join},
		}
		req := httptest.NewRequest(http.MethodPost, "/ui/validate", strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		form, err := parseForm(req)
		if err != nil {
			t.Fatal(err)
		}
		if got := form.Organizer[0].Sources[0].DetectionMode; got != tc.want {
			t.Errorf("%v %s: got %q, want %q", tc.fields, tc.join, got, tc.want)
		}
	}
}

// Spec: WEB-017
func TestNoDetectionFieldTickedIsReported(t *testing.T) {
	s := newTestServer(t)
	values := validFormValues(sampleConfig(), "active", "")
	values.Del("organizer.0.sources.0.detectionMode")
	values.Set("organizer.0.sources.0.detectionUI", "1")
	body := postForm(t, s, "/ui/validate", values, true).Body.String()
	if !strings.Contains(body, `id="err-f-organizer-0-sources-0-detectionMode" hx-swap-oob="true">organizer[0].sources[0].duplicates.detectionMode: detectionMode is required`) {
		t.Fatalf("expected the error next to the checkboxes, got %s", body)
	}
}

// Spec: WEB-017
func TestInvalidDetectionModeFromFileIsShown(t *testing.T) {
	cfg := sampleConfig()
	cfg.Organizer[0].Sources[0].Duplicates.DetectionMode = "filename+exif|filedate"
	body := pageBody(t, newConfiguredServer(t, sampleConfigPath(), cfg))
	if !strings.Contains(body, "<code>filename&#43;exif|filedate</code>") {
		t.Error("an invalid value from the file must be shown")
	}
}

// Spec: CFG-011
func TestNamesAreEditableAndShown(t *testing.T) {
	cfg := sampleConfig()
	cfg.Organizer[0].Name = "Robin's phone"
	s := newConfiguredServer(t, sampleConfigPath(), cfg)
	body := pageBody(t, s)
	if !strings.Contains(body, `name="organizer.0.name" value="Robin&#39;s phone"`) || !strings.Contains(body, `>Robin&#39;s phone</h3>`) {
		t.Error("the profile name must be editable and used as the card title")
	}
	if !strings.Contains(body, `name="duplicateDetection.0.name" value="" spellcheck="false" placeholder="Duplicate job 1"`) || !strings.Contains(body, ">Duplicate job 1</h3>") {
		t.Error("an unnamed job must show its number as title and placeholder")
	}

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s = newConfiguredServer(t, configPath, cfg)
	values := validFormValues(cfg, "active", "")
	values.Set("organizer.0.name", "  Holidays ")
	values.Set("duplicateDetection.0.name", "All photos")
	postForm(t, s, "/ui/save", values, true)
	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Organizer[0].Name != "Holidays" || saved.DuplicateDetection[0].Name != "All photos" {
		t.Fatalf("names not saved: %q, %q", saved.Organizer[0].Name, saved.DuplicateDetection[0].Name)
	}
}

// Spec: WEB-018
func TestRawEditorShowsFileWithComments(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	text := "# my notes\nstateDir: /data/state\n"
	if err := os.WriteFile(configPath, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	s := newConfiguredServer(t, configPath, sampleConfig())
	if body := get(t, s, "/ui/yaml").Body.String(); !strings.Contains(body, "# my notes\nstateDir: /data/state") || !strings.Contains(body, "Edit YAML") {
		t.Fatal("the editor must show the file text as it is")
	}
	if !strings.Contains(pageBody(t, s), `href="/ui/yaml"`) {
		t.Error("the configuration page must link to the editor")
	}
}

// Spec: WEB-018
func TestRawEditorRejectsInvalidText(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	called := false
	s, err := New(Options{Address: "127.0.0.1:0", ConfigPath: configPath, Config: sampleConfig(),
		ApplyRaw: func([]byte) (*config.Config, []string, error) { called = true; return nil, nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	for text, want := range map[string]string{
		"organizer: [":                   "parsing YAML config",
		"organizer:\n  - target: \"\"\n": "organizer[0].target is required",
	} {
		body := postForm(t, s, "/ui/yaml", url.Values{"content": {text}, "action": {"save"}}, false).Body.String()
		if !strings.Contains(body, want) {
			t.Errorf("%q: expected %q in the response", text, want)
		}
		if !strings.Contains(body, "organizer:") {
			t.Errorf("%q: the edited text must stay in the editor", text)
		}
	}
	if called {
		t.Fatal("invalid text must not be applied")
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("invalid text must not be written")
	}
}

// Spec: WEB-018
func TestRawEditorSavesTextAsItIs(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	var applied []byte
	s, err := New(Options{Address: "127.0.0.1:0", ConfigPath: configPath, Config: config.Default(),
		ApplyRaw: func(data []byte) (*config.Config, []string, error) {
			applied = data
			cfg, err := config.Parse(configPath, data)
			return cfg, nil, err
		}})
	if err != nil {
		t.Fatal(err)
	}
	// Setup mode: no file yet, the editor shows what would be written.
	if body := get(t, s, "/ui/yaml").Body.String(); !strings.Contains(body, "The file does not exist yet") || !strings.Contains(body, "stateDir: /data/state") {
		t.Fatal("without a file the editor must show the configuration in effect")
	}

	text := "# keep me\r\nstateDir: /data/state\r\norganizer:\r\n  - name: Phone\r\n    target: /photos/out\r\n    sources:\r\n      - path: /photos/in\r\n        mode: copy\r\n        schedule: \"0 2 * * *\"\r\n"
	body := postForm(t, s, "/ui/yaml", url.Values{"content": {text}, "action": {"save"}}, false).Body.String()
	if !strings.Contains(body, "Saved and applied") {
		t.Fatalf("expected the applied message, got %s", body)
	}
	if want := strings.ReplaceAll(text, "\r\n", "\n"); string(applied) != want {
		t.Fatalf("applied text = %q, want %q", applied, want)
	}
	if s.currentConfig().Organizer[0].Name != "Phone" {
		t.Error("the form must show the applied configuration")
	}

	// Validate only: nothing is applied.
	applied = nil
	body = postForm(t, s, "/ui/yaml", url.Values{"content": {text}, "action": {"validate"}}, false).Body.String()
	if applied != nil || !strings.Contains(body, "OK") {
		t.Fatal("validate must only check the text")
	}
}

// Spec: WEB-019
func TestStatusJobsLinkToTheirSchedule(t *testing.T) {
	s, err := New(Options{Address: "127.0.0.1:0", ConfigPath: sampleConfigPath(), Config: sampleConfig(),
		Status: fakeStatus{
			{Name: "organizer-0-source-1", Kind: "organizer", Title: "Robin", Label: "/in → /out", Field: "organizer.0.sources.1.schedule", Schedule: "0 2 * * *"},
			{Name: "dedup-0", Kind: "dedup", Label: "/a, /b", Field: "duplicateDetection.0.schedule", Schedule: "0 4 * * *"},
		}})
	if err != nil {
		t.Fatal(err)
	}
	body := get(t, s, "/ui/status/table").Body.String()
	for _, want := range []string{
		`href="/ui#f-organizer-0-sources-1-schedule"`,
		`href="/ui#f-duplicateDetection-0-schedule"`,
		`<strong>Robin</strong>`,
		`<strong>Duplicate detection</strong>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("status table is missing %s", want)
		}
	}
	// The link target exists on the configuration page.
	if !strings.Contains(pageBody(t, s), `id="f-duplicateDetection-0-schedule"`) {
		t.Error("the schedule field id must exist on the configuration page")
	}
}
