package ui

import (
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/myounger/overlook/internal/claudehook"
	"github.com/myounger/overlook/internal/watch"
)

// claudeMaxAge is how recently a Claude Code session must have been seen to
// count; it skips sessions that ended without a SessionEnd hook (a crash).
const claudeMaxAge = 12 * time.Hour

type claudeMsg struct{ states []claudehook.State }

// WithClaudeHook makes Overlook follow Claude Code sessions, using the
// session files the hook writes. w watches this repo's folder of them.
func (m Model) WithClaudeHook(w *watch.Watcher) Model {
	m.claudeWatcher = w
	return m
}

func (m Model) readClaude() tea.Cmd {
	if m.claudeWatcher == nil {
		return nil
	}
	commonDir := m.repo.CommonDir
	return func() tea.Msg {
		states, _ := claudehook.Read(commonDir, time.Now(), claudeMaxAge)
		return claudeMsg{states}
	}
}

func (m Model) waitForClaude() tea.Cmd {
	if m.claudeWatcher == nil {
		return nil
	}
	ch := m.claudeWatcher.Changes
	return func() tea.Msg {
		<-ch
		return claudeChangedMsg{}
	}
}

type claudeChangedMsg struct{}

// followClaude marks the worktrees sessions are in and, with
// worktrees.followClaude on, switches to the one a session just moved to.
// Only moves count: if you switch away while a session keeps working where
// it is, Overlook stays where you put it. At startup every session is new,
// so Overlook opens on the worktree the most recently active one is in.
func (m *Model) followClaude(states []claudehook.State) tea.Cmd {
	in := map[string]bool{}
	target, latest := "", time.Time{}
	seen := map[string]string{}
	for _, s := range states {
		in[s.Worktree] = true
		seen[s.Session] = s.Worktree
		if m.claudeSeen[s.Session] != s.Worktree && s.Time.After(latest) {
			target, latest = s.Worktree, s.Time
		}
	}
	m.claudeSeen = seen
	m.worktrees.claude = in

	if !m.cfg.Worktrees.FollowClaude || target == "" || target == m.repo.Root {
		return nil
	}
	if _, err := os.Stat(target); err != nil {
		return nil // removed since
	}
	return m.switchTo(target)
}
