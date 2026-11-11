package webui

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxListedDirs    = 500
	maxSuggestedDirs = 20
)

// The folder picker (WEB-010, ADR-0013) lists directories below the
// configured webUI.browseRoots only. It never writes to the filesystem.

type dirEntry struct {
	Name string
	Path string
}

type pickerData struct {
	Path   string // current folder; "" means the list of roots
	Parent string // folder for the "up" button; "" means the list of roots
	Dirs   []dirEntry
}

// withinRoots reports whether p (absolute, cleaned) lies inside one of the
// roots after resolving symlinks on both sides.
func withinRoots(roots []string, p string) bool {
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return false
	}
	for _, root := range roots {
		realRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(realRoot, real)
		if err != nil {
			continue
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// nearestAllowedDir walks up from p to the first existing directory inside
// the roots, e.g. for a target folder that does not exist yet. It returns ""
// if there is none.
func nearestAllowedDir(roots []string, p string) string {
	for {
		if isDir(p) && withinRoots(roots, p) {
			return p
		}
		parent := filepath.Dir(p)
		if parent == p {
			return ""
		}
		p = parent
	}
}

// isRoot reports whether p is one of the roots itself.
func isRoot(roots []string, p string) bool {
	for _, root := range roots {
		if a, b := filepath.Clean(root), filepath.Clean(p); a == b || (filepath.Separator == '\\' && strings.EqualFold(a, b)) {
			return true
		}
	}
	return false
}

// subdirs lists the visible subdirectories of dir that stay within roots,
// optionally filtered by a case-insensitive name prefix.
func subdirs(roots []string, dir, prefix string, limit int) []dirEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	prefix = strings.ToLower(prefix)
	var out []dirEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || !strings.HasPrefix(strings.ToLower(name), prefix) {
			continue
		}
		child := filepath.Join(dir, name)
		info, err := os.Stat(child) // follows symlinks
		if err != nil || !info.IsDir() || !withinRoots(roots, child) {
			continue
		}
		out = append(out, dirEntry{Name: name, Path: child})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Server) rootEntries() []dirEntry {
	out := make([]dirEntry, 0, len(s.browseRoots))
	for _, root := range s.browseRoots {
		out = append(out, dirEntry{Name: root, Path: filepath.Clean(root)})
	}
	return out
}

// checkPicker answers 404 when the picker is disabled and 405 for non-GET.
func (s *Server) checkPicker(w http.ResponseWriter, r *http.Request) bool {
	if len(s.browseRoots) == 0 {
		http.NotFound(w, r)
		return false
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// handleFSList renders the picker dialog body for one folder.
func (s *Server) handleFSList(w http.ResponseWriter, r *http.Request) {
	if !s.checkPicker(w, r) {
		return
	}
	lang := s.resolveLanguage(w, r)
	data := pickerData{}

	// "path" is a folder chosen inside the picker and must be allowed (403
	// otherwise). "start" is the current value of the input field: it opens the
	// nearest existing allowed folder, or the root list.
	query := r.URL.Query()
	p := ""
	if raw := strings.TrimSpace(query.Get("path")); raw != "" {
		p = filepath.Clean(raw)
		if !filepath.IsAbs(p) || !isDir(p) || !withinRoots(s.browseRoots, p) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	} else if raw := strings.TrimSpace(query.Get("start")); raw != "" && filepath.IsAbs(raw) {
		p = nearestAllowedDir(s.browseRoots, filepath.Clean(raw))
	}

	if p == "" {
		data.Dirs = s.rootEntries()
	} else {
		data.Path = p
		if !isRoot(s.browseRoots, p) {
			if parent := filepath.Dir(p); withinRoots(s.browseRoots, parent) {
				data.Parent = parent
			}
		}
		data.Dirs = subdirs(s.browseRoots, p, "", maxListedDirs)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.execute(w, "picker", lang, data)
}

// handleFSSuggest renders <option> elements completing the typed path.
func (s *Server) handleFSSuggest(w http.ResponseWriter, r *http.Request) {
	if !s.checkPicker(w, r) {
		return
	}
	q := strings.TrimSpace(r.FormValue(r.URL.Query().Get("field")))

	var dirs []dirEntry
	switch {
	case q == "" || !filepath.IsAbs(q):
		dirs = s.rootEntries()
	default:
		dir, prefix := filepath.Dir(q), filepath.Base(q)
		if strings.HasSuffix(q, "/") || strings.HasSuffix(q, string(filepath.Separator)) {
			dir, prefix = filepath.Clean(q), ""
		}
		if withinRoots(s.browseRoots, dir) {
			dirs = subdirs(s.browseRoots, dir, prefix, maxSuggestedDirs)
		} else {
			for _, root := range s.rootEntries() {
				if strings.HasPrefix(strings.ToLower(root.Path), strings.ToLower(q)) {
					dirs = append(dirs, root)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.execute(w, "suggest", "", dirs)
}
