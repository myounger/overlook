// Package git reads repository state by running the git CLI. Every command
// is read-only and passes --no-optional-locks so Overlook never writes the
// index behind your back.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Repo is the working tree Overlook is showing.
type Repo struct {
	Root      string // top of the working tree
	GitDir    string // this worktree's git dir (HEAD, index)
	CommonDir string // shared git dir (refs, packed-refs, worktrees/)
}

// Locate finds the repository containing dir.
func Locate(dir string) (Repo, error) {
	out, err := run(dir, "rev-parse", "--path-format=absolute",
		"--show-toplevel", "--git-dir", "--git-common-dir")
	if err != nil {
		return Repo{}, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		return Repo{}, fmt.Errorf("unexpected git rev-parse output: %q", out)
	}
	return Repo{Root: lines[0], GitDir: lines[1], CommonDir: lines[2]}, nil
}

// IsWorktree reports whether this is a linked worktree rather than the main
// working tree.
func (r Repo) IsWorktree() bool {
	return filepath.Clean(r.GitDir) != filepath.Clean(r.CommonDir)
}

// Name is the repository's name. In a linked worktree it's the main repo's
// folder name, not the worktree's.
func (r Repo) Name() string {
	if r.IsWorktree() && filepath.Base(r.CommonDir) == ".git" {
		return filepath.Base(filepath.Dir(r.CommonDir))
	}
	return filepath.Base(r.Root)
}

// WorktreeName is the worktree's folder name.
func (r Repo) WorktreeName() string {
	return filepath.Base(r.Root)
}

// Status is the branch information from `git status --porcelain=v2 --branch`.
type Status struct {
	Head     string // branch name, or "" when detached
	OID      string // commit hash, or "" before the first commit
	Upstream string // remote branch, or "" when none is set
	Ahead    int
	Behind   int
}

// Detached reports whether HEAD points at a commit instead of a branch.
func (s Status) Detached() bool { return s.Head == "" }

// ReadStatus runs git status in the repo and parses the result.
func ReadStatus(r Repo) (Status, error) {
	out, err := run(r.Root, "status", "--porcelain=v2", "--branch", "-z")
	if err != nil {
		return Status{}, err
	}
	return parseStatus(out), nil
}

// parseStatus reads the "# branch.*" header entries. File entries are
// skipped for now; the files panel will use them.
func parseStatus(out string) Status {
	var s Status
	for _, entry := range strings.Split(out, "\x00") {
		key, value, ok := strings.Cut(strings.TrimPrefix(entry, "# "), " ")
		if !ok || !strings.HasPrefix(entry, "# ") {
			continue
		}
		switch key {
		case "branch.oid":
			if value != "(initial)" {
				s.OID = value
			}
		case "branch.head":
			if value != "(detached)" {
				s.Head = value
			}
		case "branch.upstream":
			s.Upstream = value
		case "branch.ab":
			a, b, _ := strings.Cut(value, " ")
			s.Ahead, _ = strconv.Atoi(strings.TrimPrefix(a, "+"))
			s.Behind, _ = strconv.Atoi(strings.TrimPrefix(b, "-"))
		}
	}
	return s
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"--no-optional-locks"}, args...)...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("git %s: %s", args[0], msg)
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return stdout.String(), nil
}
