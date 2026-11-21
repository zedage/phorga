package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zedage/phorga/internal/config"
)

const startupTimeout = 5 * time.Second

// testConfig returns a valid config whose paths live in temp dirs. The
// schedule fires once a year, so no job runs during a test.
func testConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		StateDir: filepath.Join(t.TempDir(), "state"),
		Organizer: []config.OrganizerProfile{{
			Target:          filepath.Join(t.TempDir(), "original-target"),
			TargetStructure: "{year}/{month}",
			Sources: []config.OrganizerSource{{
				Path:     t.TempDir(),
				Mode:     "copy",
				Schedule: "0 0 1 1 *",
			}},
		}},
	}
}

func writeConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	return path
}

// freeAddr returns a loopback address with a port that was free a moment ago.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

// app is a Phorga instance running in the background via run.
type app struct {
	t      *testing.T
	cancel context.CancelFunc
	ch     chan int
	code   int
	exited bool
}

// startRun runs the app in the background and stops it when the test ends.
func startRun(t *testing.T, args ...string) *app {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	a := &app{t: t, cancel: cancel, ch: make(chan int, 1)}
	go func() { a.ch <- run(ctx, args, io.Discard) }()
	t.Cleanup(func() { a.stop() })
	return a
}

// wait blocks until run returns (on its own or after stop) and returns its exit code.
func (a *app) wait() int {
	a.t.Helper()
	if !a.exited {
		select {
		case a.code = <-a.ch:
			a.exited = true
		case <-time.After(startupTimeout):
			a.t.Fatal("run did not return")
		}
	}
	return a.code
}

// stop cancels the context, like SIGINT/SIGTERM in main, and returns the exit code.
func (a *app) stop() int {
	a.t.Helper()
	a.cancel()
	return a.wait()
}

// getUI polls GET http://addr/ui until it answers or the timeout expires.
func getUI(t *testing.T, addr string) string {
	t.Helper()
	deadline := time.Now().Add(startupTimeout)
	for {
		resp, err := http.Get("http://" + addr + "/ui")
		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /ui: status %d", resp.StatusCode)
			}
			return string(body)
		}
		if time.Now().After(deadline) {
			t.Fatalf("web UI at %s did not come up: %v", addr, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitForDir(t *testing.T, dir string) {
	t.Helper()
	deadline := time.Now().Add(startupTimeout)
	for {
		if _, err := os.Stat(dir); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s was not created", dir)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Spec: CFG-002
func TestRun_InvalidConfigExitsNonZero(t *testing.T) {
	cfg := testConfig(t)
	cfg.Organizer[0].Target = ""
	a := startRun(t, "--config", writeConfig(t, cfg))

	if code := a.wait(); code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
}

// Spec: CFG-003, CFG-009
func TestLoadConfig_MissingFileGivesSetupConfig(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	path, cfg, exists, err := loadConfig(missing)
	if err != nil {
		t.Fatalf("a missing config file must not fail startup: %v", err)
	}
	if path != missing || exists || cfg.HasJobs() || cfg.WebUI == nil || !cfg.WebUI.Enabled {
		t.Fatalf("path=%s exists=%v cfg=%+v", path, exists, cfg)
	}
	if _, _, _, err := loadConfig(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("a missing file with an unsupported extension must fail")
	}
}

// Spec: CFG-003
func TestRun_UnknownFlagExitsWithUsageError(t *testing.T) {
	a := startRun(t, "--no-such-flag")

	if code := a.wait(); code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
}

// Spec: WEB-001
func TestRun_WebUIEnabledByFlag(t *testing.T) {
	addr := freeAddr(t)
	a := startRun(t, "--config", writeConfig(t, testConfig(t)), "--web-ui", "--web-ui-addr", addr)

	getUI(t, addr)

	if code := a.stop(); code != exitOK {
		t.Errorf("exit code after shutdown = %d, want 0", code)
	}
	if _, err := http.Get("http://" + addr + "/ui"); err == nil {
		t.Error("web UI still answers after shutdown")
	}
}

// Spec: WEB-001
func TestRun_WebUIEnabledByConfig(t *testing.T) {
	addr := freeAddr(t)
	cfg := testConfig(t)
	cfg.WebUI = &config.WebUIConfig{Enabled: true, Address: addr}
	a := startRun(t, "--config", writeConfig(t, cfg))

	getUI(t, addr)

	if code := a.stop(); code != exitOK {
		t.Errorf("exit code after shutdown = %d, want 0", code)
	}
}

// Spec: WEB-001
func TestRun_WebUIOffByDefault(t *testing.T) {
	addr := freeAddr(t)
	cfg := testConfig(t)
	// The address is configured, but the UI is not enabled.
	cfg.WebUI = &config.WebUIConfig{Enabled: false, Address: addr}
	a := startRun(t, "--config", writeConfig(t, cfg))

	// The state dir is created right before the UI would start.
	waitForDir(t, cfg.StateDir)
	time.Sleep(200 * time.Millisecond)
	if conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		conn.Close()
		t.Error("web UI is listening although it was not enabled")
	}

	if code := a.stop(); code != exitOK {
		t.Errorf("exit code after shutdown = %d, want 0", code)
	}
}

// Spec: WEB-001
func TestRun_WebUIListenFailureExitsNonZero(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()

	a := startRun(t, "--config", writeConfig(t, testConfig(t)), "--web-ui", "--web-ui-addr", busy.Addr().String())

	if code := a.wait(); code != exitError {
		t.Errorf("exit code = %d, want %d when the UI port is taken", code, exitError)
	}
}

// Spec: CFG-008
func TestRun_ConfigIsNotReloaded(t *testing.T) {
	addr := freeAddr(t)
	cfg := testConfig(t)
	cfg.WebUI = &config.WebUIConfig{Enabled: true, Address: addr}
	path := writeConfig(t, cfg)
	originalTarget := cfg.Organizer[0].Target
	a := startRun(t, "--config", path)

	if body := getUI(t, addr); !strings.Contains(body, originalTarget) {
		t.Fatalf("UI does not show the loaded target %q", originalTarget)
	}

	changed := *cfg
	changed.Organizer = []config.OrganizerProfile{cfg.Organizer[0]}
	changed.Organizer[0].Target = filepath.Join(t.TempDir(), "changed-target")
	if err := config.Save(path, &changed); err != nil {
		t.Fatal(err)
	}

	body := getUI(t, addr)
	if !strings.Contains(body, originalTarget) || strings.Contains(body, "changed-target") {
		t.Errorf("running process picked up the changed config file; want startup target %q", originalTarget)
	}
	a.stop()
}

// emptyConfig writes a config without jobs (setup mode, CFG-009).
func emptyConfig(t *testing.T) (string, string) {
	t.Helper()
	stateDir := filepath.Join(t.TempDir(), "state")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("stateDir: "+stateDir+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path, stateDir
}

func getPath(t *testing.T, addr, path string) string {
	t.Helper()
	resp, err := http.Get("http://" + addr + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// Spec: CFG-009, WEB-001
func TestRun_StartsWebUIWithoutJobs(t *testing.T) {
	addr := freeAddr(t)
	t.Setenv(envWebUIAddr, addr)
	path, _ := emptyConfig(t)
	a := startRun(t, "--config", path)

	body := getUI(t, addr)
	if !strings.Contains(body, "Phorga has nothing to do yet") {
		t.Error("the configuration page must greet a setup without jobs")
	}
	if !strings.Contains(body, `name="webUI.enabled" checked`) {
		t.Error("in setup mode the web UI must stay enabled when the config is saved")
	}
	if code := a.stop(); code != exitOK {
		t.Errorf("exit code = %d, want 0", code)
	}
}

// Spec: CFG-008, CFG-009, WEB-004
func TestRun_SaveAppliesWithoutRestartAndPersists(t *testing.T) {
	addr := freeAddr(t)
	t.Setenv(envWebUIAddr, addr)
	path, stateDir := emptyConfig(t)
	target, source := filepath.Join(t.TempDir(), "organized"), t.TempDir()
	a := startRun(t, "--config", path)
	getUI(t, addr)

	form := url.Values{
		"stateDir":                       {stateDir},
		"webUI.enabled":                  {"on"},
		"webUI.address":                  {"127.0.0.1:8080"},
		"organizerCount":                 {"1"},
		"organizer.0.target":             {target},
		"organizer.0.targetStructure":    {"{year}/{month}"},
		"organizer.0.sourceCount":        {"1"},
		"organizer.0.sources.0.path":     {source},
		"organizer.0.sources.0.mode":     {"copy"},
		"organizer.0.sources.0.schedule": {"0 0 1 1 *"},
		"duplicateDetectionCount":        {"0"},
		"saveTarget":                     {"active"},
	}
	resp, err := http.PostForm("http://"+addr+"/ui/save", form)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Saved and applied") {
		t.Fatalf("expected the applied message, got %s", body)
	}

	// The job is scheduled at once, without a restart.
	label := source + " → " + target
	if status := getPath(t, addr, "/ui/status/table"); !strings.Contains(status, label) {
		t.Fatalf("status page does not list the new job %q", label)
	}
	if saved, err := config.Load(path); err != nil || len(saved.Organizer) != 1 || saved.Organizer[0].Target != target {
		t.Fatalf("config not persisted: %+v, %v", saved, err)
	}
	if code := a.stop(); code != exitOK {
		t.Fatalf("exit code = %d", code)
	}

	// After a restart the saved settings are still there.
	b := startRun(t, "--config", path, "--web-ui")
	if !strings.Contains(getUI(t, addr), target) {
		t.Error("the saved target is missing after a restart")
	}
	if status := getPath(t, addr, "/ui/status/table"); !strings.Contains(status, label) {
		t.Error("the saved job is missing after a restart")
	}
	b.stop()
}

// Spec: WEB-001
func TestRun_WebUIAddrFromEnv(t *testing.T) {
	addr := freeAddr(t)
	t.Setenv(envWebUIAddr, addr)
	cfg := testConfig(t)
	cfg.WebUI = &config.WebUIConfig{Enabled: true, Address: freeAddr(t)}
	a := startRun(t, "--config", writeConfig(t, cfg))

	getUI(t, addr)
	a.stop()
}

// Spec: WEB-001
func TestWebUIAddressPrecedence(t *testing.T) {
	t.Setenv(envWebUIAddr, "0.0.0.0:8080")
	if got := webUIAddress("127.0.0.1:9090"); got != "127.0.0.1:9090" {
		t.Errorf("flag must win, got %q", got)
	}
	if got := webUIAddress(""); got != "0.0.0.0:8080" {
		t.Errorf("env must be used without flag, got %q", got)
	}
	t.Setenv(envWebUIAddr, "")
	if got := webUIAddress(""); got != "" {
		t.Errorf("without flag and env the config decides, got %q", got)
	}
}

// Spec: WEB-020
func TestWebUIPasswordFromEnvironment(t *testing.T) {
	addr := freeAddr(t)
	t.Setenv(envWebUIAddr, addr)
	t.Setenv(envWebUIPassword, "s3cret")
	path, _ := emptyConfig(t)
	a := startRun(t, "--config", path)

	deadline := time.Now().Add(startupTimeout)
	for {
		resp, err := http.Get("http://" + addr + "/ui")
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("without credentials: status %d, want 401", resp.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("web UI did not come up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/ui", nil)
	req.SetBasicAuth("phorga", "s3cret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("with the password: status %d, want 200", resp.StatusCode)
	}
	a.stop()
}

// Spec: WEB-020
func TestOpenWithoutPassword(t *testing.T) {
	cases := []struct {
		address, password string
		want              bool
	}{
		{"0.0.0.0:8080", "", true},
		{":8080", "", true},
		{"192.168.1.20:8080", "", true},
		{"0.0.0.0:8080", "pw", false},
		{"127.0.0.1:8080", "", false},
		{"[::1]:8080", "", false},
		{"localhost:8080", "", false},
	}
	for _, tc := range cases {
		if got := openWithoutPassword(tc.address, tc.password); got != tc.want {
			t.Errorf("openWithoutPassword(%q, %q) = %v, want %v", tc.address, tc.password, got, tc.want)
		}
	}
}
