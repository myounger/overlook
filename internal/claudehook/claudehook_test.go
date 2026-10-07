package claudehook

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/myounger/overlook/internal/git"
)

// setup makes a repo with a Claude-style worktree and points the state
// folder at a temp dir.
func setup(t *testing.T) (repo git.Repo, wt string) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
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
	repo, err := git.Locate(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo, git.CanonicalPath(filepath.Join(dir, ".claude/worktrees/probe"))
}

func record(t *testing.T, now time.Time, ev map[string]any) {
	t.Helper()
	data, _ := json.Marshal(ev)
	if err := Record(strings.NewReader(string(data)), now); err != nil {
		t.Fatal(err)
	}
}

func TestRecordFollowsSessionIntoWorktree(t *testing.T) {
	repo, wt := setup(t)
	now := time.Now()
	record(t, now, map[string]any{"session_id": "s1", "hook_event_name": "SessionStart", "cwd": repo.Root})
	// What the probe saw: after EnterWorktree, every tool event's cwd is the worktree.
	record(t, now.Add(time.Second), map[string]any{
		"session_id": "s1", "hook_event_name": "PostToolUse", "tool_name": "Bash",
		"cwd": wt, "tool_input": map[string]any{"command": "pwd"},
	})
	states, err := Read(repo.CommonDir, now.Add(2*time.Second), time.Hour)
	if err != nil || len(states) != 1 || states[0].Worktree != wt {
		t.Fatalf("got %+v, %v; want s1 in the worktree", states, err)
	}
	// Reading from the worktree's side finds the same repo folder.
	wtRepo, _ := git.Locate(wt)
	if s, _ := Read(wtRepo.CommonDir, now, time.Hour); len(s) != 1 {
		t.Error("the worktree and main folder map to different state folders")
	}
}

func TestRecordUsesEditedFilePath(t *testing.T) {
	repo, wt := setup(t)
	now := time.Now()
	// The session's cwd is the main folder, but it edits a file in the worktree.
	record(t, now, map[string]any{
		"session_id": "s1", "hook_event_name": "PostToolUse", "tool_name": "Write",
		"cwd": repo.Root, "tool_input": map[string]any{"file_path": filepath.Join(wt, "new", "dir", "hello.txt")},
	})
	if states, _ := Read(repo.CommonDir, now, time.Hour); len(states) != 1 || states[0].Worktree != wt {
		t.Errorf("got %+v; want the edited file's worktree", states)
	}
}

func TestRecordOnlyWritesOnAMove(t *testing.T) {
	repo, _ := setup(t)
	now := time.Now()
	record(t, now, map[string]any{"session_id": "s1", "hook_event_name": "PostToolUse", "cwd": repo.Root})
	record(t, now.Add(time.Hour), map[string]any{"session_id": "s1", "hook_event_name": "PostToolUse", "cwd": repo.Root})
	states, _ := Read(repo.CommonDir, now.Add(time.Hour), 2*time.Hour)
	if len(states) != 1 || !states[0].Time.Equal(now) {
		t.Errorf("got %+v; staying in the same worktree shouldn't rewrite the file", states)
	}
}

func TestSessionEndRemovesState(t *testing.T) {
	repo, _ := setup(t)
	now := time.Now()
	record(t, now, map[string]any{"session_id": "s1", "hook_event_name": "SessionStart", "cwd": repo.Root})
	record(t, now, map[string]any{"session_id": "s2", "hook_event_name": "SessionStart", "cwd": repo.Root})
	record(t, now, map[string]any{"session_id": "s1", "hook_event_name": "SessionEnd", "cwd": repo.Root})
	states, _ := Read(repo.CommonDir, now, time.Hour)
	if len(states) != 1 || states[0].Session != "s2" {
		t.Errorf("got %+v; want only s2", states)
	}
}

func TestReadSkipsOldSessions(t *testing.T) {
	repo, _ := setup(t)
	now := time.Now()
	record(t, now.Add(-13*time.Hour), map[string]any{"session_id": "old", "hook_event_name": "SessionStart", "cwd": repo.Root})
	if states, _ := Read(repo.CommonDir, now, 12*time.Hour); len(states) != 0 {
		t.Errorf("got %+v; a session not seen for 13h should be skipped", states)
	}
}

func TestRecordIgnoresWhatItCantUse(t *testing.T) {
	setup(t)
	now := time.Now()
	for _, ev := range []map[string]any{
		{"hook_event_name": "PostToolUse", "cwd": "/"},                             // no session
		{"session_id": "s1", "hook_event_name": "PostToolUse", "cwd": t.TempDir()}, // not a repo
		{"session_id": "../../etc/x", "hook_event_name": "PostToolUse", "cwd": "/"},
	} {
		record(t, now, ev)
	}
	root, _ := Root()
	files, _ := filepath.Glob(filepath.Join(root, "*", "*"))
	if len(files) != 0 {
		t.Errorf("wrote %v", files)
	}
}

func TestOddSessionIDsStayInTheirFolder(t *testing.T) {
	repo, _ := setup(t)
	record(t, time.Now(), map[string]any{"session_id": "../../escape", "hook_event_name": "SessionStart", "cwd": repo.Root})
	dir, _ := RepoDir(repo.CommonDir)
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) != 1 {
		t.Errorf("files in the repo folder: %v", files)
	}
	root, _ := Root()
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape.json")); err == nil {
		t.Error("a session id escaped the state folder")
	}
}

func TestSettingsIsValidJSON(t *testing.T) {
	var v map[string]any
	if err := json.Unmarshal([]byte(Settings(`/Users/me/.local/bin/overlook`)), &v); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(Settings("/x/overlook"), `"/x/overlook hook"`) {
		t.Error("command missing")
	}
	if !strings.Contains(Settings("/My Tools/overlook"), `"'/My Tools/overlook' hook"`) {
		t.Errorf("a path with a space isn't quoted:\n%s", Settings("/My Tools/overlook"))
	}
}
