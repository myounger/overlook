package git

import (
	"strconv"
	"strings"
	"time"
)

// Branch is a local branch from `git for-each-ref refs/heads`.
type Branch struct {
	Name      string
	Current   bool
	Upstream  string // remote branch, or "" when none is set
	Ahead     int
	Behind    int
	Gone      bool      // the upstream was deleted on the remote (seen after a fetch --prune)
	Committed time.Time // date of the branch's last commit
	Worktree  string    // folder where it's checked out, or ""
	Merged    bool      // looks safe to delete: see markMerged
	oid       string
}

const branchFormat = "%(HEAD)%00%(refname)%00%(upstream:short)%00%(upstream:track,nobracket)%00%(committerdate:unix)%00%(worktreepath)%00%(objectname)"

// ReadBranches lists local branches. Merged is checked against mergedInto,
// or when that's empty, against the remote's default branch (falling back
// to main or master). It returns the branch it compared against.
func ReadBranches(r Repo, mergedInto string) ([]Branch, string, error) {
	out, err := run(r.Root, "for-each-ref", "--format="+branchFormat, "refs/heads")
	if err != nil {
		return nil, "", err
	}
	branches := parseBranches(out)

	base := mergedInto
	if base == "" {
		base = defaultBranch(r)
	}
	if base != "" {
		merged, err := run(r.Root, "for-each-ref", "--merged="+base, "--format=%(refname)", "refs/heads")
		baseOID, err2 := run(r.Root, "rev-parse", "--verify", "-q", base+"^{commit}")
		if err == nil && err2 == nil {
			markMerged(branches, merged, base, strings.TrimSpace(baseOID))
		}
	}
	return branches, base, nil
}

func parseBranches(out string) []Branch {
	var branches []Branch
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\x00")
		if len(f) != 7 {
			continue
		}
		b := Branch{
			Current:  f[0] == "*",
			Name:     strings.TrimPrefix(f[1], "refs/heads/"),
			Upstream: f[2],
			Worktree: f[5],
			oid:      f[6],
		}
		if secs, err := strconv.ParseInt(f[4], 10, 64); err == nil {
			b.Committed = time.Unix(secs, 0)
		}
		// track is "", "gone", "ahead 1", "behind 2", or "ahead 1, behind 2".
		for _, part := range strings.Split(f[3], ", ") {
			word, n, _ := strings.Cut(part, " ")
			switch word {
			case "gone":
				b.Gone = true
			case "ahead":
				b.Ahead, _ = strconv.Atoi(n)
			case "behind":
				b.Behind, _ = strconv.Atoi(n)
			}
		}
		branches = append(branches, b)
	}
	return branches
}

// markMerged flags branches that look done: everything on them is in base
// (listed by `for-each-ref --merged`). It skips the base branch itself,
// branches checked out in any worktree (still in use, like the one you're
// on), and branches sitting exactly on base's tip, which are almost always
// new branches with no commits yet rather than finished ones.
func markMerged(branches []Branch, mergedRefs, base, baseOID string) {
	merged := map[string]bool{}
	for _, ref := range strings.Fields(mergedRefs) {
		merged[strings.TrimPrefix(ref, "refs/heads/")] = true
	}
	_, baseName, ok := strings.Cut(base, "/")
	if !ok {
		baseName = base
	}
	for i := range branches {
		b := &branches[i]
		b.Merged = merged[b.Name] && b.Worktree == "" && b.oid != baseOID &&
			b.Name != base && b.Name != baseName
	}
}

// defaultBranch is origin's default branch (origin/HEAD) if known, else a
// local main or master.
func defaultBranch(r Repo) string {
	if out, err := run(r.Root, "symbolic-ref", "-q", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimSpace(out)
	}
	for _, name := range []string{"main", "master"} {
		if _, err := run(r.Root, "rev-parse", "--verify", "-q", "refs/heads/"+name); err == nil {
			return name
		}
	}
	return ""
}
