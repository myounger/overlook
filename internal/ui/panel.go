package ui

import (
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
