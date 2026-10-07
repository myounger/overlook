package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

func mouseModel(t *testing.T) Model {
	t.Helper()
	cfg, _ := config.Load("")
	m := sized(t, cfg, 60, 40)
	files := []git.File{
		{Path: "src/a.go", Staged: '.', Unstaged: 'M'},
		{Path: "src/b.go", Staged: '.', Unstaged: 'M'},
		{Path: "z.txt", Staged: '?', Unstaged: '?'},
	}
	next, _ := m.Update(statusMsg{root: "/r", status: git.Status{Head: "main", Files: files}})
	next, _ = next.Update(branchesMsg{root: "/r", branches: []git.Branch{{Name: "main", Current: true}, {Name: "a"}, {Name: "b"}}})
	return next.(Model)
}

func click(m Model, x, y int) Model {
	next, _ := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	return next.(Model)
}

func wheel(m Model, x, y int, b tea.MouseButton) Model {
	next, _ := m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: b})
	return next.(Model)
}

func TestClickSelectsAndFocuses(t *testing.T) {
	m := mouseModel(t)
	// Files rows: 0 "src/", 1 a.go, 2 b.go, 3 z.txt.
	f := m.rects[filesID]
	m = click(m, 5, f.y+1+2)
	if m.focus != filesID || m.files.cursor != 2 {
		t.Fatalf("focus=%d cursor=%d; want files, row 2", m.focus, m.files.cursor)
	}
	// Clicking the selected file again opens its diff.
	m = click(m, 5, f.y+1+2)
	if m.focus != diffID {
		t.Errorf("second click: focus=%d, want diff", m.focus)
	}

	// Clicking a branch focuses Branches, which expands.
	b := m.rects[branchesID]
	m = click(m, 5, b.y+1+1)
	if m.focus != branchesID || m.branches.cursor != 1 || m.rects[branchesID].h <= b.h {
		t.Errorf("focus=%d cursor=%d height %d->%d", m.focus, m.branches.cursor, b.h, m.rects[branchesID].h)
	}
}

func TestClickFolderTwiceFolds(t *testing.T) {
	m := mouseModel(t)
	f := m.rects[filesID]
	m = click(m, 5, f.y+1) // "src/" is already selected
	if !m.files.collapsed["src"] {
		t.Error("clicking the selected folder didn't fold it")
	}
}

func TestClickOutsideRowsDoesNothing(t *testing.T) {
	m := mouseModel(t)
	f := m.rects[filesID]
	for _, y := range []int{f.y, f.y + f.h - 1, f.y + 1 + 3 + 1} { // borders, past the last row
		if got := click(m, 5, y); got.files.cursor != 0 {
			t.Errorf("click at y=%d moved the cursor to %d", y, got.files.cursor)
		}
	}
}

func TestWheelScrollsPanelUnderPointer(t *testing.T) {
	m := mouseModel(t)
	b := m.rects[branchesID]
	m = wheel(m, 5, b.y+2, tea.MouseWheelDown)
	if m.branches.cursor != 1 || m.focus != filesID {
		t.Errorf("wheel over branches: cursor=%d focus=%d; want 1 and focus unchanged", m.branches.cursor, m.focus)
	}
}

func TestMouseOff(t *testing.T) {
	m := mouseModel(t)
	m.cfg.Layout.Mouse = false
	f := m.rects[filesID]
	if got := click(m, 5, f.y+3); got.files.cursor != 0 {
		t.Error("click handled with mouse off")
	}
	if m.View().MouseMode != tea.MouseModeNone {
		t.Error("mouse reporting on with mouse off")
	}
}
