package ui

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
)

// diffLine is one line of a diff: plain text plus the style to draw it in.
// Keeping them apart lets long lines wrap at any width and stay colored.
// A pager's lines carry their own colors and have a zero style.
type diffLine struct {
	style  lipgloss.Style
	text   string
	gutter bool // starts with a +, -, or space column that wrapped rows keep clear
	rule   bool // a divider: drawn as "── text ─────" across the panel
}

// diffPanel shows the diff of whatever is selected in Files. Long lines
// wrap, or with wrap off, the view scrolls sideways.
type diffPanel struct {
	cfg    config.DiffPanel
	key    string // selection the content belongs to; "" when nothing is selected
	path   string
	raw    string // last diff text, to skip re-rendering when nothing changed
	lines  []diffLine
	adds   int
	dels   int
	note   string // shown instead of lines, such as "No changes"
	offset int    // first visible row of rows()
	xoff   int
	width  int
	height int

	// rows caches lines laid out for rowsWidth.
	cache     []string
	cacheW    int
	cacheWide int // widest row, for limiting sideways scroll
}

func newDiffPanel(cfg config.DiffPanel) diffPanel {
	return diffPanel{cfg: cfg}
}

func (p *diffPanel) setSize(width, height int) {
	p.width, p.height = width, height
	p.clamp()
}

// selectTarget points the panel at a new selection, starting from the top.
// It reports whether the selection changed.
func (p *diffPanel) selectTarget(key, path string) bool {
	if key == p.key {
		return false
	}
	p.key, p.path = key, path
	p.setLines(nil)
	p.adds, p.dels, p.offset, p.xoff = 0, 0, 0, 0
	p.note = "loading…"
	if key == "" {
		p.note = "Select a file to see its changes"
	}
	return true
}

// setDiff shows a loaded diff. paged is the pager's output, if one is set;
// a note replaces the diff with a message.
func (p *diffPanel) setDiff(st styles, key, raw, paged, note string, err error, multiFile bool) {
	if key != p.key {
		return
	}
	if err != nil {
		note = err.Error()
	}
	if note != "" {
		p.setLines(nil)
		p.adds, p.dels, p.note = 0, 0, note
		return
	}
	if raw == p.raw && p.lines != nil {
		return
	}
	lines, adds, dels := renderDiff(st, raw, multiFile, p.cfg.HunkHeaders == "git", p.cfg.MaxLines)
	if p.cfg.Pager != "" {
		lines = pagedLines(paged, p.cfg.MaxLines)
	}
	p.setLines(lines)
	p.raw, p.adds, p.dels, p.note = raw, adds, dels, ""
	if len(lines) == 0 {
		p.note = "No changes"
	}
}

func (p *diffPanel) setLines(lines []diffLine) {
	p.lines, p.raw, p.cache, p.cacheW = lines, "", nil, 0
	p.clamp()
}

// wraps reports whether long lines wrap. A pager lays out its own lines.
func (p *diffPanel) wraps() bool { return p.cfg.Wrap && p.cfg.Pager == "" }

// rows lays the lines out for the current width: wrapped, or one row per
// line to be cut sideways.
func (p *diffPanel) rows() []string {
	textW := max(p.width-4, 1)
	if p.cache != nil && p.cacheW == textW {
		return p.cache
	}
	p.cache, p.cacheW, p.cacheWide = p.cache[:0], textW, 0
	for _, l := range p.lines {
		switch {
		case l.rule:
			p.cache = append(p.cache, l.style.Render(ruleLine(l.text, textW)))
		case !p.wraps() || lipgloss.Width(l.text) <= textW:
			p.cache = append(p.cache, l.style.Render(l.text))
			p.cacheWide = max(p.cacheWide, lipgloss.Width(l.text))
		case l.gutter && textW > 1:
			// Wrap after the +/- column; continuation rows leave it blank.
			for i, part := range wrapWords(l.text[1:], textW-1) {
				mark := " "
				if i == 0 {
					mark = l.text[:1]
				}
				p.cache = append(p.cache, l.style.Render(mark+part))
			}
		default:
			for _, part := range wrapWords(l.text, textW) {
				p.cache = append(p.cache, l.style.Render(part))
			}
		}
	}
	return p.cache
}

func (p *diffPanel) pageSize() int { return max(p.height-2, 1) }

func (p *diffPanel) scroll(dy int) {
	p.offset += dy
	p.clamp()
}

func (p *diffPanel) scrollX(dx int) {
	if p.wraps() {
		return
	}
	p.xoff = max(p.xoff+dx, 0)
	p.clamp()
}

func (p *diffPanel) clamp() {
	rows := p.rows()
	p.offset = max(min(p.offset, len(rows)-p.pageSize()), 0)
	p.xoff = max(min(p.xoff, p.cacheWide-(p.width-4)), 0)
}

func (p *diffPanel) title() string {
	if p.path == "" {
		return "Diff"
	}
	t := "Diff · " + p.path
	if p.adds > 0 {
		t += fmt.Sprintf(" +%d", p.adds)
	}
	if p.dels > 0 {
		t += fmt.Sprintf(" -%d", p.dels)
	}
	return t
}

func (p *diffPanel) view(st styles, active bool) string {
	textW := p.width - 4
	title := truncateLeft(p.title(), textW)
	rows := p.rows()
	if len(rows) == 0 || textW < 1 {
		return renderPanel(st, title, st.muted.Render(p.note), p.width, p.height, active)
	}
	end := min(p.offset+p.pageSize(), len(rows))
	out := make([]string, 0, end-p.offset)
	for _, r := range rows[p.offset:end] {
		out = append(out, ansi.Cut(r, p.xoff, p.xoff+textW))
	}
	return renderPanel(st, title, strings.Join(out, "\n"), p.width, p.height, active)
}

// renderDiff colors a diff line by line and counts added and removed lines.
// Each file's header block (diff --git, index, ---/+++) becomes one bold
// line naming the file, shown only when the diff covers several files.
// Notes like "binary file" or a rename with no content changes are kept.
func renderDiff(st styles, raw string, multiFile, gitHunks bool, maxLines int) (lines []diffLine, adds, dels int) {
	type header struct{ name, from, kind string }
	var hdr *header
	var notes []string
	flush := func() {
		if hdr == nil {
			return
		}
		if multiFile {
			label := hdr.name
			if hdr.from != "" && hdr.from != hdr.name {
				label = hdr.from + " → " + hdr.name
			}
			if hdr.kind != "" {
				label += " (" + hdr.kind + ")"
			}
			if len(lines) > 0 {
				lines = append(lines, diffLine{})
			}
			lines = append(lines, diffLine{style: st.diffFile, text: label})
		}
		for _, n := range notes {
			lines = append(lines, diffLine{style: st.muted, text: n})
		}
		hdr, notes = nil, nil
	}

	all := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	for _, l := range all {
		l = strings.ReplaceAll(strings.TrimRight(l, "\r"), "\t", "    ")
		switch {
		case strings.HasPrefix(l, "diff --git "):
			flush()
			from, name := diffGitPaths(l)
			hdr = &header{name: name, from: from}
			continue
		case hdr != nil && !strings.HasPrefix(l, "@@"):
			switch {
			case strings.HasPrefix(l, "new file mode"):
				hdr.kind = "new"
			case strings.HasPrefix(l, "deleted file mode"):
				hdr.kind = "deleted"
			case strings.HasPrefix(l, "rename from "):
				hdr.from = strings.TrimPrefix(l, "rename from ")
				hdr.kind = "renamed"
				notes = append(notes, "renamed from "+hdr.from)
			case strings.HasPrefix(l, "rename to "):
				hdr.name = strings.TrimPrefix(l, "rename to ")
			case strings.HasPrefix(l, "Binary files"):
				notes = append(notes, "binary file changed")
			case strings.HasPrefix(l, "old mode"), strings.HasPrefix(l, "new mode"):
				notes = append(notes, l)
			}
			continue
		}
		flush()
		switch {
		case strings.HasPrefix(l, "@@"):
			if label, ok := hunkLabel(l); ok && !gitHunks {
				lines = append(lines, diffLine{style: st.diffHunk, text: label, rule: true})
			} else {
				lines = append(lines, diffLine{style: st.diffHunk, text: l})
			}
		case strings.HasPrefix(l, "+"):
			adds++
			lines = append(lines, diffLine{style: st.diffAdd, text: l, gutter: true})
		case strings.HasPrefix(l, "-"):
			dels++
			lines = append(lines, diffLine{style: st.diffDelete, text: l, gutter: true})
		case strings.HasPrefix(l, `\`):
			lines = append(lines, diffLine{style: st.muted, text: l})
		case l == "" && len(lines) == 0:
		case strings.HasPrefix(l, " "):
			lines = append(lines, diffLine{text: l, gutter: true})
		default:
			lines = append(lines, diffLine{text: l})
		}
	}
	flush()
	return capLines(st, lines, maxLines), adds, dels
}

// diffGitPaths pulls the old and new paths out of "diff --git a/X b/Y".
func diffGitPaths(l string) (from, to string) {
	rest := strings.TrimPrefix(l, "diff --git ")
	if i := strings.Index(rest, " b/"); i >= 0 {
		return strings.TrimPrefix(rest[:i], "a/"), rest[i+3:]
	}
	return "", rest
}

// pagedLines splits a pager's output into lines, keeping its colors.
func pagedLines(out string, maxLines int) []diffLine {
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return nil
	}
	var lines []diffLine
	for _, l := range strings.Split(out, "\n") {
		lines = append(lines, diffLine{text: strings.ReplaceAll(strings.TrimRight(l, "\r"), "\t", "    ")})
	}
	return capLines(styles{}, lines, maxLines)
}

func capLines(st styles, lines []diffLine, maxLines int) []diffLine {
	if maxLines > 0 && len(lines) > maxLines {
		more := len(lines) - maxLines
		lines = append(lines[:maxLines:maxLines], diffLine{style: st.muted, text: fmt.Sprintf("… %d more lines (panels.diff.maxLines)", more)})
	}
	return lines
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@ ?(.*)$`)

// hunkLabel turns "@@ -30,6 +30,10 @@ theme:" into "line 30 · theme:": where
// the block starts in the new file (or the old one, if the block removes
// everything), plus git's hint about the enclosing section.
func hunkLabel(l string) (string, bool) {
	m := hunkHeader.FindStringSubmatch(l)
	if m == nil {
		return "", false // e.g. a merge conflict's "@@@" header
	}
	line := m[3]
	if m[4] == "0" {
		line = m[1]
	}
	label := "line " + line
	if hint := strings.TrimSpace(m[5]); hint != "" {
		label += " · " + hint
	}
	return label, true
}

// ruleLine draws "── text ─────" exactly w cells wide.
func ruleLine(text string, w int) string {
	line := ansi.Truncate("── "+text+" ", w, "… ")
	if fill := w - lipgloss.Width(line); fill > 0 {
		line += strings.Repeat("─", fill)
	}
	return line
}

// wrapWords breaks plain text into rows at most w cells wide, at the last
// space that fits. It never breaks inside the leading indentation, and cuts
// a word only when it's longer than a whole row. Continuation rows get the
// first row's indentation (up to half the width), like an editor's wrapping.
func wrapWords(s string, w int) []string {
	indent := strings.Repeat(" ", min(len(s)-len(strings.TrimLeft(s, " ")), w/2))
	var rows []string
	for lipgloss.Width(s) > w {
		fit := ansi.Truncate(s, w, "")
		lead := len(fit) - len(strings.TrimLeft(fit, " "))
		if s[len(fit)] == ' ' && len(fit) > lead {
			// A word ends exactly at the edge.
			rows = append(rows, strings.TrimRight(fit, " "))
			s = s[len(fit):]
		} else if i := strings.LastIndex(fit, " "); i > lead {
			rows = append(rows, strings.TrimRight(fit[:i], " "))
			s = s[i+1:]
		} else {
			rows = append(rows, fit)
			s = s[len(fit):]
		}
		s = indent + strings.TrimLeft(s, " ")
	}
	return append(rows, s)
}
