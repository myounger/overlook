package ui

import (
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const statusHeight = 3

// layout sizes every panel. From the top: worktree tabs, then a column of
// Status and the list panels, which share the height in proportion to
// their sizes. The diff either sits right of that column or joins the
// bottom of it. Zoomed, the focused panel gets the whole body.
func (m *Model) layout() {
	body := m.height
	if m.worktrees.showTabs() {
		body--
	}
	if m.cfg.Layout.Footer {
		body--
	}
	body = max(body, 0)

	if m.zoomed {
		m.sizePanel(m.focus, m.width, body)
		return
	}

	d := m.cfg.Panels.Diff
	m.diffRight = d.Show && (d.Position == "right" || d.Position == "auto" && m.width >= d.RightMinWidth)
	colW := m.width
	if m.diffRight {
		diffW := m.width * d.RightWidth / 100
		colW = m.width - diffW
		m.diff.setSize(diffW, body)
	}

	stackH := body
	if m.cfg.Panels.Status.Show {
		stackH = max(stackH-statusHeight, 0)
	}
	stack := m.stackedPanels()
	total := 0
	for _, id := range stack {
		total += m.panelSize(id)
	}
	left := stackH
	for i, id := range stack {
		h := stackH * m.panelSize(id) / total
		if i == len(stack)-1 {
			h = left
		}
		left -= h
		m.sizePanel(id, colW, h)
	}
}

// stackedPanels are the panels in the left column under Status.
func (m Model) stackedPanels() []panelID {
	ids := m.shownPanels()
	if m.diffRight {
		ids = slices.DeleteFunc(ids, func(id panelID) bool { return id == diffID })
	}
	return ids
}

func (m Model) panelSize(id panelID) int {
	switch id {
	case filesID:
		return m.cfg.Panels.Files.Size
	case branchesID:
		return m.cfg.Panels.Branches.Size
	default:
		return m.cfg.Panels.Diff.Size
	}
}

func (m *Model) sizePanel(id panelID, w, h int) {
	switch id {
	case filesID:
		m.files.setSize(w, h)
	case branchesID:
		m.branches.setSize(w, h)
	case diffID:
		m.diff.setSize(w, h)
	}
}

func (m Model) panelView(id panelID) string {
	active := id == m.focus
	var view string
	var w, h int
	switch id {
	case filesID:
		view, w, h = m.files.view(m.st, active), m.files.width, m.files.height
	case branchesID:
		view, w, h = m.branches.view(m.st, active, time.Now()), m.branches.width, m.branches.height
	case diffID:
		view, w, h = m.diff.view(m.st, active), m.diff.width, m.diff.height
	}
	if view == "" {
		return blank(w, h) // too small to draw a border
	}
	return view
}

func (m Model) render() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	var sections []string
	if m.worktrees.showTabs() {
		sections = append(sections, m.worktrees.view(m.st, m.repo.Name(), m.repo.Root, m.width))
	}

	if m.zoomed {
		sections = append(sections, m.panelView(m.focus))
	} else {
		var col []string
		if m.cfg.Panels.Status.Show {
			body := "loading…"
			if m.loaded {
				body = statusLine(m.st, m.cfg.Panels.Status, m.repo, m.status, m.statusErr)
			}
			colW := m.width
			if m.diffRight {
				colW -= m.diff.width
			}
			col = append(col, renderPanel(m.st, "Status", body, colW, statusHeight, false))
		}
		for _, id := range m.stackedPanels() {
			col = append(col, m.panelView(id))
		}
		column := strings.Join(slices.DeleteFunc(col, func(s string) bool { return s == "" }), "\n")
		if m.diffRight {
			column = lipgloss.JoinHorizontal(lipgloss.Top, column, m.panelView(diffID))
		}
		sections = append(sections, column)
	}

	content := strings.Join(slices.DeleteFunc(sections, func(s string) bool { return s == "" }), "\n")
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

func blank(w, h int) string {
	if h <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", max(w, 0))+"\n", h), "\n")
}

// footer shows the keys that do something in the focused panel, most
// useful first. When they don't fit, the least useful are dropped, but
// quit always stays. Moving up and down, and refresh (everything refreshes
// on its own), are left out to save room.
func (m Model) footer() string {
	k := m.cfg.Keys
	first := func(keys []string) string {
		if len(keys) == 0 {
			return ""
		}
		return keys[0]
	}
	pair := func(a, b []string) string {
		if first(a) == "" || first(b) == "" {
			return ""
		}
		return first(a) + "/" + first(b)
	}
	type hint struct{ key, action string }
	var hints []hint
	switch m.focus {
	case filesID:
		otherView := "flat"
		if m.files.flat {
			otherView = "tree"
		}
		action := "fold"
		if m.cfg.Panels.Diff.Show {
			action = "fold/diff"
		}
		hints = append(hints, hint{first(k.ToggleFolder), action}, hint{first(k.ToggleView), otherView})
	case diffID:
		if !m.diff.wraps() {
			hints = append(hints, hint{pair(k.ScrollLeft, k.ScrollRight), "scroll"})
		}
		if m.cfg.Panels.Files.Show {
			hints = append(hints, hint{first(k.Back), "back"})
		}
	}
	zoom := "zoom"
	if m.zoomed {
		zoom = "unzoom"
	}
	hints = append(hints, hint{first(k.Zoom), zoom})
	if len(m.shownPanels()) > 1 {
		hints = append(hints, hint{first(k.NextPanel), "panel"})
	}
	if m.worktrees.showTabs() {
		hints = append(hints, hint{pair(k.PrevWorktree, k.NextWorktree), "worktree"})
	}

	var parts []string
	for _, h := range hints {
		if h.key != "" {
			parts = append(parts, h.key+" "+h.action)
		}
	}
	quit := ""
	if q := first(k.Quit); q != "" {
		quit = q + " quit"
	}
	line := func() string {
		all := parts
		if quit != "" {
			all = append(slices.Clip(parts), quit)
		}
		return " " + strings.Join(all, " · ")
	}
	for len(parts) > 0 && lipgloss.Width(line()) > m.width {
		parts = parts[:len(parts)-1]
	}
	return m.st.muted.Render(ansi.Truncate(line(), m.width, "…"))
}
