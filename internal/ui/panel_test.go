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
