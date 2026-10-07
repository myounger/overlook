package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// renderPanel draws body inside a rounded border of exactly width x height
// cells, with the title set into the top edge. Lines that are too long are
// cut off with "…" rather than wrapped, and extra lines are dropped.
func renderPanel(st styles, title, body string, width, height int, active bool) string {
	if width < 4 || height < 2 {
		return ""
	}
	borderColor := st.border
	if active {
		borderColor = st.activeBorder
	}
	b := lipgloss.RoundedBorder()
	edge := lipgloss.NewStyle().Foreground(borderColor)

	inner := width - 2
	label := ansi.Truncate(" "+title+" ", inner-1, "…")
	fill := inner - 1 - lipgloss.Width(label)
	top := edge.Render(b.TopLeft+b.Top) + st.title.Render(label) +
		edge.Render(strings.Repeat(b.Top, fill)+b.TopRight)

	const padX = 1
	textWidth := max(inner-2*padX, 0)
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, textWidth, "…")
	}
	box := lipgloss.NewStyle().
		Border(b, false, true, true, true).
		BorderForeground(borderColor).
		Padding(0, padX).
		Width(width).
		Height(height - 1).
		MaxHeight(height - 1).
		Render(strings.Join(lines, "\n"))

	return top + "\n" + box
}

// withBg returns a function that adds bg to a style. A selected row applies
// it to every piece so the highlight runs edge to edge; one outer style
// can't, because each inner color code resets the background.
func withBg(bg color.Color) func(lipgloss.Style) lipgloss.Style {
	return func(s lipgloss.Style) lipgloss.Style {
		if bg != nil {
			return s.Background(bg)
		}
		return s
	}
}

// fitRow cuts or pads a styled line to exactly w cells; padding uses fill.
func fitRow(line string, w int, fill lipgloss.Style) string {
	line = ansi.Truncate(line, w, "")
	if pad := w - lipgloss.Width(line); pad > 0 {
		line += fill.Render(strings.Repeat(" ", pad))
	}
	return line
}

// truncateLeft shortens plain text to w cells by cutting from the left, so
// the end (a file name, a branch's last segment) stays readable.
func truncateLeft(s string, w int) string {
	if n := lipgloss.Width(s); w > 0 && n > w {
		return ansi.TruncateLeft(s, n-w+1, "…")
	}
	return s
}
