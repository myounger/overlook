package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/myounger/overlook/internal/config"
)

func TestRenderPanelIsExactSize(t *testing.T) {
	cfg, _ := config.Load("")
	st := newStyles(cfg.Theme)
	for _, size := range [][2]int{{40, 3}, {20, 6}, {10, 3}} {
		w, h := size[0], size[1]
		body := strings.Repeat("a very long line that will not fit ", 3) + "\nsecond\nthird\nfourth\nfifth\nsixth\nseventh"
		out := renderPanel(st, "Status", body, w, h, true)
		if got := lipgloss.Height(out); got != h {
			t.Errorf("%dx%d: height %d", w, h, got)
		}
		for i, line := range strings.Split(out, "\n") {
			if got := lipgloss.Width(line); got != w {
				t.Errorf("%dx%d: line %d width %d: %q", w, h, i, got, line)
			}
		}
	}
}

func newFilesConfig() config.FilesPanel {
	cfg, _ := config.Load("")
	return cfg.Panels.Files
}

func TestFilesViewIsExactSize(t *testing.T) {
	cfg, _ := config.Load("")
	st := newStyles(cfg.Theme)
	p := newFilesPanel(cfg.Panels.Files)
	p.setSize(30, 6)
	fs := files("internal/ui/a-very-long-file-name-that-goes-on.go", "main.go", "x/y.go", "z.go", "zz.go")
	fs[1].OrigPath = "old.go"
	p.setFiles(fs)
	for _, flat := range []bool{false, true} {
		p.flat = flat
		p.rebuild()
		out := p.view(st, true)
		if got := lipgloss.Height(out); got != 6 {
			t.Errorf("flat=%v: height %d", flat, got)
		}
		for i, line := range strings.Split(out, "\n") {
			if got := lipgloss.Width(line); got != 30 {
				t.Errorf("flat=%v: line %d width %d: %q", flat, i, got, line)
			}
		}
	}
}
