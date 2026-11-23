package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/runstatus"
)

// skipWithoutPermissions skips tests that need file permissions to deny
// access: on Windows they do not, and root ignores them.
func skipWithoutPermissions(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("file permissions do not deny access on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
}

// chmod changes the mode of path and restores it when the test ends, so
// the temp dir can be removed.
func chmod(t *testing.T, path string, mode, restore os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, restore) })
}

// Spec: RUN-003
func TestCheckStateAccess_AccessibleDirLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "organizer"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "organizer", "a.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := checkStateAccess(dir); err != nil {
		t.Fatalf("checkStateAccess: %v", err)
	}
	var names []string
	filepath.WalkDir(dir, func(p string, _ os.DirEntry, _ error) error {
		names = append(names, filepath.Base(p))
		return nil
	})
	for _, n := range names {
		if strings.HasPrefix(n, ".phorga-access-") {
			t.Errorf("probe file left behind: %s", n)
		}
	}
}

// Spec: RUN-003
func TestRun_StateDirNotWritableExitsNonZero(t *testing.T) {
	skipWithoutPermissions(t)
	cfg := testConfig(t)
	if err := os.MkdirAll(cfg.StateDir, 0755); err != nil {
		t.Fatal(err)
	}
	chmod(t, cfg.StateDir, 0555, 0755)

	var logs bytes.Buffer
	code := run(context.Background(), []string{"--config", writeConfig(t, cfg)}, &logs)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	out := logs.String()
	if !strings.Contains(out, "state directory not accessible") || !strings.Contains(out, cfg.StateDir) {
		t.Errorf("the log must name the problem and the folder:\n%s", out)
	}
	if !strings.Contains(out, `"uid":`) {
		t.Errorf("the log must name the user Phorga runs as:\n%s", out)
	}
}

// Spec: RUN-003
func TestRun_UnreadableStateFileExitsNonZero(t *testing.T) {
	skipWithoutPermissions(t)
	cfg := testConfig(t)
	stateFile := filepath.Join(cfg.StateDir, "organizer", "old.json")
	if err := os.MkdirAll(filepath.Dir(stateFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	chmod(t, stateFile, 0, 0644)

	var logs bytes.Buffer
	if code := run(context.Background(), []string{"--config", writeConfig(t, cfg)}, &logs); code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(logs.String(), stateFile) {
		t.Errorf("the log must name the unreadable file:\n%s", logs.String())
	}
}

// Spec: ORG-015, RUN-003
func TestOrganizerJob_UnreadableStateTransfersNothing(t *testing.T) {
	skipWithoutPermissions(t)
	dir := t.TempDir()
	src, target := filepath.Join(dir, "in"), filepath.Join(dir, "out")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "IMG_1.jpg"), []byte("photo"), 0644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(statePath, []byte(`{"processedFiles":{}}`), 0644); err != nil {
		t.Fatal(err)
	}
	chmod(t, statePath, 0, 0644)
	recorder := runstatus.Load(filepath.Join(dir, "run-status.json"))

	profile := config.OrganizerProfile{Target: target, TargetStructure: "{year}/{month}"}
	source := config.OrganizerSource{Path: src, Mode: "copy", Schedule: "0 2 * * *"}
	organizerJob("org", profile, source, statePath, recorder)(context.Background())

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("nothing must be copied without the state, stat error: %v", err)
	}
	rec, ok := recorder.Last("org")
	if !ok || rec.Counters["errors"] != 1 || rec.Counters["processed"] != 0 {
		t.Errorf("the run must be recorded with one error, got %+v (recorded %v)", rec, ok)
	}
}

// Spec: RUN-003
func TestCheckConfigWritable(t *testing.T) {
	dir := t.TempDir()
	if err := checkConfigWritable(filepath.Join(dir, "config.yaml")); err != nil {
		t.Errorf("a missing file in a writable folder can be created: %v", err)
	}

	skipWithoutPermissions(t)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("stateDir: /tmp\n"), 0644); err != nil {
		t.Fatal(err)
	}
	chmod(t, path, 0444, 0644)
	if err := checkConfigWritable(path); err == nil {
		t.Error("a read-only config file must be reported")
	}
}
