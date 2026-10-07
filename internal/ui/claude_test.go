package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/myounger/overlook/internal/claudehook"
	"github.com/myounger/overlook/internal/config"
)

func claudeModel(t *testing.T, follow bool) (Model, string, string) {
	t.Helper()
	cfg, _ := config.Load("")
	cfg.Worktrees.FollowClaude = follow
	m := sized(t, cfg, 60, 30)
	// Real folders, since followClaude checks the worktree still exists.
	return m, t.TempDir(), t.TempDir()
}

func claude(m Model, states ...claudehook.State) (Model, tea.Cmd) {
	next, cmd := m.Update(claudeMsg{states})
	return next.(Model), cmd
}

func TestFollowClaude(t *testing.T) {
	m, wtA, wtB := claudeModel(t, true)
	now := time.Now()

	// At startup, a session already in worktree A: open there.
	m, cmd := claude(m, claudehook.State{Session: "s1", Worktree: wtA, Time: now})
	if cmd == nil || m.switching != wtA {
		t.Fatalf("startup: switching=%q, want %q", m.switching, wtA)
	}
	m.switching, m.repo.Root = "", wtA // pretend the switch finished

	// The session keeps working in A while you switch to main by hand.
	m.repo.Root = "/r"
	if m, cmd = claude(m, claudehook.State{Session: "s1", Worktree: wtA, Time: now}); cmd != nil {
		t.Error("a session staying put pulled Overlook back")
	}

	// The session moves to B: follow it.
	if m, cmd = claude(m, claudehook.State{Session: "s1", Worktree: wtB, Time: now.Add(time.Minute)}); cmd == nil || m.switching != wtB {
		t.Errorf("move: switching=%q, want %q", m.switching, wtB)
	}
	if !m.worktrees.claude[wtB] || m.worktrees.claude[wtA] {
		t.Errorf("marked worktrees %v; want only B", m.worktrees.claude)
	}
}

func TestFollowClaudePicksTheLatestMove(t *testing.T) {
	m, wtA, wtB := claudeModel(t, true)
	now := time.Now()
	m, _ = claude(m,
		claudehook.State{Session: "s1", Worktree: wtA, Time: now},
		claudehook.State{Session: "s2", Worktree: wtB, Time: now.Add(time.Second)},
	)
	if m.switching != wtB {
		t.Errorf("switching=%q; want the more recent session's worktree", m.switching)
	}
}

func TestFollowClaudeOff(t *testing.T) {
	m, wtA, _ := claudeModel(t, false)
	m, cmd := claude(m, claudehook.State{Session: "s1", Worktree: wtA, Time: time.Now()})
	if cmd != nil {
		t.Error("switched with followClaude off")
	}
	if !m.worktrees.claude[wtA] {
		t.Error("the session's worktree should still be known")
	}
}

func TestFollowClaudeSkipsRemovedWorktree(t *testing.T) {
	m, _, _ := claudeModel(t, true)
	if _, cmd := claude(m, claudehook.State{Session: "s1", Worktree: "/gone/wt", Time: time.Now()}); cmd != nil {
		t.Error("switched to a worktree that no longer exists")
	}
}
