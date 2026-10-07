package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// emptyTree is git's well-known hash of an empty tree, used as the base
// before a repo's first commit.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// DiffTarget is what to diff: one changed file, or a folder of them.
type DiffTarget struct {
	Path      string
	OrigPath  string // a rename's old path
	Dir       bool
	Untracked bool
	NewFiles  []string // untracked files inside a Dir target, which git diff leaves out
}

// Diff returns the changes to t since the last commit, staged and unstaged
// together. Untracked files show as entirely added. With color, git's own
// ANSI colors are kept (for piping through a pager like delta).
func Diff(r Repo, t DiffTarget, hasHead, color bool) (string, error) {
	colorFlag := "--color=never"
	if color {
		colorFlag = "--color=always"
	}
	if t.Untracked {
		// --no-index exits 1 when the files differ, which they always do.
		return runAllowing(r.Root, 1, "diff", "--no-index", "--no-ext-diff", colorFlag, "--", os.DevNull, t.Path)
	}
	base := "HEAD"
	if !hasHead {
		base = emptyTree
	}
	args := []string{"diff", base, "-M", "--no-ext-diff", colorFlag, "--"}
	if !t.Dir {
		args = append(args, t.Path)
		if t.OrigPath != "" {
			args = append(args, t.OrigPath)
		}
		return run(r.Root, args...)
	}
	out, err := run(r.Root, append(args, t.Path+"/")...)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(out)
	for _, f := range t.NewFiles {
		d, err := Diff(r, DiffTarget{Path: f, Untracked: true}, hasHead, color)
		if err != nil {
			return "", err
		}
		b.WriteString(d)
	}
	return b.String(), nil
}

// runAllowing is run, but treats exit code ok as success too.
func runAllowing(dir string, ok int, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"--no-optional-locks"}, args...)...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == ok) {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("git %s: %s", args[0], msg)
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return stdout.String(), nil
}

// Pipe runs a shell command (such as "delta --paging=never") with input on
// stdin and COLUMNS set to width, and returns its output.
func Pipe(command, input string, width int) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), fmt.Sprintf("COLUMNS=%d", width))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s: %s", command, msg)
		}
		return "", fmt.Errorf("%s: %w", command, err)
	}
	return stdout.String(), nil
}
