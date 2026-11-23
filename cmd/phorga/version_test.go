package main

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
)

// Spec: RUN-004
func TestRun_VersionFlagPrintsVersion(t *testing.T) {
	orig := version
	version = "2026.11.0"
	t.Cleanup(func() { version = orig })

	var out bytes.Buffer
	if code := run(context.Background(), []string{"--version"}, &out); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if got := out.String(); got != "phorga 2026.11.0\n" {
		t.Errorf("output = %q", got)
	}
}

// Spec: RUN-004
func TestRun_StartLogNamesVersion(t *testing.T) {
	orig := version
	version = "2026.11.0"
	t.Cleanup(func() { version = orig })

	cfg := testConfig(t)
	cfg.Organizer[0].Target = "" // invalid, so run returns right after the start log
	var logs bytes.Buffer
	run(context.Background(), []string{"--config", writeConfig(t, cfg)}, &logs)
	if !strings.Contains(logs.String(), `"msg":"phorga starting","version":"2026.11.0"`) {
		t.Errorf("the start log must name the version:\n%s", logs.String())
	}
}

// Spec: RUN-004
func TestCurrentVersionWithoutLdflags(t *testing.T) {
	orig := version
	version = "dev"
	t.Cleanup(func() { version = orig })

	if got := currentVersion(); !regexp.MustCompile(`^dev(\+[0-9a-f]{7}(\.dirty)?)?$`).MatchString(got) {
		t.Errorf("currentVersion() = %q, want dev or dev+<commit>", got)
	}
}
