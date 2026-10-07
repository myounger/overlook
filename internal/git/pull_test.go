package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pullSetup makes a remote and two clones: "mine" is the one Overlook
// would be showing, "theirs" pushes new commits.
func pullSetup(t *testing.T) (mine Repo, theirs func(args ...string), sh func(dir string, args ...string)) {
	t.Helper()
	root := t.TempDir()
	sh = func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	remote := filepath.Join(root, "remote.git")
	sh(root, "init", "-q", "--bare", "-b", "main", remote)
	for _, name := range []string{"theirs", "mine"} {
		sh(root, "clone", "-q", remote, name)
	}
	theirsDir := filepath.Join(root, "theirs")
	os.WriteFile(filepath.Join(theirsDir, "f.txt"), []byte("1\n"), 0o644)
	sh(theirsDir, "add", ".")
	sh(theirsDir, "commit", "-q", "-m", "one")
	sh(theirsDir, "push", "-q", "origin", "main")
	mineDir := filepath.Join(root, "mine")
	sh(mineDir, "pull", "-q", "origin", "main")
	sh(mineDir, "branch", "-q", "-u", "origin/main")
	theirs = func(args ...string) { sh(theirsDir, args...) }
	return Repo{Root: mineDir}, theirs, sh
}

func TestPullFastForwards(t *testing.T) {
	mine, theirs, _ := pullSetup(t)
	theirs("commit", "-q", "--allow-empty", "-m", "two")
	theirs("commit", "-q", "--allow-empty", "-m", "three")
	theirs("push", "-q", "origin", "main")

	res, err := Pull(context.Background(), mine, "origin/main")
	if err != nil || res.Commits != 2 {
		t.Fatalf("got %+v, %v; want 2 commits", res, err)
	}
	res, err = Pull(context.Background(), mine, "origin/main")
	if err != nil || res.Commits != 0 {
		t.Errorf("second pull: got %+v, %v; want up to date", res, err)
	}
}

func TestPullRefusesWhenDiverged(t *testing.T) {
	mine, theirs, sh := pullSetup(t)
	theirs("commit", "-q", "--allow-empty", "-m", "theirs")
	theirs("push", "-q", "origin", "main")
	sh(mine.Root, "commit", "-q", "--allow-empty", "-m", "mine")
	before, _ := run(mine.Root, "rev-parse", "HEAD")

	_, err := Pull(context.Background(), mine, "origin/main")
	if err == nil || !strings.Contains(err.Error(), "diverged") {
		t.Fatalf("got %v, want a diverged error", err)
	}
	if after, _ := run(mine.Root, "rev-parse", "HEAD"); after != before {
		t.Error("a refused pull moved the branch")
	}
}

func TestPullRefusesOverLocalChanges(t *testing.T) {
	mine, theirs, _ := pullSetup(t)
	theirs("commit", "-q", "--allow-empty", "-m", "noop") // keep history simple
	os.WriteFile(filepath.Join(filepath.Dir(mine.Root), "theirs", "f.txt"), []byte("1\n2\n"), 0o644)
	theirs("commit", "-q", "-am", "two")
	theirs("push", "-q", "origin", "main")
	os.WriteFile(filepath.Join(mine.Root, "f.txt"), []byte("1\nmine\n"), 0o644)

	_, err := Pull(context.Background(), mine, "origin/main")
	if err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("got %v, want a local-changes error", err)
	}
	if b, _ := os.ReadFile(filepath.Join(mine.Root, "f.txt")); string(b) != "1\nmine\n" {
		t.Error("a refused pull touched the local edit")
	}
}

// TestPullCantReachTerminal checks the mechanism that keeps ssh from
// asking for a passphrase: a fake ssh tries to open /dev/tty, as ssh does
// to prompt, and records whether it could. (When the tests themselves run
// without a terminal this passes either way; run them from a terminal to
// make it meaningful.)
func TestPullCantReachTerminal(t *testing.T) {
	mine, _, sh := pullSetup(t)
	report := filepath.Join(t.TempDir(), "tty")
	fakeSSH := filepath.Join(t.TempDir(), "fake-ssh")
	os.WriteFile(fakeSSH, []byte("#!/bin/sh\nif (: </dev/tty) 2>/dev/null; then echo opened >"+report+"; else echo none >"+report+"; fi\nexit 1\n"), 0o755)
	sh(mine.Root, "remote", "set-url", "origin", "ssh://example.invalid/repo.git")
	sh(mine.Root, "config", "core.sshCommand", fakeSSH)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := Pull(ctx, mine, "origin/main"); err == nil {
		t.Fatal("expected an error")
	}
	got, _ := os.ReadFile(report)
	if strings.TrimSpace(string(got)) != "none" {
		t.Errorf("ssh could open the terminal (%q); it could prompt and hang", got)
	}
}

func TestPullError(t *testing.T) {
	tests := map[string]string{
		"hint: Diverging branches can't be fast-forwarded\nfatal: Not possible to fast-forward, aborting.\n": "diverged",
		"error: Your local changes to the following files would be overwritten by merge:\n\tf.txt\n":         "uncommitted changes",
		"There is no tracking information for the current branch.\n":                                         "no upstream",
		"fatal: could not read Username for 'https://github.com': terminal prompts disabled\n":               "password",
		"ssh: Could not resolve hostname github.com\nfatal: Could not read from remote repository.\n":        "can't reach the remote",
		"fatal: something new\n": "something new",
	}
	for out, want := range tests {
		if got := pullError(out).Error(); !strings.Contains(got, want) {
			t.Errorf("%q: got %q, want it to mention %q", out, got, want)
		}
	}
}
