package ui

import (
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const statusHeight = 3

// rect is a panel's place on screen, in cells.
type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// layout sizes and places every panel. From the top: worktree tabs, then a
// column of Status and the stacked panels, which share the height in
// proportion to their sizes. The diff either sits right of that column or
// joins the stack. Zoomed, the focused panel gets the whole body.
func (m *Model) layout() {
	top, body := 0, m.height
	if m.worktrees.showTabs() {
		top, body = 1, body-1
	}
	if m.cfg.Layout.Footer {
		body--
	}
	body = max(body, 0)
	m.rects = map[panelID]rect{}

	if m.zoomed {
		m.place(m.focus, rect{0, top, m.width, body})
		return
	}

	d := m.cfg.Panels.Diff
	m.diffRight = d.Show && (d.Position == "right" || d.Position == "auto" && m.width >= d.RightMinWidth)
	colW := m.width
	if m.diffRight {
		diffW := m.width * d.RightWidth / 100
		colW = m.width - diffW
		m.place(diffID, rect{colW, top, diffW, body})
	}

	y, stackH := top, body
	if m.cfg.Panels.Status.Show {
		m.rects[statusID] = rect{0, y, colW, statusHeight}
		y, stackH = y+statusHeight, max(stackH-statusHeight, 0)
	}
	stack := m.stackedPanels()
	for i, h := range m.stackHeights(stack, stackH) {
		m.place(stack[i], rect{0, y, colW, h})
		y += h
	}
}

// stackHeights splits the column's height between the stacked panels by
// their sizes. With layout.expandFocused, the list panel you're not in
// (Files or Branches) then shrinks to layout.collapsedRows rows, and the
// rows it gives up go to the list panel you're in, or were last in. Every
// other panel, the diff included, keeps its height either way.
func (m Model) stackHeights(stack []panelID, total int) []int {
	hs := make([]int, len(stack))
	weight := 0
	for _, id := range stack {
		weight += m.panelSize(id)
	}
	left := total
	for i, id := range stack {
		hs[i] = total * m.panelSize(id) / weight
		if i == len(stack)-1 {
			hs[i] = left
		}
		left -= hs[i]
	}

	if !m.cfg.Layout.ExpandFocused {
		return hs
	}
	expanded := slices.Index(stack, m.expandedList(stack))
	if expanded < 0 {
		return hs
	}
	collapsed := m.cfg.Layout.CollapsedRows + 2 // plus the border
	for i, id := range stack {
		if isList(id) && i != expanded && hs[i] > collapsed {
			hs[expanded] += hs[i] - collapsed
			hs[i] = collapsed
		}
	}
	return hs
}

// expandedList is the list panel that gets the room when the other one
// collapses: the one you're in or were last in.
func (m Model) expandedList(stack []panelID) panelID {
	if slices.Contains(stack, m.lastList) {
		return m.lastList
	}
	for _, id := range stack {
		if isList(id) {
			return id
		}
	}
	return m.lastList
}

func isList(id panelID) bool { return id == filesID || id == branchesID || id == logID }

// place records where a panel goes and sizes it to fit.
func (m *Model) place(id panelID, r rect) {
	m.rects[id] = r
	m.sizePanel(id, r.w, r.h)
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
	case logID:
		return m.cfg.Panels.Log.Size
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
	case logID:
		m.log.setSize(w, h)
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
	case logID:
		view, w, h = m.log.view(m.st, active, time.Now()), m.log.width, m.log.height
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
			switch {
			case m.msg.text != "" && !m.cfg.Layout.Footer:
				body = m.messageLine() // no footer to show it in
			case m.loaded:
				body = statusLine(m.st, m.cfg.Panels.Status, m.repo, m.status, m.statusErr)
			}
			col = append(col, renderPanel(m.st, "Status", body, m.rects[statusID].w, statusHeight, false))
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
	if m.msg.text != "" {
		return ansi.Truncate(" "+m.messageLine(), m.width, "…")
	}
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
	case logID:
		if m.cfg.Panels.Diff.Show {
			hints = append(hints, hint{first(k.ToggleFolder), "diff"})
		}
	case diffID:
		if !m.diff.wraps() {
			hints = append(hints, hint{pair(k.ScrollLeft, k.ScrollRight), "scroll"})
		}
		if m.cfg.Panels.Files.Show {
			hints = append(hints, hint{first(k.Back), "back"})
		}
	}
	if m.cfg.Pull.Enabled && m.status.Upstream != "" {
		hints = append(hints, hint{first(k.Pull), "pull"})
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

func (m Model) messageLine() string {
	switch m.msg.kind {
	case successMessage:
		return m.st.success.Render(m.msg.text)
	case errorMessage:
		return m.st.errText.Render(m.msg.text)
	default:
		return m.st.muted.Render(m.msg.text)
	}
}
