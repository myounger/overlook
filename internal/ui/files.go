package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

// filesPanel is the list of changed files, as a folder tree or a flat list.
// Folding and the cursor survive refreshes; the cursor follows its file.
type filesPanel struct {
	cfg       config.FilesPanel
	files     []git.File
	root      *treeNode
	rows      []row
	collapsed map[string]bool
	flat      bool
	listView
}

func newFilesPanel(cfg config.FilesPanel) filesPanel {
	return filesPanel{cfg: cfg, collapsed: map[string]bool{}, flat: cfg.View == "flat"}
}

func (p *filesPanel) setFiles(files []git.File) {
	p.files = files
	p.root = buildTree(files, p.cfg.CompactFolders)
	p.rebuild()
}

// rebuild recomputes the visible rows and puts the cursor back on the row
// it was on, or as close as it can get if that row is gone.
func (p *filesPanel) rebuild() {
	selected := p.selectedKey()
	if p.flat {
		p.rows = flatRows(p.files)
	} else {
		p.rows = treeRows(p.root, p.collapsed)
	}
	p.setCount(len(p.rows))
	p.selectKey(selected)
}

func (p *filesPanel) selectKey(key string) {
	for i, r := range p.rows {
		if r.node.key() == key {
			p.setCursor(i)
			return
		}
	}
}

func (p *filesPanel) selectedKey() string {
	if p.cursor < len(p.rows) {
		return p.rows[p.cursor].node.key()
	}
	return ""
}

// toggleFolder folds or unfolds the folder under the cursor.
func (p *filesPanel) toggleFolder() {
	if p.flat || p.cursor >= len(p.rows) || !p.rows[p.cursor].node.isDir() {
		return
	}
	path := p.rows[p.cursor].node.path
	p.collapsed[path] = !p.collapsed[path]
	p.rebuild()
}

// setAllFolded folds or unfolds every folder. When folding, the cursor
// moves to the top-level row that contained it.
func (p *filesPanel) setAllFolded(folded bool) {
	if p.root == nil || p.flat {
		return
	}
	clear(p.collapsed)
	top := ""
	if folded {
		for _, path := range folders(p.root) {
			p.collapsed[path] = true
		}
		for i := min(p.cursor, len(p.rows)-1); i >= 0; i-- {
			if p.rows[i].depth == 0 {
				top = p.rows[i].node.key()
				break
			}
		}
	}
	p.rebuild()
	if top != "" {
		p.selectKey(top)
	}
}

func (p *filesPanel) toggleView() {
	p.flat = !p.flat
	p.rebuild()
}

func (p *filesPanel) title() string {
	if len(p.files) == 0 {
		return "Files"
	}
	return fmt.Sprintf("Files · %d", len(p.files))
}

// view draws the visible rows inside the panel border.
func (p *filesPanel) view(st styles, active bool) string {
	textW := p.width - 4
	if len(p.rows) == 0 || textW < 1 {
		return renderPanel(st, p.title(), st.muted.Render("No changes"), p.width, p.height, active)
	}
	start, end := p.visible()
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		var bg color.Color
		if active && i == p.cursor {
			bg = st.selectedBg
		}
		lines = append(lines, p.renderRow(st, p.rows[i], textW, bg))
	}
	return renderPanel(st, p.title(), strings.Join(lines, "\n"), p.width, p.height, active)
}

// renderRow draws one row exactly textW cells wide.
func (p *filesPanel) renderRow(st styles, r row, textW int, bg color.Color) string {
	on := withBg(bg)
	plain := on(lipgloss.NewStyle())
	n := r.node
	indent := strings.Repeat("  ", r.depth)

	var prefix, name, suffix string
	var prefixW int
	var nameStyle lipgloss.Style
	if n.isDir() {
		arrow := "▼"
		if p.collapsed[n.path] {
			arrow = "▶"
			suffix = on(st.muted).Render(fmt.Sprintf(" (%d)", n.files))
		}
		prefix = plain.Render(indent) + on(st.folder).Render(arrow) + plain.Render(" ")
		prefixW = len(indent) + 2
		name, nameStyle = n.name, st.folder
	} else {
		f := n.file
		// Untracked "??" isn't staged, so both letters get the unstaged color.
		stagedStyle := st.staged
		if f.Untracked() {
			stagedStyle = st.unstaged
		}
		prefix = plain.Render(indent) + on(stagedStyle).Render(statusChar(f.Staged)) +
			on(st.unstaged).Render(statusChar(f.Unstaged)) + plain.Render(" ")
		prefixW = len(indent) + 3
		name, nameStyle = n.name, st.kindStyle(f.Kind())
		if f.OrigPath != "" && p.flat {
			name = f.OrigPath + " → " + f.Path
		}
	}

	// Cut long names from the left so the file name and extension stay.
	name = truncateLeft(name, textW-prefixW-lipgloss.Width(suffix))
	return fitRow(prefix+on(nameStyle).Render(name)+suffix, textW, plain)
}

// statusChar shows git's "." (unchanged) as a blank, like lazygit.
func statusChar(c byte) string {
	if c == '.' {
		return " "
	}
	return string(c)
}
