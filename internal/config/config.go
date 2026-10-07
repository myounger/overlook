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
	"slices"
	"strings"
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
	Border        string `yaml:"border"`
	ActiveBorder  string `yaml:"activeBorder"`
	Title         string `yaml:"title"`
	Repo          string `yaml:"repo"`
	Branch        string `yaml:"branch"`
	Worktree      string `yaml:"worktree"`
	Upstream      string `yaml:"upstream"`
	InSync        string `yaml:"inSync"`
	Muted         string `yaml:"muted"`
	SelectedBg    string `yaml:"selectedBg"`
	Folder        string `yaml:"folder"`
	Staged        string `yaml:"staged"`
	Unstaged      string `yaml:"unstaged"`
	Added         string `yaml:"added"`
	Modified      string `yaml:"modified"`
	Deleted       string `yaml:"deleted"`
	Renamed       string `yaml:"renamed"`
	Conflicted    string `yaml:"conflicted"`
	CurrentBranch string `yaml:"currentBranch"`
	Gone          string `yaml:"gone"`
	Merged        string `yaml:"merged"`
}

type Panels struct {
	Status   StatusPanel   `yaml:"status"`
	Files    FilesPanel    `yaml:"files"`
	Branches BranchesPanel `yaml:"branches"`
}

type StatusPanel struct {
	Show     bool `yaml:"show"`
	Upstream bool `yaml:"upstream"`
	Worktree bool `yaml:"worktree"`
}

type FilesPanel struct {
	Show           bool   `yaml:"show"`
	View           string `yaml:"view"`
	CompactFolders bool   `yaml:"compactFolders"`
	Untracked      string `yaml:"untracked"`
	Size           int    `yaml:"size"`
}

type BranchesPanel struct {
	Show       bool   `yaml:"show"`
	Size       int    `yaml:"size"`
	Sort       string `yaml:"sort"`
	Age        bool   `yaml:"age"`
	Upstream   bool   `yaml:"upstream"`
	Worktree   bool   `yaml:"worktree"`
	MergedInto string `yaml:"mergedInto"`
}

type Layout struct {
	Footer bool `yaml:"footer"`
}

type Refresh struct {
	Debounce time.Duration `yaml:"debounce"`
	Poll     time.Duration `yaml:"poll"`
}

type Keys struct {
	Quit         []string `yaml:"quit"`
	Refresh      []string `yaml:"refresh"`
	Up           []string `yaml:"up"`
	Down         []string `yaml:"down"`
	PageUp       []string `yaml:"pageUp"`
	PageDown     []string `yaml:"pageDown"`
	Top          []string `yaml:"top"`
	Bottom       []string `yaml:"bottom"`
	ToggleFolder []string `yaml:"toggleFolder"`
	FoldAll      []string `yaml:"foldAll"`
	UnfoldAll    []string `yaml:"unfoldAll"`
	ToggleView   []string `yaml:"toggleView"`
	NextPanel    []string `yaml:"nextPanel"`
	PrevPanel    []string `yaml:"prevPanel"`
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
	if err := cfg.validate(); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func (c Config) validate() error {
	oneOf := func(field, value string, allowed ...string) error {
		if slices.Contains(allowed, value) {
			return nil
		}
		return fmt.Errorf("%s is %q; use one of: %s", field, value, strings.Join(allowed, ", "))
	}
	atLeast1 := func(field string, n int) error {
		if n >= 1 {
			return nil
		}
		return fmt.Errorf("%s is %d; use 1 or more", field, n)
	}
	return errors.Join(
		oneOf("panels.files.view", c.Panels.Files.View, "tree", "flat"),
		oneOf("panels.files.untracked", c.Panels.Files.Untracked, "all", "folders", "none"),
		oneOf("panels.branches.sort", c.Panels.Branches.Sort, "recent", "name"),
		atLeast1("panels.files.size", c.Panels.Files.Size),
		atLeast1("panels.branches.size", c.Panels.Branches.Size),
	)
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
