package ui

import (
	"crypto/sha256"
	"os"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/watch"
)

type configChangedMsg struct{}

// WithConfigFile turns on live reload: when the file at path changes, as
// reported by w, Overlook reads it again and applies it.
func (m Model) WithConfigFile(path string, w *watch.Watcher) Model {
	m.configPath, m.configWatcher = path, w
	data, _ := os.ReadFile(path) // a missing file reads as empty, like the defaults
	m.configSum = sha256.Sum256(data)
	return m
}

func (m Model) waitForConfig() tea.Cmd {
	if m.configWatcher == nil {
		return nil
	}
	ch := m.configWatcher.Changes
	return func() tea.Msg {
		<-ch
		return configChangedMsg{}
	}
}

// reloadConfig re-reads the config file if its contents changed. Editors
// often write a file more than once per save, and other files in the
// folder can change too, so an unchanged file is ignored. A file that
// doesn't load leaves the current settings in place and says why.
func (m *Model) reloadConfig() tea.Cmd {
	data, _ := os.ReadFile(m.configPath)
	sum := sha256.Sum256(data)
	if sum == m.configSum {
		return nil
	}
	m.configSum = sum
	cfg, err := config.Load(m.configPath)
	if err != nil {
		reason := strings.ReplaceAll(err.Error(), m.configPath+": ", "")
		return m.say(errorMessage, "✗ Config not reloaded: "+strings.Join(strings.Fields(reason), " "))
	}
	return m.applyConfig(cfg)
}

// applyConfig switches to new settings while running: styles, panels,
// layout, keys, and refresh timing.
func (m *Model) applyConfig(cfg config.Config) tea.Cmd {
	old := m.cfg
	m.cfg, m.st = cfg, newStyles(cfg.Theme)

	m.files.cfg = cfg.Panels.Files
	if cfg.Panels.Files.View != old.Panels.Files.View {
		m.files.flat = cfg.Panels.Files.View == "flat"
	}
	m.files.setFiles(m.files.files)
	m.branches.cfg = cfg.Panels.Branches
	m.branches.setBranches(m.branches.branches, m.branches.err)
	m.diff.cfg = cfg.Panels.Diff
	m.log.cfg = cfg.Panels.Log
	if !cfg.Panels.Log.Show && m.diffFrom == logID {
		m.diffFrom = filesID
	}
	m.diff.setLines(m.diff.lines) // drop the old colors and wrapping; syncDiff below re-renders
	m.worktrees.cfg = cfg.Worktrees
	if m.watcher != nil {
		m.watcher.SetDebounce(cfg.Refresh.Debounce)
	}

	if shown := m.shownPanels(); !slices.Contains(shown, m.focus) {
		m.zoomed = false
		if len(shown) > 0 {
			m.setFocus(shown[0])
		}
	}
	m.layout()

	cmds := []tea.Cmd{m.load(), m.syncDiff(true), m.say(successMessage, "✓ Reloaded config")}
	if old.Refresh.Poll != cfg.Refresh.Poll {
		m.pollGen++ // retire the old timer, which may have a different interval
		cmds = append(cmds, m.poll())
	}
	return tea.Batch(cmds...)
}
