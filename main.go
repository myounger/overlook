// Overlook is a read-only terminal viewer for a git repo.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
	"github.com/myounger/overlook/internal/ui"
	"github.com/myounger/overlook/internal/watch"
)

// version is set at build time by the Makefile.
var version = "dev"

func main() {
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
		fmt.Fprintln(os.Stderr, "\nShows the git repo at path (default: the current folder).\n\nFlags:")
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

	_, err = tea.NewProgram(ui.New(cfg, repo, w)).Run()
	return err
}
