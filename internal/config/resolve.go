package config

import (
	"fmt"
	"os"
)

const (
	envConfigPath  = "PHORGANIZER_CONFIG"
	defaultConfigFile = "config.yaml"
)

// ResolveConfigPath determines the config file path using this priority:
//  1. Explicit CLI argument (if non-empty)
//  2. PHORGANIZER_CONFIG environment variable
//  3. ./config.yaml in the current working directory
func ResolveConfigPath(cliPath string) (string, error) {
	if cliPath != "" {
		if _, err := os.Stat(cliPath); err != nil {
			return "", fmt.Errorf("config file not found: %s", cliPath)
		}
		return cliPath, nil
	}

	if envPath := os.Getenv(envConfigPath); envPath != "" {
		if _, err := os.Stat(envPath); err != nil {
			return "", fmt.Errorf("config file from %s not found: %s", envConfigPath, envPath)
		}
		return envPath, nil
	}

	if _, err := os.Stat(defaultConfigFile); err != nil {
		return "", fmt.Errorf("no config file found: pass --config, set %s, or create %s", envConfigPath, defaultConfigFile)
	}
	return defaultConfigFile, nil
}

