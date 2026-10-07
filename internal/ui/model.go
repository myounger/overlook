// Package ui is the Bubble Tea app: it holds the latest repo state and draws
// the panels.
package ui

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
	"github.com/myounger/overlook/internal/watch"
)

const statusHeight = 3

type Model struct {
	cfg     config.Config
	st      styles
	repo    git.Repo
	watcher *watch.Watcher

	status    git.Status
	statusErr error
	loaded    bool
	files     filesPanel

	width, height int
}

func New(cfg config.Config, repo git.Repo, w *watch.Watcher) Model {
	return Model{
		cfg:     cfg,
		st:      newStyles(cfg.Theme),
		repo:    repo,
		watcher: w,
		files:   newFilesPanel(cfg.Panels.Files),
	}
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
	repo, untracked := m.repo, git.Untracked(m.cfg.Panels.Files.Untracked)
	return func() tea.Msg {
		s, err := git.ReadStatus(repo, untracked)
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
		m.layout()
	case tea.KeyPressMsg:
		return m.handleKey(msg.String())
	case tea.FocusMsg:
		return m, m.load()
	case statusMsg:
		m.statusErr, m.loaded = msg.err, true
		if msg.err == nil {
			m.status = msg.status
			m.files.setFiles(msg.status.Files)
		}
	case changedMsg:
		return m, tea.Batch(m.load(), m.waitForChange())
	case pollMsg:
		return m, tea.Batch(m.load(), m.poll())
	}
	return m, nil
}

func (m Model) handleKey(key string) (tea.Model, tea.Cmd) {
	k := m.cfg.Keys
	is := func(keys []string) bool { return slices.Contains(keys, key) }
	switch {
	case is(k.Quit):
		return m, tea.Quit
	case is(k.Refresh):
		return m, m.load()
	}
	if !m.cfg.Panels.Files.Show {
		return m, nil
	}
	f := &m.files
	switch {
	case is(k.Up):
		f.move(-1)
	case is(k.Down):
		f.move(1)
	case is(k.PageUp):
		f.move(-f.pageSize())
	case is(k.PageDown):
		f.move(f.pageSize())
	case is(k.Top):
		f.move(-len(f.rows))
	case is(k.Bottom):
		f.move(len(f.rows))
	case is(k.ToggleFolder):
		f.toggleFolder()
	case is(k.FoldAll):
		f.setAllFolded(true)
	case is(k.UnfoldAll):
		f.setAllFolded(false)
	case is(k.ToggleView):
		f.toggleView()
	}
	return m, nil
}

// layout stacks the panels top to bottom: Status, then Files filling what's
// left above the footer.
func (m *Model) layout() {
	h := m.height
	if m.cfg.Panels.Status.Show {
		h -= statusHeight
	}
	if m.cfg.Layout.Footer {
		h--
	}
	m.files.setSize(m.width, max(h, 0))
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
		sections = append(sections, renderPanel(m.st, "Status", body, m.width, statusHeight, false))
	}
	if m.cfg.Panels.Files.Show && m.files.height >= 2 {
		sections = append(sections, m.files.view(m.st, true))
	}

	content := strings.Join(sections, "\n")
	lines := strings.Count(content, "\n") + 1
	if content == "" {
		lines = 0
	}
	if m.cfg.Layout.Footer {
		if gap := m.height - 1 - lines; gap > 0 {
			content += strings.Repeat("\n", gap)
		}
		if content != "" {
			content += "\n"
		}
		content += m.footer()
	}
	return content
}

func (m Model) footer() string {
	k := m.cfg.Keys
	first := func(keys []string) string {
		if len(keys) == 0 {
			return ""
		}
		return keys[0]
	}
	type hint struct{ key, action string }
	var hints []hint
	if m.cfg.Panels.Files.Show {
		otherView := "flat"
		if m.files.flat {
			otherView = "tree"
		}
		move := first(k.Down) + "/" + first(k.Up)
		if move == "/" {
			move = ""
		}
		hints = append(hints, hint{move, "move"}, hint{first(k.ToggleFolder), "fold"}, hint{first(k.ToggleView), otherView})
	}
	hints = append(hints, hint{first(k.Refresh), "refresh"}, hint{first(k.Quit), "quit"})

	var parts []string
	for _, h := range hints {
		if h.key != "" {
			parts = append(parts, h.key+" "+h.action)
		}
	}
	return m.st.muted.Render(ansi.Truncate(" "+strings.Join(parts, " · "), m.width, "…"))
}
