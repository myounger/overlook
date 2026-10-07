package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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
	cursor    int
	offset    int // first visible row
	width     int
	height    int
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
	p.cursor = min(p.cursor, max(len(p.rows)-1, 0))
	for i, r := range p.rows {
		if r.node.key() == selected {
			p.cursor = i
			break
		}
	}
	p.scroll()
}

func (p *filesPanel) selectedKey() string {
	if p.cursor < len(p.rows) {
		return p.rows[p.cursor].node.key()
	}
	return ""
}

func (p *filesPanel) setSize(width, height int) {
	p.width, p.height = width, height
	p.scroll()
}

// pageSize is how many rows fit inside the border.
func (p *filesPanel) pageSize() int { return max(p.height-2, 1) }

func (p *filesPanel) move(delta int) {
	p.cursor = max(min(p.cursor+delta, len(p.rows)-1), 0)
	p.scroll()
}

// scroll moves the window just enough to keep the cursor in view.
func (p *filesPanel) scroll() {
	h := p.pageSize()
	p.offset = min(p.offset, p.cursor)
	p.offset = max(p.offset, p.cursor-h+1)
	p.offset = max(min(p.offset, len(p.rows)-h), 0)
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
	for i, r := range p.rows {
		if r.node.key() == top {
			p.cursor = i
			p.scroll()
		}
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
	end := min(p.offset+p.pageSize(), len(p.rows))
	lines := make([]string, 0, end-p.offset)
	for i := p.offset; i < end; i++ {
		var bg color.Color
		if active && i == p.cursor {
			bg = st.selectedBg
		}
		lines = append(lines, p.renderRow(st, p.rows[i], textW, bg))
	}
	return renderPanel(st, p.title(), strings.Join(lines, "\n"), p.width, p.height, active)
}

// renderRow draws one row exactly textW cells wide. Every piece gets the
// background so a selected row is highlighted edge to edge; an outer style
// can't do that because each inner color code resets the background.
func (p *filesPanel) renderRow(st styles, r row, textW int, bg color.Color) string {
	on := func(s lipgloss.Style) lipgloss.Style {
		if bg != nil {
			return s.Background(bg)
		}
		return s
	}
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
	avail := textW - prefixW - lipgloss.Width(suffix)
	if w := lipgloss.Width(name); avail > 0 && w > avail {
		name = ansi.TruncateLeft(name, w-avail+1, "…")
	}
	line := prefix + on(nameStyle).Render(name) + suffix
	line = ansi.Truncate(line, textW, "")
	if pad := textW - lipgloss.Width(line); pad > 0 {
		line += plain.Render(strings.Repeat(" ", pad))
	}
	return line
}

// statusChar shows git's "." (unchanged) as a blank, like lazygit.
func statusChar(c byte) string {
	if c == '.' {
		return " "
	}
	return string(c)
}
