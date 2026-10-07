package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

func sized(t *testing.T, cfg config.Config, w, h int) Model {
	t.Helper()
	var m tea.Model = New(cfg, git.Repo{Root: "/r", GitDir: "/r/.git", CommonDir: "/r/.git"}, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m.(Model)
}

func heights(m Model) (files, diff, branches int) {
	return m.rects[filesID].h, m.rects[diffID].h, m.rects[branchesID].h
}

func TestExpandFocused(t *testing.T) {
	cfg, _ := config.Load("")
	m := sized(t, cfg, 60, 40) // narrow: files, diff, branches stacked under status
	// 40 rows - footer 1 - status 3 = 36, split 2:3:1 as 12, 18, 6. Branches
	// collapses to 3 rows + border and gives its spare row to Files.
	if f, d, b := heights(m); f != 13 || d != 18 || b != 5 {
		t.Errorf("files focused: files=%d diff=%d branches=%d; want 13, 18, 5", f, d, b)
	}
	f0, d0, _ := heights(m)

	m.cycleFocus(1) // to diff: the layout shouldn't change
	if f, d, b := heights(m); f != f0 || d != d0 || b != 5 {
		t.Errorf("diff focused: files=%d diff=%d branches=%d; want unchanged", f, d, b)
	}
	m.cycleFocus(1) // to branches: it expands, files collapses, the diff stays
	if f, d, b := heights(m); f != 5 || d != d0 || b != 13 {
		t.Errorf("branches focused: files=%d diff=%d branches=%d; want 5, %d, 13", f, d, b, d0)
	}

	// Mouse and layout code rely on panels tiling the column exactly.
	y := m.rects[statusID].y + statusHeight
	for _, id := range m.stackedPanels() {
		if m.rects[id].y != y {
			t.Errorf("panel %d at y=%d, want %d", id, m.rects[id].y, y)
		}
		y += m.rects[id].h
	}
}

func TestExpandFocusedOff(t *testing.T) {
	cfg, _ := config.Load("")
	cfg.Layout.ExpandFocused = false
	m := sized(t, cfg, 60, 40)
	before := m.rects
	m.cycleFocus(2)
	if m.rects[filesID] != before[filesID] || m.rects[branchesID] != before[branchesID] {
		t.Error("layout changed with focus while expandFocused is off")
	}
}

func TestExpandFocusedTooShort(t *testing.T) {
	cfg, _ := config.Load("")
	cfg.Layout.CollapsedRows = 30
	m := sized(t, cfg, 60, 20)
	f, d, b := heights(m)
	if f+d+b != 16 || b >= 16 {
		t.Errorf("files=%d diff=%d branches=%d; collapsing should give way when it doesn't fit", f, d, b)
	}
}
