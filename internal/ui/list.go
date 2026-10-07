package ui

// listView is the cursor and scroll position of a bordered list panel.
type listView struct {
	cursor int
	offset int // first visible row
	count  int
	width  int
	height int // including the border
}

func (l *listView) setSize(width, height int) {
	l.width, l.height = width, height
	l.scroll()
}

// setCount updates the number of rows, keeping the cursor in range.
func (l *listView) setCount(n int) {
	l.count = n
	l.cursor = max(min(l.cursor, n-1), 0)
	l.scroll()
}

func (l *listView) setCursor(i int) {
	l.cursor = max(min(i, l.count-1), 0)
	l.scroll()
}

func (l *listView) move(delta int) { l.setCursor(l.cursor + delta) }

// pageSize is how many rows fit inside the border.
func (l *listView) pageSize() int { return max(l.height-2, 1) }

// visible returns the range of rows to draw.
func (l *listView) visible() (start, end int) {
	return l.offset, min(l.offset+l.pageSize(), l.count)
}

// scroll moves the window just enough to keep the cursor in view.
func (l *listView) scroll() {
	h := l.pageSize()
	l.offset = min(l.offset, l.cursor)
	l.offset = max(l.offset, l.cursor-h+1)
	l.offset = max(min(l.offset, l.count-h), 0)
}
