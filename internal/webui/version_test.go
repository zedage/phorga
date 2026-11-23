package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Spec: RUN-004
func TestTopBarShowsVersion(t *testing.T) {
	s, err := New(Options{Address: "127.0.0.1:0", ConfigPath: sampleConfigPath(), Config: sampleConfig(), Version: "2026.11.0"})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/ui", "/ui/status", "/ui/help", "/ui/yaml"} {
		resp := serve(s, httptest.NewRequest(http.MethodGet, target, nil))
		if resp.Code != http.StatusOK {
			t.Fatalf("%s: status %d", target, resp.Code)
		}
		if !strings.Contains(resp.Body.String(), `<small class="brand-version">2026.11.0</small>`) {
			t.Errorf("%s: the top bar must show the version", target)
		}
	}
}

// Spec: RUN-004
func TestTopBarWithoutVersion(t *testing.T) {
	s, err := New(Options{Address: "127.0.0.1:0", ConfigPath: sampleConfigPath(), Config: sampleConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if body := serve(s, httptest.NewRequest(http.MethodGet, "/ui", nil)).Body.String(); strings.Contains(body, "brand-version") {
		t.Error("without a version, the top bar must show none")
	}
}
