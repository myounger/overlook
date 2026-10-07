package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	return cmd
}

func TestReadLog(t *testing.T) {
	mine, _, sh := pullSetup(t) // "mine" tracks origin/main with one pushed commit
	os.WriteFile(filepath.Join(mine.Root, "a.txt"), []byte("a\n"), 0o644)
	sh(mine.Root, "add", ".")
	sh(mine.Root, "commit", "-q", "-m", "local one")
	sh(mine.Root, "commit", "-q", "--allow-empty", "-m", "local two")

	commits, err := ReadLog(mine, 10)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range commits {
		mark := " "
		if c.Unpushed {
			mark = "↑"
		}
		got = append(got, mark+c.Subject)
	}
	if want := "↑local two|↑local one| one"; strings.Join(got, "|") != want {
		t.Errorf("got %q, want %q", strings.Join(got, "|"), want)
	}
	if c := commits[0]; len(c.Short) < 7 || !strings.HasPrefix(c.Hash, c.Short) || c.Author != "t" || c.Time.IsZero() {
		t.Errorf("fields: %+v", c)
	}
	if few, _ := ReadLog(mine, 1); len(few) != 1 {
		t.Errorf("n=1 gave %d commits", len(few))
	}

	diff, err := CommitDiff(mine, commits[1].Hash, false)
	if err != nil || !strings.Contains(diff, "+a") {
		t.Errorf("commit diff: %v\n%s", err, diff)
	}
}

func TestReadLogNoUpstreamMarksNothing(t *testing.T) {
	mine, _, sh := pullSetup(t)
	sh(mine.Root, "checkout", "-q", "-b", "local-only")
	sh(mine.Root, "commit", "-q", "--allow-empty", "-m", "x")
	commits, err := ReadLog(mine, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range commits {
		if c.Unpushed {
			t.Errorf("%q marked unpushed with no upstream to compare", c.Subject)
		}
	}
}

func TestReadLogEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	sh := func(args ...string) {
		if out, err := gitCmd(dir, args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	sh("init", "-q", "-b", "main")
	commits, err := ReadLog(Repo{Root: dir}, 10)
	if err != nil || len(commits) != 0 {
		t.Errorf("got %v, %v; want an empty log", commits, err)
	}
}

func TestCommitDiffOfMerge(t *testing.T) {
	mine, _, sh := pullSetup(t)
	sh(mine.Root, "checkout", "-q", "-b", "feature")
	os.WriteFile(filepath.Join(mine.Root, "feature.txt"), []byte("feature\n"), 0o644)
	sh(mine.Root, "add", ".")
	sh(mine.Root, "commit", "-q", "-m", "feature work")
	sh(mine.Root, "checkout", "-q", "main")
	sh(mine.Root, "commit", "-q", "--allow-empty", "-m", "main moves")
	sh(mine.Root, "merge", "-q", "--no-ff", "-m", "merge feature", "feature")

	commits, _ := ReadLog(mine, 1)
	diff, err := CommitDiff(mine, commits[0].Hash, false)
	if err != nil || !strings.Contains(diff, "+feature") {
		t.Errorf("a merge should show what it brought in: %v\n%s", err, diff)
	}
}
