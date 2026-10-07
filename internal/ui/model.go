// Package ui is the Bubble Tea app: it holds the latest repo state and draws
// the panels.
package ui

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
	"github.com/myounger/overlook/internal/watch"
)

type Model struct {
	cfg     config.Config
	st      styles
	repo    git.Repo
	watcher *watch.Watcher

	status    git.Status
	statusErr error
	loaded    bool

	width, height int
}

func New(cfg config.Config, repo git.Repo, w *watch.Watcher) Model {
	return Model{cfg: cfg, st: newStyles(cfg.Theme), repo: repo, watcher: w}
}

type (
	statusMsg struct {
		status git.Status
		err    error
	}
	changedMsg struct{}
	pollMsg    struct{}
)

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.load(), m.waitForChange(), m.poll())
}

func (m Model) load() tea.Cmd {
	repo := m.repo
	return func() tea.Msg {
		s, err := git.ReadStatus(repo)
		return statusMsg{s, err}
	}
}

func (m Model) waitForChange() tea.Cmd {
	if m.watcher == nil {
		return nil
	}
	ch := m.watcher.Changes
	return func() tea.Msg {
		<-ch
		return changedMsg{}
	}
}

func (m Model) poll() tea.Cmd {
	if m.cfg.Refresh.Poll <= 0 {
		return nil
	}
	return tea.Tick(m.cfg.Refresh.Poll, func(time.Time) tea.Msg { return pollMsg{} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		key := msg.String()
		switch {
		case slices.Contains(m.cfg.Keys.Quit, key):
			return m, tea.Quit
		case slices.Contains(m.cfg.Keys.Refresh, key):
			return m, m.load()
		}
	case tea.FocusMsg:
		return m, m.load()
	case statusMsg:
		m.status, m.statusErr, m.loaded = msg.status, msg.err, true
	case changedMsg:
		return m, tea.Batch(m.load(), m.waitForChange())
	case pollMsg:
		return m, tea.Batch(m.load(), m.poll())
	}
	return m, nil
}

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.ReportFocus = true
	v.WindowTitle = "overlook · " + m.repo.Name()
	return v
}

func (m Model) render() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	var sections []string
	if m.cfg.Panels.Status.Show {
		body := "loading…"
		if m.loaded {
			body = statusLine(m.st, m.cfg.Panels.Status, m.repo, m.status, m.statusErr)
		}
		sections = append(sections, renderPanel(m.st, "Status", body, m.width, 3, true))
	}

	footer := ""
	if m.cfg.Layout.Footer {
		footer = m.footer()
	}
	content := strings.Join(sections, "\n")
	gap := m.height - lipgloss.Height(content) - lipgloss.Height(footer)
	if gap > 0 {
		content += strings.Repeat("\n", gap)
	}
	if footer != "" {
		content += "\n" + footer
	}
	return content
}

func (m Model) footer() string {
	hint := func(keys []string, action string) string {
		if len(keys) == 0 {
			return ""
		}
		return keys[0] + " " + action
	}
	var hints []string
	for _, h := range []string{hint(m.cfg.Keys.Refresh, "refresh"), hint(m.cfg.Keys.Quit, "quit")} {
		if h != "" {
			hints = append(hints, h)
		}
	}
	return m.st.muted.Render(" " + strings.Join(hints, " · "))
}
