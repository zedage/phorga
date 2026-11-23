package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newPasswordServer(t *testing.T, password string) *Server {
	t.Helper()
	s, err := New(Options{Address: "127.0.0.1:0", ConfigPath: sampleConfigPath(), Config: sampleConfig(), Password: password})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func serve(s *Server, req *http.Request) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	s.Handler().ServeHTTP(resp, req)
	return resp
}

// Spec: WEB-020
func TestPasswordProtectsEveryPage(t *testing.T) {
	s := newPasswordServer(t, "s3cret")

	for _, target := range []string{"/ui", "/ui/status", "/ui/help", "/ui/yaml", "/ui/static/phorga.css"} {
		resp := serve(s, httptest.NewRequest(http.MethodGet, target, nil))
		if resp.Code != http.StatusUnauthorized {
			t.Errorf("%s without credentials: status %d, want 401", target, resp.Code)
		}
		if got := resp.Header().Get("WWW-Authenticate"); got != `Basic realm="Phorga", charset="UTF-8"` {
			t.Errorf("%s: WWW-Authenticate = %q", target, got)
		}
		if strings.Contains(resp.Body.String(), "/photos/organized") {
			t.Errorf("%s leaks the configuration without credentials", target)
		}
	}

	wrong := httptest.NewRequest(http.MethodGet, "/ui", nil)
	wrong.SetBasicAuth("anyone", "wrong")
	if resp := serve(s, wrong); resp.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: status %d, want 401", resp.Code)
	}

	right := httptest.NewRequest(http.MethodGet, "/ui", nil)
	right.SetBasicAuth("anyone", "s3cret")
	resp := serve(s, right)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "/photos/organized") {
		t.Fatalf("right password: status %d", resp.Code)
	}
}

// Spec: WEB-020
func TestNoPasswordNoPrompt(t *testing.T) {
	resp := get(t, newPasswordServer(t, ""), "/ui")
	if resp.Code != http.StatusOK || resp.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("without a password the UI must open directly, got %d", resp.Code)
	}
}

// Spec: WEB-021
func TestCrossSiteWritesAreRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := "# mine\norganizer: []\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := sampleConfig()
	s := newConfiguredServer(t, path, cfg)

	// Cross-site requests try to save; the allowed ones only validate.
	post := func(action string, headers map[string]string) *httptest.ResponseRecorder {
		values := url.Values{"content": {"# changed\n"}, "action": {action}}
		req := httptest.NewRequest(http.MethodPost, "http://nas:8080/ui/yaml", strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return serve(s, req)
	}

	if resp := post("save", map[string]string{"Sec-Fetch-Site": "cross-site"}); resp.Code != http.StatusForbidden {
		t.Errorf("cross-site post: status %d, want 403", resp.Code)
	}
	if resp := post("save", map[string]string{"Origin": "http://evil.example"}); resp.Code != http.StatusForbidden {
		t.Errorf("post from another origin: status %d, want 403", resp.Code)
	}
	if resp := post("validate", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://nas:8080"}); resp.Code != http.StatusOK {
		t.Errorf("same-origin post: status %d, want 200", resp.Code)
	}
	if resp := post("validate", nil); resp.Code != http.StatusOK {
		t.Errorf("post without browser headers: status %d, want 200", resp.Code)
	}
	// Reading stays possible from anywhere, e.g. a link from another page.
	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if resp := serve(s, req); resp.Code != http.StatusOK {
		t.Errorf("cross-site GET: status %d, want 200", resp.Code)
	}
	if data, _ := os.ReadFile(path); string(data) != original {
		t.Fatalf("the config file changed: %q", data)
	}
}
