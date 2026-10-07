package git

import (
	"strconv"
	"strings"
	"time"
)

// Commit is one entry of the current branch's history.
type Commit struct {
	Hash     string
	Short    string
	Subject  string
	Author   string
	Time     time.Time
	Unpushed bool // on this branch but not yet on its upstream
}

const logFormat = "%H%x00%h%x00%s%x00%an%x00%ct"

// ReadLog returns the latest n commits on the current branch, newest first.
// A repo with no commits yet has an empty log, not an error.
func ReadLog(r Repo, n int) ([]Commit, error) {
	out, err := run(r.Root, "log", "-n", strconv.Itoa(n), "--format="+logFormat, "HEAD")
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") ||
			strings.Contains(err.Error(), "ambiguous argument 'HEAD'") {
			return nil, nil
		}
		return nil, err
	}
	commits := parseLog(out)

	// Commits not yet pushed. Without an upstream this fails and nothing is
	// marked: there's nothing to compare against.
	if unpushed, err := run(r.Root, "rev-list", "--max-count="+strconv.Itoa(n), "@{upstream}..HEAD"); err == nil {
		set := map[string]bool{}
		for _, h := range strings.Fields(unpushed) {
			set[h] = true
		}
		for i := range commits {
			commits[i].Unpushed = set[commits[i].Hash]
		}
	}
	return commits, nil
}

func parseLog(out string) []Commit {
	var commits []Commit
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\x00")
		if len(f) != 5 {
			continue
		}
		c := Commit{Hash: f[0], Short: f[1], Subject: f[2], Author: f[3]}
		if secs, err := strconv.ParseInt(f[4], 10, 64); err == nil {
			c.Time = time.Unix(secs, 0)
		}
		commits = append(commits, c)
	}
	return commits
}

// CommitDiff returns the changes a commit made. A merge commit shows what
// it brought in compared to its first parent, rather than git's usual
// combined diff, which is often empty.
func CommitDiff(r Repo, hash string, color bool) (string, error) {
	colorFlag := "--color=never"
	if color {
		colorFlag = "--color=always"
	}
	return run(r.Root, "show", "--format=", "-M", "--diff-merges=first-parent", "--no-ext-diff", colorFlag, hash, "--")
}
