package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func get(t *testing.T, s *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	resp := httptest.NewRecorder()
	s.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, target, nil))
	return resp
}

func postForm(t *testing.T, s *Server, target string, values url.Values, htmx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if htmx {
		req.Header.Set("HX-Request", "true")
	}
	resp := httptest.NewRecorder()
	s.Handler().ServeHTTP(resp, req)
	return resp
}

// Spec: WEB-007
func TestPageRendersSelectsWithCurrentValue(t *testing.T) {
	cfg := sampleConfig()
	cfg.DuplicateDetection[0].Action.Method = "explode"
	body := get(t, newConfiguredServer(t, sampleConfigPath(), cfg), "/ui").Body.String()

	for _, want := range []string{
		`<select id="f-organizer-0-sources-0-mode" name="organizer.0.sources.0.mode"`,
		`<option value="copy" selected>copy</option>`,
		`<option value="move">move</option>`,
		`<option value="addSuffix">addSuffix</option>`,
		`<option value="content" selected>content</option>`,
		`<option value="explode" selected>explode (unknown)</option>`,
		`id="f-organizer-0-target-help"`,
		"copy keeps the original",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page is missing %q", want)
		}
	}
}

// Spec: WEB-007, WEB-005
func TestUIMessagesExistInAllLanguages(t *testing.T) {
	for key := range uiMessages["en"] {
		for lang := range supportedLanguages {
			if _, ok := uiMessages[lang][key]; !ok {
				t.Errorf("key %q missing for language %s", key, lang)
			}
		}
	}
	for lang, messages := range uiMessages {
		if len(messages) != len(uiMessages["en"]) {
			t.Errorf("language %s has %d keys, en has %d", lang, len(messages), len(uiMessages["en"]))
		}
	}
}

// Spec: WEB-007
func TestPageUsesOnlyKnownKeys(t *testing.T) {
	for _, lang := range []string{"de", "en", "it", "fr"} {
		body := get(t, newTestServer(t), "/ui?lang="+lang).Body.String()
		for _, prefix := range []string{">help_", ">browse", ">not_set", ">nav_"} {
			if strings.Contains(body, prefix) {
				t.Errorf("%s: page renders an untranslated key starting with %q", lang, prefix[1:])
			}
		}
	}
}

// Spec: WEB-008
func TestPreviewStructure(t *testing.T) {
	s := newTestServer(t)
	ok := postForm(t, s, "/ui/preview/structure?field=organizer.0.targetStructure",
		url.Values{"organizer.0.targetStructure": {"{year}/{month}/{day}"}}, true).Body.String()
	if !strings.Contains(ok, `class="ok"`) || !strings.Contains(ok, "<code>2024/03/15</code>") {
		t.Fatalf("unexpected preview %s", ok)
	}

	empty := postForm(t, s, "/ui/preview/structure?field=f", url.Values{"f": {""}}, true).Body.String()
	if !strings.Contains(empty, "<code>2024/03</code>") {
		t.Fatalf("empty structure must preview the default, got %s", empty)
	}

	bad := postForm(t, s, "/ui/preview/structure?field=organizer.0.targetStructure",
		url.Values{"organizer.0.targetStructure": {"{year}/{week}"}}, true).Body.String()
	if !strings.Contains(bad, `class="bad"`) || !strings.Contains(bad, "unknown token") {
		t.Fatalf("expected token error, got %s", bad)
	}
}

// Spec: WEB-008
func TestPreviewSchedule(t *testing.T) {
	old := now
	now = func() time.Time { return time.Date(2026, 11, 11, 10, 0, 0, 0, time.Local) }
	defer func() { now = old }()

	s := newTestServer(t)
	body := postForm(t, s, "/ui/preview/schedule?field=x", url.Values{"x": {"0 2 * * *"}}, true).Body.String()
	for _, want := range []string{"2026-11-12 02:00", "2026-11-13 02:00", "2026-11-14 02:00"} {
		if !strings.Contains(body, want) {
			t.Errorf("schedule preview missing %q: %s", want, body)
		}
	}

	bad := postForm(t, s, "/ui/preview/schedule?field=x", url.Values{"x": {"every day"}}, true).Body.String()
	if !strings.Contains(bad, `class="bad"`) {
		t.Fatalf("expected invalid preview, got %s", bad)
	}
}

// Spec: WEB-008
func TestValidateReturnsFieldErrorsOutOfBand(t *testing.T) {
	s := newTestServer(t)
	values := validFormValues(sampleConfig(), "active", "")
	values.Set("organizer.0.target", "")
	values.Set("organizer.0.sources.0.actionMethod", "explode")

	body := postForm(t, s, "/ui/validate", values, true).Body.String()
	for _, want := range []string{
		`id="err-f-organizer-0-target" hx-swap-oob="true">organizer[0].target is required`,
		`id="err-f-organizer-0-sources-0-actionMethod" hx-swap-oob="true">organizer[0].sources[0].duplicates.action.method`,
		"Validation failed",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing %q:\n%s", want, body)
		}
	}

	// Without HTMX the full page shows the error next to the field.
	page := postForm(t, s, "/ui/validate", values, false).Body.String()
	if !strings.Contains(page, `id="err-f-organizer-0-target">organizer[0].target is required`) {
		t.Errorf("full page does not show the field error inline")
	}
}

// Spec: WEB-008
func TestFieldForProblem(t *testing.T) {
	cases := map[string]string{
		"organizer[0].target is required":                                             "organizer.0.target",
		"organizer[1].sources[2].mode must be 'move' or 'copy', got 'x'":              "organizer.1.sources.2.mode",
		"organizer[0].sources[0].duplicates.detectionMode: detectionMode is required": "organizer.0.sources.0.detectionMode",
		"organizer[0].sources[0].duplicates.action.duplicateFolder is required":       "organizer.0.sources.0.duplicateFolder",
		"duplicateDetection[0].action.method must be 'moveToFolder' or 'skip'":        "duplicateDetection.0.actionMethod",
		"duplicateDetection[0].schedule: invalid cron expression":                     "duplicateDetection.0.schedule",
		"webUI.browseRoots[3] 'x' does not exist":                                     "webUI.browseRoots",
		"separatePath is required when saving to a separate file":                     "separatePath",
	}
	for problem, want := range cases {
		if got := fieldForProblem(problem); got != want {
			t.Errorf("fieldForProblem(%q) = %q, want %q", problem, got, want)
		}
	}
}

// Spec: WEB-009
func TestHTMXIsServedFromBinary(t *testing.T) {
	s := newTestServer(t)
	resp := get(t, s, "/ui/static/htmx.min.js")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "htmx") {
		t.Fatalf("expected embedded htmx, got %d", resp.Code)
	}
	if resp.Header().Get("Cache-Control") == "" {
		t.Error("expected a Cache-Control header on static assets")
	}
	for _, page := range []string{"/ui", "/ui/status"} {
		body := get(t, s, page).Body.String()
		if strings.Contains(body, "unpkg.com") || !strings.Contains(body, `src="/ui/static/htmx.min.js"`) {
			t.Errorf("%s must load htmx from /ui/static", page)
		}
		if strings.Contains(body, `href="http`) || strings.Contains(body, `src="http`) {
			t.Errorf("%s must not load assets from external hosts", page)
		}
	}
}

// Spec: WEB-012
func TestThemeFollowsSystemColorScheme(t *testing.T) {
	s := newTestServer(t)
	for _, page := range []string{"/ui", "/ui/status"} {
		body := get(t, s, page).Body.String()
		if !strings.Contains(body, `<meta name="color-scheme" content="light dark">`) {
			t.Errorf("%s must declare color-scheme light dark", page)
		}
		if !strings.Contains(body, `href="/ui/static/phorga.css?v=`) {
			t.Errorf("%s must load the embedded stylesheet", page)
		}
	}
	resp := get(t, s, "/ui/static/phorga.css")
	if resp.Code != http.StatusOK {
		t.Fatalf("stylesheet: got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("stylesheet Content-Type = %q, want text/css", ct)
	}
	if resp.Header().Get("Cache-Control") == "" {
		t.Error("expected a Cache-Control header on the stylesheet")
	}
	css := resp.Body.String()
	for _, want := range []string{"color-scheme: light dark", "@media (prefers-color-scheme: dark)"} {
		if !strings.Contains(css, want) {
			t.Errorf("stylesheet must contain %q", want)
		}
	}
}

type fakeStatus []JobStatus

func (f fakeStatus) Jobs() []JobStatus { return f }

// Spec: WEB-011
func TestStatusTableShowsJobs(t *testing.T) {
	jobs := fakeStatus{
		{
			Name: "organizer-0-source-0", Kind: "organizer", Label: "/in → /out", Schedule: "0 2 * * *",
			Next: time.Date(2026, 11, 12, 2, 0, 0, 0, time.Local), Running: true,
			HasLast: true, LastStart: time.Date(2026, 11, 11, 2, 0, 0, 0, time.Local), LastDuration: 3 * time.Second,
			Counters: []Counter{{"processed", 7}, {"skipped", 1}, {"errors", 0}}, DryRun: true,
		},
		{Name: "dedup-0", Kind: "dedup", Label: "/out", Schedule: "0 4 * * *"},
	}
	s, err := New(Options{Address: "127.0.0.1:0", ConfigPath: sampleConfigPath(), Config: sampleConfig(), Status: jobs})
	if err != nil {
		t.Fatal(err)
	}

	body := get(t, s, "/ui/status/table").Body.String()
	for _, want := range []string{
		"/in → /out", "2026-11-12 02:00:00", "running", "2026-11-11 02:00:00", "3s",
		"7 processed · 1 skipped · 0 errors", "dry run", "Duplicate detection", "not run yet",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("status table missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<html") {
		t.Error("table endpoint must return a fragment")
	}

	page := get(t, s, "/ui/status").Body.String()
	if !strings.Contains(page, `hx-get="/ui/status/table" hx-trigger="every 10s"`) {
		t.Error("status page must poll the table every 10 s")
	}
}

// Spec: WEB-011
func TestStatusPageWithoutSource(t *testing.T) {
	body := get(t, newTestServer(t), "/ui/status").Body.String()
	if !strings.Contains(body, "Status information is not available.") {
		t.Fatalf("expected unavailable notice, got %s", body)
	}
	if code := postForm(t, newTestServer(t), "/ui/status", nil, false).Code; code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", code)
	}
}

// Spec: WEB-013
func TestThemeSwitchIsRendered(t *testing.T) {
	s := newTestServer(t)
	labels := map[string][]string{
		"en": {"System", "Light", "Dark"},
		"de": {"System", "Hell", "Dunkel"},
		"it": {"Sistema", "Chiaro", "Scuro"},
		"fr": {"Système", "Clair", "Sombre"},
	}
	for lang, want := range labels {
		for _, page := range []string{"/ui", "/ui/status"} {
			body := get(t, s, page+"?lang="+lang).Body.String()
			for _, choice := range []string{"auto", "light", "dark"} {
				if !strings.Contains(body, `data-theme-set="`+choice+`"`) {
					t.Errorf("%s %s: missing theme choice %q", lang, page, choice)
				}
			}
			for _, label := range want {
				if !strings.Contains(body, `aria-label="`+label+`"`) {
					t.Errorf("%s %s: missing translated label %q", lang, page, label)
				}
			}
			if strings.Contains(body, ">theme_") || strings.Contains(body, `"theme_`) {
				t.Errorf("%s %s: untranslated theme key", lang, page)
			}
			if !strings.Contains(body, "localStorage.getItem('phorga-theme')") {
				t.Errorf("%s %s: stored theme must be applied in <head>", lang, page)
			}
		}
	}
	css := get(t, s, "/ui/static/phorga.css").Body.String()
	for _, sel := range []string{`:root[data-theme="dark"]`, `:root:not([data-theme="light"])`, `:root[data-theme="light"]`} {
		if !strings.Contains(css, sel) {
			t.Errorf("stylesheet must contain selector %s", sel)
		}
	}
}
