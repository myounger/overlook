package ui

import (
	"fmt"
	"strings"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

// statusLine renders the one-line summary: repo, worktree, branch, and how
// it compares to the remote branch.
//
//	overlook → main ✓
//	overlook ⎇ fix-login → fix-login ↑2 ↓1
func statusLine(st styles, cfg config.StatusPanel, repo git.Repo, s git.Status, err error) string {
	if err != nil {
		return st.errText.Render(err.Error())
	}

	parts := []string{st.repo.Render(repo.Name())}
	if cfg.Worktree && repo.IsWorktree() {
		parts = append(parts, st.worktree.Render("⎇ "+repo.WorktreeName()))
	}
	parts = append(parts, st.muted.Render("→"), st.branch.Render(branchLabel(s)))

	if cfg.Upstream && !s.Detached() {
		parts = append(parts, upstreamLabel(st, s))
	}
	return strings.Join(parts, " ")
}

func branchLabel(s git.Status) string {
	switch {
	case !s.Detached():
		return s.Head
	case len(s.OID) >= 7:
		return "detached at " + s.OID[:7]
	default:
		return "detached"
	}
}

func upstreamLabel(st styles, s git.Status) string {
	if s.Upstream == "" {
		return st.muted.Render("(no upstream)")
	}
	if s.Ahead == 0 && s.Behind == 0 {
		return st.inSync.Render("✓")
	}
	var counts []string
	if s.Ahead > 0 {
		counts = append(counts, fmt.Sprintf("↑%d", s.Ahead))
	}
	if s.Behind > 0 {
		counts = append(counts, fmt.Sprintf("↓%d", s.Behind))
	}
	return st.upstream.Render(strings.Join(counts, " "))
}
