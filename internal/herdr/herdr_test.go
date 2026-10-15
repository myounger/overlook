package herdr

import (
	"errors"
	"slices"
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
			"pane split": `{"id":"cli:pane:split","result":{"pane":{"pane_id":"w1:p7"}}}`,
			"pane run":   `{"id":"cli:pane:run","result":{}}`,
			"pane close": `{"id":"cli:pane:close","result":{}}`,
		},
		fail: map[string]bool{},
	}
	return f, Env{Run: f.run, Bin: "herdr", InHerdr: true, Installed: true}
}

func TestSplitOpensPaneAndRunsCommand(t *testing.T) {
	f, env := newFake()
	if err := env.Split(35, "/repo", "/usr/local/bin/overlook", "--config", "/my config.yml"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"herdr", "pane", "split", "--current", "--direction", "right", "--ratio", "0.65", "--cwd", "/repo", "--no-focus"},
		{"herdr", "pane", "run", "w1:p7", "/usr/local/bin/overlook --config '/my config.yml'"},
	}
	if !slices.EqualFunc(f.calls, want, slices.Equal) {
		t.Errorf("calls:\n%q\nwant:\n%q", f.calls, want)
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
	got := shellJoin([]string{"/Users/me/bin/overlook", "--config", "/Users/me/My Configs/it's.yml", "a;b"})
	want := `/Users/me/bin/overlook --config '/Users/me/My Configs/it'\''s.yml' 'a;b'`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
