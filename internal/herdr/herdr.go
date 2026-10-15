// Package herdr opens Overlook in a pane of its own when it's run inside
// herdr, the terminal multiplexer for coding agents (`overlook --split`).
// Written against herdr 0.9.5.
package herdr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Runner runs a program and returns everything it printed. Tests swap it out.
type Runner func(name string, args ...string) ([]byte, error)

func execCommand(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

type Env struct {
	Run       Runner
	Bin       string // the herdr binary
	InHerdr   bool   // HERDR_ENV is set: we're in a herdr pane
	Installed bool   // Bin can be found
}

// FromEnviron reads the variables herdr sets in each of its panes.
func FromEnviron() Env {
	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		bin = "herdr"
	}
	_, err := exec.LookPath(bin)
	return Env{
		Run:       execCommand,
		Bin:       bin,
		InHerdr:   os.Getenv("HERDR_ENV") != "",
		Installed: err == nil,
	}
}

var (
	ErrNotInstalled = errors.New("--split opens Overlook in a herdr pane, but herdr isn't installed")
	ErrNotInHerdr   = errors.New("--split only works inside herdr; run it from a herdr pane")
)

// Split opens a pane to the right of the current one, in dir, without
// moving focus, taking width percent of the space, and runs command in it.
// herdr's --ratio is the share the original pane keeps, so it gets
// 100 - width. If the command can't be started, the new pane is closed.
func (e Env) Split(width int, dir string, command ...string) error {
	switch {
	case !e.InHerdr && !e.Installed:
		return ErrNotInstalled
	case !e.InHerdr:
		return ErrNotInHerdr
	}
	var p struct {
		Pane struct {
			PaneID string `json:"pane_id"`
		} `json:"pane"`
	}
	ratio := fmt.Sprintf("%.2f", float64(100-width)/100)
	if err := e.call(&p, "pane", "split", "--current", "--direction", "right", "--ratio", ratio, "--cwd", dir, "--no-focus"); err != nil {
		return err
	}
	if p.Pane.PaneID == "" {
		return errors.New("herdr pane split returned no pane id")
	}
	if err := e.call(nil, "pane", "run", p.Pane.PaneID, shellJoin(command)); err != nil {
		_ = e.call(nil, "pane", "close", p.Pane.PaneID)
		return err
	}
	return nil
}

// call runs a herdr subcommand and decodes .result into out. herdr prints
// {"error": {"code", "message"}} when it refuses; that message is the error.
func (e Env) call(out any, args ...string) error {
	data, err := e.Run(e.Bin, args...)
	what := "herdr " + strings.Join(args[:2], " ")
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	jerr := json.Unmarshal(data, &resp)
	switch {
	case jerr == nil && resp.Error != nil:
		return fmt.Errorf("%s: %s", what, resp.Error.Message)
	case err != nil:
		if text := strings.TrimSpace(string(data)); text != "" {
			return fmt.Errorf("%s: %v: %s", what, err, text)
		}
		return fmt.Errorf("%s: %v", what, err)
	case out == nil:
		return nil
	case jerr != nil || len(resp.Result) == 0:
		return fmt.Errorf("%s: unexpected reply %q", what, strings.TrimSpace(string(data)))
	}
	return json.Unmarshal(resp.Result, out)
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellJoin builds a command line for the pane's shell, since herdr types
// the command in rather than running it directly. Anything beyond plain
// path characters is single-quoted, which bash, zsh and fish all read the
// same way.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		if shellSafe.MatchString(a) {
			quoted[i] = a
		} else {
			quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(quoted, " ")
}
