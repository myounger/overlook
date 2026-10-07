package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

func TestRelativeAge(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	tests := map[time.Duration]string{
		30 * time.Second:     "30s",
		5 * time.Minute:      "5m",
		3 * time.Hour:        "3h",
		6 * 24 * time.Hour:   "6d",
		20 * 24 * time.Hour:  "2w",
		100 * 24 * time.Hour: "3mo",
		800 * 24 * time.Hour: "2y",
	}
	for ago, want := range tests {
		if got := relativeAge(now.Add(-ago), now); got != want {
			t.Errorf("%v ago: got %q, want %q", ago, got, want)
		}
	}
}

func testBranches(now time.Time) []git.Branch {
	return []git.Branch{
		{Name: "old", Committed: now.Add(-30 * 24 * time.Hour), Upstream: "origin/old", Gone: true},
		{Name: "main", Committed: now.Add(-2 * time.Hour), Upstream: "origin/main"},
		{Name: "feature/login", Current: true, Committed: now.Add(-5 * time.Minute), Upstream: "origin/feature/login", Ahead: 2},
		{Name: "done", Committed: now.Add(-3 * 24 * time.Hour), Merged: true},
		{Name: "claude/wt", Committed: now.Add(-time.Hour), Worktree: "/code/app-wt/claude-wt"},
	}
}

func TestBranchesSortCurrentFirst(t *testing.T) {
	now := time.Now()
	for sort, want := range map[string]string{
		"recent": "feature/login claude/wt main done old",
		"name":   "feature/login claude/wt done main old",
	} {
		p := newBranchesPanel(config.BranchesPanel{Sort: sort}, "/code/app")
		p.setSize(40, 10)
		p.setBranches(testBranches(now), nil)
		var names []string
		for _, b := range p.branches {
			names = append(names, b.Name)
		}
		if got := strings.Join(names, " "); got != want {
			t.Errorf("sort %s: got %q, want %q", sort, got, want)
		}
	}
}

func TestBranchesCursorFollowsBranch(t *testing.T) {
	now := time.Now()
	p := newBranchesPanel(config.BranchesPanel{Sort: "recent"}, "/code/app")
	p.setSize(40, 10)
	p.setBranches(testBranches(now), nil)
	p.move(2) // main
	bs := testBranches(now)
	bs[1].Committed = now // main is now the newest after the current branch
	p.setBranches(bs, nil)
	if got := p.branches[p.cursor].Name; got != "main" {
		t.Errorf("cursor on %q, want main", got)
	}
}

func TestBranchesView(t *testing.T) {
	now := time.Now()
	cfg, _ := config.Load("")
	st := newStyles(cfg.Theme)
	p := newBranchesPanel(cfg.Panels.Branches, "/code/app")
	p.setSize(44, 7)
	p.setBranches(testBranches(now), nil)
	out := p.view(st, true, now)
	lines := strings.Split(out, "\n")
	if len(lines) != 7 {
		t.Fatalf("%d lines", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 44 {
			t.Errorf("line %d width %d: %q", i, w, ansi.Strip(line))
		}
	}
	want := []string{
		" 5m * feature/login ↑2",
		" 1h   claude/wt ⎇ claude-wt",
		" 2h   main ✓",
		" 3d   done merged",
		" 4w   old gone",
	}
	for i, w := range want {
		got := strings.TrimRight(strings.Trim(ansi.Strip(lines[i+1]), "│"), " ")
		if got != w {
			t.Errorf("row %d: got %q, want %q", i, got, w)
		}
	}
}
