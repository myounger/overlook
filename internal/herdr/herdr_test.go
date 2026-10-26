package herdr

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// fakeHerdr records each call and answers from replies, keyed by the
// first two arguments ("pane split").
type fakeHerdr struct {
	calls   [][]string
	replies map[string]string
	fail    map[string]bool
}

func (f *fakeHerdr) run(name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	key := strings.Join(args[:2], " ")
	if f.fail[key] {
		return []byte(f.replies[key]), errors.New("exit status 1")
	}
	return []byte(f.replies[key]), nil
}

func newFake() (*fakeHerdr, Env) {
	f := &fakeHerdr{
		replies: map[string]string{
			"pane split":        `{"id":"cli:pane:split","result":{"pane":{"pane_id":"w1:p7"}}}`,
			"pane process-info": processInfo("/bin/zsh", "zsh"),
			"pane run":          `{"id":"cli:pane:run","result":{}}`,
			"pane close":        `{"id":"cli:pane:close","result":{}}`,
		},
		fail: map[string]bool{},
	}
	return f, Env{Run: f.run, Bin: "herdr", InHerdr: true, Installed: true, OS: "darwin"}
}

// processInfo is herdr 0.9.5's reply to `pane process-info` for a pane
// sitting at a shell prompt, trimmed to the fields that matter.
func processInfo(argv0, name string) string {
	return `{"id":"cli:pane:process_info","result":{"process_info":{"foreground_processes":[` +
		`{"argv0":` + strconv.Quote(argv0) + `,"name":` + strconv.Quote(name) + `,"pid":4242}],` +
		`"pane_id":"w1:p7","shell_pid":4242},"type":"pane_process_info"}}`
}

func TestSplitOpensPaneAndRunsCommand(t *testing.T) {
	f, env := newFake()
	if err := env.Split(35, "/repo", "/usr/local/bin/overlook", "--config", "/my config.yml"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"herdr", "pane", "split", "--current", "--direction", "right", "--ratio", "0.65", "--cwd", "/repo", "--no-focus"},
		{"herdr", "pane", "process-info", "--pane", "w1:p7"},
		{"herdr", "pane", "run", "w1:p7", "/usr/local/bin/overlook --config '/my config.yml'"},
	}
	if !slices.EqualFunc(f.calls, want, slices.Equal) {
		t.Errorf("calls:\n%q\nwant:\n%q", f.calls, want)
	}
}

// herdr types the command into the new pane's shell, so it must be quoted
// for that shell. On Windows herdr starts PowerShell, where a quoted path on
// its own is a string to print, not a program to run.
func TestSplitQuotesForThePanesShell(t *testing.T) {
	exe := `C:\Program Files\overlook\overlook.exe`
	cfg := `C:\Users\me\it's.yml`
	for _, tc := range []struct {
		name         string
		os           string
		argv0, pname string // the pane's shell; empty if herdr can't say
		want         string
	}{
		{"powershell", "windows", `C:\windows\System32\WindowsPowerShell\v1.0\powershell.exe`, "powershell.exe",
			`& 'C:\Program Files\overlook\overlook.exe' --config 'C:\Users\me\it''s.yml'`},
		{"pwsh", "windows", `C:\Program Files\PowerShell\7\pwsh.exe`, "pwsh.exe",
			`& 'C:\Program Files\overlook\overlook.exe' --config 'C:\Users\me\it''s.yml'`},
		{"cmd", "windows", `C:\windows\system32\cmd.exe`, "cmd.exe",
			`"C:\Program Files\overlook\overlook.exe" --config "C:\Users\me\it's.yml"`},
		{"git bash", "windows", `C:\Program Files\Git\usr\bin\bash.exe`, "bash.exe",
			`'C:\Program Files\overlook\overlook.exe' --config 'C:\Users\me\it'\''s.yml'`},
		{"unknown on windows", "windows", "", "",
			`& 'C:\Program Files\overlook\overlook.exe' --config 'C:\Users\me\it''s.yml'`},
		{"unknown elsewhere", "linux", "", "",
			`'C:\Program Files\overlook\overlook.exe' --config 'C:\Users\me\it'\''s.yml'`},
	} {
		f, env := newFake()
		env.OS = tc.os
		if tc.pname == "" {
			f.replies["pane process-info"] = `{"error":{"code":"unknown","message":"no process info"}}`
			f.fail["pane process-info"] = true
		} else {
			f.replies["pane process-info"] = processInfo(tc.argv0, tc.pname)
		}
		if err := env.Split(35, `C:\repo`, exe, "--config", cfg); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := f.calls[len(f.calls)-1][4]; got != tc.want {
			t.Errorf("%s:\ngot  %s\nwant %s", tc.name, got, tc.want)
		}
	}
}

func TestSplitOutsideHerdr(t *testing.T) {
	for _, tc := range []struct {
		installed bool
		want      error
	}{
		{false, ErrNotInstalled},
		{true, ErrNotInHerdr},
	} {
		f, env := newFake()
		env.InHerdr, env.Installed = false, tc.installed
		if err := env.Split(35, "/repo", "overlook"); err != tc.want {
			t.Errorf("installed=%v: got %v, want %v", tc.installed, err, tc.want)
		}
		if len(f.calls) != 0 {
			t.Errorf("installed=%v: ran herdr anyway: %q", tc.installed, f.calls)
		}
	}
}

func TestSplitReportsHerdrsMessage(t *testing.T) {
	f, env := newFake()
	f.replies["pane split"] = `{"error":{"code":"pane_not_found","message":"pane w1:p1 not found"},"id":"cli:pane:split"}`
	f.fail["pane split"] = true
	err := env.Split(35, "/repo", "overlook")
	if err == nil || err.Error() != "herdr pane split: pane w1:p1 not found" {
		t.Errorf("got %v", err)
	}
}

func TestSplitClosesPaneWhenRunFails(t *testing.T) {
	f, env := newFake()
	f.replies["pane run"] = `{"error":{"code":"pane_not_found","message":"pane w1:p7 not found"}}`
	f.fail["pane run"] = true
	if err := env.Split(35, "/repo", "overlook"); err == nil {
		t.Fatal("expected an error")
	}
	last := f.calls[len(f.calls)-1]
	if !slices.Equal(last, []string{"herdr", "pane", "close", "w1:p7"}) {
		t.Errorf("last call %q, want the new pane closed", last)
	}
}

func TestShellJoin(t *testing.T) {
	for _, tc := range []struct {
		sh   Shell
		args []string
		want string
	}{
		{POSIX, []string{"/Users/me/bin/overlook", "--config", "/Users/me/My Configs/it's.yml", "a;b"},
			`/Users/me/bin/overlook --config '/Users/me/My Configs/it'\''s.yml' 'a;b'`},
		// Plain Windows paths need no quotes, but PowerShell still gets &.
		{PowerShell, []string{`C:\Users\me\bin\overlook.exe`, "--config", `C:\cfg.yml`},
			`& C:\Users\me\bin\overlook.exe --config C:\cfg.yml`},
		// , and @ mean something to PowerShell, so they're quoted there.
		{PowerShell, []string{`C:\bin\overlook.exe`, "a,b", "@x"},
			`& C:\bin\overlook.exe 'a,b' '@x'`},
		{Cmd, []string{`C:\Users\me\bin\overlook.exe`, "--config", `C:\My Configs\cfg.yml`},
			`C:\Users\me\bin\overlook.exe --config "C:\My Configs\cfg.yml"`},
	} {
		if got := shellJoin(tc.sh, tc.args); got != tc.want {
			t.Errorf("shell %d:\ngot  %s\nwant %s", tc.sh, got, tc.want)
		}
	}
}

func TestShellNamed(t *testing.T) {
	for name, want := range map[string]Shell{
		"/bin/zsh": POSIX, "fish": POSIX, `C:\Program Files\Git\usr\bin\bash.exe`: POSIX,
		"powershell.exe": PowerShell, "PWSH.EXE": PowerShell, `C:\windows\system32\cmd.exe`: Cmd,
	} {
		if got, ok := shellNamed(name); !ok || got != want {
			t.Errorf("%s: got %d, %v; want %d", name, got, ok, want)
		}
	}
	if _, ok := shellNamed("nu"); ok {
		t.Error("nu: recognised a shell we don't know how to quote for")
	}
}
