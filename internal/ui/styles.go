package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

type styles struct {
	border        color.Color
	activeBorder  color.Color
	title         lipgloss.Style
	repo          lipgloss.Style
	branch        lipgloss.Style
	worktree      lipgloss.Style
	upstream      lipgloss.Style
	inSync        lipgloss.Style
	muted         lipgloss.Style
	errText       lipgloss.Style
	selectedBg    color.Color
	folder        lipgloss.Style
	staged        lipgloss.Style
	unstaged      lipgloss.Style
	added         lipgloss.Style
	modified      lipgloss.Style
	deleted       lipgloss.Style
	renamed       lipgloss.Style
	conflicted    lipgloss.Style
	currentBranch lipgloss.Style
	gone          lipgloss.Style
	merged        lipgloss.Style
	tab           lipgloss.Style
	activeTab     lipgloss.Style
}

func newStyles(t config.Theme) styles {
	fg := func(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(colorOf(c)) }
	return styles{
		border:        colorOf(t.Border),
		activeBorder:  colorOf(t.ActiveBorder),
		title:         fg(t.Title).Bold(true),
		repo:          fg(t.Repo).Bold(true),
		branch:        fg(t.Branch),
		worktree:      fg(t.Worktree),
		upstream:      fg(t.Upstream),
		inSync:        fg(t.InSync),
		muted:         fg(t.Muted),
		errText:       lipgloss.NewStyle().Foreground(lipgloss.Red),
		selectedBg:    colorOf(t.SelectedBg),
		folder:        fg(t.Folder),
		staged:        fg(t.Staged),
		unstaged:      fg(t.Unstaged),
		added:         fg(t.Added),
		modified:      fg(t.Modified),
		deleted:       fg(t.Deleted),
		renamed:       fg(t.Renamed),
		conflicted:    fg(t.Conflicted),
		currentBranch: fg(t.CurrentBranch).Bold(true),
		gone:          fg(t.Gone),
		merged:        fg(t.Merged),
		tab:           fg(t.Tab),
		activeTab:     fg(t.ActiveTab).Bold(true).Background(colorOf(t.SelectedBg)),
	}
}

func (st styles) kindStyle(k git.Kind) lipgloss.Style {
	switch k {
	case git.Added:
		return st.added
	case git.Deleted:
		return st.deleted
	case git.Renamed:
		return st.renamed
	case git.Conflicted:
		return st.conflicted
	default:
		return st.modified
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
