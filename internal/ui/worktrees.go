package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

// worktreeSet tracks the repo's working trees between refreshes and decides
// when to switch to a different one.
type worktreeSet struct {
	cfg      config.Worktrees
	trees    []git.Worktree // usable ones (not prunable), main first
	activity map[string]git.Activity
	seen     map[string]bool // paths in the previous list; nil before the first one
	claude   map[string]bool // paths where a Claude Code session is working
}

func newWorktreeSet(cfg config.Worktrees) worktreeSet {
	return worktreeSet{cfg: cfg, activity: map[string]git.Activity{}}
}

// enabled reports whether Overlook needs the worktree list at all.
func (s *worktreeSet) enabled() bool {
	return s.cfg.Tabs || s.cfg.AutoSwitch || s.cfg.Follow
}

// update takes a fresh worktree list plus activity for the ones that aren't
// active, and returns the path to switch to, or "" to stay put.
//
//   - A worktree that wasn't there last time wins, if autoSwitch is on.
//   - With follow on, so does one whose files changed since last time.
//   - If the active worktree is gone, fall back to the main one.
func (s *worktreeSet) update(list []git.Worktree, acts map[string]git.Activity, active string) string {
	s.trees = s.trees[:0]
	for _, w := range list {
		if !w.Prunable {
			s.trees = append(s.trees, w)
		}
	}
	first := s.seen == nil

	target := ""
	for _, w := range s.trees {
		if w.Path == active {
			continue
		}
		act, ok := acts[w.Path]
		prev, hadPrev := s.activity[w.Path]
		switch {
		case first:
		case !s.seen[w.Path] && s.cfg.AutoSwitch:
			target = w.Path
		case s.cfg.Follow && ok && hadPrev && act.Fingerprint != prev.Fingerprint && target == "":
			target = w.Path
		}
		if ok {
			s.activity[w.Path] = act
		}
	}
	s.seen = map[string]bool{}
	for _, w := range s.trees {
		s.seen[w.Path] = true
	}
	for path := range s.activity {
		if !s.seen[path] {
			delete(s.activity, path)
		}
	}

	if target == "" && !s.has(active) && len(s.trees) > 0 {
		target = s.trees[0].Path
	}
	return target
}

func (s *worktreeSet) has(path string) bool {
	return s.index(path) >= 0
}

func (s *worktreeSet) index(path string) int {
	for i, w := range s.trees {
		if w.Path == path {
			return i
		}
	}
	return -1
}

// setActive records activity for the worktree on screen, which comes from
// the main status refresh rather than the worktree list.
func (s *worktreeSet) setActive(path string, act git.Activity) {
	s.activity[path] = act
}

// neighbor returns the worktree step places from active, wrapping around.
func (s *worktreeSet) neighbor(active string, step int) string {
	n := len(s.trees)
	if n < 2 {
		return ""
	}
	i := max(s.index(active), 0)
	return s.trees[((i+step)%n+n)%n].Path
}

func (s *worktreeSet) showTabs() bool {
	return s.cfg.Tabs && len(s.trees) > 1
}

// tab is one worktree tab as placed on the tab row.
type tab struct {
	path   string
	label  string
	x      int
	active bool
}

// tabs places one tab per worktree, scrolled so the active one is visible.
// scrolled reports whether tabs are hidden off the left edge, which a "‹"
// marker in the first cell shows.
func (s *worktreeSet) tabs(repoName, active string, width int) (tabs []tab, scrolled bool) {
	all := make([]tab, len(s.trees))
	activeIdx := 0
	for i, w := range s.trees {
		label := w.Name(repoName)
		if s.claude[w.Path] && s.cfg.FollowClaude {
			label += " ✻"
		}
		if act, ok := s.activity[w.Path]; ok && s.cfg.Counts && act.Changed > 0 {
			label += fmt.Sprintf(" %d", act.Changed)
		}
		all[i] = tab{path: w.Path, label: " " + label + " ", active: w.Path == active}
		if all[i].active {
			activeIdx = i
		}
	}

	// Start far enough right that everything up to the active tab fits.
	start, used := activeIdx, lipgloss.Width(all[activeIdx].label)
	for start > 0 && used+1+lipgloss.Width(all[start-1].label)+2 <= width {
		start--
		used += 1 + lipgloss.Width(all[start].label)
	}
	scrolled = start > 0
	x := 0
	if scrolled {
		x = 2 // "‹ "
	}
	for _, t := range all[start:] {
		if x >= width {
			break
		}
		t.x = x
		tabs = append(tabs, t)
		x += lipgloss.Width(t.label) + 1
	}
	return tabs, scrolled
}

// view draws the tab row.
func (s *worktreeSet) view(st styles, repoName, active string, width int) string {
	tabs, scrolled := s.tabs(repoName, active, width)
	var b strings.Builder
	if scrolled {
		b.WriteString(st.muted.Render("‹") + " ")
	}
	for i, t := range tabs {
		if i > 0 {
			b.WriteString(" ")
		}
		style := st.tab
		if t.active {
			style = st.activeTab
		}
		b.WriteString(style.Render(t.label))
	}
	return ansi.Truncate(b.String(), width, "›")
}

// tabAt returns the worktree whose tab is at column x, or "".
func (s *worktreeSet) tabAt(repoName, active string, width, x int) string {
	tabs, _ := s.tabs(repoName, active, width)
	for _, t := range tabs {
		if x >= t.x && x < t.x+lipgloss.Width(t.label) {
			return t.path
		}
	}
	return ""
}
