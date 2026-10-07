// Package claudehook lets a Claude Code hook tell Overlook which worktree a
// session is working in.
//
// Claude Code runs `overlook hook` on its events and passes each one as
// JSON on stdin. Record works out the folder the session is in (the edited
// file's folder for an edit, otherwise the session's cwd), finds the
// worktree that folder belongs to, and writes it to a small state file:
//
//	$XDG_STATE_HOME/overlook/claude/<repo key>/<session id>.json
//
// There's one folder per repo, so an Overlook showing one repo ignores
// sessions in another, and one file per session, so two sessions in the
// same repo don't overwrite each other. Overlook watches its repo's folder.
package claudehook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/myounger/overlook/internal/git"
)

// State is where one Claude Code session was last seen working.
type State struct {
	Session  string    `json:"session"`
	Worktree string    `json:"worktree"`
	Event    string    `json:"event"`
	Time     time.Time `json:"time"`
}

// staleAfter is when a session file is old enough to delete; Record prunes
// them when a session starts, in case a session ended without saying so.
const staleAfter = 7 * 24 * time.Hour

// Root is $XDG_STATE_HOME/overlook/claude, or ~/.local/state/overlook/claude.
func Root() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "overlook", "claude"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "overlook", "claude"), nil
}

// RepoDir is the folder holding session files for the repo whose shared git
// dir is commonDir.
func RepoDir(commonDir string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(git.CanonicalPath(commonDir)))
	return filepath.Join(root, hex.EncodeToString(sum[:8])), nil
}

// event is the part of Claude Code's hook input that Record uses.
type event struct {
	SessionID string `json:"session_id"`
	Name      string `json:"hook_event_name"`
	Cwd       string `json:"cwd"`
	ToolInput struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"tool_input"`
}

var safeName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// Record handles one hook event. It only reads and writes Overlook's own
// state files; anything it can't use (no session, a folder outside any
// repo) is quietly ignored.
func Record(input io.Reader, now time.Time) error {
	var ev event
	if err := json.NewDecoder(input).Decode(&ev); err != nil {
		return err
	}
	if ev.SessionID == "" {
		return nil
	}
	name := ev.SessionID
	if !safeName.MatchString(name) {
		sum := sha256.Sum256([]byte(name))
		name = hex.EncodeToString(sum[:8])
	}

	where := ev.Cwd
	for _, p := range []string{ev.ToolInput.FilePath, ev.ToolInput.NotebookPath} {
		if filepath.IsAbs(p) {
			where = filepath.Dir(p)
			break
		}
	}
	if where == "" {
		return nil
	}
	repo, err := git.Locate(existingDir(where))
	if err != nil {
		return nil // not in a git repo
	}
	dir, err := RepoDir(repo.CommonDir)
	if err != nil {
		return err
	}
	file := filepath.Join(dir, name+".json")

	switch ev.Name {
	case "SessionEnd":
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	case "SessionStart":
		prune(now)
	}

	// Only a move to a different worktree is news.
	if prev, err := readState(file); err == nil && prev.Worktree == repo.Root {
		return nil
	}
	data, err := json.Marshal(State{Session: ev.SessionID, Worktree: repo.Root, Event: ev.Name, Time: now})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Write then rename, so Overlook never reads half a file.
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), file)
}

// Read returns the sessions recorded for a repo that were seen within
// maxAge, which leaves out sessions that ended without a SessionEnd.
func Read(commonDir string, now time.Time, maxAge time.Duration) ([]State, error) {
	dir, err := RepoDir(commonDir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var states []State
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		s, err := readState(filepath.Join(dir, e.Name()))
		if err == nil && now.Sub(s.Time) <= maxAge {
			states = append(states, s)
		}
	}
	return states, nil
}

func readState(file string) (State, error) {
	var s State
	data, err := os.ReadFile(file)
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(data, &s)
}

// existingDir walks up from p to the nearest folder that exists, since an
// edit can name a file whose folder was just removed.
func existingDir(p string) string {
	for {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
		parent := filepath.Dir(p)
		if parent == p {
			return p
		}
		p = parent
	}
}

// prune deletes session files nobody has touched in a week.
func prune(now time.Time) {
	root, err := Root()
	if err != nil {
		return
	}
	files, _ := filepath.Glob(filepath.Join(root, "*", "*.json"))
	for _, f := range files {
		if info, err := os.Stat(f); err == nil && now.Sub(info.ModTime()) > staleAfter {
			os.Remove(f)
		}
	}
}

// Settings returns the hooks block to add to ~/.claude/settings.json, using
// the full path to this overlook binary.
//
// PostToolUse runs after every tool and carries the session's current
// folder, which is how a move into a worktree shows up (CwdChanged doesn't
// fire for that). It's async so it never slows Claude down. SessionStart
// and SessionEnd are quick and run in step so SessionEnd finishes before
// Claude Code exits.
func Settings(binary string) string {
	if strings.ContainsAny(binary, " '\"$\\") {
		binary = "'" + strings.ReplaceAll(binary, "'", `'\''`) + "'" // quoted for the shell
	}
	cmd, _ := json.Marshal(binary + " hook")
	return `{
  "hooks": {
    "PostToolUse": [
      { "matcher": "*", "hooks": [{ "type": "command", "command": ` + string(cmd) + `, "async": true }] }
    ],
    "SessionStart": [
      { "hooks": [{ "type": "command", "command": ` + string(cmd) + `, "timeout": 5 }] }
    ],
    "SessionEnd": [
      { "hooks": [{ "type": "command", "command": ` + string(cmd) + `, "timeout": 5 }] }
    ]
  }
}
`
}
