package ui

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/myounger/overlook/internal/config"
)

// diffScrollX is how many columns h and l move the diff sideways.
const diffScrollX = 8

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
	case is(k.Zoom):
		m.zoomed = !m.zoomed
		m.layout()
		return m, nil
	case is(k.Back):
		switch {
		case m.zoomed:
			m.zoomed = false
			m.layout()
		case m.focus == diffID && m.cfg.Panels.Files.Show:
			m.focus = filesID
		}
		return m, nil
	}

	switch m.focus {
	case diffID:
		m.diffKey(is)
		return m, nil
	case branchesID:
		listKey(&m.branches.listView, is, k)
		return m, nil
	case filesID:
		listKey(&m.files.listView, is, k)
		switch {
		case is(k.ToggleFolder):
			if m.files.cursor < len(m.files.rows) && !m.files.rows[m.files.cursor].node.isDir() {
				if m.cfg.Panels.Diff.Show {
					m.focus = diffID
				}
			} else {
				m.files.toggleFolder()
			}
		case is(k.FoldAll):
			m.files.setAllFolded(true)
		case is(k.UnfoldAll):
			m.files.setAllFolded(false)
		case is(k.ToggleView):
			m.files.toggleView()
		}
		return m, m.syncDiff(false)
	}
	return m, nil
}

// listKey moves a list's cursor.
func listKey(list *listView, is func([]string) bool, k config.Keys) {
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
}

// diffKey scrolls the diff.
func (m *Model) diffKey(is func([]string) bool) {
	k, d := m.cfg.Keys, &m.diff
	switch {
	case is(k.Up):
		d.scroll(-1)
	case is(k.Down):
		d.scroll(1)
	case is(k.PageUp):
		d.scroll(-d.pageSize())
	case is(k.PageDown):
		d.scroll(d.pageSize())
	case is(k.Top):
		d.scroll(-len(d.lines))
	case is(k.Bottom):
		d.scroll(len(d.lines))
	case is(k.ScrollLeft):
		d.scrollX(-diffScrollX)
	case is(k.ScrollRight):
		d.scrollX(diffScrollX)
	}
}

// shownPanels lists the focusable panels turned on in the config, in
// layout.order (top to bottom, and Tab order). Panels left out of the order
// follow in their default order.
func (m Model) shownPanels() []panelID {
	byName := map[string]struct {
		id   panelID
		show bool
	}{
		"files":    {filesID, m.cfg.Panels.Files.Show},
		"diff":     {diffID, m.cfg.Panels.Diff.Show},
		"branches": {branchesID, m.cfg.Panels.Branches.Show},
	}
	var ids []panelID
	for _, name := range append(slices.Clip(m.cfg.Layout.Order), config.PanelNames...) {
		p, ok := byName[name]
		if ok && p.show && !slices.Contains(ids, p.id) {
			ids = append(ids, p.id)
		}
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
	if m.zoomed {
		m.layout()
	}
}
