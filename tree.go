package main

// Tree view (Ctrl+T, v1.14). The left panel is replaced by the folder tree of the
// drive of the current folder, opened down to that folder; the right panel shows the
// folder under the tree cursor, 150 ms after the cursor stops (no reading of every
// folder while scrolling fast). Ctrl+T again goes back to two panels, both on the
// folder under the tree cursor.
//
// Keys in the tree: ↑ ↓ PgUp PgDn Home End move, → opens a branch (or goes to its
// first subfolder), ← closes it (or goes to the parent), Backspace goes to the parent,
// Enter and Tab move to the right panel, Shift+letter shows another drive, Alt+H
// shows/hides hidden folders, Alt+R re-reads the tree. File operations are done in
// the right panel; copy and move (F5/F6) and compare (F12) need two panels and are
// refused while the tree is shown. The tree is not available from the drive list, a
// share list, an archive or the flat view, and it is not remembered at exit.

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// treeDelay is how long the tree cursor must rest before the right panel follows it.
var treeDelay = 150 * time.Millisecond

// treeNode is a folder of the tree. Children are read when the node is first opened.
type treeNode struct {
	path, name string
	depth      int
	parent     *treeNode
	children   []*treeNode
	loaded     bool // children read
	expanded   bool
	hidden     bool
	isLink     bool
}

// leaf: opened and without subfolders (no marker).
func (n *treeNode) leaf() bool { return n.loaded && len(n.children) == 0 }

type treeView struct {
	fm    *FM
	table *tview.Table
	root  *treeNode
	rows  []*treeNode // visible nodes, in display order
	quiet bool        // cursor moved by gofm itself: the right panel must not follow
	timer *time.Timer
	gen   int          // generation of the pending update; a newer cursor move cancels it
	queue func(func()) // runs a function on the interface goroutine (tests replace it)
	after func(func()) *time.Timer
}

// treeRoot is the root folder of a path: C:\ for C:\x\y, \\server\share\ for a UNC
// path, / elsewhere.
func treeRoot(dir string) string {
	if vol := filepath.VolumeName(dir); vol != "" {
		return vol + string(filepath.Separator)
	}
	return string(filepath.Separator)
}

// treeRefusal explains why the tree cannot start from a panel ("" = it can).
func treeRefusal(p *Panel) string {
	switch {
	case p.arch != nil:
		return "The tree is not available inside an archive:\nleave it first (←)."
	case p.flat:
		return "The tree is not available in the flat view:\nleave it first (Ctrl+B)."
	case p.inDriveList():
		return "The tree starts from a folder:\nopen a drive first (→ or Enter)."
	case p.inShareList():
		return "The tree starts from a folder:\nopen a share first (→ or Enter)."
	}
	return ""
}

// toggleTree is Ctrl+T.
func (fm *FM) toggleTree() {
	if fm.tree != nil {
		fm.closeTree()
		return
	}
	p := fm.active
	if why := treeRefusal(p); why != "" {
		fm.showMessage("Tree", why)
		return
	}
	dir := p.dir
	t := newTreeView(fm)
	fm.tree = t
	t.quiet = true
	t.reveal(dir)
	t.quiet = false
	fm.panelsBox.Clear().
		AddItem(t.table, 0, 1, true).
		AddItem(fm.right.table, 0, 1, false)
	r := fm.right
	if !(r.onDisk() && !r.flat && samePath(r.dir, dir)) {
		r.chdir(dir, "")
	}
	fm.setActive(fm.left, true)
	fm.notice = "Tree: → ← open and close folders · Tab or Enter to the files · Ctrl+T back to two panels"
	fm.updateStatus()
}

// closeTree goes back to two panels, both on the folder under the tree cursor.
func (fm *FM) closeTree() {
	t := fm.tree
	t.cancelPending()
	sel := t.selectedPath()
	fm.tree = nil
	fm.panelsBox.Clear().
		AddItem(fm.left.table, 0, 1, true).
		AddItem(fm.right.table, 0, 1, false)
	if sel != "" {
		fm.left.chdir(sel, "")
		if r := fm.right; !(r.onDisk() && !r.flat && samePath(r.dir, sel)) {
			r.chdir(sel, "")
		}
	}
	fm.setActive(fm.active, true)
	fm.saveState()
}

func newTreeView(fm *FM) *treeView {
	t := &treeView{fm: fm, table: tview.NewTable().SetSelectable(true, false)}
	t.queue = func(f func()) { fm.app.QueueUpdateDraw(f) }
	t.after = func(f func()) *time.Timer { return time.AfterFunc(treeDelay, f) }
	t.table.SetBorder(true).SetTitleAlign(tview.AlignLeft)
	t.table.SetBackgroundColor(colBackground)
	t.table.SetTitleColor(colTitle)
	t.table.SetSelectionChangedFunc(func(row, col int) {
		if !t.quiet {
			t.schedule()
		}
		fm.updateStatus()
	})
	t.table.SetFocusFunc(func() { fm.setActive(fm.left, false) })
	return t
}

func (t *treeView) showHidden() bool { return t.fm.left.showHidden }

// load reads the subfolders of a node, sorted by name. Links to folders are listed
// (and can be opened); hidden folders follow Alt+H.
func (t *treeNode) load(showHidden bool) {
	t.loaded, t.children = true, nil
	des, err := os.ReadDir(t.path)
	if err != nil {
		return
	}
	for _, de := range des {
		info, err := de.Info()
		if err != nil {
			continue
		}
		full := filepath.Join(t.path, de.Name())
		isDir, isLink := info.IsDir(), false
		if info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			isLink = true
			if st, err := os.Stat(full); err == nil && st.IsDir() {
				isDir = true
			}
		}
		if !isDir {
			continue
		}
		_, winHidden := fileAttrs(info)
		hidden := winHidden || strings.HasPrefix(de.Name(), ".")
		if hidden && !showHidden {
			continue
		}
		t.children = append(t.children, &treeNode{path: full, name: de.Name(), depth: t.depth + 1,
			parent: t, hidden: hidden, isLink: isLink})
	}
	sort.SliceStable(t.children, func(i, j int) bool {
		return strings.ToLower(t.children[i].name) < strings.ToLower(t.children[j].name)
	})
}

// sameName compares path components the way the file system does.
func sameName(a, b string) bool {
	if onWindows {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// reveal opens the tree down to dir and puts the cursor on it (on its nearest listed
// ancestor if a part is hidden or missing). A dir on another drive gets a new root.
func (t *treeView) reveal(dir string) {
	dir = filepath.Clean(dir)
	root := treeRoot(dir)
	if t.root == nil || !samePath(t.root.path, root) {
		t.root = &treeNode{path: root, name: root}
	}
	n := t.root
	rel := strings.TrimPrefix(dir, root)
	if rel != "" && rel != "." {
		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			if part == "" {
				continue
			}
			if !n.loaded {
				n.load(t.showHidden())
			}
			var next *treeNode
			for _, c := range n.children {
				if sameName(c.name, part) {
					next = c
				}
			}
			if next == nil {
				break
			}
			n.expanded = true
			n = next
		}
	}
	if !t.root.loaded {
		t.root.load(t.showHidden())
	}
	t.root.expanded = true
	t.selectNode(n)
}

// selectNode rebuilds the rows and moves the cursor to n (without moving the right
// panel when quiet is set by the caller).
func (t *treeView) selectNode(n *treeNode) {
	t.rebuild()
	for i, r := range t.rows {
		if r == n {
			t.table.Select(i, 0)
			return
		}
	}
}

func (t *treeView) rebuild() {
	t.rows = t.rows[:0]
	var walk func(n *treeNode)
	walk = func(n *treeNode) {
		t.rows = append(t.rows, n)
		if n.expanded {
			for _, c := range n.children {
				walk(c)
			}
		}
	}
	if t.root != nil {
		walk(t.root)
	}
	t.render()
}

// Branch markers: Nerd Font chevrons, or + and - with the basic symbols (Alt+I).
const (
	treeClosedNF, treeOpenNF = '\uf054', '\uf078'
)

func (t *treeView) marker(n *treeNode) string {
	basic := t.fm.basicIcons
	switch {
	case n.leaf():
		return " "
	case n.expanded && basic:
		return "-"
	case n.expanded:
		return string(treeOpenNF)
	case basic:
		return "+"
	}
	return string(treeClosedNF)
}

func (t *treeView) render() {
	tb := t.table
	row, _ := tb.GetSelection()
	rowOff, colOff := tb.GetOffset()
	was := t.quiet
	t.quiet, tb = true, tb.Clear()
	for i, n := range t.rows {
		e := entry{name: n.name, isDir: true, hidden: n.hidden, isLink: n.isLink}
		color, attr := entryStyle(e, false)
		name := n.name
		if n.isLink {
			name += " ~"
		}
		text := strings.Repeat("  ", n.depth) + t.marker(n) + " " + string(iconFor(e, t.fm.basicIcons)) + " " + name
		tb.SetCell(i, 0, tview.NewTableCell(tview.Escape(text)).SetTextColor(color).SetAttributes(attr).SetExpansion(1))
	}
	if row >= len(t.rows) {
		row = len(t.rows) - 1
	}
	if row >= 0 {
		tb.Select(row, 0)
	}
	tb.SetOffset(rowOff, colOff)
	t.quiet = was
	title := "Tree  " + t.rootPath()
	if t.showHidden() {
		title += "  [hidden shown]"
	}
	tb.SetTitle(" " + tview.Escape(title) + " ")
}

func (t *treeView) rootPath() string {
	if t.root == nil {
		return ""
	}
	return t.root.path
}

func (t *treeView) selected() *treeNode {
	row, _ := t.table.GetSelection()
	if row < 0 || row >= len(t.rows) {
		return nil
	}
	return t.rows[row]
}

func (t *treeView) selectedPath() string {
	if n := t.selected(); n != nil {
		return n.path
	}
	return ""
}

// schedule makes the right panel follow the cursor after treeDelay; a newer move
// replaces the pending one.
func (t *treeView) schedule() {
	t.cancelPending()
	g := t.gen
	t.timer = t.after(func() {
		t.queue(func() {
			if t.fm.tree == t && t.gen == g {
				t.apply()
			}
		})
	})
}

func (t *treeView) cancelPending() {
	t.gen++
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}

// flush shows the selected folder in the right panel now (Enter, Tab).
func (t *treeView) flush() {
	t.cancelPending()
	t.apply()
}

// apply shows the selected folder in the right panel. A folder that cannot be read
// is reported in the status bar (no dialog: the cursor may be just passing by).
func (t *treeView) apply() {
	n := t.selected()
	if n == nil {
		return
	}
	r := t.fm.right
	if r.onDisk() && !r.flat && samePath(r.dir, n.path) {
		return
	}
	if err := r.tryChdir(n.path, ""); err != nil {
		t.fm.notice = "Cannot open " + n.name + ": " + err.Error()
	}
	t.fm.updateStatus()
}

// open is →: open a closed branch, or go to the first subfolder of an open one.
func (t *treeView) open() {
	n := t.selected()
	if n == nil {
		return
	}
	if !n.loaded {
		n.load(t.showHidden())
	}
	switch {
	case len(n.children) == 0:
		t.render() // now known to be a leaf
	case !n.expanded:
		n.expanded = true
		t.quiet = true
		t.selectNode(n)
		t.quiet = false
	default:
		t.selectNode(n.children[0]) // the cursor moves: the right panel follows
	}
}

// close is ←: close an open branch, or go to the parent folder.
func (t *treeView) close() {
	n := t.selected()
	if n == nil {
		return
	}
	if n.expanded && len(n.children) > 0 {
		n.expanded = false
		t.quiet = true
		t.selectNode(n)
		t.quiet = false
		return
	}
	t.parent()
}

func (t *treeView) parent() {
	if n := t.selected(); n != nil && n.parent != nil {
		t.selectNode(n.parent)
	}
}

// refresh re-reads every opened folder, keeping what was open and the cursor.
func (t *treeView) refresh() {
	sel := t.selectedPath()
	t.refreshNode(t.root)
	t.quiet = true
	t.reveal(sel)
	t.quiet = false
}

func (t *treeView) refreshNode(n *treeNode) {
	if n == nil || !n.loaded {
		return
	}
	old := map[string]*treeNode{}
	for _, c := range n.children {
		old[c.name] = c
	}
	n.load(t.showHidden())
	for i, c := range n.children {
		if o, ok := old[c.name]; ok && o.loaded {
			o.path, o.depth, o.parent, o.hidden, o.isLink = c.path, c.depth, n, c.hidden, c.isLink
			n.children[i] = o
			t.refreshNode(o)
		}
	}
}

// refreshPath re-reads one folder of the tree, if it has been opened (after the right
// panel shows a change there, e.g. a new folder).
func (t *treeView) refreshPath(dir string) {
	n := t.find(dir)
	if n == nil || !n.loaded {
		return
	}
	sel := t.selected()
	t.refreshNode(n)
	if len(n.children) == 0 {
		n.expanded = false
	}
	t.quiet = true
	if sel != nil && t.contains(sel) {
		t.selectNode(sel)
	} else {
		t.selectNode(n)
	}
	t.quiet = false
}

func (t *treeView) contains(n *treeNode) bool {
	for p := n; p != nil; p = p.parent {
		if p == t.root {
			return true
		}
		if p.parent != nil {
			found := false
			for _, c := range p.parent.children {
				found = found || c == p
			}
			if !found {
				return false
			}
		}
	}
	return false
}

// find returns the loaded node for dir, or nil.
func (t *treeView) find(dir string) *treeNode {
	var walk func(n *treeNode) *treeNode
	walk = func(n *treeNode) *treeNode {
		if samePath(n.path, dir) {
			return n
		}
		for _, c := range n.children {
			if r := walk(c); r != nil {
				return r
			}
		}
		return nil
	}
	if t.root == nil {
		return nil
	}
	return walk(t.root)
}

// treeOnlyNotice is shown for the keys that work on files.
const treeOnlyNotice = "Tree: Tab or Enter moves to the files on the right · Ctrl+T back to two panels"

// treeKeys handles the keys while the tree has the focus.
func (fm *FM) treeKeys(ev *tcell.EventKey) *tcell.EventKey {
	t := fm.tree
	if l, ok := ctrlLetter(ev); ok {
		if l == 'T' {
			fm.toggleTree()
		} else {
			fm.notice = treeOnlyNotice
		}
		return nil
	}
	if ev.Key() >= tcell.KeyF1 && ev.Key() <= tcell.KeyF64 && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) != 0 {
		ctrl, alt := ev.Modifiers()&tcell.ModCtrl != 0, ev.Modifiers()&tcell.ModAlt != 0
		switch {
		case ctrl && !alt && ev.Key() == tcell.KeyF2:
			fm.showKeys()
		case ctrl && !alt && ev.Key() == tcell.KeyF9:
			fm.cycleScheme()
		default:
			fm.notice = treeOnlyNotice
		}
		return nil
	}
	if r, ok := altLetter(ev); ok {
		switch r {
		case 'h':
			fm.left.showHidden = !fm.left.showHidden
			t.refresh()
		case 'r':
			fm.reloadSettings()
			t.refresh()
			fm.right.resetSizes()
			fm.right.reload()
		case 'i':
			fm.toggleIcons()
		case 'q':
			fm.app.Stop()
		default:
			fm.notice = treeOnlyNotice
		}
		return nil
	}
	if ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModAlt|tcell.ModCtrl) == 0 {
		if l, ok := driveKey(ev.Rune()); ok {
			fm.treeDrive(l)
		}
		return nil // no quick search in the tree
	}
	switch ev.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
		return ev // the table moves the cursor; the right panel follows after treeDelay
	case tcell.KeyRight:
		t.open()
	case tcell.KeyLeft:
		t.close()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		t.parent()
	case tcell.KeyEnter, tcell.KeyTab, tcell.KeyBacktab:
		t.flush()
		fm.setActive(fm.right, true)
	case tcell.KeyF10:
		fm.app.Stop()
	case tcell.KeyEscape:
	default:
		fm.notice = treeOnlyNotice
	}
	fm.updateStatus()
	return nil
}

// treeDrive (Shift+letter in the tree) shows another drive, on its last folder.
func (fm *FM) treeDrive(l byte) {
	target, ok := fm.driveTarget(l)
	if !ok {
		return
	}
	fm.tree.reveal(target) // the cursor moves: the right panel follows
}
