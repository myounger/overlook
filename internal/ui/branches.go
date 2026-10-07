package ui

import (
	"cmp"
	"fmt"
	"image/color"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

// branchesPanel lists local branches, the one you're on first. The cursor
// follows its branch across refreshes.
type branchesPanel struct {
	cfg      config.BranchesPanel
	repoRoot string
	branches []git.Branch
	err      error
	listView
}

func newBranchesPanel(cfg config.BranchesPanel, repoRoot string) branchesPanel {
	return branchesPanel{cfg: cfg, repoRoot: repoRoot}
}

func (p *branchesPanel) setBranches(branches []git.Branch, err error) {
	p.err = err
	if err != nil {
		return
	}
	selected := ""
	if p.cursor < len(p.branches) {
		selected = p.branches[p.cursor].Name
	}
	slices.SortStableFunc(branches, func(a, b git.Branch) int {
		if a.Current != b.Current {
			if a.Current {
				return -1
			}
			return 1
		}
		if p.cfg.Sort == "recent" {
			if c := b.Committed.Compare(a.Committed); c != 0 {
				return c
			}
		}
		return cmp.Compare(a.Name, b.Name)
	})
	p.branches = branches
	p.setCount(len(branches))
	for i, b := range branches {
		if b.Name == selected {
			p.setCursor(i)
		}
	}
}

func (p *branchesPanel) title() string {
	if len(p.branches) == 0 {
		return "Branches"
	}
	return fmt.Sprintf("Branches · %d", len(p.branches))
}

func (p *branchesPanel) view(st styles, active bool, now time.Time) string {
	textW := p.width - 4
	switch {
	case p.err != nil:
		return renderPanel(st, p.title(), st.errText.Render(p.err.Error()), p.width, p.height, active)
	case len(p.branches) == 0 || textW < 1:
		return renderPanel(st, p.title(), st.muted.Render("No branches yet"), p.width, p.height, active)
	}

	start, end := p.visible()
	ages := make([]string, end-start)
	ageW := 0
	if p.cfg.Age {
		for i := range ages {
			ages[i] = relativeAge(p.branches[start+i].Committed, now)
			ageW = max(ageW, len(ages[i]))
		}
	}
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		var bg color.Color
		if active && i == p.cursor {
			bg = st.selectedBg
		}
		lines = append(lines, p.renderRow(st, p.branches[i], ages[i-start], ageW, textW, bg))
	}
	return renderPanel(st, p.title(), strings.Join(lines, "\n"), p.width, p.height, active)
}

// renderRow draws one branch:
//
//	2h * first-panels ↑1
//	3d   fix-login ⎇ fix-login merged
func (p *branchesPanel) renderRow(st styles, b git.Branch, age string, ageW, textW int, bg color.Color) string {
	on := withBg(bg)
	plain := on(lipgloss.NewStyle())

	prefix, prefixW := "", 0
	if ageW > 0 {
		prefix = on(st.muted).Render(fmt.Sprintf("%*s", ageW, age)) + plain.Render(" ")
		prefixW = ageW + 1
	}
	marker, nameStyle := "  ", lipgloss.NewStyle()
	if b.Current {
		marker, nameStyle = "* ", st.currentBranch
	}
	prefix += on(nameStyle).Render(marker)
	prefixW += 2

	var tags []string
	if p.cfg.Upstream {
		switch {
		case b.Gone:
			tags = append(tags, on(st.gone).Render("gone"))
		case b.Upstream != "" && b.Ahead == 0 && b.Behind == 0:
			tags = append(tags, on(st.inSync).Render("✓"))
		case b.Upstream != "":
			tags = append(tags, on(st.upstream).Render(aheadBehind(b.Ahead, b.Behind)))
		}
	}
	if p.cfg.Worktree && b.Worktree != "" && filepath.Clean(b.Worktree) != filepath.Clean(p.repoRoot) {
		tags = append(tags, on(st.worktree).Render("⎇ "+filepath.Base(b.Worktree)))
	}
	if b.Merged {
		tags = append(tags, on(st.merged).Render("merged"))
	}
	suffix := ""
	if len(tags) > 0 {
		suffix = plain.Render(" ") + strings.Join(tags, plain.Render(" "))
	}

	name := truncateLeft(b.Name, textW-prefixW-lipgloss.Width(suffix))
	return fitRow(prefix+on(nameStyle).Render(name)+suffix, textW, plain)
}

func aheadBehind(ahead, behind int) string {
	var parts []string
	if ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", ahead))
	}
	if behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", behind))
	}
	return strings.Join(parts, " ")
}

// relativeAge is a compact "how long ago": 45s, 12m, 3h, 5d, 2w, 4mo, 1y.
func relativeAge(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := max(now.Sub(t), 0)
	day := 24 * time.Hour
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < day:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 14*day:
		return fmt.Sprintf("%dd", d/day)
	case d < 60*day:
		return fmt.Sprintf("%dw", d/(7*day))
	case d < 365*day:
		return fmt.Sprintf("%dmo", d/(30*day))
	default:
		return fmt.Sprintf("%dy", d/(365*day))
	}
}
