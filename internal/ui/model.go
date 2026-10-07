// Package ui is the Bubble Tea app: it holds the latest repo state and draws
// the panels.
package ui

import (
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
	"github.com/myounger/overlook/internal/watch"
)

// panelID names the panels that can take focus, in Tab order.
type panelID int

const (
	filesID panelID = iota
	branchesID
	diffID
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
	diff      diffPanel
	worktrees worktreeSet
	switching string // worktree path being switched to
	pulling   bool
	msg       message

	focus     panelID
	zoomed    bool // the focused panel fills the window
	diffRight bool // the diff sits right of the other panels instead of below
	width     int
	height    int
}

func New(cfg config.Config, repo git.Repo, w *watch.Watcher) Model {
	m := Model{
		cfg:       cfg,
		st:        newStyles(cfg.Theme),
		repo:      repo,
		watcher:   w,
		files:     newFilesPanel(cfg.Panels.Files),
		branches:  newBranchesPanel(cfg.Panels.Branches, repo.Root),
		diff:      newDiffPanel(cfg.Panels.Diff),
		worktrees: newWorktreeSet(cfg.Worktrees),
	}
	if shown := m.shownPanels(); len(shown) > 0 {
		m.focus = shown[0]
	}
	return m
}

// Results carry the worktree root (or, for a diff, the selection) they were
// read for, so a result that arrives after you've moved on is dropped.
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
	diffMsg struct {
		key   string
		raw   string
		paged string
		note  string
		multi bool
		err   error
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

// syncDiff points the diff at the Files selection. It loads the diff when
// the selection changed, or always with reload (after a refresh, since the
// file itself may have changed).
func (m *Model) syncDiff(reload bool) tea.Cmd {
	if !m.cfg.Panels.Diff.Show || !m.loaded {
		return nil
	}
	key, target, ok := m.files.selection()
	label := ""
	if ok {
		key = m.repo.Root + "\x00" + key
		label = target.Path
		if target.Dir {
			label += "/"
		}
	}
	changed := m.diff.selectTarget(key, label)
	if !ok || !(changed || reload) {
		return nil
	}

	repo, hasHead := m.repo, m.status.OID != ""
	pager, width := m.cfg.Panels.Diff.Pager, max(m.diff.width-4, 20)
	return func() tea.Msg {
		if target.Untracked && strings.HasSuffix(target.Path, "/") {
			return diffMsg{key: key, note: "A new folder. Set panels.files.untracked to all to list its files."}
		}
		raw, err := git.Diff(repo, target, hasHead, pager != "")
		msg := diffMsg{key: key, raw: raw, multi: target.Dir, err: err}
		if err == nil && pager != "" && raw != "" {
			msg.paged, msg.err = git.Pipe(pager, raw, width)
			msg.raw = ansi.Strip(raw)
		}
		return msg
	}
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
		if m.cfg.Panels.Diff.Pager != "" {
			return m, m.syncDiff(true) // the pager laid it out for the old width
		}
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
		return m, m.syncDiff(true)
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
		m.diff.selectTarget("", "")
		return m, m.load()
	case diffMsg:
		m.diff.setDiff(m.st, msg.key, msg.raw, msg.paged, msg.note, msg.err, msg.multi)
	case pullDoneMsg:
		return m, m.finishPull(msg)
	case clearMessageMsg:
		if msg.id == m.msg.id {
			m.msg = message{}
		}
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
	if m.repo.IsWorktree() {
		v.WindowTitle += " ⎇ " + m.repo.WorktreeName()
	}
	return v
}
