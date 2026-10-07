# Overlook

A read-only terminal viewer for a git repo, meant to run in a panel next to Claude Code so you can watch what changes as it works.

Think lazygit, but with only the panels you want and no ability to change anything - with one exception, pulling the latest code.

## Goals

- **Read-only.** Changed files, diffs, staged vs unstaged, recent commits, current branch. No staging, committing, or discarding.
- **One write action: pull.** Always `git pull --ff-only`, so it either fast-forwards cleanly or refuses. It never creates a merge or a conflict.
- **Follows worktrees.** When Claude Code spins off a git worktree, Overlook notices and can switch to it, instead of staying stuck on the main folder the way lazygit does.
- **Configurable layout.** Choose which panels show, their sizes, and keybindings from a config file.

## Build and install

You need Go (`brew install go`). Then, from this folder:

```sh
make build     # compiles ./overlook
make install   # builds, then copies it to ~/.local/bin (must be on your PATH)
make test      # go vet + unit tests
```

Run `overlook` inside any repo, or `overlook path/to/repo`. Press `r` to refresh and `q` to quit.

## Configuration

`overlook --default-config` prints every setting with comments. Save it to `~/.config/overlook/config.yml` and edit it. Settings you leave out keep their defaults, and a misspelled key is reported as an error.

## Planned stack

- **Go** - builds to a single binary with no runtime to install. lazygit is also Go, so its source is a useful reference.
- **Bubble Tea** (TUI framework), **Lip Gloss** (styling), **Bubbles** (lists, viewports).
- **The `git` CLI** for all git data (`git status --porcelain=v2`, `git diff`, `git log`, `git worktree list --porcelain`) rather than a git library.
- **fsnotify** to watch the repo and refresh live, debounced so a burst of edits doesn't trigger a burst of refreshes.
- **Config** in `~/.config/overlook/config.yml`.

## Worktree detection

1. Watch `.git/worktrees/` in the main repo. Git adds a folder there for each linked worktree and removes it when the worktree goes away. On a change, run `git worktree list --porcelain` for paths and branches.
2. Decide which worktree to show: auto-switch to a newly created one, follow whichever has the most recent file activity, and show a tab strip of all worktrees with change counts.
3. Optional, later: a Claude Code hook that writes the session's working directory to a file Overlook watches, for an exact answer instead of a guess.

As of October 2026, none of lazygit, gitui, tig, or delta follows a worktree created outside the tool. lazygit only switches manually (Worktrees tab).

## Phases

1. **MVP** - changed-files list, diff pane, live refresh, keyboard navigation.
2. **Worktrees** - detection, auto-switch, tab strip.
3. **Comfort** - syntax-highlighted diffs (or pipe through `delta`), staged/unstaged sections, commit log panel, pull.
4. **Configurability** - panels on/off, layout, sizes, keybindings.
5. **Distribution** - release builds, maybe a Homebrew tap.
