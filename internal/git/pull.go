package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// PullResult describes a pull that succeeded.
type PullResult struct {
	Commits  int // how many commits the branch moved forward; 0 if already up to date
	Upstream string
}

// Pull runs `git pull --ff-only` in the worktree: it fast-forwards the
// branch to its upstream, or refuses and changes nothing. This is the only
// command Overlook runs that changes the repo.
//
// It can't ask for anything: GIT_TERMINAL_PROMPT=0 stops git's own
// username/password prompts, and detach (see pull_unix.go and
// pull_windows.go) cuts git and ssh off from the terminal so ssh can't ask
// for a passphrase there, which would otherwise fight Overlook for the
// screen. Credentials from a keychain or ssh-agent still work.
func Pull(ctx context.Context, r Repo, upstream string) (PullResult, error) {
	before, err := run(r.Root, "rev-parse", "HEAD")
	if err != nil {
		return PullResult{}, err
	}

	cmd := exec.CommandContext(ctx, "git", "pull", "--ff-only", "--no-rebase")
	cmd.Dir = r.Root
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_MERGE_AUTOEDIT=no")
	detach(cmd)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return PullResult{}, errors.New("pull timed out")
		}
		return PullResult{}, pullError(out.String())
	}

	res := PullResult{Upstream: upstream}
	after, err := run(r.Root, "rev-parse", "HEAD")
	if err != nil {
		return res, err
	}
	before, after = strings.TrimSpace(before), strings.TrimSpace(after)
	if before != after {
		count, err := run(r.Root, "rev-list", "--count", before+".."+after)
		if err == nil {
			res.Commits, _ = strconv.Atoi(strings.TrimSpace(count))
		}
	}
	return res, nil
}

// pullError turns git's output from a failed pull into one short phrase,
// most important words first, since a narrow footer cuts off the end.
func pullError(out string) error {
	has := func(s string) bool { return strings.Contains(out, s) }
	switch {
	case has("Not possible to fast-forward"), has("Diverging branches"):
		return errors.New("can't pull: branches have diverged")
	case has("would be overwritten by merge"):
		return errors.New("can't pull: it would overwrite uncommitted changes")
	case has("no tracking information"):
		return errors.New("no upstream to pull from")
	case has("terminal prompts disabled"), has("could not read Username"), has("Permission denied"),
		has("Authentication failed"), has("Host key verification failed"):
		return errors.New("can't pull: needs a password; pull once in a terminal")
	case has("Could not resolve host"), has("unable to access"), has("Connection refused"), has("Operation timed out"):
		return errors.New("can't reach the remote")
	}
	// Otherwise, git's last line that says what went wrong.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		for _, p := range []string{"fatal: ", "error: "} {
			if rest, ok := strings.CutPrefix(l, p); ok {
				return errors.New(rest)
			}
		}
	}
	if len(lines) > 0 && lines[len(lines)-1] != "" {
		return errors.New(strings.TrimSpace(lines[len(lines)-1]))
	}
	return fmt.Errorf("pull failed")
}
