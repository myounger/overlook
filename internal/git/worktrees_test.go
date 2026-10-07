package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseWorktrees(t *testing.T) {
	out := "worktree /code/app\x00HEAD abc\x00branch refs/heads/main\x00\x00" +
		"worktree /code/app-wt/fix\x00HEAD def\x00branch refs/heads/fix/login\x00\x00" +
		"worktree /code/app-wt/look\x00HEAD def\x00detached\x00\x00" +
		"worktree /code/app-wt/gone\x00HEAD def\x00branch refs/heads/gone\x00prunable gitdir file points to non-existent location\x00\x00"
	want := []Worktree{
		{Path: "/code/app", Branch: "main", Main: true},
		{Path: "/code/app-wt/fix", Branch: "fix/login"},
		{Path: "/code/app-wt/look"},
		{Path: "/code/app-wt/gone", Branch: "gone", Prunable: true},
	}
	if got := parseWorktrees(out); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseWorktreesSkipsBare(t *testing.T) {
	out := "worktree /code/app.git\x00bare\x00\x00worktree /code/app-wt/a\x00HEAD abc\x00branch refs/heads/a\x00\x00"
	got := parseWorktrees(out)
	if len(got) != 1 || got[0].Path != "/code/app-wt/a" {
		t.Errorf("got %+v", got)
	}
}

func TestActivityChangesWhenAModifiedFileIsEditedAgain(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	file := filepath.Join(dir, "a.txt")
	os.WriteFile(file, []byte("one\n"), 0o644)
	run("init", "-q")
	run("add", ".")
	run("commit", "-q", "-m", "init")

	os.WriteFile(file, []byte("two\n"), 0o644)
	first, err := ReadActivity(Repo{Root: dir}, UntrackedAll)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(file, []byte("three\n"), 0o644)
	os.Chtimes(file, time.Now().Add(time.Second), time.Now().Add(time.Second))
	second, _ := ReadActivity(Repo{Root: dir}, UntrackedAll)

	if first.Changed != 1 || second.Changed != 1 {
		t.Errorf("changed counts %d, %d; want 1, 1", first.Changed, second.Changed)
	}
	if first.Fingerprint == second.Fingerprint {
		t.Error("fingerprint didn't change after editing an already-modified file")
	}
	again, _ := ReadActivity(Repo{Root: dir}, UntrackedAll)
	if again != second {
		t.Error("fingerprint changed with no edits")
	}
	status, _ := ReadStatus(Repo{Root: dir}, UntrackedAll)
	if ActivityOf(dir, status.Files) != second {
		t.Error("ActivityOf and ReadActivity disagree on the same state")
	}
}

// Claude Code creates worktrees inside the repo (.claude/worktrees/<name>),
// which git lists as an untracked folder in the main working tree.
func TestStatusHidesOwnNestedWorktrees(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "init")
	run("worktree", "add", "-q", ".claude/worktrees/probe", "-b", "worktree-probe")
	// A separate nested repo is someone else's project and stays visible.
	os.MkdirAll(filepath.Join(dir, "vendor", "other"), 0o755)
	run("init", "-q", "vendor/other")
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644)

	repo, _ := Locate(dir)
	s, err := ReadStatus(repo, UntrackedAll)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range s.Files {
		paths = append(paths, f.Path)
	}
	if got := strings.Join(paths, " "); got != "new.txt vendor/other/" {
		t.Errorf("got %q, want the new file and the nested repo, not the worktree", got)
	}
	// The worktree tabs' change counts go through the same filter.
	if act, _ := ReadActivity(repo, UntrackedAll); act.Changed != 2 {
		t.Errorf("activity counts %d changes, want 2", act.Changed)
	}
}
