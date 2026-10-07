package ui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/myounger/overlook/internal/git"
)

type messageKind int

const (
	infoMessage messageKind = iota
	successMessage
	errorMessage
)

// message is a one-line note shown in place of the footer's key hints.
// Success messages go away after layout.messageTime; errors stay until the
// next key press; info (like "Pulling…") stays until replaced.
type message struct {
	text string
	kind messageKind
	id   int // so an expiring message doesn't clear a newer one
}

type (
	pullDoneMsg struct {
		result git.PullResult
		err    error
	}
	clearMessageMsg struct{ id int }
)

// say shows a message and returns the command that will clear it, if it
// expires.
func (m *Model) say(kind messageKind, text string) tea.Cmd {
	m.msg = message{text: text, kind: kind, id: m.msg.id + 1}
	if kind != successMessage || m.cfg.Layout.MessageTime <= 0 {
		return nil
	}
	id := m.msg.id
	return tea.Tick(m.cfg.Layout.MessageTime, func(time.Time) tea.Msg { return clearMessageMsg{id} })
}

// startPull fast-forwards the current branch, unless a pull is already
// running or there's nothing to pull from.
func (m *Model) startPull() tea.Cmd {
	switch {
	case !m.cfg.Pull.Enabled || m.pulling || !m.loaded:
		return nil
	case m.status.Detached():
		return m.say(errorMessage, "✗ Not on a branch; nothing to pull")
	case m.status.Upstream == "":
		return m.say(errorMessage, fmt.Sprintf("✗ %s has no upstream to pull from", m.status.Head))
	}
	m.pulling = true
	repo, upstream, timeout := m.repo, m.status.Upstream, m.cfg.Pull.Timeout
	return tea.Batch(m.say(infoMessage, "Pulling "+upstream+"…"), func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		res, err := git.Pull(ctx, repo, upstream)
		return pullDoneMsg{res, err}
	})
}

func (m *Model) finishPull(msg pullDoneMsg) tea.Cmd {
	m.pulling = false
	var say tea.Cmd
	switch r := msg.result; {
	case msg.err != nil:
		say = m.say(errorMessage, "✗ "+msg.err.Error())
	case r.Commits == 0:
		say = m.say(successMessage, "✓ Already up to date with "+r.Upstream)
	case r.Commits == 1:
		say = m.say(successMessage, "✓ Pulled 1 commit from "+r.Upstream)
	default:
		say = m.say(successMessage, fmt.Sprintf("✓ Pulled %d commits from %s", r.Commits, r.Upstream))
	}
	return tea.Batch(say, m.load())
}
