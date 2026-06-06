package webui

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/zedage/phorga/internal/config"
)

type pageData struct {
	Lang       string
	ConfigPath string
	Form       configForm
	Flash      flashMessage
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
	Organizer          []organizerProfileForm
	DuplicateDetection []duplicateDetectionForm
	SaveTarget         string
	SeparatePath       string
}

type webUIForm struct {
	Enabled bool
	Address string
}

type organizerProfileForm struct {
	DryRun          bool
	NtfyURL         string
	Target          string
	TargetStructure string
	FallbackFolder  string
	Sources         []organizerSourceForm
}

type organizerSourceForm struct {
	Path              string
	Mode              string
	Schedule          string
	DuplicatesEnabled bool
	DetectionMode     string
	ActionMethod      string
	DuplicateFolder   string
}

type duplicateDetectionForm struct {
	Enabled         bool
	DryRun          bool
	Schedule        string
	Sources         []string
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
		if cfg.WebUI.Address != "" {
			form.WebUI.Address = cfg.WebUI.Address
		}
	}

	form.Organizer = make([]organizerProfileForm, 0, len(cfg.Organizer))
	for _, profile := range cfg.Organizer {
		item := organizerProfileForm{
			DryRun:          profile.DryRun,
			Target:          profile.Target,
			TargetStructure: profile.TargetStructure,
			FallbackFolder:  profile.FallbackFolder,
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
			}
			if source.Duplicates != nil {
				src.DuplicatesEnabled = true
				src.DetectionMode = source.Duplicates.DetectionMode
				src.ActionMethod = source.Duplicates.Action.Method
				src.DuplicateFolder = source.Duplicates.Action.DuplicateFolder
			}
			item.Sources = append(item.Sources, src)
		}
		form.Organizer = append(form.Organizer, item)
	}

	form.DuplicateDetection = make([]duplicateDetectionForm, 0, len(cfg.DuplicateDetection))
	for _, dd := range cfg.DuplicateDetection {
		item := duplicateDetectionForm{
			Enabled:         dd.Enabled,
			DryRun:          dd.DryRun,
			Schedule:        dd.Schedule,
			Sources:         append([]string(nil), dd.Sources...),
			DetectionMode:   dd.DetectionMode,
			ActionMethod:    dd.Action.Method,
			DuplicateFolder: dd.Action.DuplicateFolder,
		}
		if dd.Ntfy != nil {
			item.NtfyURL = dd.Ntfy.URL
		}
		form.DuplicateDetection = append(form.DuplicateDetection, item)
	}

	return form
}

func parseForm(r *http.Request) (configForm, error) {
	if err := r.ParseForm(); err != nil {
		return configForm{}, fmt.Errorf("parsing form: %w", err)
	}

	form := configForm{
		StateDir:     strings.TrimSpace(r.FormValue("stateDir")),
		SaveTarget:   strings.TrimSpace(r.FormValue("saveTarget")),
		SeparatePath: strings.TrimSpace(r.FormValue("separatePath")),
		WebUI: webUIForm{
			Enabled: r.FormValue("webUI.enabled") != "",
			Address: strings.TrimSpace(r.FormValue("webUI.address")),
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
			DryRun:          r.FormValue(prefix+".dryRun") != "",
			NtfyURL:         strings.TrimSpace(r.FormValue(prefix + ".ntfyURL")),
			Target:          strings.TrimSpace(r.FormValue(prefix + ".target")),
			TargetStructure: strings.TrimSpace(r.FormValue(prefix + ".targetStructure")),
			FallbackFolder:  strings.TrimSpace(r.FormValue(prefix + ".fallbackFolder")),
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
				DetectionMode:     strings.TrimSpace(r.FormValue(srcPrefix + ".detectionMode")),
				ActionMethod:      strings.TrimSpace(r.FormValue(srcPrefix + ".actionMethod")),
				DuplicateFolder:   strings.TrimSpace(r.FormValue(srcPrefix + ".duplicateFolder")),
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
			Enabled:         r.FormValue(prefix+".enabled") != "",
			DryRun:          r.FormValue(prefix+".dryRun") != "",
			Schedule:        strings.TrimSpace(r.FormValue(prefix + ".schedule")),
			Sources:         sources,
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

	for _, profile := range f.Organizer {
		item := config.OrganizerProfile{
			DryRun:          profile.DryRun,
			Target:          strings.TrimSpace(profile.Target),
			TargetStructure: strings.TrimSpace(profile.TargetStructure),
			FallbackFolder:  strings.TrimSpace(profile.FallbackFolder),
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
			Enabled:       dd.Enabled,
			DryRun:        dd.DryRun,
			Schedule:      strings.TrimSpace(dd.Schedule),
			Sources:       make([]string, 0, len(dd.Sources)),
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
