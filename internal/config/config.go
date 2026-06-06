package config

// Config is the root configuration structure for Phorga.
type Config struct {
	StateDir           string               `yaml:"stateDir,omitempty" json:"stateDir,omitempty"`
	WebUI              *WebUIConfig         `yaml:"webUI,omitempty" json:"webUI,omitempty"`
	Organizer          []OrganizerProfile   `yaml:"organizer" json:"organizer"`
	DuplicateDetection []DuplicateDetection `yaml:"duplicateDetection" json:"duplicateDetection"`
}

// WebUIConfig configures the optional built-in configuration web UI.
type WebUIConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Address string `yaml:"address,omitempty" json:"address,omitempty"`
}

// OrganizerProfile defines a single organizer target with its sources.
type OrganizerProfile struct {
	DryRun          bool              `yaml:"dryRun" json:"dryRun"`
	Ntfy            *NtfyConfig       `yaml:"ntfy,omitempty" json:"ntfy,omitempty"`
	Target          string            `yaml:"target" json:"target"`
	TargetStructure string            `yaml:"targetStructure" json:"targetStructure"`
	FallbackFolder  string            `yaml:"fallbackFolder,omitempty" json:"fallbackFolder,omitempty"`
	Sources         []OrganizerSource `yaml:"sources" json:"sources"`
}

// OrganizerSource defines a single source directory for the organizer.
type OrganizerSource struct {
	Path       string              `yaml:"path" json:"path"`
	Mode       string              `yaml:"mode" json:"mode"` // "move" or "copy"
	Duplicates *OrganizerDuplicate `yaml:"duplicates,omitempty" json:"duplicates,omitempty"`
	Schedule   string              `yaml:"schedule" json:"schedule"`
}

// OrganizerDuplicate configures lightweight duplicate handling during organization.
type OrganizerDuplicate struct {
	DetectionMode string          `yaml:"detectionMode" json:"detectionMode"` // filename, filedate, exif (with + or |)
	Action        DuplicateAction `yaml:"action" json:"action"`
}

// DuplicateAction defines what to do when a duplicate is detected.
type DuplicateAction struct {
	Method          string `yaml:"method" json:"method"`                                       // replace, addSuffix, moveToFolder, skip
	DuplicateFolder string `yaml:"duplicateFolder,omitempty" json:"duplicateFolder,omitempty"` // required if method=moveToFolder
}

// DuplicateDetection defines an independent duplicate detection job.
type DuplicateDetection struct {
	Enabled       bool            `yaml:"enabled" json:"enabled"`
	DryRun        bool            `yaml:"dryRun" json:"dryRun"`
	Schedule      string          `yaml:"schedule" json:"schedule"`
	Sources       []string        `yaml:"sources" json:"sources"`
	DetectionMode string          `yaml:"detectionMode" json:"detectionMode"` // "content" or "metadata"
	Action        DuplicateAction `yaml:"action" json:"action"`
	Ntfy          *NtfyConfig     `yaml:"ntfy,omitempty" json:"ntfy,omitempty"`
}

// NtfyConfig holds ntfy.sh notification settings.
type NtfyConfig struct {
	URL string `yaml:"url" json:"url"`
}
