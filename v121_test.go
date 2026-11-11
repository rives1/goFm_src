package main

import (
	"os"
	"path/filepath"
	"testing"
)

// hiddenTreeFixture: root/a/.hid/inner, root/a/.other, root/a/b.
func hiddenTreeFixture(t *testing.T) (root string) {
	root = visibleTempDir(t)
	for _, d := range []string{"a/.hid/inner", "a/.other", "a/b"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func treeRowNames(tv *treeView) map[string]*treeNode {
	rows := map[string]*treeNode{}
	for _, n := range tv.rows {
		rows[n.name] = n
	}
	return rows
}

// v1.21: Ctrl+T from a folder under a hidden folder opens the tree down to it; the
// hidden folder on the way is listed (dimmed), the other hidden folders are not.
func TestTreeHiddenAncestor(t *testing.T) {
	root := hiddenTreeFixture(t)
	inner := filepath.Join(root, "a", ".hid", "inner")
	fm := newTestFM(t)
	fm.left.chdir(inner, "")
	fm.setActive(fm.left, true)
	fm.keys(ctrlKey('T'))
	tv := fm.tree
	if tv == nil {
		t.Fatal("no tree")
	}
	syncTree(tv)
	if tv.selectedPath() != inner || fm.right.dir != inner {
		t.Fatalf("tree on %q, right panel on %q, want %q", tv.selectedPath(), fm.right.dir, inner)
	}
	rows := treeRowNames(tv)
	if n := rows[".hid"]; n == nil || !n.hidden {
		t.Error("the hidden folder on the way is not listed as hidden")
	}
	if rows[".other"] != nil {
		t.Error("another hidden folder is listed")
	}
	if rows["b"] == nil {
		t.Error("the visible sibling is missing")
	}
	// Ctrl+T back: both panels on the folder under the cursor.
	fm.keys(ctrlKey('T'))
	if fm.tree != nil || fm.left.dir != inner || fm.right.dir != inner {
		t.Errorf("after the tree: left %q right %q", fm.left.dir, fm.right.dir)
	}
}

// A branch read before without the hidden folder is read again when the tree is
// opened down into it (the right panel entering a hidden folder, Alt+R).
func TestTreeRevealReadsBranchAgain(t *testing.T) {
	root := hiddenTreeFixture(t)
	a, inner := filepath.Join(root, "a"), filepath.Join(root, "a", ".hid", "inner")
	fm := newTestFM(t)
	fm.left.chdir(a, "")
	fm.setActive(fm.left, true)
	fm.keys(ctrlKey('T'))
	tv := fm.tree
	syncTree(tv)
	tv.reveal(a)
	tv.open() // →: a read and opened, its hidden children not listed
	if n := tv.find(a); n == nil || !n.loaded {
		t.Fatal("branch a not read")
	}
	if treeRowNames(tv)[".hid"] != nil {
		t.Fatal(".hid listed before it is on the way")
	}
	tv.reveal(inner)
	if tv.selectedPath() != inner {
		t.Fatalf("reveal: tree on %q, want %q", tv.selectedPath(), inner)
	}
	tv.refresh() // Alt+R keeps the way open
	if tv.selectedPath() != inner || treeRowNames(tv)[".hid"] == nil {
		t.Errorf("after refresh: tree on %q", tv.selectedPath())
	}
	// A missing folder: the cursor stays on the nearest ancestor that exists.
	tv.reveal(filepath.Join(a, "missing", "x"))
	if tv.selectedPath() != a {
		t.Errorf("missing folder: tree on %q, want %q", tv.selectedPath(), a)
	}
}
