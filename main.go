// Overlook is a read-only terminal viewer for a git repo.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/myounger/overlook/internal/claudehook"
	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
	"github.com/myounger/overlook/internal/ui"
	"github.com/myounger/overlook/internal/watch"
)

// version is set at build time by the Makefile.
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		os.Exit(runHook(os.Args[2:]))
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "overlook:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "config file (default ~/.config/overlook/config.yml)")
	printDefault := flag.Bool("default-config", false, "print the default config and exit")
	printVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: overlook [flags] [path]")
		fmt.Fprintln(os.Stderr, "       overlook hook --settings")
		fmt.Fprintln(os.Stderr, "\nShows the git repo at path (default: the current folder).")
		fmt.Fprintln(os.Stderr, "`overlook hook --settings` prints the Claude Code hooks that let Overlook\nfollow Claude into worktrees; add them to ~/.claude/settings.json.\n\nFlags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	switch {
	case *printVersion:
		fmt.Println("overlook", version)
		return nil
	case *printDefault:
		_, err := os.Stdout.Write(config.Default)
		return err
	}

	if *configPath == "" {
		p, err := config.Path()
		if err != nil {
			return err
		}
		*configPath = p
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	repo, err := git.Locate(dir)
	if err != nil {
		return err
	}

	// HEAD lives in this worktree's git dir; branch and remote refs, and the
	// list of linked worktrees, live in the shared one.
	w, err := watch.New(
		[]string{repo.GitDir, repo.CommonDir},
		[]string{filepath.Join(repo.CommonDir, "refs"), filepath.Join(repo.CommonDir, "worktrees")},
		cfg.Refresh.Debounce,
	)
	if err != nil {
		return fmt.Errorf("watching %s: %w", repo.GitDir, err)
	}
	defer w.Close()

	model := ui.New(cfg, repo, w)
	// Live reload: watch the config file's folder, since many editors save
	// by replacing the file. If the folder doesn't exist yet, reload is off.
	if cw, err := watch.New([]string{filepath.Dir(*configPath)}, nil, 100*time.Millisecond); err == nil {
		defer cw.Close()
		model = model.WithConfigFile(*configPath, cw)
	}

	// Claude Code hook: watch this repo's folder of session files.
	if dir, err := claudehook.RepoDir(repo.CommonDir); err == nil && os.MkdirAll(dir, 0o755) == nil {
		if hw, err := watch.New([]string{dir}, nil, 100*time.Millisecond); err == nil {
			defer hw.Close()
			model = model.WithClaudeHook(hw)
		}
	}

	_, err = tea.NewProgram(model).Run()
	return err
}

// runHook is what Claude Code runs on each hook event (see
// `overlook hook --settings`). It records where the session is working and
// always succeeds quietly: a hook must never get in Claude's way.
func runHook(args []string) int {
	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	settings := fs.Bool("settings", false, "print the hooks to add to ~/.claude/settings.json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *settings {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "overlook:", err)
			return 1
		}
		fmt.Print(claudehook.Settings(git.CanonicalPath(exe)))
		return 0
	}
	_ = claudehook.Record(os.Stdin, time.Now())
	return 0
}
