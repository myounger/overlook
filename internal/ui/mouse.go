package ui

import (
	tea "charm.land/bubbletea/v2"
)

// diffWheelRows is how far one wheel step scrolls the diff.
const diffWheelRows = 3

// handleMouse makes the panels clickable and scrollable:
//
//   - The wheel scrolls the panel under the pointer without focusing it:
//     the diff moves by a few rows, a list moves its selection by one.
//   - Clicking a row selects it and focuses its panel. Clicking the row
//     that's already selected acts like enter: it folds a folder or opens a
//     file's diff.
//   - Clicking a worktree tab switches to it.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.cfg.Layout.Mouse {
		return m, nil
	}
	mouse := msg.Mouse()
	switch msg.(type) {
	case tea.MouseWheelMsg:
		return m, m.wheel(mouse)
	case tea.MouseClickMsg:
		if mouse.Button == tea.MouseLeft {
			return m, m.click(mouse.X, mouse.Y)
		}
	}
	return m, nil
}

// panelAt returns the focusable panel at a screen cell.
func (m Model) panelAt(x, y int) (panelID, rect, bool) {
	for _, id := range m.shownPanels() {
		if r, ok := m.rects[id]; ok && r.contains(x, y) && (!m.zoomed || id == m.focus) {
			return id, r, true
		}
	}
	return 0, rect{}, false
}

func (m *Model) wheel(mouse tea.Mouse) tea.Cmd {
	id, _, ok := m.panelAt(mouse.X, mouse.Y)
	if !ok {
		return nil
	}
	step := 0
	switch mouse.Button {
	case tea.MouseWheelUp:
		step = -1
	case tea.MouseWheelDown:
		step = 1
	case tea.MouseWheelLeft:
		if id == diffID {
			m.diff.scrollX(-diffScrollX)
		}
		return nil
	case tea.MouseWheelRight:
		if id == diffID {
			m.diff.scrollX(diffScrollX)
		}
		return nil
	}
	switch id {
	case diffID:
		m.diff.scroll(step * diffWheelRows)
	case branchesID:
		m.branches.move(step)
	case filesID:
		m.files.move(step)
		return m.syncDiff(false)
	}
	return nil
}

func (m *Model) click(x, y int) tea.Cmd {
	if y == 0 && m.worktrees.showTabs() {
		return m.switchTo(m.worktrees.tabAt(m.repo.Name(), m.repo.Root, m.width, x))
	}
	id, r, ok := m.panelAt(x, y)
	if !ok {
		return nil
	}
	wasFocused := m.focus == id
	m.setFocus(id)
	if m.zoomed {
		m.layout()
	}

	var list *listView
	switch id {
	case filesID:
		list = &m.files.listView
	case branchesID:
		list = &m.branches.listView
	default:
		return nil
	}
	// The first row of a list is just inside the top border.
	row := list.offset + y - r.y - 1
	if y <= r.y || y >= r.y+r.h-1 || row >= list.count {
		return nil
	}
	if id == filesID && wasFocused && row == list.cursor {
		// A second click on the selected row: fold the folder or open the diff.
		if m.files.rows[row].node.isDir() {
			m.files.toggleFolder()
		} else if m.cfg.Panels.Diff.Show {
			m.setFocus(diffID)
		}
		return nil
	}
	list.setCursor(row)
	if id == filesID {
		return m.syncDiff(false)
	}
	return nil
}
