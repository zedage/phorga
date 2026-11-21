package webui

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/zedage/phorga/internal/config"
)

type pageData struct {
	Lang          string
	ConfigPath    string
	Form          configForm
	Flash         flashMessage
	FieldErrors   map[string][]string
	BrowseEnabled bool
	Choices       choices
	// Synology lists the Synology Photos users that can be added (SYN-003).
	Synology synologyView
	// Focus is the id of the element to scroll to after an edit (WEB-015).
	Focus string
	// Setup is true while the running configuration has no jobs (CFG-009).
	Setup bool
}

// choices holds the allowed values for the select fields (WEB-007). They come
// from the config validation rules so UI and validation cannot drift apart.
type choices struct {
	SourceModes  []string
	OrgMethods   []string
	DedupMethods []string
	DedupModes   []string
	// DetectionFields are the organizer duplicate fields (WEB-017).
	DetectionFields []string
}

func newChoices() choices {
	return choices{
		SourceModes:     config.SourceModes(),
		OrgMethods:      config.OrganizerDuplicateMethods(),
		DedupMethods:    config.DedupMethods(),
		DedupModes:      config.DedupModes(),
		DetectionFields: config.DetectionFields(),
	}
}

type flashMessage struct {
	Level   string
	Title   string
	Message string
	Errors  []string
}

type configForm struct {
	StateDir           string
	WebUI              webUIForm
	SynologyHomes      string
	Organizer          []organizerProfileForm
	DuplicateDetection []duplicateDetectionForm
	SaveTarget         string
	SeparatePath       string
}

type webUIForm struct {
	Enabled     bool
	Address     string
	BrowseRoots []string
}

type organizerProfileForm struct {
	Name            string
	DryRun          bool
	NtfyURL         string
	Target          string
	TargetStructure string
	FallbackFolder  string
	IgnoreModTime   bool
	Sources         []organizerSourceForm
}

type organizerSourceForm struct {
	Path              string
	Mode              string
	Schedule          string
	DuplicatesEnabled bool
	DetectionMode     string
	// DetectionFields and DetectionAny are DetectionMode as checkboxes and
	// an all/any choice (WEB-017). DetectionInvalid keeps a value from the
	// file that the checkboxes cannot show.
	DetectionFields  []string
	DetectionAny     bool
	DetectionInvalid string
	ActionMethod     string
	DuplicateFolder  string
	Exclude          []string
}

type duplicateDetectionForm struct {
	Name            string
	Enabled         bool
	DryRun          bool
	Schedule        string
	Sources         []string
	Exclude         []string
	DetectionMode   string
	ActionMethod    string
	DuplicateFolder string
	NtfyURL         string
}

func newFormFromConfig(cfg *config.Config) configForm {
	form := configForm{
		StateDir:   cfg.StateDir,
		SaveTarget: "active",
		WebUI: webUIForm{
			Address: config.DefaultWebUIAddress,
		},
	}
	if cfg.WebUI != nil {
		form.WebUI.Enabled = cfg.WebUI.Enabled
		form.WebUI.BrowseRoots = append([]string(nil), cfg.WebUI.BrowseRoots...)
		if cfg.WebUI.Address != "" {
			form.WebUI.Address = cfg.WebUI.Address
		}
	}

	if cfg.SynologyPhotos != nil {
		form.SynologyHomes = cfg.SynologyPhotos.Homes
	}

	form.Organizer = make([]organizerProfileForm, 0, len(cfg.Organizer))
	for _, profile := range cfg.Organizer {
		form.Organizer = append(form.Organizer, newProfileForm(profile))
	}

	form.DuplicateDetection = make([]duplicateDetectionForm, 0, len(cfg.DuplicateDetection))
	for _, dd := range cfg.DuplicateDetection {
		form.DuplicateDetection = append(form.DuplicateDetection, newDedupForm(dd))
	}

	return form
}

// newProfileForm converts one organizer profile into its form model.
func newProfileForm(profile config.OrganizerProfile) organizerProfileForm {
	item := organizerProfileForm{
		Name:            profile.Name,
		DryRun:          profile.DryRun,
		Target:          profile.Target,
		TargetStructure: profile.TargetStructure,
		FallbackFolder:  profile.FallbackFolder,
		IgnoreModTime:   profile.IgnoreModTime,
		Sources:         make([]organizerSourceForm, 0, len(profile.Sources)),
	}
	if profile.Ntfy != nil {
		item.NtfyURL = profile.Ntfy.URL
	}
	for _, source := range profile.Sources {
		src := organizerSourceForm{
			Path:     source.Path,
			Mode:     source.Mode,
			Schedule: source.Schedule,
			Exclude:  append([]string(nil), source.Exclude...),
		}
		if source.Duplicates != nil {
			src.DuplicatesEnabled = true
			src.setDetectionMode(source.Duplicates.DetectionMode)
			src.ActionMethod = source.Duplicates.Action.Method
			src.DuplicateFolder = source.Duplicates.Action.DuplicateFolder
		}
		item.Sources = append(item.Sources, src)
	}
	return item
}

// newDedupForm converts one duplicate detection job into its form model.
func newDedupForm(dd config.DuplicateDetection) duplicateDetectionForm {
	item := duplicateDetectionForm{
		Name:            dd.Name,
		Enabled:         dd.Enabled,
		DryRun:          dd.DryRun,
		Schedule:        dd.Schedule,
		Sources:         append([]string(nil), dd.Sources...),
		Exclude:         append([]string(nil), dd.Exclude...),
		DetectionMode:   dd.DetectionMode,
		ActionMethod:    dd.Action.Method,
		DuplicateFolder: dd.Action.DuplicateFolder,
	}
	if dd.Ntfy != nil {
		item.NtfyURL = dd.Ntfy.URL
	}
	return item
}

func parseForm(r *http.Request) (configForm, error) {
	if err := r.ParseForm(); err != nil {
		return configForm{}, fmt.Errorf("parsing form: %w", err)
	}

	form := configForm{
		StateDir:      strings.TrimSpace(r.FormValue("stateDir")),
		SaveTarget:    strings.TrimSpace(r.FormValue("saveTarget")),
		SeparatePath:  strings.TrimSpace(r.FormValue("separatePath")),
		SynologyHomes: strings.TrimSpace(r.FormValue("synologyPhotos.homes")),
		WebUI: webUIForm{
			Enabled:     r.FormValue("webUI.enabled") != "",
			Address:     strings.TrimSpace(r.FormValue("webUI.address")),
			BrowseRoots: splitLines(r.FormValue("webUI.browseRoots")),
		},
	}
	if form.SaveTarget == "" {
		form.SaveTarget = "active"
	}

	organizerCount := parseCount(r.FormValue("organizerCount"))
	form.Organizer = make([]organizerProfileForm, 0, organizerCount)
	for i := 0; i < organizerCount; i++ {
		prefix := fmt.Sprintf("organizer.%d", i)
		profile := organizerProfileForm{
			Name:            strings.TrimSpace(r.FormValue(prefix + ".name")),
			DryRun:          r.FormValue(prefix+".dryRun") != "",
			NtfyURL:         strings.TrimSpace(r.FormValue(prefix + ".ntfyURL")),
			Target:          strings.TrimSpace(r.FormValue(prefix + ".target")),
			TargetStructure: strings.TrimSpace(r.FormValue(prefix + ".targetStructure")),
			FallbackFolder:  strings.TrimSpace(r.FormValue(prefix + ".fallbackFolder")),
			IgnoreModTime:   r.FormValue(prefix+".ignoreModTime") != "",
		}
		sourceCount := parseCount(r.FormValue(prefix + ".sourceCount"))
		profile.Sources = make([]organizerSourceForm, 0, sourceCount)
		for j := 0; j < sourceCount; j++ {
			srcPrefix := fmt.Sprintf("%s.sources.%d", prefix, j)
			source := organizerSourceForm{
				Path:              strings.TrimSpace(r.FormValue(srcPrefix + ".path")),
				Mode:              strings.TrimSpace(r.FormValue(srcPrefix + ".mode")),
				Schedule:          strings.TrimSpace(r.FormValue(srcPrefix + ".schedule")),
				DuplicatesEnabled: r.FormValue(srcPrefix+".duplicatesEnabled") != "",

				ActionMethod:    strings.TrimSpace(r.FormValue(srcPrefix + ".actionMethod")),
				DuplicateFolder: strings.TrimSpace(r.FormValue(srcPrefix + ".duplicateFolder")),
				Exclude:         splitLines(r.FormValue(srcPrefix + ".exclude")),
			}
			if r.FormValue(srcPrefix+".detectionUI") != "" {
				// Checkboxes (WEB-017); no box ticked gives an empty mode.
				source.setDetectionFields(r.Form[srcPrefix+".detectionFields"], r.FormValue(srcPrefix+".detectionJoin") == "any")
			} else {
				source.setDetectionMode(strings.TrimSpace(r.FormValue(srcPrefix + ".detectionMode")))
			}
			profile.Sources = append(profile.Sources, source)
		}
		form.Organizer = append(form.Organizer, profile)
	}

	ddCount := parseCount(r.FormValue("duplicateDetectionCount"))
	form.DuplicateDetection = make([]duplicateDetectionForm, 0, ddCount)
	for i := 0; i < ddCount; i++ {
		prefix := fmt.Sprintf("duplicateDetection.%d", i)
		sourceCount := parseCount(r.FormValue(prefix + ".sourceCount"))
		sources := make([]string, 0, sourceCount)
		for j := 0; j < sourceCount; j++ {
			sources = append(sources, strings.TrimSpace(r.FormValue(fmt.Sprintf("%s.sources.%d", prefix, j))))
		}
		item := duplicateDetectionForm{
			Name:            strings.TrimSpace(r.FormValue(prefix + ".name")),
			Enabled:         r.FormValue(prefix+".enabled") != "",
			DryRun:          r.FormValue(prefix+".dryRun") != "",
			Schedule:        strings.TrimSpace(r.FormValue(prefix + ".schedule")),
			Sources:         sources,
			Exclude:         splitLines(r.FormValue(prefix + ".exclude")),
			DetectionMode:   strings.TrimSpace(r.FormValue(prefix + ".detectionMode")),
			ActionMethod:    strings.TrimSpace(r.FormValue(prefix + ".actionMethod")),
			DuplicateFolder: strings.TrimSpace(r.FormValue(prefix + ".duplicateFolder")),
			NtfyURL:         strings.TrimSpace(r.FormValue(prefix + ".ntfyURL")),
		}
		form.DuplicateDetection = append(form.DuplicateDetection, item)
	}

	return form, nil
}

func (f configForm) toConfig() *config.Config {
	cfg := &config.Config{
		StateDir:           strings.TrimSpace(f.StateDir),
		Organizer:          make([]config.OrganizerProfile, 0, len(f.Organizer)),
		DuplicateDetection: make([]config.DuplicateDetection, 0, len(f.DuplicateDetection)),
	}
	cfg.WebUI = &config.WebUIConfig{Enabled: f.WebUI.Enabled, Address: strings.TrimSpace(f.WebUI.Address)}
	for _, root := range f.WebUI.BrowseRoots {
		if root = strings.TrimSpace(root); root != "" {
			cfg.WebUI.BrowseRoots = append(cfg.WebUI.BrowseRoots, root)
		}
	}
	if homes := strings.TrimSpace(f.SynologyHomes); homes != "" {
		cfg.SynologyPhotos = &config.SynologyPhotos{Homes: homes}
	}

	for _, profile := range f.Organizer {
		item := config.OrganizerProfile{
			Name:            strings.TrimSpace(profile.Name),
			DryRun:          profile.DryRun,
			Target:          strings.TrimSpace(profile.Target),
			TargetStructure: strings.TrimSpace(profile.TargetStructure),
			FallbackFolder:  strings.TrimSpace(profile.FallbackFolder),
			IgnoreModTime:   profile.IgnoreModTime,
			Sources:         make([]config.OrganizerSource, 0, len(profile.Sources)),
		}
		if profile.NtfyURL != "" {
			item.Ntfy = &config.NtfyConfig{URL: strings.TrimSpace(profile.NtfyURL)}
		}
		for _, source := range profile.Sources {
			src := config.OrganizerSource{
				Path:     strings.TrimSpace(source.Path),
				Mode:     strings.TrimSpace(source.Mode),
				Schedule: strings.TrimSpace(source.Schedule),
				Exclude:  nonEmptyLines(source.Exclude),
			}
			if source.DuplicatesEnabled || source.DetectionMode != "" || source.ActionMethod != "" || source.DuplicateFolder != "" {
				src.Duplicates = &config.OrganizerDuplicate{
					DetectionMode: strings.TrimSpace(source.DetectionMode),
					Action: config.DuplicateAction{
						Method:          strings.TrimSpace(source.ActionMethod),
						DuplicateFolder: strings.TrimSpace(source.DuplicateFolder),
					},
				}
			}
			item.Sources = append(item.Sources, src)
		}
		cfg.Organizer = append(cfg.Organizer, item)
	}

	for _, dd := range f.DuplicateDetection {
		item := config.DuplicateDetection{
			Name:          strings.TrimSpace(dd.Name),
			Enabled:       dd.Enabled,
			DryRun:        dd.DryRun,
			Schedule:      strings.TrimSpace(dd.Schedule),
			Sources:       make([]string, 0, len(dd.Sources)),
			Exclude:       nonEmptyLines(dd.Exclude),
			DetectionMode: strings.TrimSpace(dd.DetectionMode),
			Action: config.DuplicateAction{
				Method:          strings.TrimSpace(dd.ActionMethod),
				DuplicateFolder: strings.TrimSpace(dd.DuplicateFolder),
			},
		}
		for _, source := range dd.Sources {
			item.Sources = append(item.Sources, strings.TrimSpace(source))
		}
		if dd.NtfyURL != "" {
			item.Ntfy = &config.NtfyConfig{URL: strings.TrimSpace(dd.NtfyURL)}
		}
		cfg.DuplicateDetection = append(cfg.DuplicateDetection, item)
	}

	return cfg
}

func parseCount(value string) int {
	count, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || count < 0 {
		return 0
	}
	return count
}

// splitLines splits a textarea value into trimmed, non-empty lines.
func splitLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// nonEmptyLines trims each entry and drops empty ones; no entries gives nil.
func nonEmptyLines(list []string) []string {
	var out []string
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// listField matches list values edited in one textarea; problems with any
// entry belong to the whole field.
var listField = regexp.MustCompile(`^(.*\.(?:exclude|browseRoots))\.\d+$`)

var problemPath = regexp.MustCompile(`^[A-Za-z]+(?:\[\d+\])?(?:\.[A-Za-z]+(?:\[\d+\])?)*`)
var problemIndex = regexp.MustCompile(`\[(\d+)\]`)

// fieldForProblem maps a validation problem such as
// "organizer[0].sources[1].duplicates.action.method must be ..." to the form
// field name "organizer.0.sources.1.actionMethod" (WEB-008). It returns ""
// when the problem does not belong to a single field.
func fieldForProblem(problem string) string {
	path := problemPath.FindString(problem)
	if path == "" {
		return ""
	}
	name := problemIndex.ReplaceAllString(path, ".$1")
	replacements := []struct{ from, to string }{
		{".duplicates.detectionMode", ".detectionMode"},
		{".duplicates.action.method", ".actionMethod"},
		{".duplicates.action.duplicateFolder", ".duplicateFolder"},
		{".action.method", ".actionMethod"},
		{".action.duplicateFolder", ".duplicateFolder"},
	}
	for _, r := range replacements {
		if strings.HasSuffix(name, r.from) {
			name = strings.TrimSuffix(name, r.from) + r.to
			break
		}
	}
	if strings.HasPrefix(name, "webUI.browseRoots") {
		return "webUI.browseRoots"
	}
	if m := listField.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return name
}

// fieldErrors groups problems by the form field they belong to.
func fieldErrors(problems []string) map[string][]string {
	out := map[string][]string{}
	for _, p := range problems {
		if field := fieldForProblem(p); field != "" {
			out[field] = append(out[field], p)
		}
	}
	return out
}

// setDetectionMode fills the checkbox view of an organizer detectionMode
// such as "filename+exif" (WEB-017). A value the checkboxes cannot show is
// kept in DetectionInvalid, with the known fields ticked, so validation can
// report it and the user can fix it by ticking boxes.
func (s *organizerSourceForm) setDetectionMode(mode string) {
	s.DetectionMode, s.DetectionFields, s.DetectionAny, s.DetectionInvalid = mode, nil, false, ""
	if mode == "" {
		return
	}
	expr, err := config.ParseDetectionMode(mode)
	if err != nil {
		s.DetectionInvalid = mode
		for _, f := range config.DetectionFields() {
			if strings.Contains(mode, f) {
				s.DetectionFields = append(s.DetectionFields, f)
			}
		}
		return
	}
	s.DetectionFields = orderedFields(expr.Fields)
	s.DetectionAny = expr.Any && len(expr.Fields) > 1
}

// setDetectionFields builds detectionMode from ticked fields and the
// all/any choice, e.g. filename and exif with "all" give "filename+exif".
func (s *organizerSourceForm) setDetectionFields(fields []string, any bool) {
	s.DetectionFields = orderedFields(fields)
	s.DetectionAny = any && len(s.DetectionFields) > 1
	s.DetectionInvalid = ""
	sep := "+"
	if s.DetectionAny {
		sep = "|"
	}
	s.DetectionMode = strings.Join(s.DetectionFields, sep)
}

// orderedFields keeps the known detection fields of list, in their
// canonical order, without duplicates.
func orderedFields(list []string) []string {
	var out []string
	for _, f := range config.DetectionFields() {
		for _, v := range list {
			if strings.TrimSpace(v) == f {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// HasErrors reports whether any of the named form fields, or a field below
// one of them (a name ending in "."), has a validation problem.
func (d pageData) HasErrors(names ...string) bool {
	for field := range d.FieldErrors {
		for _, n := range names {
			if field == n || (strings.HasSuffix(n, ".") && strings.HasPrefix(field, n)) {
				return true
			}
		}
	}
	return false
}

// Open reports whether a collapsible card starts open (WEB-016): when it
// was just added or changed (Focus), or when one of its fields has a
// problem. prefix is the form name prefix of its fields, e.g. "organizer.0.".
func (d pageData) Open(id, prefix string) bool {
	if d.Focus == id || strings.HasPrefix(d.Focus, id+"-") {
		return true
	}
	return d.HasErrors(prefix)
}
