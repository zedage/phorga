package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	envConfigPath     = "PHORGA_CONFIG"
	defaultConfigFile = "config.yaml"
)

// ResolveConfigPath determines the config file path using this priority
// (CFG-003):
//  1. Explicit CLI argument (if non-empty)
//  2. PHORGA_CONFIG environment variable
//  3. ./config.yaml in the current working directory
//
// The file does not have to exist: Phorga then starts without jobs and the
// web UI creates the file on the first save (CFG-009).
func ResolveConfigPath(cliPath string) string {
	if cliPath != "" {
		return cliPath
	}
	if envPath := os.Getenv(envConfigPath); envPath != "" {
		return envPath
	}
	return defaultConfigFile
}

// LoadOrDefault loads the config file at path. When the file does not exist
// it returns Default() and exists=false (CFG-009). The extension must still
// be one that can be saved later.
func LoadOrDefault(path string) (cfg *Config, exists bool, err error) {
	if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".yaml", ".yml", ".json":
			return Default(), false, nil
		default:
			return nil, false, fmt.Errorf("unsupported config file extension: %s (use .yaml, .yml, or .json)", filepath.Ext(path))
		}
	}
	cfg, err = Load(path)
	if err != nil {
		return nil, true, err
	}
	return cfg, true, nil
}

// Default returns the configuration used when there is no config file yet:
// no jobs, and the web UI turned on so it can be set up there (CFG-009).
func Default() *Config {
	cfg := &Config{WebUI: &WebUIConfig{Enabled: true}}
	applyDefaults(cfg)
	return cfg
}

// HasJobs reports whether the configuration defines any organizer profile
// or duplicate detection job.
func (c *Config) HasJobs() bool {
	return c != nil && (len(c.Organizer) > 0 || len(c.DuplicateDetection) > 0)
}
