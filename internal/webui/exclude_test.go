package webui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zedage/phorga/internal/config"
)

// Spec: CFG-007
func TestExcludeIsEditableOnePatternPerLine(t *testing.T) {
	cfg := sampleConfig()
	cfg.Organizer[0].Sources[0].Exclude = []string{"Screenshots", "2019/Scans"}
	cfg.DuplicateDetection[0].Exclude = []string{"*_edited.jpg"}
	activePath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(activePath, cfg); err != nil {
		t.Fatal(err)
	}
	s := newConfiguredServer(t, activePath, cfg)

	page := get(t, s, "/ui?lang=en").Body.String()
	for _, want := range []string{
		`name="organizer.0.sources.0.exclude"`,
		"Screenshots\n2019/Scans",
		`name="duplicateDetection.0.exclude"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %q", want)
		}
	}

	values := validFormValues(cfg, "active", "")
	values.Set("organizer.0.sources.0.exclude", " Screenshots \n\n*/WhatsApp\n")
	values.Set("duplicateDetection.0.exclude", "")
	postForm(t, s, "/ui/save", values, true)

	loaded, err := config.Load(activePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Organizer[0].Sources[0].Exclude; !reflect.DeepEqual(got, []string{"Screenshots", "*/WhatsApp"}) {
		t.Errorf("organizer exclude = %#v", got)
	}
	if got := loaded.DuplicateDetection[0].Exclude; got != nil {
		t.Errorf("duplicateDetection exclude = %#v, want nil", got)
	}
}

// Spec: CFG-007, WEB-008
func TestInvalidExcludeShowsFieldError(t *testing.T) {
	s := newTestServer(t)
	values := validFormValues(sampleConfig(), "active", "")
	values.Set("organizer.0.sources.0.exclude", "ok\n[abc")

	body := postForm(t, s, "/ui/validate", values, true).Body.String()
	want := `id="err-f-organizer-0-sources-0-exclude" hx-swap-oob="true">organizer[0].sources[0].exclude[1] is not a valid pattern`
	if !strings.Contains(body, want) {
		t.Errorf("response missing %q:\n%s", want, body)
	}
}
