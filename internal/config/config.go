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
	Theme     Theme     `yaml:"theme"`
	Panels    Panels    `yaml:"panels"`
	Worktrees Worktrees `yaml:"worktrees"`
	Pull      Pull      `yaml:"pull"`
	Layout    Layout    `yaml:"layout"`
	Refresh   Refresh   `yaml:"refresh"`
	Keys      Keys      `yaml:"keys"`
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
	Success       string `yaml:"success"`
	Error         string `yaml:"error"`
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
	ActiveTab     string `yaml:"activeTab"`
	Tab           string `yaml:"tab"`
	DiffAdd       string `yaml:"diffAdd"`
	DiffDelete    string `yaml:"diffDelete"`
	DiffHunk      string `yaml:"diffHunk"`
	DiffFile      string `yaml:"diffFile"`
}

type Panels struct {
	Status   StatusPanel   `yaml:"status"`
	Files    FilesPanel    `yaml:"files"`
	Branches BranchesPanel `yaml:"branches"`
	Diff     DiffPanel     `yaml:"diff"`
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

type DiffPanel struct {
	Show          bool   `yaml:"show"`
	Position      string `yaml:"position"`
	RightMinWidth int    `yaml:"rightMinWidth"`
	RightWidth    int    `yaml:"rightWidth"`
	Size          int    `yaml:"size"`
	Wrap          bool   `yaml:"wrap"`
	HunkHeaders   string `yaml:"hunkHeaders"`
	MaxLines      int    `yaml:"maxLines"`
	Pager         string `yaml:"pager"`
}

type Worktrees struct {
	Tabs       bool `yaml:"tabs"`
	Counts     bool `yaml:"counts"`
	AutoSwitch bool `yaml:"autoSwitch"`
	Follow     bool `yaml:"follow"`
}

type Pull struct {
	Enabled bool          `yaml:"enabled"`
	Timeout time.Duration `yaml:"timeout"`
}

type Layout struct {
	Order         []string      `yaml:"order"`
	ExpandFocused bool          `yaml:"expandFocused"`
	CollapsedRows int           `yaml:"collapsedRows"`
	Footer        bool          `yaml:"footer"`
	MessageTime   time.Duration `yaml:"messageTime"`
}

// PanelNames are the panels layout.order can arrange, in default order.
var PanelNames = []string{"files", "diff", "branches"}

type Refresh struct {
	Debounce time.Duration `yaml:"debounce"`
	Poll     time.Duration `yaml:"poll"`
}

type Keys struct {
	Quit         []string `yaml:"quit"`
	Pull         []string `yaml:"pull"`
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
	NextWorktree []string `yaml:"nextWorktree"`
	PrevWorktree []string `yaml:"prevWorktree"`
	ScrollLeft   []string `yaml:"scrollLeft"`
	ScrollRight  []string `yaml:"scrollRight"`
	Zoom         []string `yaml:"zoom"`
	Back         []string `yaml:"back"`
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
	between := func(field string, n, lo, hi int) error {
		if n >= lo && n <= hi {
			return nil
		}
		return fmt.Errorf("%s is %d; use %d to %d", field, n, lo, hi)
	}
	atLeast1 := func(field string, n int) error { return between(field, n, 1, 1<<30) }
	var orderErrs []error
	for i, name := range c.Layout.Order {
		if err := oneOf("layout.order", name, PanelNames...); err != nil {
			orderErrs = append(orderErrs, err)
		} else if slices.Contains(c.Layout.Order[:i], name) {
			orderErrs = append(orderErrs, fmt.Errorf("layout.order lists %q twice", name))
		}
	}
	return errors.Join(
		errors.Join(orderErrs...),
		oneOf("panels.files.view", c.Panels.Files.View, "tree", "flat"),
		oneOf("panels.files.untracked", c.Panels.Files.Untracked, "all", "folders", "none"),
		oneOf("panels.branches.sort", c.Panels.Branches.Sort, "recent", "name"),
		atLeast1("panels.files.size", c.Panels.Files.Size),
		between("layout.collapsedRows", c.Layout.CollapsedRows, 1, 50),
		atLeast1("panels.branches.size", c.Panels.Branches.Size),
		oneOf("panels.diff.position", c.Panels.Diff.Position, "auto", "right", "bottom"),
		oneOf("panels.diff.hunkHeaders", c.Panels.Diff.HunkHeaders, "lines", "git"),
		between("panels.diff.rightWidth", c.Panels.Diff.RightWidth, 20, 80),
		atLeast1("panels.diff.size", c.Panels.Diff.Size),
		atLeast1("panels.diff.maxLines", c.Panels.Diff.MaxLines),
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
