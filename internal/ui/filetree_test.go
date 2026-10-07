package ui

import (
	"strings"
	"testing"

	"github.com/myounger/overlook/internal/git"
)

func files(paths ...string) []git.File {
	out := make([]git.File, len(paths))
	for i, p := range paths {
		out[i] = git.File{Path: p, Staged: '.', Unstaged: 'M'}
	}
	return out
}

// outline renders rows as indented names so trees are easy to compare.
func outline(rows []row) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(strings.Repeat("  ", r.depth) + r.node.name)
		if r.node.isDir() {
			b.WriteString("/")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestTreeRows(t *testing.T) {
	fs := files("main.go", "internal/ui/model.go", "internal/ui/files.go", "internal/git/git.go", "README.md", "a/b/c/deep.txt")
	tests := []struct {
		name      string
		compact   bool
		collapsed map[string]bool
		want      string
	}{
		{
			name:    "compact folders first, then files, sorted",
			compact: true,
			want: `a/b/c/
  deep.txt
internal/
  git/
    git.go
  ui/
    files.go
    model.go
README.md
main.go
`,
		},
		{
			name: "not compact",
			want: `a/
  b/
    c/
      deep.txt
internal/
  git/
    git.go
  ui/
    files.go
    model.go
README.md
main.go
`,
		},
		{
			name:      "collapsed folder hides its contents",
			compact:   true,
			collapsed: map[string]bool{"internal": true},
			want: `a/b/c/
  deep.txt
internal/
README.md
main.go
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := outline(treeRows(buildTree(fs, tt.compact), tt.collapsed))
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestTreeCountsAndUntrackedFolders(t *testing.T) {
	root := buildTree(files("src/a.go", "src/b.go", "src/sub/c.go", "newdir/"), true)
	if root.files != 4 {
		t.Errorf("root counts %d files, want 4", root.files)
	}
	got := outline(treeRows(root, nil))
	want := "src/\n  sub/\n    c.go\n  a.go\n  b.go\nnewdir/\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	for _, r := range treeRows(root, nil) {
		if r.node.path == "src" && r.node.files != 3 {
			t.Errorf("src counts %d files, want 3", r.node.files)
		}
	}
}

func TestFilesPanelKeepsCursorOnRefresh(t *testing.T) {
	p := newFilesPanel(newFilesConfig())
	p.setSize(40, 10)
	p.setFiles(files("b.go", "c.go"))
	p.move(1) // on c.go
	p.setFiles(files("a.go", "b.go", "c.go"))
	if got := p.rows[p.cursor].node.path; got != "c.go" {
		t.Errorf("cursor on %q after a file was added above, want c.go", got)
	}
	p.setFiles(files("a.go", "b.go"))
	if got := p.rows[p.cursor].node.path; got != "b.go" {
		t.Errorf("cursor on %q after its file went away, want b.go", got)
	}
	p.setFiles(nil)
	if p.cursor != 0 {
		t.Errorf("cursor %d with no files", p.cursor)
	}
}

func TestFilesPanelScrollsToCursor(t *testing.T) {
	p := newFilesPanel(newFilesConfig())
	p.setSize(40, 5) // 3 visible rows
	p.setFiles(files("1", "2", "3", "4", "5", "6"))
	p.move(4)
	if p.offset != 2 {
		t.Errorf("offset %d, want 2", p.offset)
	}
	p.move(-1)
	if p.offset != 2 {
		t.Errorf("offset moved to %d while cursor was still visible", p.offset)
	}
	p.move(-10)
	if p.offset != 0 {
		t.Errorf("offset %d at top, want 0", p.offset)
	}
}
