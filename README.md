# Overlook

A read-only terminal view of a git repo, made to sit in a small panel next to [Claude Code](https://claude.com/claude-code) so you can watch what changes as it works. It follows Claude into git worktrees.

```
 overlook 2   claude-fix ✻ 3
╭─ Status ──────────────────────────────────────╮
│ overlook ⎇ claude-fix → worktree-fix ↑1       │
╰───────────────────────────────────────────────╯
╭─ Files · 3 ───────────────────────────────────╮
│ ▼ internal/ui                                 │
│      M model.go                               │
│     ?? pull.go                                │
│  M README.md                                  │
╰───────────────────────────────────────────────╯
╭─ Diff · internal/ui/model.go +12 -3 ──────────╮
│ ── line 42 · func (m Model) Update ────────── │
│      case tea.KeyPressMsg:                    │
│ -        return m.handleKey(msg.String())     │
│ +        return m.handleKey(msg)              │
╰───────────────────────────────────────────────╯
╭─ Branches · 4 ────────────────────────────────╮
│ 2m * worktree-fix ⎇ claude-fix                │
│ 1h   main ✓                                   │
╰───────────────────────────────────────────────╯
 enter fold/diff · ` flat · p pull · z zoom · q quit
```

## Why

[lazygit](https://github.com/jesseduffield/lazygit) is great, but as a side panel it shows more than you need, and you can't hide the panels you don't use. And when Claude Code moves into a git worktree, lazygit stays on the main folder. Overlook shows only what you choose, and goes where Claude goes.

## What it does

- **Files**: changed files as a foldable tree or a flat list, with lazygit-style status letters (staged green, unstaged red) and names colored by kind of change.
- **Diff**: the selected file's or folder's changes since the last commit. Long lines wrap, and each block of changes starts with `── line 42 · func … ──` instead of git's `@@` header. Optionally piped through [delta](https://github.com/dandavison/delta).
- **Branches**: local branches with last-commit age, ahead/behind, the worktree each is checked out in, and cleanup hints: **gone** (the remote branch was deleted, e.g. after a PR merged) and **merged**.
- **Log** (off by default): recent commits, with `↑` on unpushed ones. Selecting one shows its diff.
- **Worktrees**: a tab for the main folder and each worktree, with change counts. Overlook switches to a new worktree as soon as it's created, and falls back to the main folder if the one you're viewing is removed.
- **Live**: branch switches, staging, commits, and fetches show up instantly; file edits within two seconds.
- **Read-only**, with one exception: **pull**, which only fast-forwards (see below).
- **Configurable**: every color, key, and panel, and the layout. Changes apply as soon as you save.

## Install

**Homebrew** (macOS and Linux):

```sh
brew install --cask myounger/tap/overlook
```

**Download**: grab the archive for your platform from the [latest release](https://github.com/myounger/overlook/releases/latest) (macOS, Linux, and Windows; Intel and ARM), unpack it, and put `overlook` somewhere on your `PATH`.

**With Go** (1.27 or newer):

```sh
go install github.com/myounger/overlook@latest
```

**From source**:

```sh
git clone https://github.com/myounger/overlook && cd overlook
make install   # builds and copies the binary to ~/.local/bin
```

Overlook needs `git` on your `PATH`.

## Use

Run `overlook` inside a repo, or `overlook path/to/repo`.

| Key | Does |
|---|---|
| `tab` / `shift+tab` | move between panels |
| `j` `k` / arrows | move; in the diff, scroll |
| `g` / `G`, `ctrl+u` / `ctrl+d` | top / bottom, page up / down |
| `enter` | fold a folder; on a file or commit, open its diff |
| `esc` | back from the diff, or out of zoom |
| `-` / `=` | fold / unfold every folder |
| `` ` `` | Files as a tree or a flat list |
| `z` | zoom: the focused panel fills the window |
| `[` / `]` | previous / next worktree |
| `p` | pull |
| `r` | refresh now (everything refreshes on its own) |
| `q` | quit |

The mouse works too: the wheel scrolls the panel under the pointer, clicking a row selects it (click again to fold a folder or open a diff), and clicking a worktree tab switches to it. Hold Shift (Option in iTerm2) to select text, or set `layout.mouse: false` to turn the mouse off.

## Following Claude Code

On its own, Overlook switches to a worktree as soon as one appears. For an exact answer, let Claude Code tell it which worktree each session is working in:

```sh
overlook hook --settings
```

This prints a `hooks` block to add to `~/.claude/settings.json`. If you already have hooks for `PostToolUse`, `SessionStart`, or `SessionEnd`, add Overlook's entries next to them rather than replacing yours. After that, whenever a session moves to a different worktree, Overlook follows, and the worktree's tab shows ✻ while a session works there. If you switch away by hand while Claude keeps working in the same place, Overlook stays where you put it.

The hook only observes: it runs in the background after each tool call, prints nothing, and always succeeds, so it can't slow Claude down or change what it does. It keeps one small file per session in `~/.local/state/overlook/claude/` and removes it when the session ends. Set `worktrees.followClaude: false` to stop following without removing the hook.

## Pull

`p` runs `git pull --ff-only`: your branch either fast-forwards to its upstream or nothing changes. It never merges, rebases, or leaves a conflict. It also can't stop to ask for a password, which would freeze the screen: if git needs credentials it can't get from your keychain or ssh-agent, pull fails with a message saying so. Set `pull.enabled: false` to make Overlook strictly read-only.

## Configure

```sh
mkdir -p ~/.config/overlook
overlook --default-config > ~/.config/overlook/config.yml
```

The file lists every setting with a comment; delete what you don't change. Some highlights:

- `panels.<name>.show` and `size`, and `layout.order`: which panels appear, how much room each gets, and their order.
- `layout.expandFocused`: the list panel you're in gets the room, and the others shrink to a few rows.
- `panels.diff.position`: diff on the right in wide windows, below in narrow ones, or always one or the other.
- `panels.diff.pager`: e.g. `delta --paging=never` for syntax highlighting.
- `worktrees.follow`: also switch to whichever worktree has files changing.
- `theme.*`: every color, as an ANSI number (follows your terminal's theme) or `#hex`.
- `keys.*`: every key.

Overlook picks up changes as soon as you save. If the file has a mistake, it says which line and keeps the old settings.

## Build

```sh
make build              # ./overlook
make test               # go vet and the tests
make install            # build and copy to ~/.local/bin
make release-snapshot   # build every release platform into dist/, publishing nothing
```

Releases are made by pushing a version tag (`git tag v0.1.0 && git push origin v0.1.0`). GitHub Actions then runs [GoReleaser](https://goreleaser.com), which publishes the archives and updates the Homebrew cask.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), and [fsnotify](https://github.com/fsnotify/fsnotify).

## License

[MIT](LICENSE)
