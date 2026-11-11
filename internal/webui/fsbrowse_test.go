package webui

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

)

// pickerServer creates a server whose browseRoots is a temp dir containing
// albums/, archive/, .hidden/ and notes.txt, and returns it with the root.
func pickerServer(t *testing.T) (*Server, string) {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"albums/2024", "archive", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := sampleConfig()
	cfg.WebUI.BrowseRoots = []string{root}
	return newConfiguredServer(t, sampleConfigPath(), cfg), root
}

func listURL(param, path string) string {
	return "/ui/fs/list?" + url.Values{param: {path}}.Encode()
}

// Spec: WEB-010
func TestPickerListsOnlyVisibleSubfolders(t *testing.T) {
	s, root := pickerServer(t)

	resp := get(t, s, listURL("path", root))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	for _, want := range []string{"📁 albums", "📁 archive", "Select this folder"} {
		if !strings.Contains(body, want) {
			t.Errorf("listing missing %q:\n%s", want, body)
		}
	}
	for _, unwanted := range []string{".hidden", "notes.txt"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("listing must not contain %q", unwanted)
		}
	}
	// At a root, "up" leads back to the list of roots.
	if !strings.Contains(body, `hx-get="/ui/fs/list?path="`) {
		t.Errorf("root listing must offer the way back to the root list:\n%s", body)
	}

	sub := get(t, s, listURL("path", filepath.Join(root, "albums"))).Body.String()
	if !strings.Contains(sub, "📁 2024") || !strings.Contains(sub, "↑ Up") {
		t.Errorf("sub-folder listing missing entries or up button:\n%s", sub)
	}

	top := get(t, s, "/ui/fs/list").Body.String()
	if !strings.Contains(top, "Allowed folders") || !strings.Contains(top, filepath.Base(root)) {
		t.Errorf("empty path must list the roots:\n%s", top)
	}
}

// Spec: WEB-010
func TestPickerRejectsPathsOutsideRoots(t *testing.T) {
	s, root := pickerServer(t)
	outside := t.TempDir()

	for name, p := range map[string]string{
		"outside":  outside,
		"dot-dot":  filepath.Join(root, "albums") + string(filepath.Separator) + ".." + string(filepath.Separator) + "..",
		"relative": "albums",
		"file":     filepath.Join(root, "notes.txt"),
	} {
		if code := get(t, s, listURL("path", p)).Code; code != http.StatusForbidden {
			t.Errorf("%s: expected 403 for %s, got %d", name, p, code)
		}
	}

	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Logf("symlinks not available, skipping symlink case: %v", err)
	} else {
		if code := get(t, s, listURL("path", link)).Code; code != http.StatusForbidden {
			t.Errorf("symlink escaping the root: expected 403, got %d", code)
		}
		if body := get(t, s, listURL("path", root)).Body.String(); strings.Contains(body, "escape") {
			t.Error("a symlink pointing outside the roots must not be listed")
		}
	}
}

// Spec: WEB-010
func TestPickerStartsAtNearestAllowedFolder(t *testing.T) {
	s, root := pickerServer(t)

	notYet := filepath.Join(root, "albums", "2025", "new")
	body := get(t, s, listURL("start", notYet)).Body.String()
	if !strings.Contains(body, filepath.Join(root, "albums")) || !strings.Contains(body, "📁 2024") {
		t.Errorf("start at a missing folder must open its nearest existing parent:\n%s", body)
	}

	outside := get(t, s, listURL("start", t.TempDir()))
	if outside.Code != http.StatusOK || !strings.Contains(outside.Body.String(), "Allowed folders") {
		t.Errorf("start outside the roots must open the root list, got %d", outside.Code)
	}
}

// Spec: WEB-010
func TestPickerSuggestCompletesPrefix(t *testing.T) {
	s, root := pickerServer(t)

	q := filepath.Join(root, "al")
	body := get(t, s, "/ui/fs/suggest?"+url.Values{"field": {"x"}, "x": {q}}.Encode()).Body.String()
	if !strings.Contains(body, `<option value="`+filepath.Join(root, "albums")+`">`) {
		t.Errorf("expected albums suggestion, got %s", body)
	}
	if strings.Contains(body, "archive") {
		t.Errorf("archive does not match the prefix: %s", body)
	}

	none := get(t, s, "/ui/fs/suggest?"+url.Values{"field": {"x"}, "x": {t.TempDir() + string(filepath.Separator)}}.Encode()).Body.String()
	if strings.Contains(none, "<option") {
		t.Errorf("no suggestions expected outside the roots, got %s", none)
	}
}

// Spec: WEB-010
func TestPickerDisabledWithoutRoots(t *testing.T) {
	s := newTestServer(t) // sample config has no browseRoots
	for _, path := range []string{"/ui/fs/list", "/ui/fs/suggest"} {
		if code := get(t, s, path).Code; code != http.StatusNotFound {
			t.Errorf("%s: expected 404 without browseRoots, got %d", path, code)
		}
	}
	body := get(t, s, "/ui").Body.String()
	if strings.Contains(body, "Browse…") || strings.Contains(body, `id="picker"`) {
		t.Error("picker controls must be hidden without browseRoots")
	}

	enabled, _ := pickerServer(t)
	page := get(t, enabled, "/ui").Body.String()
	if !strings.Contains(page, "Browse…") || !strings.Contains(page, `list="f-organizer-0-target-dl"`) {
		t.Error("picker controls must be shown when browseRoots is set")
	}
}

// Spec: WEB-010
func TestPickerNeverWrites(t *testing.T) {
	s, root := pickerServer(t)
	before := snapshot(t, root)
	get(t, s, listURL("path", root))
	get(t, s, listURL("start", filepath.Join(root, "new", "folder")))
	get(t, s, "/ui/fs/suggest?"+url.Values{"field": {"x"}, "x": {filepath.Join(root, "new")}}.Encode())
	if after := snapshot(t, root); after != before {
		t.Fatalf("picker changed the filesystem:\nbefore %s\nafter  %s", before, after)
	}
}

func snapshot(t *testing.T, root string) string {
	t.Helper()
	var parts []string
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil {
			parts = append(parts, p)
		}
		return nil
	})
	return strings.Join(parts, "|")
}
