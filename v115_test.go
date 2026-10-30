package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestScrollThumb(t *testing.T) {
	if _, _, ok := scrollThumb(10, 10, 0); ok {
		t.Error("a list that fits has no bar")
	}
	for _, c := range []struct{ n, h, off, start, length int }{
		{100, 10, 0, 0, 1}, {100, 10, 90, 9, 1}, {100, 10, 45, 5, 1},
		{20, 10, 0, 0, 5}, {20, 10, 10, 5, 5}, {11, 10, 1, 1, 9}, {1000, 20, 999, 19, 1},
	} {
		s, l, ok := scrollThumb(c.n, c.h, c.off)
		if !ok || s != c.start || l != c.length {
			t.Errorf("%+v → start %d length %d", c, s, l)
		}
	}
	if scrollTarget(100, 10, 0) != 0 || scrollTarget(100, 10, 9) != 99 || scrollTarget(100, 10, 20) != 99 || scrollTarget(5, 1, 0) != 0 {
		t.Error("scrollTarget ends")
	}
	if r := scrollTarget(101, 11, 5); r != 50 {
		t.Errorf("middle → %d", r)
	}
}

// screenOf draws a primitive on a simulated screen.
func screenOf(t *testing.T, p tview.Primitive, w, h int) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(w, h)
	p.SetRect(0, 0, w, h)
	p.Draw(s)
	return s
}

func thumbRows(s tcell.SimulationScreen, x, h int) (rows []int) {
	for y := 0; y < h; y++ {
		if r, _, _, _ := s.GetContent(x, y); r == '█' {
			rows = append(rows, y)
		}
	}
	return rows
}

// The bar appears on the right border only when the list is longer than the box, and
// a click or drag on it moves the cursor there.
func TestPanelScrollbar(t *testing.T) {
	d := t.TempDir()
	for i := 0; i < 50; i++ {
		_ = os.WriteFile(filepath.Join(d, fmt.Sprintf("f%02d.txt", i)), nil, 0o644)
	}
	fm := newTestFM(t)
	p := fm.left
	p.chdir(d, "")
	s := screenOf(t, p.table, 60, 12)
	rows := thumbRows(s, 59, 12)
	if len(rows) == 0 || rows[0] != 1 || len(rows) > 3 {
		t.Fatalf("thumb rows %v", rows)
	}
	// click at the bottom of the bar: cursor on the last row, focus on the panel
	fm.setActive(fm.right, true)
	var focused tview.Primitive
	h := p.table.MouseHandler()
	ok, _ := h(tview.MouseLeftDown, tcell.NewEventMouse(59, 10, tcell.Button1, 0), func(x tview.Primitive) { focused = x })
	if !ok || focused != p.table {
		t.Fatal("click on the bar not handled")
	}
	if row, _ := p.table.GetSelection(); row != len(p.entries)-1 {
		t.Fatalf("row %d after a click at the bottom", row)
	}
	// drag to the top
	h(tview.MouseMove, tcell.NewEventMouse(59, 1, tcell.Button1, 0), func(tview.Primitive) {})
	h(tview.MouseLeftUp, tcell.NewEventMouse(59, 1, 0, 0), func(tview.Primitive) {})
	if row, _ := p.table.GetSelection(); row != 0 {
		t.Fatalf("row %d after dragging to the top", row)
	}
	// after the drag, moving the mouse does nothing
	h(tview.MouseMove, tcell.NewEventMouse(59, 10, 0, 0), func(tview.Primitive) {})
	if row, _ := p.table.GetSelection(); row != 0 {
		t.Fatal("moved without a drag")
	}
	s = screenOf(t, p.table, 60, 12)
	if rows := thumbRows(s, 59, 12); len(rows) == 0 || rows[0] != 1 {
		t.Errorf("thumb after scrolling back %v", rows)
	}
	// a short list: no bar, and a click on the border is not taken
	for i := 3; i < 50; i++ {
		_ = os.Remove(filepath.Join(d, fmt.Sprintf("f%02d.txt", i)))
	}
	p.reload()
	s = screenOf(t, p.table, 60, 12)
	if rows := thumbRows(s, 59, 12); len(rows) != 0 {
		t.Errorf("bar on a short list: %v", rows)
	}
	if ok, _ := h(tview.MouseLeftDown, tcell.NewEventMouse(59, 5, tcell.Button1, 0), func(tview.Primitive) {}); ok && p.table.dragging {
		t.Error("drag started on a short list")
	}
}

// The tree has the bar too.
func TestTreeScrollbar(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 40; i++ {
		_ = os.MkdirAll(filepath.Join(root, fmt.Sprintf("d%02d", i)), 0o755)
	}
	fm := newTestFM(t)
	fm.left.chdir(root, "")
	fm.setActive(fm.left, true)
	fm.keys(ctrlKey('T'))
	tv := fm.tree
	syncTree(tv)
	fm.keys(key(tcell.KeyRight, 0, 0)) // root shown with its 40 subfolders (and its parents)
	s := screenOf(t, tv.table, 40, 10)
	if rows := thumbRows(s, 39, 10); len(rows) == 0 {
		t.Fatal("no bar on the tree")
	}
	ok, _ := tv.table.MouseHandler()(tview.MouseLeftDown, tcell.NewEventMouse(39, 1, tcell.Button1, 0), func(tview.Primitive) {})
	if row, _ := tv.table.GetSelection(); !ok || row != 0 {
		t.Fatalf("click on the tree bar: row %d", row)
	}
	fm.keys(ctrlKey('T'))
}

func TestShiftRows(t *testing.T) {
	for _, c := range []struct {
		from, to, n int
		single      bool
		want        string
	}{
		{3, 4, 10, true, "3"}, {3, 2, 10, true, "3"}, {9, 9, 10, true, "9"},
		{2, 7, 20, false, "2 3 4 5 6"}, {2, 19, 20, false, "2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19"},
		{5, 0, 10, false, "5 4 3 2 1 0"}, {8, 3, 10, false, "8 7 6 5 4"}, {4, 4, 10, false, "4"},
	} {
		got := strings.Trim(fmt.Sprint(shiftRows(c.from, c.to, c.n, c.single)), "[]")
		if got != c.want {
			t.Errorf("%+v → %s", c, got)
		}
	}
}

// Shift+movement marks, Total Commander style; ".." is never marked.
func TestShiftMarks(t *testing.T) {
	d := t.TempDir()
	_ = os.Mkdir(filepath.Join(d, "d1"), 0o755)
	for _, f := range []string{"f1", "f2", "f3", "f4", "f5"} {
		_ = os.WriteFile(filepath.Join(d, f), nil, 0o644)
	}
	fm := newTestFM(t)
	p := fm.left
	p.chdir(d, "")
	fm.setActive(p, true)
	marked := func() string {
		var out []string
		for _, e := range p.entries {
			if p.marked[e.name] {
				out = append(out, e.name)
			}
		}
		return strings.Join(out, ",")
	}
	row := func() int { r, _ := p.table.GetSelection(); return r }
	shift := func(k tcell.Key) { fm.keys(tcell.NewEventKey(k, 0, tcell.ModShift)) }
	if p.entries[0].name != ".." {
		t.Fatal("fixture")
	}
	p.table.Select(0, 0)
	shift(tcell.KeyDown)
	if marked() != "" || row() != 1 {
		t.Fatalf(".. marked: %q", marked())
	}
	shift(tcell.KeyDown)
	shift(tcell.KeyDown)
	if marked() != "d1,f1" || row() != 3 {
		t.Fatalf("%q at %d", marked(), row())
	}
	if p.sizes[p.fullPath("d1")] == nil {
		t.Error("marked folder not measured")
	}
	shift(tcell.KeyUp) // inverts f2, goes to f1
	shift(tcell.KeyUp) // inverts f1 (unmark), goes to d1
	if marked() != "d1,f2" || row() != 1 {
		t.Fatalf("going back: %q at %d", marked(), row())
	}
	shift(tcell.KeyEnd) // d1 (unmark), f1 … f5 (f2 unmark), the last row included
	if marked() != "f1,f3,f4,f5" || row() != len(p.entries)-1 {
		t.Fatalf("Shift+End: %q", marked())
	}
	shift(tcell.KeyHome)
	if marked() != "d1,f2" || row() != 0 {
		t.Fatalf("Shift+Home: %q", marked())
	}
	// plain arrows do not mark
	fm.keys(key(tcell.KeyDown, 0, 0))
	if marked() != "d1,f2" {
		t.Error("plain ↓ changed the marks")
	}
	// Ctrl+Shift or Alt+Shift: not a marking move
	if isShiftMove(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModShift|tcell.ModCtrl)) || !isShiftMove(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModShift)) {
		t.Error("isShiftMove")
	}
}

// Copying onto itself: one question per item, Skip (default), Skip all, Cancel; a
// folder copied inside itself can be skipped too.
func TestSelfCopy(t *testing.T) {
	d := t.TempDir()
	for _, f := range []string{"a.txt", "b.txt"} {
		writeFile(t, filepath.Join(d, f), f, time.Now())
	}
	_ = os.MkdirAll(filepath.Join(d, "dir", "sub"), 0o755)
	fm := newTestFM(t)
	fm.left.chdir(d, "")
	fm.right.chdir(d, "")
	fm.setActive(fm.left, true)
	fm.left.marked = map[string]bool{"a.txt": true, "b.txt": true}
	for _, k := range []tcell.Key{tcell.KeyF5, tcell.KeyF6} {
		fm.left.marked = map[string]bool{"a.txt": true, "b.txt": true}
		fm.keys(key(k, 0, 0))
		if len(fm.modals) != 1 {
			t.Fatalf("%v: no question", k)
		}
		if b, ok := fm.app.GetFocus().(*tview.Button); !ok || b.GetLabel() != selfSkip {
			t.Fatal("Skip is not the default")
		}
		pressEnter(fm) // Skip a.txt → question for b.txt
		if len(fm.modals) != 1 {
			t.Fatal("no second question")
		}
		pressEnter(fm)
		if len(fm.modals) != 0 || !strings.Contains(fm.notice, "nothing to do") {
			t.Fatalf("after two Skip: %d dialogs, %q", len(fm.modals), fm.notice)
		}
	}
	if readFile(t, filepath.Join(d, "a.txt")) != "a.txt" {
		t.Fatal("file changed")
	}
	// a folder into its own subfolder: skippable, Esc cancels
	fm.right.chdir(filepath.Join(d, "dir", "sub"), "")
	fm.left.marked = map[string]bool{}
	fm.left.jumpTo("dir")
	fm.keys(key(tcell.KeyF5, 0, 0))
	if len(fm.modals) != 1 {
		t.Fatal("no question for a folder into itself")
	}
	fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
	if len(fm.modals) != 0 {
		t.Fatal("Esc did not cancel")
	}

	// askSelfCopies: Skip keeps the others, Skip all skips the rest, Cancel does nothing
	other := copyPair{from: filepath.Join(d, "a.txt"), to: filepath.Join(t.TempDir(), "a.txt")}
	self1 := copyPair{from: filepath.Join(d, "a.txt"), to: filepath.Join(d, "a.txt")}
	self2 := copyPair{from: filepath.Join(d, "dir"), to: filepath.Join(d, "dir", "sub", "dir")}
	if !strings.Contains(selfCopyText(self1, "Copy", 0, 2), "(1 of 2)") || !strings.Contains(selfCopyText(self2, "Copy", 1, 2), "inside itself") {
		t.Error("texts")
	}
	for _, c := range []struct {
		press  []string
		called bool
	}{{[]string{selfSkip, selfSkip}, true}, {[]string{selfSkipAll}, true}, {[]string{selfSkip, selfCancel}, false}} {
		var kept []copyPair
		called := false
		fm.askSelfCopies([]copyPair{self1, other, self2}, "Copy", func(k []copyPair) { kept, called = k, true })
		for _, label := range c.press {
			pressButton(t, fm, label)
		}
		if called != c.called || (called && (len(kept) != 1 || kept[0] != other)) || len(fm.modals) != 0 {
			t.Errorf("%v: called %v kept %v", c.press, called, kept)
		}
	}
	// no self copies: done at once, nothing asked
	done := false
	fm.askSelfCopies([]copyPair{other}, "Copy", func([]copyPair) { done = true })
	if !done || len(fm.modals) != 0 {
		t.Error("asked without reason")
	}
}

// pressButton moves the focus of the open question to the button and presses it.
func pressButton(t *testing.T, fm *FM, label string) {
	t.Helper()
	for i := 0; i < 5; i++ {
		if b, ok := fm.app.GetFocus().(*tview.Button); ok && b.GetLabel() == label {
			pressEnter(fm)
			return
		}
		fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, 0), func(p tview.Primitive) { fm.app.SetFocus(p) })
	}
	t.Fatalf("button %q not found", label)
}
