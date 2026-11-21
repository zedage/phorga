package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

// Save writes the configuration to disk using the format inferred from the target file extension.
// Supported output formats are YAML (.yaml, .yml) and JSON (.json).
func Save(path string, cfg *Config) error {
	data, err := Marshal(path, cfg)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0644)
}

// Marshal serializes the configuration using the format inferred from path.
func Marshal(path string, cfg *Config) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(path))
	normalized := normalizedCopy(cfg)

	switch ext {
	case ".yaml", ".yml":
		data, err := yaml.Marshal(normalized)
		if err != nil {
			return nil, fmt.Errorf("marshalling YAML config: %w", err)
		}
		return data, nil
	case ".json":
		data, err := json.MarshalIndent(normalized, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshalling JSON config: %w", err)
		}
		return append(data, '\n'), nil
	default:
		return nil, fmt.Errorf("unsupported config file extension: %s (use .yaml, .yml, or .json)", ext)
	}
}

func normalizedCopy(cfg *Config) *Config {
	if cfg == nil {
		return &Config{}
	}

	clone := *cfg
	clone.Organizer = append([]OrganizerProfile(nil), cfg.Organizer...)
	clone.DuplicateDetection = append([]DuplicateDetection(nil), cfg.DuplicateDetection...)
	if cfg.WebUI != nil {
		webUI := *cfg.WebUI
		webUI.BrowseRoots = append([]string(nil), cfg.WebUI.BrowseRoots...)
		if len(webUI.BrowseRoots) == 0 {
			webUI.BrowseRoots = nil
		}
		clone.WebUI = &webUI
	}
	if cfg.SynologyPhotos != nil {
		syn := *cfg.SynologyPhotos
		clone.SynologyPhotos = &syn
	}

	for i := range clone.Organizer {
		clone.Organizer[i].Sources = append([]OrganizerSource(nil), cfg.Organizer[i].Sources...)
		if cfg.Organizer[i].Ntfy != nil {
			ntfy := *cfg.Organizer[i].Ntfy
			clone.Organizer[i].Ntfy = &ntfy
		}
		for j := range clone.Organizer[i].Sources {
			clone.Organizer[i].Sources[j].Exclude = copyList(cfg.Organizer[i].Sources[j].Exclude)
			if cfg.Organizer[i].Sources[j].Duplicates != nil {
				dup := *cfg.Organizer[i].Sources[j].Duplicates
				clone.Organizer[i].Sources[j].Duplicates = &dup
			}
		}
	}

	for i := range clone.DuplicateDetection {
		clone.DuplicateDetection[i].Sources = append([]string(nil), cfg.DuplicateDetection[i].Sources...)
		clone.DuplicateDetection[i].Exclude = copyList(cfg.DuplicateDetection[i].Exclude)
		if cfg.DuplicateDetection[i].Ntfy != nil {
			ntfy := *cfg.DuplicateDetection[i].Ntfy
			clone.DuplicateDetection[i].Ntfy = &ntfy
		}
	}

	applyDefaults(&clone)
	return &clone
}

// copyList copies an optional list; an empty list becomes nil, so it is
// left out of the saved file.
func copyList(list []string) []string {
	if len(list) == 0 {
		return nil
	}
	return append([]string(nil), list...)
}

// WriteFile writes data as the config file at path, atomically, creating
// missing folders. The web UI uses it to save edited text as it is, with its
// comments (WEB-018).
func WriteFile(path string, data []byte) error {
	return writeFileAtomic(path, data, 0644)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp config file: %w", err)
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("writing temp config file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		cleanup()
		return fmt.Errorf("setting temp config permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("closing temp config file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		if errors.Is(err, syscall.EBUSY) {
			// Docker bind mount of the file itself: it cannot be replaced.
			return fmt.Errorf("replacing config file atomically: %w (the file is mounted on its own; mount its folder instead, e.g. ./config:/config)", err)
		}
		return fmt.Errorf("replacing config file atomically: %w", err)
	}
	return nil
}
