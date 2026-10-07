package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

func pullModel(t *testing.T, s git.Status) Model {
	t.Helper()
	cfg, _ := config.Load("")
	var m tea.Model = New(cfg, git.Repo{Root: "/r", GitDir: "/r/.git", CommonDir: "/r/.git"}, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
	m, _ = m.Update(statusMsg{root: "/r", status: s})
	return m.(Model)
}

func footerText(m Model) string { return strings.TrimSpace(ansi.Strip(m.footer())) }

func press(m Model, key string) (Model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyPressMsg{Code: rune(key[0]), Text: key})
	return next.(Model), cmd
}

func TestPullRefusesWithoutUpstream(t *testing.T) {
	m := pullModel(t, git.Status{Head: "feature"})
	m, _ = press(m, "p")
	if m.pulling || !strings.Contains(footerText(m), "feature has no upstream") {
		t.Errorf("pulling=%v footer=%q", m.pulling, footerText(m))
	}
	// The error stays until the next key, which still does its job.
	m, _ = press(m, "j")
	if strings.Contains(footerText(m), "upstream") {
		t.Errorf("error still shown after a key press: %q", footerText(m))
	}

	m = pullModel(t, git.Status{OID: "abc"})
	m, _ = press(m, "p")
	if !strings.Contains(footerText(m), "Not on a branch") {
		t.Errorf("detached: footer=%q", footerText(m))
	}
}

func TestPullMessages(t *testing.T) {
	m := pullModel(t, git.Status{Head: "main", Upstream: "origin/main"})
	if !strings.Contains(footerText(m), "p pull") {
		t.Errorf("no pull hint: %q", footerText(m))
	}
	m, cmd := press(m, "p")
	if !m.pulling || cmd == nil || footerText(m) != "Pulling origin/main…" {
		t.Fatalf("pulling=%v footer=%q", m.pulling, footerText(m))
	}
	if again, cmd := press(m, "p"); cmd != nil || again.msg.text != m.msg.text {
		t.Error("a second p started another pull")
	}

	next, _ := m.Update(pullDoneMsg{result: git.PullResult{Commits: 3, Upstream: "origin/main"}})
	m = next.(Model)
	if m.pulling || footerText(m) != "✓ Pulled 3 commits from origin/main" {
		t.Errorf("pulling=%v footer=%q", m.pulling, footerText(m))
	}
	// The success message expires; an old timer can't clear a newer message.
	id := m.msg.id
	next, _ = m.Update(clearMessageMsg{id - 1})
	if m = next.(Model); m.msg.text == "" {
		t.Error("a stale timer cleared the message")
	}
	next, _ = m.Update(clearMessageMsg{id})
	if m = next.(Model); m.msg.text != "" || !strings.Contains(footerText(m), "quit") {
		t.Errorf("message not cleared: %q", footerText(m))
	}

	m, _ = press(m, "p")
	next, _ = m.Update(pullDoneMsg{err: errors.New("can't fast-forward: diverged")})
	if m = next.(Model); footerText(m) != "✗ can't fast-forward: diverged" {
		t.Errorf("footer=%q", footerText(m))
	}
}

func TestPullDisabled(t *testing.T) {
	m := pullModel(t, git.Status{Head: "main", Upstream: "origin/main"})
	m.cfg.Pull.Enabled = false
	if m, cmd := press(m, "p"); m.pulling || cmd != nil || strings.Contains(footerText(m), "pull") {
		t.Errorf("pull ran or was advertised while disabled: %q", footerText(m))
	}
}
