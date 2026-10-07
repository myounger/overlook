package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

func testCommits(now time.Time) []git.Commit {
	return []git.Commit{
		{Hash: "aaa1111", Short: "aaa1111", Subject: "Fix the login redirect after a timeout", Time: now.Add(-2 * time.Hour), Unpushed: true},
		{Hash: "bbb2222", Short: "bbb2222", Subject: "Add the Files panel", Time: now.Add(-3 * 24 * time.Hour)},
	}
}

func TestLogView(t *testing.T) {
	now := time.Now()
	cfg, _ := config.Load("")
	st := newStyles(cfg.Theme)
	p := newLogPanel(cfg.Panels.Log)
	p.setSize(36, 5)
	p.setCommits(testCommits(now), nil)
	lines := strings.Split(ansi.Strip(p.view(st, true, now)), "\n")
	if !strings.Contains(lines[0], "Log · ↑1") {
		t.Errorf("title %q; want the unpushed count", lines[0])
	}
	want := []string{"2h ↑ aaa1111 Fix the login redi…", "3d   bbb2222 Add the Files panel"}
	for i, w := range want {
		if got := strings.TrimSpace(strings.Trim(lines[i+1], "│")); got != w {
			t.Errorf("row %d: got %q, want %q", i, got, w)
		}
	}
}

func TestLogCursorFollowsCommit(t *testing.T) {
	now := time.Now()
	cfg, _ := config.Load("")
	p := newLogPanel(cfg.Panels.Log)
	p.setSize(40, 10)
	p.setCommits(testCommits(now), nil)
	p.move(1) // bbb2222
	// A new commit lands on top; the cursor stays on bbb2222.
	p.setCommits(append([]git.Commit{{Hash: "ccc3333", Short: "ccc3333", Subject: "new"}}, testCommits(now)...), nil)
	if c, _ := p.selected(); c.Hash != "bbb2222" {
		t.Errorf("cursor on %s, want bbb2222", c.Hash)
	}
}

func logModel(t *testing.T) Model {
	t.Helper()
	cfg, _ := config.Load("")
	cfg.Panels.Log.Show = true
	m := sized(t, cfg, 60, 40)
	next, _ := m.Update(statusMsg{root: "/r", status: git.Status{Head: "main", OID: "aaa1111", Files: []git.File{{Path: "a.go", Staged: '.', Unstaged: 'M'}}}})
	next, _ = next.Update(logMsg{root: "/r", commits: testCommits(time.Now())})
	return next.(Model)
}

func TestDiffFollowsLastListPanel(t *testing.T) {
	m := logModel(t)
	if m.diff.path != "a.go" {
		t.Fatalf("diff starts on %q, want the selected file", m.diff.path)
	}
	// Tab order is files, diff, branches, log: three Tabs reach Log.
	for range 3 {
		next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m = next.(Model)
	}
	if m.focus != logID || m.diff.path != "aaa1111" {
		t.Fatalf("focus=%d diff=%q; want Log and its first commit", m.focus, m.diff.path)
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m = next.(Model); m.diff.path != "bbb2222" {
		t.Errorf("diff %q after moving down, want bbb2222", m.diff.path)
	}
	// Enter goes to the diff, and esc comes back to Log, not Files.
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m = next.(Model); m.focus != logID {
		t.Errorf("esc from a commit's diff went to %d, want Log", m.focus)
	}
	// Back in Files, the diff shows the file again.
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m = next.(Model); m.focus != filesID || m.diff.path != "a.go" {
		t.Errorf("focus=%d diff=%q; want Files and a.go", m.focus, m.diff.path)
	}
}

func TestCommitDiffIntro(t *testing.T) {
	cfg, _ := config.Load("")
	st := newStyles(cfg.Theme)
	p := newDiffPanel(cfg.Panels.Diff)
	p.setSize(50, 12)
	p.selectTarget("k", "aaa1111")
	p.setDiff(st, "k", "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n", "", "", nil, true, []string{"Fix it", "Matt · Tue Oct 7 2026 13:00"})
	got := plainLines(p.lines)
	if !strings.HasPrefix(got, "Fix it\nMatt · Tue Oct 7 2026 13:00\n\nx\n") {
		t.Errorf("got:\n%s", got)
	}
}

func TestLogHiddenByDefault(t *testing.T) {
	cfg, _ := config.Load("")
	m := sized(t, cfg, 60, 30)
	for _, id := range m.shownPanels() {
		if id == logID {
			t.Error("Log shown by default")
		}
	}
}
