// Package ui is the Bubble Tea app: it holds the latest repo state and draws
// the panels.
package ui

import (
	"slices"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
	"github.com/myounger/overlook/internal/watch"
)

const statusHeight = 3

// panelID names the list panels that can take focus, in screen order.
type panelID int

const (
	filesID panelID = iota
	branchesID
)

type Model struct {
	cfg     config.Config
	st      styles
	repo    git.Repo
	watcher *watch.Watcher

	status    git.Status
	statusErr error
	loaded    bool
	files     filesPanel
	branches  branchesPanel
	focus     panelID
	worktrees worktreeSet
	switching string // worktree path being switched to

	width, height int
}

func New(cfg config.Config, repo git.Repo, w *watch.Watcher) Model {
	m := Model{
		cfg:       cfg,
		st:        newStyles(cfg.Theme),
		repo:      repo,
		watcher:   w,
		files:     newFilesPanel(cfg.Panels.Files),
		branches:  newBranchesPanel(cfg.Panels.Branches, repo.Root),
		worktrees: newWorktreeSet(cfg.Worktrees),
	}
	if shown := m.shownPanels(); len(shown) > 0 {
		m.focus = shown[0]
	}
	return m
}

// Results carry the worktree root they were read from, so a result that
// arrives after switching to another worktree is dropped.
type (
	statusMsg struct {
		root     string
		status   git.Status
		activity git.Activity
		err      error
	}
	branchesMsg struct {
		root     string
		branches []git.Branch
		err      error
	}
	worktreesMsg struct {
		root     string
		list     []git.Worktree
		activity map[string]git.Activity
		err      error
	}
	switchedMsg struct {
		repo git.Repo
		err  error
	}
	changedMsg struct{}
	pollMsg    struct{}
)

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.load(), m.waitForChange(), m.poll())
}

func (m Model) load() tea.Cmd {
	repo, untracked := m.repo, git.Untracked(m.cfg.Panels.Files.Untracked)
	cmds := []tea.Cmd{func() tea.Msg {
		s, err := git.ReadStatus(repo, untracked)
		return statusMsg{repo.Root, s, git.ActivityOf(repo.Root, s.Files), err}
	}}
	if m.cfg.Panels.Branches.Show {
		mergedInto := m.cfg.Panels.Branches.MergedInto
		cmds = append(cmds, func() tea.Msg {
			b, _, err := git.ReadBranches(repo, mergedInto)
			return branchesMsg{repo.Root, b, err}
		})
	}
	if m.worktrees.enabled() {
		needActivity := m.cfg.Worktrees.Counts || m.cfg.Worktrees.Follow
		cmds = append(cmds, func() tea.Msg { return loadWorktrees(repo, untracked, needActivity) })
	}
	return tea.Batch(cmds...)
}

// loadWorktrees lists the worktrees and, for every one but the active
// worktree, reads its activity. Those git status calls run in parallel.
func loadWorktrees(repo git.Repo, untracked git.Untracked, needActivity bool) worktreesMsg {
	list, err := git.ListWorktrees(repo)
	msg := worktreesMsg{root: repo.Root, list: list, err: err, activity: map[string]git.Activity{}}
	if err != nil || !needActivity {
		return msg
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, w := range list {
		if w.Path == repo.Root || w.Prunable {
			continue
		}
		wg.Go(func() {
			if act, err := git.ReadActivity(w.Path, untracked); err == nil {
				mu.Lock()
				msg.activity[w.Path] = act
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return msg
}

// switchTo moves Overlook to the worktree at path.
func (m *Model) switchTo(path string) tea.Cmd {
	if path == "" || path == m.repo.Root || path == m.switching {
		return nil
	}
	m.switching = path
	return func() tea.Msg {
		repo, err := git.Locate(path)
		return switchedMsg{repo, err}
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
		if msg.root != m.repo.Root {
			return m, nil
		}
		m.statusErr, m.loaded = msg.err, true
		if msg.err == nil {
			m.status = msg.status
			m.files.setFiles(msg.status.Files)
			m.worktrees.setActive(msg.root, msg.activity)
		}
	case branchesMsg:
		if msg.root != m.repo.Root {
			return m, nil
		}
		m.branches.setBranches(msg.branches, msg.err)
	case worktreesMsg:
		if msg.err != nil || msg.root != m.repo.Root {
			return m, nil
		}
		hadTabs := m.worktrees.showTabs()
		target := m.worktrees.update(msg.list, msg.activity, m.repo.Root)
		if m.worktrees.showTabs() != hadTabs {
			m.layout()
		}
		return m, m.switchTo(target)
	case switchedMsg:
		m.switching = ""
		if msg.err != nil {
			return m, nil
		}
		m.repo = msg.repo
		m.status, m.statusErr, m.loaded = git.Status{}, nil, false
		files := newFilesPanel(m.cfg.Panels.Files)
		files.flat = m.files.flat
		files.setSize(m.files.width, m.files.height)
		m.files = files
		m.branches.repoRoot = msg.repo.Root
		return m, m.load()
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
	case is(k.NextPanel):
		m.cycleFocus(1)
		return m, nil
	case is(k.PrevPanel):
		m.cycleFocus(-1)
		return m, nil
	case is(k.NextWorktree):
		return m, m.switchTo(m.worktrees.neighbor(m.repo.Root, 1))
	case is(k.PrevWorktree):
		return m, m.switchTo(m.worktrees.neighbor(m.repo.Root, -1))
	}

	list := m.focusedList()
	if list == nil {
		return m, nil
	}
	switch {
	case is(k.Up):
		list.move(-1)
	case is(k.Down):
		list.move(1)
	case is(k.PageUp):
		list.move(-list.pageSize())
	case is(k.PageDown):
		list.move(list.pageSize())
	case is(k.Top):
		list.setCursor(0)
	case is(k.Bottom):
		list.setCursor(list.count - 1)
	}
	if m.focus != filesID {
		return m, nil
	}
	switch {
	case is(k.ToggleFolder):
		m.files.toggleFolder()
	case is(k.FoldAll):
		m.files.setAllFolded(true)
	case is(k.UnfoldAll):
		m.files.setAllFolded(false)
	case is(k.ToggleView):
		m.files.toggleView()
	}
	return m, nil
}

// shownPanels lists the list panels turned on in the config, in screen order.
func (m Model) shownPanels() []panelID {
	var ids []panelID
	if m.cfg.Panels.Files.Show {
		ids = append(ids, filesID)
	}
	if m.cfg.Panels.Branches.Show {
		ids = append(ids, branchesID)
	}
	return ids
}

func (m *Model) cycleFocus(step int) {
	shown := m.shownPanels()
	if len(shown) == 0 {
		return
	}
	i := slices.Index(shown, m.focus)
	m.focus = shown[(i+step+len(shown))%len(shown)]
}

func (m *Model) focusedList() *listView {
	if !slices.Contains(m.shownPanels(), m.focus) {
		return nil
	}
	switch m.focus {
	case filesID:
		return &m.files.listView
	case branchesID:
		return &m.branches.listView
	}
	return nil
}

// layout stacks the panels top to bottom: Status, then the list panels
// sharing what's left above the footer in proportion to their sizes.
func (m *Model) layout() {
	h := m.height
	if m.worktrees.showTabs() {
		h--
	}
	if m.cfg.Panels.Status.Show {
		h -= statusHeight
	}
	if m.cfg.Layout.Footer {
		h--
	}
	h = max(h, 0)

	shown := m.shownPanels()
	sizes := map[panelID]int{filesID: m.cfg.Panels.Files.Size, branchesID: m.cfg.Panels.Branches.Size}
	total := 0
	for _, id := range shown {
		total += sizes[id]
	}
	left := h
	for i, id := range shown {
		ph := h * sizes[id] / total
		if i == len(shown)-1 {
			ph = left
		}
		left -= ph
		switch id {
		case filesID:
			m.files.setSize(m.width, ph)
		case branchesID:
			m.branches.setSize(m.width, ph)
		}
	}
}

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.ReportFocus = true
	v.WindowTitle = "overlook · " + m.repo.Name()
	if m.repo.IsWorktree() {
		v.WindowTitle += " ⎇ " + m.repo.WorktreeName()
	}
	return v
}

func (m Model) render() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	var sections []string
	if m.worktrees.showTabs() {
		sections = append(sections, m.worktrees.view(m.st, m.repo.Name(), m.repo.Root, m.width))
	}
	if m.cfg.Panels.Status.Show {
		body := "loading…"
		if m.loaded {
			body = statusLine(m.st, m.cfg.Panels.Status, m.repo, m.status, m.statusErr)
		}
		sections = append(sections, renderPanel(m.st, "Status", body, m.width, statusHeight, false))
	}
	for _, id := range m.shownPanels() {
		active := id == m.focus
		switch {
		case id == filesID && m.files.height >= 2:
			sections = append(sections, m.files.view(m.st, active))
		case id == branchesID && m.branches.height >= 2:
			sections = append(sections, m.branches.view(m.st, active, time.Now()))
		}
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

// footer shows the keys that do something in the focused panel. Moving up
// and down, and refresh (everything refreshes on its own), are left out to
// save room.
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
	if m.focus == filesID && m.cfg.Panels.Files.Show {
		otherView := "flat"
		if m.files.flat {
			otherView = "tree"
		}
		hints = append(hints, hint{first(k.ToggleFolder), "fold"}, hint{first(k.ToggleView), otherView})
	}
	if len(m.shownPanels()) > 1 {
		hints = append(hints, hint{first(k.NextPanel), "panel"})
	}
	if m.worktrees.showTabs() {
		if p, n := first(k.PrevWorktree), first(k.NextWorktree); p != "" && n != "" {
			hints = append(hints, hint{p + n, "worktree"})
		}
	}
	hints = append(hints, hint{first(k.Quit), "quit"})

	var parts []string
	for _, h := range hints {
		if h.key != "" {
			parts = append(parts, h.key+" "+h.action)
		}
	}
	return m.st.muted.Render(ansi.Truncate(" "+strings.Join(parts, " · "), m.width, "…"))
}
