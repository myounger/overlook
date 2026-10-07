package ui

import (
	"cmp"
	"slices"
	"strings"

	"github.com/myounger/overlook/internal/git"
)

// treeNode is a folder (file == nil) or a changed file.
type treeNode struct {
	name     string // what to display; a compacted folder chain looks like "internal/ui"
	path     string // full path from the repo root
	file     *git.File
	children []*treeNode
	files    int // changed files under a folder
}

func (n *treeNode) isDir() bool { return n.file == nil }

// key identifies a row across refreshes. The prefix keeps a folder from
// colliding with a file entry of the same path.
func (n *treeNode) key() string {
	if n.isDir() {
		return "d:" + n.path
	}
	return "f:" + n.path
}

// buildTree arranges files into folders, sorted folders-first. With compact,
// a folder whose only child is another folder is merged into one row, the
// way lazygit and most editors show "internal/ui".
func buildTree(files []git.File, compact bool) *treeNode {
	root := &treeNode{}
	for i := range files {
		f := &files[i]
		// An untracked folder entry ("newdir/") is a leaf, not a folder.
		parts := strings.Split(strings.TrimSuffix(f.Path, "/"), "/")
		if strings.HasSuffix(f.Path, "/") {
			parts[len(parts)-1] += "/"
		}
		dir := root
		for j, part := range parts[:len(parts)-1] {
			dir = dir.child(part, strings.Join(parts[:j+1], "/"))
		}
		dir.children = append(dir.children, &treeNode{name: parts[len(parts)-1], path: f.Path, file: f})
	}
	finish(root, compact)
	return root
}

func (n *treeNode) child(name, path string) *treeNode {
	for _, c := range n.children {
		if c.isDir() && c.name == name {
			return c
		}
	}
	c := &treeNode{name: name, path: path}
	n.children = append(n.children, c)
	return c
}

// finish sorts, compacts, and counts files, bottom up.
func finish(n *treeNode, compact bool) {
	n.files = 0
	for i, c := range n.children {
		if !c.isDir() {
			n.files++
			continue
		}
		finish(c, compact)
		for compact && len(c.children) == 1 && c.children[0].isDir() {
			only := c.children[0]
			c = &treeNode{name: c.name + "/" + only.name, path: only.path, children: only.children, files: only.files}
		}
		n.children[i] = c
		n.files += c.files
	}
	slices.SortFunc(n.children, func(a, b *treeNode) int {
		if a.isDir() != b.isDir() {
			if a.isDir() {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.name, b.name)
	})
}

// row is one visible line of the Files panel.
type row struct {
	node  *treeNode
	depth int
}

// treeRows lists what's visible, skipping the contents of collapsed folders.
func treeRows(root *treeNode, collapsed map[string]bool) []row {
	var rows []row
	var walk func(n *treeNode, depth int)
	walk = func(n *treeNode, depth int) {
		for _, c := range n.children {
			rows = append(rows, row{c, depth})
			if c.isDir() && !collapsed[c.path] {
				walk(c, depth+1)
			}
		}
	}
	walk(root, 0)
	return rows
}

// flatRows lists every file by its full path, in path order.
func flatRows(files []git.File) []row {
	rows := make([]row, len(files))
	for i := range files {
		rows[i] = row{&treeNode{name: files[i].Path, path: files[i].Path, file: &files[i]}, 0}
	}
	slices.SortFunc(rows, func(a, b row) int { return cmp.Compare(a.node.path, b.node.path) })
	return rows
}

// folders returns the path of every folder in the tree.
func folders(root *treeNode) []string {
	var paths []string
	var walk func(n *treeNode)
	walk = func(n *treeNode) {
		for _, c := range n.children {
			if c.isDir() {
				paths = append(paths, c.path)
				walk(c)
			}
		}
	}
	walk(root)
	return paths
}
