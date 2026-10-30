package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// treeFixture: root/a/b/c, root/a/b/c2, root/a/x, root/a/.hid, root/a/file.txt.
func treeFixture(t *testing.T) (root string) {
	root = t.TempDir()
	for _, d := range []string{"a/b/c", "a/b/c2", "a/x", "a/.hid"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(filepath.Join(root, "a", "file.txt"), []byte("x"), 0o644)
	return root
}

// syncTree makes the right panel follow the tree cursor at once (no timer).
func syncTree(tv *treeView) {
	tv.after = func(f func()) *time.Timer { f(); return nil }
	tv.queue = func(f func()) { f() }
}

func ctrlKey(l byte) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyCtrlA+tcell.Key(l-'A'), rune(l), tcell.ModCtrl)
}

func TestTreeRoot(t *testing.T) {
	if onWindows {
		for in, want := range map[string]string{`C:\x\y`: `C:\`, `D:\`: `D:\`, `\\srv\sh\x`: `\\srv\sh\`} {
			if got := treeRoot(in); got != want {
				t.Errorf("%s → %s", in, got)
			}
		}
		return
	}
	if treeRoot("/a/b") != "/" || treeRoot("/") != "/" {
		t.Error("treeRoot")
	}
}

func TestTreeView(t *testing.T) {
	root := treeFixture(t)
	a, b := filepath.Join(root, "a"), filepath.Join(root, "a", "b")
	fm := newTestFM(t)
	fm.left.chdir(b, "")
	fm.right.chdir(root, "")
	fm.setActive(fm.left, true)

	fm.keys(ctrlKey('T'))
	tv := fm.tree
	if tv == nil {
		t.Fatal("no tree")
	}
	syncTree(tv)
	if tv.selectedPath() != b || fm.right.dir != b || fm.app.GetFocus() != tv.table || fm.active != fm.left {
		t.Fatalf("start: tree on %q, right on %q", tv.selectedPath(), fm.right.dir)
	}
	if item := fm.panelsBox.GetItem(0); item != tv.table {
		t.Fatal("the tree is not in place of the left panel")
	}
	if tv.rootPath() != treeRoot(b) || !strings.HasPrefix(tv.table.GetTitle(), " Tree  ") {
		t.Errorf("root %q title %q", tv.rootPath(), tv.table.GetTitle())
	}
	// rows: indentation and marker; b not opened yet
	cell := tv.table.GetCell(len(tv.rows)-1, 0).Text
	for _, n := range tv.rows {
		if n.name == ".hid" {
			t.Fatal("hidden folder listed")
		}
	}
	if tv.selected().depth == 0 || !strings.Contains(cell, " ") {
		t.Errorf("row %q", cell)
	}

	// → opens b (cursor stays), → again goes to c, which the right panel shows
	fm.keys(key(tcell.KeyRight, 0, 0))
	if !tv.selected().expanded || tv.selectedPath() != b || fm.right.dir != b {
		t.Fatal("→ did not open b")
	}
	fm.keys(key(tcell.KeyRight, 0, 0))
	if tv.selectedPath() != filepath.Join(b, "c") || fm.right.dir != filepath.Join(b, "c") {
		t.Fatalf("→ on an open branch: %q / %q", tv.selectedPath(), fm.right.dir)
	}
	// c has no subfolders: → marks it as a leaf
	fm.keys(key(tcell.KeyRight, 0, 0))
	if !tv.selected().leaf() || strings.Contains(tv.table.GetCell(func() int { r, _ := tv.table.GetSelection(); return r }(), 0).Text, string(treeClosedNF)) {
		t.Error("leaf still has a marker")
	}
	// ← on a leaf goes to the parent, ← again closes it, ← again goes to a
	fm.keys(key(tcell.KeyLeft, 0, 0))
	if tv.selectedPath() != b {
		t.Fatal("← to parent")
	}
	fm.keys(key(tcell.KeyLeft, 0, 0))
	if tv.selected().expanded {
		t.Fatal("← did not close")
	}
	fm.keys(key(tcell.KeyLeft, 0, 0))
	if tv.selectedPath() != a || fm.right.dir != a {
		t.Fatal("← to a")
	}
	// Alt+H shows hidden folders in the tree
	fm.keys(key(tcell.KeyRune, 'h', tcell.ModAlt))
	found := false
	for _, n := range tv.rows {
		found = found || n.name == ".hid"
	}
	if !found || !strings.Contains(tv.table.GetTitle(), "hidden shown") {
		t.Error("Alt+H")
	}
	// file keys in the tree only explain; no dialog
	fm.keys(key(tcell.KeyF2, 0, 0))
	if fm.notice != treeOnlyNotice || len(fm.modals) != 0 {
		t.Errorf("F2 in the tree: %q", fm.notice)
	}
	// Tab: to the right panel; copy, move, compare need two panels
	fm.keys(key(tcell.KeyTab, 0, 0))
	if fm.active != fm.right || fm.app.GetFocus() != fm.right.table {
		t.Fatal("Tab")
	}
	for _, k := range []tcell.Key{tcell.KeyF5, tcell.KeyF6, tcell.KeyF12} {
		fm.keys(key(k, 0, 0))
		if len(fm.modals) != 1 {
			t.Fatalf("%v not refused", k)
		}
		closeAll(fm)
	}
	// the right panel goes into b: the tree follows (without moving the panel back)
	fm.right.jumpTo("b")
	fm.keys(key(tcell.KeyRight, 0, 0))
	if fm.right.dir != b || tv.selectedPath() != b {
		t.Fatalf("tree did not follow: %q", tv.selectedPath())
	}
	// a folder created in the right panel appears in the tree after the re-read
	_ = os.Mkdir(filepath.Join(b, "new"), 0o755)
	fm.right.reload()
	tv.find(b).expanded = true
	tv.rebuild()
	if tv.find(filepath.Join(b, "new")) == nil {
		t.Error("new folder not in the tree")
	}
	// Tab back to the tree; the state remembers the tree's folder
	fm.keys(key(tcell.KeyTab, 0, 0))
	if fm.app.GetFocus() != tv.table || fm.currentState().Left != b {
		t.Fatal("Tab back / state")
	}
	// Ctrl+T: two panels, both on the folder of the tree
	fm.keys(key(tcell.KeyDown, 0, 0)) // the table moves the cursor (handled by the table)
	tv.table.InputHandler()(key(tcell.KeyDown, 0, 0), func(tview.Primitive) {})
	sel := tv.selectedPath()
	fm.keys(ctrlKey('T'))
	if fm.tree != nil || fm.panelsBox.GetItem(0) != fm.left.table {
		t.Fatal("tree not closed")
	}
	if fm.left.dir != sel || fm.right.dir != sel || fm.active != fm.left {
		t.Fatalf("after Ctrl+T: %q %q, want %q", fm.left.dir, fm.right.dir, sel)
	}
}

// The right panel follows the cursor 150 ms after it stops: quick moves load only
// the last folder.
func TestTreeDelay(t *testing.T) {
	root := treeFixture(t)
	fm := newTestFM(t)
	fm.left.chdir(filepath.Join(root, "a"), "")
	fm.setActive(fm.left, true)
	fm.keys(ctrlKey('T'))
	tv := fm.tree
	runs := make(chan func(), 10)
	tv.queue = func(f func()) { runs <- f }
	old := treeDelay
	treeDelay = 30 * time.Millisecond
	defer func() { treeDelay = old }()
	fm.keys(key(tcell.KeyRight, 0, 0)) // opens a: no move
	start := fm.right.dir
	for i := 0; i < 2; i++ {
		tv.table.InputHandler()(key(tcell.KeyDown, 0, 0), func(tview.Primitive) {})
	}
	want := tv.selectedPath()
	if fm.right.dir != start {
		t.Fatal("the right panel moved before the delay")
	}
	deadline := time.After(2 * time.Second)
	ran := 0
	for fm.right.dir != want {
		select {
		case f := <-runs:
			f()
			ran++
		case <-deadline:
			t.Fatalf("right panel on %q, want %q", fm.right.dir, want)
		}
	}
	time.Sleep(3 * treeDelay)
	for len(runs) > 0 {
		(<-runs)()
	}
	if fm.right.dir != want {
		t.Fatal("an older move overrode the last one")
	}
}

// Ctrl+T is refused from an archive, the flat view and the drive list.
func TestTreeRefused(t *testing.T) {
	root := treeFixture(t)
	fm := newTestFM(t)
	p := fm.left
	p.chdir(root, "")
	fm.setActive(p, true)
	p.flat = true // as after Ctrl+B (built in a background task)
	defer func() { p.flat = false }()
	fm.keys(ctrlKey('T'))
	if fm.tree != nil || len(fm.modals) != 1 {
		t.Fatal("tree from the flat view")
	}
	closeAll(fm)
	if treeRefusal(&Panel{dir: ""}) == "" || treeRefusal(&Panel{dir: root, arch: &archView{}}) == "" || treeRefusal(&Panel{dir: root}) != "" {
		t.Error("treeRefusal")
	}
}

func TestPrefixPreview(t *testing.T) {
	if !strings.Contains(prefixPreview([]string{"a"}, ""), "empty text cancels") {
		t.Error("empty")
	}
	if !strings.Contains(prefixPreview([]string{"a"}, "x/"), "cannot contain") {
		t.Error("bad chars")
	}
	names := []string{"1", "2", "3", "4", "5", "6", "7", "sub" + string(filepath.Separator) + "8"}
	pv := prefixPreview(names, "p_")
	lines := strings.Split(pv, "\n")
	if len(lines) != maxPreview+1 || lines[0] != "1 → p_1" || lines[maxPreview] != "… and 2 more" {
		t.Fatalf("%q", pv)
	}
}

// Files and folders get the prefix; an existing new name is skipped and reported;
// in the flat view the item stays in its subfolder.
func TestAddPrefix(t *testing.T) {
	d := t.TempDir()
	for _, f := range []string{"a.txt", "b.txt", "x_b.txt", "sub/c.txt"} {
		writeFile(t, filepath.Join(d, filepath.FromSlash(f)), f, time.Now())
	}
	_ = os.Mkdir(filepath.Join(d, "dir"), 0o755)
	o := &opCtx{}
	r := o.addPrefix(d, []string{"a.txt", "b.txt", "dir", filepath.Join("sub", "c.txt")}, "x_")
	if len(o.errs) != 0 || len(r.renamed) != 3 || len(r.skipped) != 1 || r.skipped[0] != "b.txt" {
		t.Fatalf("%+v %v", r, o.errs)
	}
	for _, f := range []string{"x_a.txt", "b.txt", "x_dir", filepath.Join("sub", "x_c.txt")} {
		if _, err := os.Stat(filepath.Join(d, f)); err != nil {
			t.Errorf("%s missing", f)
		}
	}
	if readFile(t, filepath.Join(d, "x_b.txt")) != "x_b.txt" {
		t.Error("existing file overwritten")
	}
	if txt := skippedText(r, 4); !strings.Contains(txt, "Renamed 3 of 4") || !strings.HasSuffix(txt, "\nb.txt") {
		t.Errorf("%q", txt)
	}
}

// Ctrl+K opens the dialog for the marked items (or the one under the cursor); the
// characters that names cannot contain cannot be typed.
func TestPrefixDialog(t *testing.T) {
	d := t.TempDir()
	for _, f := range []string{"a.txt", "b.txt"} {
		writeFile(t, filepath.Join(d, f), f, time.Now())
	}
	fm := newTestFM(t)
	p := fm.left
	p.chdir(d, "")
	fm.setActive(p, true)
	p.jumpTo("a.txt")
	if n := p.selectedNames(); len(n) != 1 || n[0] != "a.txt" {
		t.Fatalf("cursor: %q", n)
	}
	p.table.Select(0, 0) // ".."
	if n := p.selectedNames(); len(n) != 0 {
		t.Fatalf("..: %q", n)
	}
	p.marked = map[string]bool{"a.txt": true, "b.txt": true}
	fm.keys(ctrlKey('K'))
	if len(fm.modals) != 1 {
		t.Fatal("no dialog")
	}
	in, ok := fm.app.GetFocus().(*tview.InputField)
	if !ok {
		t.Fatalf("focus on %T", fm.app.GetFocus())
	}
	for _, r := range "p/\\_" {
		in.InputHandler()(tcell.NewEventKey(tcell.KeyRune, r, 0), func(tview.Primitive) {})
	}
	if in.GetText() != "p_" {
		t.Errorf("typed %q", in.GetText())
	}
	in.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
	closeAll(fm)
	if len(fm.modals) != 0 || !exists(filepath.Join(d, "a.txt")) {
		t.Fatal("Esc renamed or left the dialog open")
	}
}
