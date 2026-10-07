package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiff(t *testing.T) {
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
	write := func(name, body string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
	}
	repo := Repo{Root: dir}

	sh("init", "-q")
	write("staged.txt", "one\n")
	sh("add", ".")
	// Before the first commit there's no HEAD; diff against the empty tree.
	out, err := Diff(repo, DiffTarget{Path: "staged.txt"}, false, false)
	if err != nil || !strings.Contains(out, "+one") {
		t.Fatalf("no HEAD: %v\n%s", err, out)
	}
	write("src/a.go", "package a\n")
	write("old.txt", "keep\n")
	sh("add", ".")
	sh("commit", "-q", "-m", "init")

	// Staged and unstaged changes show together.
	write("staged.txt", "one\ntwo\n")
	sh("add", "staged.txt")
	write("staged.txt", "one\ntwo\nthree\n")
	out, _ = Diff(repo, DiffTarget{Path: "staged.txt"}, true, false)
	if !strings.Contains(out, "+two") || !strings.Contains(out, "+three") {
		t.Errorf("combined diff missing a change:\n%s", out)
	}

	write("new.txt", "brand new\n")
	out, err = Diff(repo, DiffTarget{Path: "new.txt", Untracked: true}, true, false)
	if err != nil || !strings.Contains(out, "+brand new") {
		t.Errorf("untracked: %v\n%s", err, out)
	}

	sh("mv", "old.txt", "renamed.txt")
	out, _ = Diff(repo, DiffTarget{Path: "renamed.txt", OrigPath: "old.txt"}, true, false)
	if !strings.Contains(out, "rename from old.txt") {
		t.Errorf("rename not detected:\n%s", out)
	}

	write("src/a.go", "package a\n\nfunc A() {}\n")
	write("src/b.go", "package a\n")
	sh("add", "src/b.go")
	write("src/c.go", "package c\n")
	out, _ = Diff(repo, DiffTarget{Path: "src", Dir: true, NewFiles: []string{"src/c.go"}}, true, false)
	for _, want := range []string{"src/a.go", "src/b.go", "+package c"} {
		if !strings.Contains(out, want) {
			t.Errorf("folder diff missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "staged.txt") {
		t.Errorf("folder diff should cover only src/:\n%s", out)
	}

	out, _ = Diff(repo, DiffTarget{Path: "staged.txt"}, true, true)
	if !strings.Contains(out, "\x1b[") {
		t.Error("color requested but no ANSI codes in output")
	}
}

func TestPipe(t *testing.T) {
	out, err := Pipe(`tr a-z A-Z; echo "$COLUMNS"`, "diff\n", 42)
	if err != nil || out != "DIFF\n42\n" {
		t.Errorf("got %q, %v", out, err)
	}
	if _, err := Pipe("exit 3", "", 10); err == nil {
		t.Error("expected an error from a failing pager")
	}
}
