package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/myounger/overlook/internal/config"
	"github.com/myounger/overlook/internal/git"
)

const twoFileDiff = `diff --git a/src/a.go b/src/a.go
index 111..222 100644
--- a/src/a.go
+++ b/src/a.go
@@ -1,2 +1,3 @@ package a
 package a
-func Old() {}
+func New() {}
+func	Tabbed() {}
diff --git a/src/b.go b/src/b.go
new file mode 100644
index 000..333
--- /dev/null
+++ b/src/b.go
@@ -0,0 +1 @@
+package b
\ No newline at end of file
diff --git a/img.png b/img.png
index 444..555 100644
Binary files a/img.png and b/img.png differ
`

func plainLines(lines []diffLine) string {
	var out []string
	for _, l := range lines {
		out = append(out, l.text)
	}
	return strings.Join(out, "\n")
}

func TestRenderDiffMultiFile(t *testing.T) {
	cfg, _ := config.Load("")
	lines, adds, dels := renderDiff(newStyles(cfg.Theme), twoFileDiff, true, true, 100)
	if adds != 3 || dels != 1 {
		t.Errorf("adds %d dels %d, want 3 and 1", adds, dels)
	}
	want := `src/a.go
@@ -1,2 +1,3 @@ package a
 package a
-func Old() {}
+func New() {}
+func    Tabbed() {}

src/b.go (new)
@@ -0,0 +1 @@
+package b
\ No newline at end of file

img.png
binary file changed`
	if got := plainLines(lines); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderDiffSingleFileHidesHeader(t *testing.T) {
	cfg, _ := config.Load("")
	raw := "diff --git a/x b/x\nindex 1..2 100644\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n"
	lines, _, _ := renderDiff(newStyles(cfg.Theme), raw, false, true, 100)
	if got := plainLines(lines); got != "@@ -1 +1 @@\n-a\n+b" {
		t.Errorf("got:\n%s", got)
	}
	rename := "diff --git a/old b/new\nsimilarity index 100%\nrename from old\nrename to new\n"
	lines, _, _ = renderDiff(newStyles(cfg.Theme), rename, false, true, 100)
	if got := plainLines(lines); got != "renamed from old" {
		t.Errorf("pure rename: got %q", got)
	}
}

func TestRenderDiffCapsLines(t *testing.T) {
	cfg, _ := config.Load("")
	var b strings.Builder
	b.WriteString("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -0,0 +1,10 @@\n")
	for i := range 10 {
		fmt.Fprintf(&b, "+line %d\n", i)
	}
	lines, adds, _ := renderDiff(newStyles(cfg.Theme), b.String(), false, true, 5)
	if len(lines) != 6 || adds != 10 {
		t.Fatalf("%d lines, %d adds", len(lines), adds)
	}
	if !strings.Contains(lines[5].text, "6 more lines") {
		t.Errorf("last line %q", lines[5].text)
	}
}

func TestDiffPanelScrollLimits(t *testing.T) {
	cfg, _ := config.Load("")
	st := newStyles(cfg.Theme)
	cfg.Panels.Diff.Wrap = false
	p := newDiffPanel(cfg.Panels.Diff)
	p.setSize(24, 6) // 4 rows, 20 columns of text
	p.selectTarget("k", "src/")
	p.setDiff(st, "k", twoFileDiff, "", "", nil, true)
	p.scroll(100)
	if p.offset != len(p.lines)-4 {
		t.Errorf("offset %d, want %d", p.offset, len(p.lines)-4)
	}
	p.scrollX(100)
	widest := len(`\ No newline at end of file`)
	if want := widest - 20; p.xoff != want {
		t.Errorf("xoff %d, want %d", p.xoff, want)
	}
	p.scrollX(-1000)
	p.scroll(-1000)
	if p.offset != 0 || p.xoff != 0 {
		t.Errorf("offset %d xoff %d after scrolling back", p.offset, p.xoff)
	}
	// A result for a selection you've moved away from is dropped.
	p.selectTarget("other", "x")
	p.setDiff(st, "k", twoFileDiff, "", "", nil, true)
	if p.lines != nil {
		t.Error("stale diff was shown")
	}
}

// TestRenderFillsWindow checks every layout fills the window exactly.
func TestRenderFillsWindow(t *testing.T) {
	cfg, _ := config.Load("")
	files := []git.File{{Path: "src/a.go", Staged: '.', Unstaged: 'M'}, {Path: "src/b.go", Staged: '?', Unstaged: '?'}}
	for _, size := range [][2]int{{60, 30}, {140, 40}, {100, 12}, {30, 8}} {
		for _, zoom := range []bool{false, true} {
			var m tea.Model = New(cfg, git.Repo{Root: "/r", GitDir: "/r/.git", CommonDir: "/r/.git"}, nil)
			m, _ = m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m, _ = m.Update(statusMsg{root: "/r", status: git.Status{Head: "main", Files: files}})
			m, _ = m.Update(branchesMsg{root: "/r", branches: []git.Branch{{Name: "main", Current: true}}})
			if zoom {
				m, _ = m.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
			}
			out := m.(Model).render()
			lines := strings.Split(out, "\n")
			if len(lines) != size[1] {
				t.Errorf("%dx%d zoom=%v: %d lines", size[0], size[1], zoom, len(lines))
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w != size[0] && !(i == len(lines)-1 && w < size[0]) {
					t.Errorf("%dx%d zoom=%v: line %d width %d: %q", size[0], size[1], zoom, i, w, ansi.Strip(l))
				}
			}
			if right := m.(Model).diffRight; right != (size[0] >= 100 && !zoom) && !zoom {
				t.Errorf("%dx%d: diffRight=%v", size[0], size[1], right)
			}
		}
	}
}

func TestDiffPanelWraps(t *testing.T) {
	cfg, _ := config.Load("")
	st := newStyles(cfg.Theme)
	p := newDiffPanel(cfg.Panels.Diff)
	p.setSize(15, 10) // 11 columns of text
	p.selectTarget("k", "x")
	p.setDiff(st, "k", "@@ -1 +1 @@\n+abcdefghijklmnopqrstuvwxy\n", "", "", nil, false)
	out := p.view(st, true)
	var got []string
	for _, l := range strings.Split(ansi.Strip(out), "\n")[1:5] {
		got = append(got, strings.TrimRight(strings.Trim(l, "│"), " "))
	}
	// A word longer than the row is cut; continuation rows keep the +
	// column blank.
	want := []string{" ── line 1 ─", " +abcdefghij", "  klmnopqrst", "  uvwxy"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: got %q, want %q", i, got[i], want[i])
		}
	}
	// Wrapped continuation rows keep the added-line color.
	row := strings.Split(out, "\n")[3]
	if !strings.Contains(row, "\x1b[") {
		t.Errorf("continuation row lost its color: %q", row)
	}
	p.scrollX(8)
	if p.xoff != 0 {
		t.Error("sideways scroll should do nothing while wrapping")
	}
}

func TestPanelOrder(t *testing.T) {
	names := map[panelID]string{filesID: "files", diffID: "diff", branchesID: "branches"}
	for order, want := range map[string]string{
		"":                       "files diff branches",
		"branches, files, diff":  "branches files diff",
		"diff":                   "diff files branches",
		"branches, diff, files ": "branches diff files",
	} {
		cfg, _ := config.Load("")
		if order != "" {
			cfg.Layout.Order = strings.Split(strings.ReplaceAll(order, " ", ""), ",")
		}
		var got []string
		for _, id := range New(cfg, git.Repo{}, nil).shownPanels() {
			got = append(got, names[id])
		}
		if strings.Join(got, " ") != want {
			t.Errorf("order %q: got %v, want %s", order, got, want)
		}
	}
	cfg, _ := config.Load("")
	cfg.Panels.Diff.Show = false
	if got := New(cfg, git.Repo{}, nil).shownPanels(); len(got) != 2 || got[1] != branchesID {
		t.Errorf("hidden diff: got %v", got)
	}
}

func TestHunkLabel(t *testing.T) {
	tests := map[string]string{
		"@@ -30,6 +30,10 @@ theme:":      "line 30 · theme:",
		"@@ -55,6 +59,20 @@ panels:":     "line 59 · panels:",
		"@@ -1 +1 @@":                    "line 1",
		"@@ -0,0 +1,3 @@":                "line 1",
		"@@ -12,4 +0,0 @@":               "line 12", // the whole block was removed
		"@@ -3,2 +3,2 @@ func  main() {": "line 3 · func  main() {",
	}
	for in, want := range tests {
		if got, ok := hunkLabel(in); !ok || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, ok, want)
		}
	}
	if _, ok := hunkLabel("@@@ -1,2 -1,2 +1,3 @@@"); ok {
		t.Error("a merge conflict header should be left as is")
	}
}

func TestRenderDiffFriendlyHunks(t *testing.T) {
	cfg, _ := config.Load("")
	lines, _, _ := renderDiff(newStyles(cfg.Theme), "@@ -30,6 +30,10 @@ theme:\n x\n+y\n", false, false, 100)
	if !lines[0].rule || lines[0].text != "line 30 · theme:" {
		t.Errorf("got %+v", lines[0])
	}
	if got := ruleLine("line 30 · theme:", 24); got != "── line 30 · theme: ────" {
		t.Errorf("rule %q", got)
	}
	if got := ruleLine("line 30 · a very long section name", 20); lipgloss.Width(got) != 20 {
		t.Errorf("long rule %q is %d wide", got, lipgloss.Width(got))
	}
}

func TestWrapWords(t *testing.T) {
	tests := []struct {
		in   string
		w    int
		want []string
	}{
		// Continuation rows are indented like the first row.
		{`  gone: "1"   # "gone": the remote branch was deleted`, 24,
			[]string{`  gone: "1"   # "gone":`, `  the remote branch was`, `  deleted`}},
		// Indentation is never a break point; a too-long word is cut.
		{`    a_very_long_identifier_with_no_spaces = 1`, 20,
			[]string{`    a_very_long_iden`, `    tifier_with_no_s`, `    paces = 1`}},
		// Indentation is capped at half the width so rows still have room.
		{`            deep and long line`, 16,
			[]string{`            deep`, `        and long`, `        line`}},
		{`short`, 20, []string{`short`}},
		{`exactly ten`, 11, []string{`exactly ten`}},
	}
	for _, tt := range tests {
		got := wrapWords(tt.in, tt.w)
		if strings.Join(got, "|") != strings.Join(tt.want, "|") {
			t.Errorf("wrapWords(%q, %d):\n got %q\nwant %q", tt.in, tt.w, got, tt.want)
		}
		for _, row := range got {
			if lipgloss.Width(row) > tt.w {
				t.Errorf("row %q wider than %d", row, tt.w)
			}
		}
	}
}
