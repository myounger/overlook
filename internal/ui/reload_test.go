package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

func reloadModel(t *testing.T, initial string) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	os.WriteFile(path, []byte(initial), 0o644)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m := sized(t, cfg, 60, 40).WithConfigFile(path, nil)
	next, _ := m.Update(statusMsg{root: "/r", status: git.Status{Head: "main", Files: []git.File{{Path: "a/b.go", Staged: '.', Unstaged: 'M'}}}})
	return next.(Model), path
}

func changed(m Model) Model {
	next, _ := m.Update(configChangedMsg{})
	return next.(Model)
}

func TestReloadAppliesChanges(t *testing.T) {
	m, path := reloadModel(t, "")
	os.WriteFile(path, []byte("panels:\n  files:\n    view: flat\n  branches:\n    show: false\nkeys:\n  quit: [x]\n"), 0o644)
	m = changed(m)
	if !m.files.flat || m.cfg.Panels.Branches.Show || m.cfg.Keys.Quit[0] != "x" {
		t.Errorf("not applied: flat=%v branches=%v quit=%v", m.files.flat, m.cfg.Panels.Branches.Show, m.cfg.Keys.Quit)
	}
	if _, ok := m.rects[branchesID]; ok {
		t.Error("hidden Branches panel still laid out")
	}
	if !strings.Contains(footerText(m), "Reloaded config") {
		t.Errorf("footer %q", footerText(m))
	}
}

func TestReloadIgnoresUnchangedFile(t *testing.T) {
	m, path := reloadModel(t, "layout:\n  mouse: false\n")
	os.WriteFile(path, []byte("layout:\n  mouse: false\n"), 0o644) // saved again, same content
	if m = changed(m); m.msg.text != "" {
		t.Errorf("unchanged file produced %q", m.msg.text)
	}
}

func TestReloadKeepsOldSettingsOnError(t *testing.T) {
	m, path := reloadModel(t, "")
	os.WriteFile(path, []byte("panels:\n  files:\n    view: sideways\n"), 0o644)
	m = changed(m)
	if m.cfg.Panels.Files.View != "tree" {
		t.Error("a bad config was applied")
	}
	if got := footerText(m); !strings.Contains(got, "Config not reloaded") || !strings.Contains(got, "sideways") || strings.Contains(got, path) {
		t.Errorf("footer %q; want the reason without the file path", got)
	}
	// Fixing the file applies it.
	os.WriteFile(path, []byte("panels:\n  files:\n    view: flat\n"), 0o644)
	if m = changed(m); !m.files.flat {
		t.Error("fixed config not applied")
	}
}

func TestReloadRetiresOldPollTimer(t *testing.T) {
	m, path := reloadModel(t, "refresh:\n  poll: 2s\n")
	os.WriteFile(path, []byte("refresh:\n  poll: 5s\n"), 0o644)
	m = changed(m)
	next, cmd := m.Update(pollMsg{gen: m.pollGen - 1})
	if cmd != nil {
		t.Error("a timer from before the reload kept polling")
	}
	if _, cmd = next.Update(pollMsg{gen: m.pollGen}); cmd == nil {
		t.Error("the current timer stopped")
	}
}

func TestReloadKeepsFocusOnAShownPanel(t *testing.T) {
	m, path := reloadModel(t, "")
	m.cycleFocus(2) // Branches
	if m.focus != branchesID {
		t.Fatalf("focus %d", m.focus)
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	m = next.(Model)
	os.WriteFile(path, []byte("panels:\n  branches:\n    show: false\n"), 0o644)
	if m = changed(m); m.focus != filesID || m.zoomed {
		t.Errorf("focus=%d zoomed=%v after hiding the focused, zoomed panel", m.focus, m.zoomed)
	}
}
