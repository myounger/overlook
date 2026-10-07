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

// view draws one tab per worktree, scrolled so the active tab is visible.
func (s *worktreeSet) view(st styles, repoName, active string, width int) string {
	labels := make([]string, len(s.trees))
	activeIdx := 0
	for i, w := range s.trees {
		label := w.Name(repoName)
		if act, ok := s.activity[w.Path]; ok && s.cfg.Counts && act.Changed > 0 {
			label += fmt.Sprintf(" %d", act.Changed)
		}
		labels[i] = " " + label + " "
		if w.Path == active {
			activeIdx = i
		}
	}

	// Start far enough right that everything up to the active tab fits.
	start, used := activeIdx, lipgloss.Width(labels[activeIdx])
	for start > 0 && used+1+lipgloss.Width(labels[start-1])+2 <= width {
		start--
		used += 1 + lipgloss.Width(labels[start])
	}

	var b strings.Builder
	if start > 0 {
		b.WriteString(st.muted.Render("‹"))
	}
	for i := start; i < len(labels); i++ {
		if i > start || start > 0 {
			b.WriteString(" ")
		}
		style := st.tab
		if i == activeIdx {
			style = st.activeTab
		}
		b.WriteString(style.Render(labels[i]))
	}
	return ansi.Truncate(b.String(), width, "›")
}
