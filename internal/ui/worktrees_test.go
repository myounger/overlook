package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

func trees(paths ...string) []git.Worktree {
	out := make([]git.Worktree, len(paths))
	for i, p := range paths {
		out[i] = git.Worktree{Path: p, Main: i == 0}
	}
	return out
}

func acts(fp map[string]uint64) map[string]git.Activity {
	out := map[string]git.Activity{}
	for p, f := range fp {
		out[p] = git.Activity{Changed: 1, Fingerprint: f}
	}
	return out
}

func TestWorktreeAutoSwitch(t *testing.T) {
	s := newWorktreeSet(config.Worktrees{AutoSwitch: true})
	if got := s.update(trees("/app", "/wt/a"), nil, "/app"); got != "" {
		t.Errorf("first list switched to %q; existing worktrees shouldn't trigger a switch", got)
	}
	if got := s.update(trees("/app", "/wt/a", "/wt/b"), nil, "/app"); got != "/wt/b" {
		t.Errorf("new worktree: got %q, want /wt/b", got)
	}
	if got := s.update(trees("/app", "/wt/a", "/wt/b"), nil, "/wt/b"); got != "" {
		t.Errorf("no change: got %q", got)
	}
	// b is removed while you're viewing it: back to main.
	if got := s.update(trees("/app", "/wt/a"), nil, "/wt/b"); got != "/app" {
		t.Errorf("active removed: got %q, want /app", got)
	}
	// b comes back under the same path: that's new again.
	if got := s.update(trees("/app", "/wt/a", "/wt/b"), nil, "/app"); got != "/wt/b" {
		t.Errorf("re-created worktree: got %q, want /wt/b", got)
	}
}

func TestWorktreeAutoSwitchOff(t *testing.T) {
	s := newWorktreeSet(config.Worktrees{})
	s.update(trees("/app"), nil, "/app")
	if got := s.update(trees("/app", "/wt/b"), nil, "/app"); got != "" {
		t.Errorf("autoSwitch off: got %q", got)
	}
	if got := s.update(trees("/app"), nil, "/wt/b"); got != "/app" {
		t.Errorf("fallback to main should still happen: got %q", got)
	}
}

func TestWorktreeFollow(t *testing.T) {
	s := newWorktreeSet(config.Worktrees{Follow: true})
	s.update(trees("/app", "/wt/a", "/wt/b"), acts(map[string]uint64{"/wt/a": 1, "/wt/b": 1}), "/app")
	if got := s.update(trees("/app", "/wt/a", "/wt/b"), acts(map[string]uint64{"/wt/a": 1, "/wt/b": 1}), "/app"); got != "" {
		t.Errorf("nothing changed: got %q", got)
	}
	if got := s.update(trees("/app", "/wt/a", "/wt/b"), acts(map[string]uint64{"/wt/a": 1, "/wt/b": 2}), "/app"); got != "/wt/b" {
		t.Errorf("b changed: got %q, want /wt/b", got)
	}

	off := newWorktreeSet(config.Worktrees{})
	off.update(trees("/app", "/wt/b"), acts(map[string]uint64{"/wt/b": 1}), "/app")
	if got := off.update(trees("/app", "/wt/b"), acts(map[string]uint64{"/wt/b": 2}), "/app"); got != "" {
		t.Errorf("follow off: got %q", got)
	}
}

func TestWorktreePrunableIgnored(t *testing.T) {
	s := newWorktreeSet(config.Worktrees{AutoSwitch: true, Tabs: true})
	s.update(trees("/app"), nil, "/app")
	list := trees("/app", "/wt/gone")
	list[1].Prunable = true
	if got := s.update(list, nil, "/app"); got != "" {
		t.Errorf("prunable worktree triggered a switch to %q", got)
	}
	if s.showTabs() {
		t.Error("tabs shown for a single usable worktree")
	}
}

func TestWorktreeNeighbor(t *testing.T) {
	s := newWorktreeSet(config.Worktrees{})
	s.update(trees("/app", "/wt/a", "/wt/b"), nil, "/app")
	if got := s.neighbor("/app", -1); got != "/wt/b" {
		t.Errorf("prev from main: got %q, want /wt/b (wraps)", got)
	}
	if got := s.neighbor("/wt/b", 1); got != "/app" {
		t.Errorf("next from last: got %q, want /app (wraps)", got)
	}
}

func TestWorktreeTabsKeepActiveVisible(t *testing.T) {
	cfg, _ := config.Load("")
	st := newStyles(cfg.Theme)
	s := newWorktreeSet(cfg.Worktrees)
	s.update(trees("/code/overlook", "/wt/first-long-name", "/wt/second-long-name", "/wt/third"), nil, "/code/overlook")
	s.setActive("/wt/third", git.Activity{Changed: 4})

	wide := ansi.Strip(s.view(st, "overlook", "/code/overlook", 80))
	if want := " overlook   first-long-name   second-long-name   third 4 "; wide != want {
		t.Errorf("wide: got %q, want %q", wide, want)
	}
	narrow := s.view(st, "overlook", "/wt/third", 30)
	if w := lipgloss.Width(narrow); w > 30 {
		t.Errorf("narrow width %d > 30", w)
	}
	if plain := ansi.Strip(narrow); !strings.Contains(plain, "third 4") || !strings.HasPrefix(plain, "‹") {
		t.Errorf("narrow: active tab not visible or no scroll marker: %q", plain)
	}
}
