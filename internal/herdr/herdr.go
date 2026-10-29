// Package herdr opens Overlook in a pane of its own when it's run inside
// herdr, the terminal multiplexer for coding agents (`overlook --split`).
// It works with both herdrs: the terminal UI (written against herdr 0.9.5),
// which sets HERDR_ENV in its panes, and the Herdr app, which runs its
// terminals in herdr-core, sets HC_TERMINAL_ID, and arranges them with
// `herdr-app`.
package herdr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
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
	App       string // the herdr-app binary
	InHerdr   bool   // HERDR_ENV is set: we're in a herdr pane
	Terminal  string // HC_TERMINAL_ID: our terminal, when we're in the Herdr app
	Installed bool   // Bin or App can be found
	OS        string // runtime.GOOS, for guessing a pane's shell when herdr can't say
}

// FromEnviron reads the variables herdr sets in each of its panes.
func FromEnviron() Env {
	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		bin = "herdr"
	}
	_, errBin := exec.LookPath(bin)
	_, errApp := exec.LookPath("herdr-app")
	return Env{
		Run:       execCommand,
		Bin:       bin,
		App:       "herdr-app",
		InHerdr:   os.Getenv("HERDR_ENV") != "",
		Terminal:  os.Getenv("HC_TERMINAL_ID"),
		Installed: errBin == nil || errApp == nil,
		OS:        runtime.GOOS,
	}
}

var (
	ErrNotInstalled = errors.New("--split opens Overlook in a herdr pane, but herdr isn't installed")
	ErrNotInHerdr   = errors.New("--split only works inside herdr; run it from a herdr pane")
)

// Split opens a pane to the right of the current one, in dir, without
// moving focus, taking width percent of the space, and runs command in it.
// herdr's --ratio is the share the original pane keeps, so it gets
// 100 - width. herdr types the command into the new pane's shell, so it's
// quoted for whichever shell that is. If the command can't be started, the
// new pane is closed.
//
// In the Herdr app it's appSplit instead. A herdr pane inside an app
// terminal has both variables set; the herdr pane is the nearer one.
func (e Env) Split(width int, dir string, command ...string) error {
	switch {
	case !e.InHerdr && e.Terminal != "":
		return e.appSplit(command)
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
	line := shellJoin(e.paneShell(p.Pane.PaneID), command)
	if err := e.call(nil, "pane", "run", p.Pane.PaneID, line); err != nil {
		_ = e.call(nil, "pane", "close", p.Pane.PaneID)
		return err
	}
	return nil
}

// appSplit opens a pane to the right of ours in the Herdr app, running
// command, and puts focus back on ours. The app starts the program itself,
// so nothing is quoted. Its pane.split takes neither a size nor a folder:
// the app picks the width, and command has to name the repo.
func (e Env) appSplit(command []string) error {
	pane, err := e.paneOf(e.Terminal)
	if err != nil {
		return err
	}
	if err := e.app("pane.split", map[string]any{"direction": "right", "pane": pane, "argv": command}); err != nil {
		return err
	}
	return e.app("pane.focus", map[string]any{"pane": pane})
}

// paneOf finds the pane showing terminal in `herdr-app ls`.
func (e Env) paneOf(terminal string) (string, error) {
	data, err := e.Run(e.App, "ls")
	if err != nil {
		return "", commandFailed("herdr-app ls", err, data)
	}
	var ls struct {
		Spaces []struct {
			Tabs []struct {
				Terminals []struct {
					Terminal string `json:"terminal"`
					Pane     string `json:"pane"`
				} `json:"terminals"`
			} `json:"tabs"`
		} `json:"spaces"`
	}
	if err := json.Unmarshal(data, &ls); err != nil {
		return "", fmt.Errorf("herdr-app ls: unexpected reply %q", strings.TrimSpace(string(data)))
	}
	for _, s := range ls.Spaces {
		for _, t := range s.Tabs {
			for _, p := range t.Terminals {
				if p.Terminal == terminal && p.Pane != "" {
					return p.Pane, nil
				}
			}
		}
	}
	return "", fmt.Errorf("terminal %s has no pane in the Herdr window", terminal)
}

// app runs a herdr-app command with its arguments as one JSON object.
// herdr-app prints {"error": "..."} when it refuses.
func (e Env) app(command string, args map[string]any) error {
	params, err := json.Marshal(args)
	if err != nil {
		return err
	}
	data, err := e.Run(e.App, "do", command, string(params))
	var resp struct {
		Error string `json:"error"`
	}
	if jerr := json.Unmarshal(data, &resp); jerr == nil && resp.Error != "" {
		return fmt.Errorf("herdr-app %s: %s", command, resp.Error)
	}
	if err != nil {
		return commandFailed("herdr-app "+command, err, data)
	}
	return nil
}

// commandFailed describes a command that exited badly, with what it printed.
func commandFailed(what string, err error, out []byte) error {
	if text := strings.TrimSpace(string(out)); text != "" {
		return fmt.Errorf("%s: %v: %s", what, err, text)
	}
	return fmt.Errorf("%s: %v", what, err)
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
		return commandFailed(what, err, data)
	case out == nil:
		return nil
	case jerr != nil || len(resp.Result) == 0:
		return fmt.Errorf("%s: unexpected reply %q", what, strings.TrimSpace(string(data)))
	}
	return json.Unmarshal(resp.Result, out)
}

// Shell is the family of shell a pane runs, which decides how a command
// line typed into it has to be quoted.
type Shell int

const (
	POSIX      Shell = iota // sh, bash, zsh, fish
	PowerShell              // Windows PowerShell and pwsh
	Cmd                     // cmd.exe
)

// paneShell asks herdr which shell the pane is running. If herdr can't say
// or it's a shell we don't know, it falls back to herdr's default for the
// OS: PowerShell on Windows, a POSIX shell everywhere else.
func (e Env) paneShell(pane string) Shell {
	var p struct {
		ProcessInfo struct {
			ShellPID  int `json:"shell_pid"`
			Processes []struct {
				PID  int    `json:"pid"`
				Name string `json:"name"`
			} `json:"foreground_processes"`
		} `json:"process_info"`
	}
	if e.call(&p, "pane", "process-info", "--pane", pane) == nil {
		for _, proc := range p.ProcessInfo.Processes {
			if proc.PID != p.ProcessInfo.ShellPID {
				continue
			}
			if sh, ok := shellNamed(proc.Name); ok {
				return sh
			}
		}
	}
	if e.OS == "windows" {
		return PowerShell
	}
	return POSIX
}

// shellNamed recognises a shell from its process name or path, e.g.
// "pwsh.exe" or "/bin/zsh".
func shellNamed(name string) (Shell, bool) {
	name = strings.ToLower(name[strings.LastIndexAny(name, `/\`)+1:])
	switch strings.TrimSuffix(name, ".exe") {
	case "sh", "bash", "zsh", "fish", "dash", "ksh":
		return POSIX, true
	case "powershell", "pwsh":
		return PowerShell, true
	case "cmd":
		return Cmd, true
	}
	return POSIX, false
}

var (
	shellSafe   = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)
	windowsSafe = regexp.MustCompile(`^[A-Za-z0-9_:./\\-]+$`)
)

// shellJoin builds a command line for the pane's shell, since herdr types
// the command in rather than running it directly. Plain path characters are
// left alone and anything else is quoted:
//   - POSIX shells: single quotes, which bash, zsh and fish read the same way.
//   - PowerShell: single quotes too, but a quoted string on its own is just a
//     value there, so the line starts with the call operator, &.
//   - cmd: double quotes. Windows paths can't contain a double quote, so
//     there's nothing to escape inside them.
func shellJoin(sh Shell, args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = quoteArg(sh, a)
	}
	line := strings.Join(quoted, " ")
	if sh == PowerShell {
		return "& " + line
	}
	return line
}

func quoteArg(sh Shell, a string) string {
	switch sh {
	case PowerShell:
		if windowsSafe.MatchString(a) {
			return a
		}
		return "'" + strings.ReplaceAll(a, "'", "''") + "'"
	case Cmd:
		if windowsSafe.MatchString(a) {
			return a
		}
		return `"` + a + `"`
	}
	if shellSafe.MatchString(a) {
		return a
	}
	return "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
}
