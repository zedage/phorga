package webui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/zedage/phorga/internal/config"
)

// Spec: ORG-002
func TestIgnoreModTimeIsEditable(t *testing.T) {
	cfg := sampleConfig()
	cfg.Organizer[0].IgnoreModTime = true
	activePath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(activePath, cfg); err != nil {
		t.Fatal(err)
	}
	s := newConfiguredServer(t, activePath, cfg)

	page := get(t, s, "/ui?lang=en").Body.String()
	if !strings.Contains(page, `name="organizer.0.ignoreModTime"`) || !strings.Contains(page, "Ignore modification time") {
		t.Fatal("page has no ignoreModTime checkbox")
	}

	// An unchecked box is not sent, which turns the option off.
	values := validFormValues(cfg, "active", "")
	values.Del("organizer.0.ignoreModTime")
	postForm(t, s, "/ui/save", values, true)
	loaded, err := config.Load(activePath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Organizer[0].IgnoreModTime {
		t.Error("ignoreModTime must be off after saving an unchecked box")
	}

	values.Set("organizer.0.ignoreModTime", "on")
	postForm(t, s, "/ui/save", values, true)
	if loaded, err = config.Load(activePath); err != nil {
		t.Fatal(err)
	}
	if !loaded.Organizer[0].IgnoreModTime {
		t.Error("ignoreModTime must be on after saving a checked box")
	}
}
