// Package config loads Overlook's settings from YAML, layered over the
// embedded defaults in default.yml.
package config

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"
)

//go:embed default.yml
var Default []byte

type Config struct {
	Theme   Theme   `yaml:"theme"`
	Panels  Panels  `yaml:"panels"`
	Layout  Layout  `yaml:"layout"`
	Refresh Refresh `yaml:"refresh"`
	Keys    Keys    `yaml:"keys"`
}

type Theme struct {
	Border       string `yaml:"border"`
	ActiveBorder string `yaml:"activeBorder"`
	Title        string `yaml:"title"`
	Repo         string `yaml:"repo"`
	Branch       string `yaml:"branch"`
	Worktree     string `yaml:"worktree"`
	Upstream     string `yaml:"upstream"`
	InSync       string `yaml:"inSync"`
	Muted        string `yaml:"muted"`
}

type Panels struct {
	Status StatusPanel `yaml:"status"`
}

type StatusPanel struct {
	Show     bool `yaml:"show"`
	Upstream bool `yaml:"upstream"`
	Worktree bool `yaml:"worktree"`
}

type Layout struct {
	Footer bool `yaml:"footer"`
}

type Refresh struct {
	Debounce time.Duration `yaml:"debounce"`
	Poll     time.Duration `yaml:"poll"`
}

type Keys struct {
	Quit    []string `yaml:"quit"`
	Refresh []string `yaml:"refresh"`
}

// Path returns the config file location: $XDG_CONFIG_HOME/overlook/config.yml,
// falling back to ~/.config/overlook/config.yml.
func Path() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "overlook", "config.yml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "overlook", "config.yml"), nil
}

// Load reads the defaults, then the file at path on top of them. A missing
// file is not an error; you just get the defaults.
func Load(path string) (Config, error) {
	var cfg Config
	if err := decode(Default, &cfg); err != nil {
		return cfg, fmt.Errorf("built-in defaults: %w", err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := decode(data, &cfg); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// decode rejects unknown keys so a typo in the config file is reported
// instead of silently ignored.
func decode(data []byte, cfg *Config) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
