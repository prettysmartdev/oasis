// Package config manages the oasis CLI configuration file (~/.oasis/config.json).
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config holds the CLI configuration persisted to ~/.oasis/config.json.
type Config struct {
	MgmtEndpoint     string `json:"mgmtEndpoint"`
	ContainerName    string `json:"containerName"`
	LastKnownVersion string `json:"lastKnownVersion,omitempty"`
}

// DefaultPath returns the default path for the CLI config file (~/.oasis/config.json).
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".oasis/config.json"
	}
	return filepath.Join(home, ".oasis", "config.json")
}

// defaults returns a Config populated with default values.
func defaults() *Config {
	return &Config{
		MgmtEndpoint:  "http://127.0.0.1:04515",
		ContainerName: "oasis",
	}
}

// Load reads and JSON-decodes the config file at path.
// If the file does not exist, it returns the default config without error.
// It errors only on permission or parse issues.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults(), nil
		}
		return nil, err
	}

	cfg := defaults()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save atomically writes cfg as JSON to path, creating parent directories as needed.
// It uses a temporary file and rename to avoid partial writes.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Write to a temp file in the same directory then rename atomically.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".oasis-config-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	return os.Rename(tmpName, path)
}
