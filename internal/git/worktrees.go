package git

import (
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Worktree is one entry from `git worktree list`. The first is always the
// main working tree.
type Worktree struct {
	Path     string
	Branch   string // "" when detached
	Main     bool
	Prunable bool // its folder is gone; git will clean it up on prune
}

// Name is how a worktree is labeled: the main one by the repo's name, the
// others by their folder name.
func (w Worktree) Name(repoName string) string {
	if w.Main {
		return repoName
	}
	return filepath.Base(w.Path)
}

// ListWorktrees lists the repo's working trees, main first. Bare entries
// (no working tree) are left out. It runs from the shared git dir, which
// still exists after the worktree you were viewing has been removed.
func ListWorktrees(r Repo) ([]Worktree, error) {
	out, err := run(r.CommonDir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out), nil
}

// parseWorktrees reads NUL-separated attribute lines; an empty field ends
// each worktree's block.
func parseWorktrees(out string) []Worktree {
	var trees []Worktree
	var cur *Worktree
	bare := false
	flush := func() {
		if cur != nil && !bare {
			cur.Main = len(trees) == 0
			trees = append(trees, *cur)
		}
		cur, bare = nil, false
	}
	for _, field := range strings.Split(out, "\x00") {
		key, value, _ := strings.Cut(field, " ")
		switch key {
		case "":
			flush()
		case "worktree":
			flush()
			cur = &Worktree{Path: CanonicalPath(value)}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "bare":
			bare = true
		case "prunable":
			if cur != nil {
				cur.Prunable = true
			}
		}
	}
	flush()
	return trees
}

// Activity summarizes a worktree's changes: how many files differ, and a
// fingerprint that changes whenever the set of changes or any changed
// file's modification time does. Comparing fingerprints between refreshes
// shows which worktree is being worked in.
type Activity struct {
	Changed     int
	Fingerprint uint64
}

// ReadActivity reads the activity of the worktree r.
func ReadActivity(r Repo, untracked Untracked) (Activity, error) {
	s, err := ReadStatus(r, untracked)
	if err != nil {
		return Activity{}, err
	}
	return ActivityOf(r.Root, s.Files), nil
}

// ActivityOf summarizes an already-read list of changed files. It hashes
// each file's path and status letters plus its modification time; status
// alone isn't enough, because editing a file that's already modified
// doesn't change its status.
func ActivityOf(root string, files []File) Activity {
	h := fnv.New64a()
	for _, f := range files {
		h.Write([]byte(f.Path))
		h.Write([]byte{f.Staged, f.Unstaged, 0})
		if info, err := os.Stat(filepath.Join(root, f.Path)); err == nil {
			h.Write([]byte(info.ModTime().Format(time.RFC3339Nano)))
		}
	}
	return Activity{Changed: len(files), Fingerprint: h.Sum64()}
}

// CanonicalPath resolves symlinks (on macOS /tmp is really /private/tmp)
// so paths from different git commands compare equal.
func CanonicalPath(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return filepath.Clean(p)
}
