package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

// logPanel lists recent commits on the current branch, newest first. The
// cursor follows its commit across refreshes.
type logPanel struct {
	cfg     config.LogPanel
	commits []git.Commit
	err     error
	listView
}

func newLogPanel(cfg config.LogPanel) logPanel {
	return logPanel{cfg: cfg}
}

func (p *logPanel) setCommits(commits []git.Commit, err error) {
	p.err = err
	if err != nil {
		return
	}
	selected := ""
	if c, ok := p.selected(); ok {
		selected = c.Hash
	}
	p.commits = commits
	p.setCount(len(commits))
	for i, c := range commits {
		if c.Hash == selected {
			p.setCursor(i)
		}
	}
}

func (p *logPanel) selected() (git.Commit, bool) {
	if p.cursor < len(p.commits) {
		return p.commits[p.cursor], true
	}
	return git.Commit{}, false
}

// title counts unpushed commits, since that's the number worth knowing.
func (p *logPanel) title() string {
	unpushed := 0
	for _, c := range p.commits {
		if c.Unpushed {
			unpushed++
		}
	}
	if unpushed > 0 {
		return fmt.Sprintf("Log · ↑%d", unpushed)
	}
	return "Log"
}

func (p *logPanel) view(st styles, active bool, now time.Time) string {
	textW := p.width - 4
	switch {
	case p.err != nil:
		return renderPanel(st, p.title(), st.errText.Render(p.err.Error()), p.width, p.height, active)
	case len(p.commits) == 0 || textW < 1:
		return renderPanel(st, p.title(), st.muted.Render("No commits yet"), p.width, p.height, active)
	}

	start, end := p.visible()
	ages := make([]string, end-start)
	ageW := 0
	if p.cfg.Age {
		for i := range ages {
			ages[i] = relativeAge(p.commits[start+i].Time, now)
			ageW = max(ageW, len(ages[i]))
		}
	}
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		var bg color.Color
		if active && i == p.cursor {
			bg = st.selectedBg
		}
		lines = append(lines, p.renderRow(st, p.commits[i], ages[i-start], ageW, textW, bg))
	}
	return renderPanel(st, p.title(), strings.Join(lines, "\n"), p.width, p.height, active)
}

// renderRow draws one commit:
//
//	2h ↑ a1b2c3d Fix the login redirect
//	3d   9f8e7d6 Add the Files panel
func (p *logPanel) renderRow(st styles, c git.Commit, age string, ageW, textW int, bg color.Color) string {
	on := withBg(bg)
	plain := on(lipgloss.NewStyle())
	line := ""
	if ageW > 0 {
		line = on(st.muted).Render(fmt.Sprintf("%*s", ageW, age)) + plain.Render(" ")
	}
	mark := " "
	if c.Unpushed {
		mark = "↑"
	}
	line += on(st.upstream).Render(mark) + plain.Render(" ") + on(st.hash).Render(c.Short) + plain.Render(" ")
	// Subjects are cut at the end: their start says the most.
	subject := ansi.Truncate(c.Subject, max(textW-lipgloss.Width(line), 0), "…")
	return fitRow(line+plain.Render(subject), textW, plain)
}
