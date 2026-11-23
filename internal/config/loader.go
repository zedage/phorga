package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const DefaultWebUIAddress = "127.0.0.1:8080"

// Load reads and parses a configuration file. Supports YAML (.yaml, .yml) and JSON (.json).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	return Parse(path, data)
}

// Parse reads a configuration from data in the format given by the extension
// of path, and applies the defaults. It is used for files and for the text
// edited in the web UI (WEB-018).
func Parse(path string, data []byte) (*Config, error) {
	cfg := &Config{}
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file extension: %s (use .yaml, .yml, or .json)", ext)
	}

	// Apply defaults
	applyDefaults(cfg)

	return cfg, nil
}

// applyDefaults sets default values for optional fields.
func applyDefaults(cfg *Config) {
	if cfg.StateDir == "" {
		cfg.StateDir = "/data/state"
	}
	if cfg.WebUI != nil && cfg.WebUI.Address == "" {
		cfg.WebUI.Address = DefaultWebUIAddress
	}
	for i := range cfg.Organizer {
		if cfg.Organizer[i].TargetStructure == "" {
			cfg.Organizer[i].TargetStructure = "{year}/{month}"
		}
	}
}
