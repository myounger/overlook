package git

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestParseBranches(t *testing.T) {
	out := "*\x00refs/heads/main\x00origin/main\x00\x001791386981\x00/code/app\x00a1\n" +
		" \x00refs/heads/feature/login\x00origin/feature/login\x00ahead 2, behind 1\x001791300000\x00\x00b2\n" +
		" \x00refs/heads/old\x00origin/old\x00gone\x001791200000\x00\x00c3\n" +
		" \x00refs/heads/wt\x00\x00\x001791100000\x00/code/app-wt/wt\x00d4\n"
	want := []Branch{
		{Name: "main", Current: true, Upstream: "origin/main", Committed: time.Unix(1791386981, 0), Worktree: "/code/app", oid: "a1"},
		{Name: "feature/login", Upstream: "origin/feature/login", Ahead: 2, Behind: 1, Committed: time.Unix(1791300000, 0), oid: "b2"},
		{Name: "old", Upstream: "origin/old", Gone: true, Committed: time.Unix(1791200000, 0), oid: "c3"},
		{Name: "wt", Committed: time.Unix(1791100000, 0), Worktree: "/code/app-wt/wt", oid: "d4"},
	}
	if got := parseBranches(out); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestMarkMerged(t *testing.T) {
	branches := []Branch{
		{Name: "main", oid: "tip"},
		{Name: "done", oid: "older"},
		{Name: "here", Current: true, Worktree: "/code/app", oid: "older"},
		{Name: "in-wt", Worktree: "/code/app-wt/x", oid: "older"},
		{Name: "brand-new", oid: "tip"},
		{Name: "open", oid: "ahead"},
	}
	markMerged(branches, "refs/heads/main\nrefs/heads/done\nrefs/heads/here\nrefs/heads/in-wt\nrefs/heads/brand-new\n", "origin/main", "tip")
	for _, b := range branches {
		if want := b.Name == "done"; b.Merged != want {
			t.Errorf("%s: merged=%v, want %v", b.Name, b.Merged, want)
		}
	}
}

// TestReadBranches runs against a real throwaway repo.
func TestReadBranches(t *testing.T) {
	dir := t.TempDir()
	sh := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	sh("init", "-q", "-b", "main")
	sh("commit", "-q", "--allow-empty", "-m", "one")
	sh("branch", "done")
	sh("branch", "brand-new-later")
	sh("commit", "-q", "--allow-empty", "-m", "main moves on")
	sh("branch", "brand-new")
	sh("checkout", "-q", "-b", "open")
	sh("commit", "-q", "--allow-empty", "-m", "two")
	sh("worktree", "add", "-q", filepath.Join(dir, "wt"), "-b", "in-wt", "main~1")

	repo, err := Locate(dir)
	if err != nil {
		t.Fatal(err)
	}
	branches, base, err := ReadBranches(repo, "")
	if err != nil {
		t.Fatal(err)
	}
	if base != "main" {
		t.Errorf("base %q, want main", base)
	}
	got := map[string]Branch{}
	for _, b := range branches {
		got[b.Name] = b
	}
	if !got["open"].Current || got["open"].Merged {
		t.Errorf("open: %+v", got["open"])
	}
	for name, want := range map[string]bool{"done": true, "brand-new-later": true, "main": false, "brand-new": false, "in-wt": false} {
		if got[name].Merged != want {
			t.Errorf("%s: merged=%v, want %v", name, got[name].Merged, want)
		}
	}
	if filepath.Base(got["in-wt"].Worktree) != "wt" {
		t.Errorf("in-wt worktree %q", got["in-wt"].Worktree)
	}
}
