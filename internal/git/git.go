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
	return Repo{Root: CanonicalPath(lines[0]), GitDir: CanonicalPath(lines[1]), CommonDir: CanonicalPath(lines[2])}, nil
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

// Status is the result of `git status --porcelain=v2 --branch`.
type Status struct {
	Head     string // branch name, or "" when detached
	OID      string // commit hash, or "" before the first commit
	Upstream string // remote branch, or "" when none is set
	Ahead    int
	Behind   int
	Files    []File
}

// Detached reports whether HEAD points at a commit instead of a branch.
func (s Status) Detached() bool { return s.Head == "" }

// File is one changed path. Staged and Unstaged are git's X and Y status
// letters: '.' unchanged, 'M' modified, 'A' added, 'D' deleted, 'R' renamed,
// 'C' copied, 'T' type changed, 'U' unmerged, '?' untracked.
type File struct {
	Path       string
	OrigPath   string // the old path of a rename or copy
	Staged     byte
	Unstaged   byte
	Conflicted bool
}

func (f File) Untracked() bool { return f.Staged == '?' }

// Kind is the single word that best describes the change.
type Kind int

const (
	Modified Kind = iota
	Added
	Deleted
	Renamed
	Conflicted
)

func (f File) Kind() Kind {
	has := func(c byte) bool { return f.Staged == c || f.Unstaged == c }
	switch {
	case f.Conflicted:
		return Conflicted
	case f.Untracked(), has('A'):
		return Added
	case has('D'):
		return Deleted
	case has('R'), has('C'):
		return Renamed
	default:
		return Modified
	}
}

// Untracked selects how untracked files are listed.
type Untracked string

const (
	UntrackedAll     Untracked = "all"     // every file, even inside new folders
	UntrackedFolders Untracked = "folders" // a new folder shows as one entry
	UntrackedNone    Untracked = "none"
)

func (u Untracked) flag() string {
	switch u {
	case UntrackedFolders:
		return "--untracked-files=normal"
	case UntrackedNone:
		return "--untracked-files=no"
	default:
		return "--untracked-files=all"
	}
}

// ReadStatus runs git status in the repo and parses the result.
func ReadStatus(r Repo, untracked Untracked) (Status, error) {
	out, err := run(r.Root, "status", "--porcelain=v2", "--branch", "-z", untracked.flag())
	if err != nil {
		return Status{}, err
	}
	return parseStatus(out), nil
}

// parseStatus reads porcelain v2 output with -z. Each entry ends in NUL; a
// rename or copy entry is followed by one more NUL-terminated original path.
func parseStatus(out string) Status {
	var s Status
	entries := strings.Split(out, "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry == "" {
			continue
		}
		switch entry[0] {
		case '#':
			parseHeader(&s, entry)
		case '1':
			if f := strings.SplitN(entry, " ", 9); len(f) == 9 {
				s.Files = append(s.Files, File{Path: f[8], Staged: f[1][0], Unstaged: f[1][1]})
			}
		case '2':
			if f := strings.SplitN(entry, " ", 10); len(f) == 10 {
				file := File{Path: f[9], Staged: f[1][0], Unstaged: f[1][1]}
				if i+1 < len(entries) {
					i++
					file.OrigPath = entries[i]
				}
				s.Files = append(s.Files, file)
			}
		case 'u':
			if f := strings.SplitN(entry, " ", 11); len(f) == 11 {
				s.Files = append(s.Files, File{Path: f[10], Staged: f[1][0], Unstaged: f[1][1], Conflicted: true})
			}
		case '?':
			s.Files = append(s.Files, File{Path: strings.TrimPrefix(entry, "? "), Staged: '?', Unstaged: '?'})
		}
	}
	return s
}

func parseHeader(s *Status, entry string) {
	key, value, _ := strings.Cut(strings.TrimPrefix(entry, "# "), " ")
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
