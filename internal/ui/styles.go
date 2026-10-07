package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/myounger/overlook/internal/config"
)

type styles struct {
	border       color.Color
	activeBorder color.Color
	title        lipgloss.Style
	repo         lipgloss.Style
	branch       lipgloss.Style
	worktree     lipgloss.Style
	upstream     lipgloss.Style
	inSync       lipgloss.Style
	muted        lipgloss.Style
	errText      lipgloss.Style
}

func newStyles(t config.Theme) styles {
	fg := func(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(colorOf(c)) }
	return styles{
		border:       colorOf(t.Border),
		activeBorder: colorOf(t.ActiveBorder),
		title:        fg(t.Title).Bold(true),
		repo:         fg(t.Repo).Bold(true),
		branch:       fg(t.Branch),
		worktree:     fg(t.Worktree),
		upstream:     fg(t.Upstream),
		inSync:       fg(t.InSync),
		muted:        fg(t.Muted),
		errText:      lipgloss.NewStyle().Foreground(lipgloss.Red),
	}
}

// colorOf maps "" to the terminal's default color; anything else goes
// through lipgloss.Color (ANSI number or hex).
func colorOf(s string) color.Color {
	if s == "" {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(s)
}
