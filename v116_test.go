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

// runUI runs the application on a simulated screen and returns a function that runs
// code on the interface goroutine and waits for it (background tasks, their dialogs
// and their completion then work as in the real program).
func runUI(t *testing.T, fm *FM) func(func()) {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	sim.SetSize(120, 30)
	fm.app.SetScreen(sim)
	stopped := make(chan struct{})
	go func() { _ = fm.app.Run(); close(stopped) }()
	ui := func(f func()) {
		t.Helper()
		done := make(chan struct{})
		fm.app.QueueUpdateDraw(func() { f(); close(done) })
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("interface not responding")
		}
	}
	t.Cleanup(func() {
		select {
		case <-stopped:
			return // already stopped by the program (quit): Stop again would race with Run
		default:
		}
		fm.app.Stop()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
		}
	})
	return ui
}

// waitUI polls cond on the interface goroutine until it is true.
func waitUI(t *testing.T, ui func(func()), what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ok := false
		ui(func() { ok = cond() })
		if ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout: %s", what)
}

// Ctrl+W and Alt+Q quit; F10 no longer does.
func TestQuitKeys(t *testing.T) {
	fm := newTestFM(t)
	fm.keys(key(tcell.KeyF10, 0, 0))
	if fm.quitting || len(fm.modals) != 0 {
		t.Fatal("F10 still quits")
	}
	fm.keys(ctrlKey('W'))
	if !fm.quitting {
		t.Fatal("Ctrl+W does not quit")
	}
	fm2 := newTestFM(t)
	fm2.keys(key(tcell.KeyRune, 'q', tcell.ModAlt))
	if !fm2.quitting {
		t.Fatal("Alt+Q does not quit")
	}
	// also with the tree view
	d := t.TempDir()
	fm3 := newTestFM(t)
	fm3.left.chdir(d, "")
	fm3.setActive(fm3.left, true)
	fm3.keys(ctrlKey('T'))
	fm3.keys(key(tcell.KeyF10, 0, 0))
	if fm3.quitting {
		t.Fatal("F10 quits from the tree")
	}
	fm3.keys(ctrlKey('W'))
	if !fm3.quitting {
		t.Fatal("Ctrl+W in the tree")
	}
}

// Ctrl+O / Ctrl+I put items in gofm's clipboard, Ctrl+P pastes them into the folder
// of the active panel; cut items are pasted once.
func TestClipboard(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, "a.txt"), "A", time.Now())
	writeFile(t, filepath.Join(src, "b.txt"), "B", time.Now())
	writeFile(t, filepath.Join(src, "dir", "c.txt"), "C", time.Now())
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(src, "")
		fm.right.chdir(dst, "")
		fm.setActive(fm.left, true)
		fm.keys(ctrlKey('P'))
	})
	ui(func() {
		if !strings.Contains(fm.notice, "Nothing to paste") {
			t.Errorf("empty clipboard: %q", fm.notice)
		}
		fm.left.marked = map[string]bool{"a.txt": true, "dir": true}
		fm.keys(ctrlKey('O'))
	})
	ui(func() {
		if fm.clip == nil || fm.clip.cut || len(fm.clip.items) != 2 || !strings.Contains(fm.notice, "2 items copied") {
			t.Fatalf("Ctrl+O: %+v %q", fm.clip, fm.notice)
		}
		fm.setActive(fm.right, true)
		fm.keys(ctrlKey('P'))
	})
	waitUI(t, ui, "copy pasted", func() bool {
		return exists(filepath.Join(dst, "a.txt")) && exists(filepath.Join(dst, "dir", "c.txt")) && len(fm.modals) == 0 && len(fm.queue.jobs) == 0
	})
	ui(func() {
		if fm.clip == nil || !exists(filepath.Join(src, "a.txt")) {
			t.Fatal("copy: clipboard emptied or source removed")
		}
		// cut b.txt from the left panel, paste it on the right
		fm.setActive(fm.left, true)
		fm.left.marked = map[string]bool{}
		fm.left.jumpTo("b.txt")
		fm.keys(ctrlKey('I'))
		if !fm.clip.cut || !strings.Contains(fm.notice, "\"b.txt\" cut") {
			t.Fatalf("Ctrl+I: %q", fm.notice)
		}
		fm.setActive(fm.right, true)
		fm.keys(ctrlKey('P'))
	})
	waitUI(t, ui, "cut pasted", func() bool {
		return exists(filepath.Join(dst, "b.txt")) && !exists(filepath.Join(src, "b.txt")) && fm.clip == nil
	})
	// paste where the item already is: the question of v1.15 (Skip)
	ui(func() {
		fm.setActive(fm.left, true)
		fm.left.jumpTo("a.txt")
		fm.keys(ctrlKey('O'))
		fm.keys(ctrlKey('P'))
	})
	waitUI(t, ui, "self question", func() bool { return len(fm.modals) == 1 })
	ui(func() {
		pressEnter(fm) // Skip
	})
	waitUI(t, ui, "skipped", func() bool { return len(fm.modals) == 0 && strings.Contains(fm.notice, "nothing to do") })
	// the copied item was deleted meanwhile
	ui(func() {
		_ = os.Remove(filepath.Join(src, "a.txt"))
		fm.setActive(fm.right, true)
		fm.keys(ctrlKey('P'))
		if len(fm.modals) != 1 || fm.clip != nil {
			t.Error("missing items: no message")
		}
		closeAll(fm)
	})
}

// With the tree view, Ctrl+P works in the right panel (the destination is that panel).
func TestPasteInTree(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, "a.txt"), "A", time.Now())
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(src, "")
		fm.setActive(fm.left, true)
		fm.left.jumpTo("a.txt")
		fm.keys(ctrlKey('O'))
		fm.left.chdir(dst, "")
		fm.keys(ctrlKey('T'))
		fm.tree.flush()
		fm.keys(key(tcell.KeyTab, 0, 0))
		fm.keys(ctrlKey('P'))
	})
	waitUI(t, ui, "pasted with the tree", func() bool {
		return exists(filepath.Join(dst, "a.txt")) && len(fm.modals) == 0 && len(fm.queue.jobs) == 0
	})
}

// Shift+Del: no confirmation, also on network drives; the warning for folders with
// subfolders stays.
func TestShiftDelete(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the freedesktop Recycle Bin of the test
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "a.txt"), "A", time.Now())
	writeFile(t, filepath.Join(d, "deep", "sub", "x.txt"), "X", time.Now())
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(d, "")
		fm.setActive(fm.left, true)
		fm.left.jumpTo("a.txt")
		fm.keys(key(tcell.KeyDelete, 0, tcell.ModShift))
		if b, ok := fm.app.GetFocus().(*tview.Button); ok && b.GetLabel() == "Yes" {
			t.Error("Shift+Del asked for confirmation")
		}
	})
	waitUI(t, ui, "deleted without a question", func() bool { return !exists(filepath.Join(d, "a.txt")) })
	ui(func() {
		fm.left.jumpTo("deep")
		fm.keys(key(tcell.KeyDelete, 0, tcell.ModShift))
	})
	waitUI(t, ui, "subfolder warning", func() bool {
		if len(fm.modals) != 1 {
			return false
		}
		b, ok := fm.app.GetFocus().(*tview.Button)
		return ok && b.GetLabel() == deepButton(false)
	})
	ui(func() {
		fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
	})
	waitUI(t, ui, "warning closed", func() bool { return len(fm.modals) == 0 })
	if !exists(filepath.Join(d, "deep", "sub", "x.txt")) {
		t.Fatal("Esc on the warning deleted")
	}
	// plain Del still asks
	ui(func() {
		fm.keys(key(tcell.KeyDelete, 0, 0))
		if len(fm.modals) != 1 {
			t.Error("Del without a question")
		}
		closeAll(fm)
	})
}

// Ctrl+K proposes the last prefix of the session.
func TestPrefixRemembered(t *testing.T) {
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "a.txt"), "A", time.Now())
	writeFile(t, filepath.Join(d, "b.txt"), "B", time.Now())
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(d, "")
		fm.setActive(fm.left, true)
		fm.left.jumpTo("a.txt")
		fm.keys(ctrlKey('K'))
		in := fm.app.GetFocus().(*tview.InputField)
		if in.GetText() != "" {
			t.Errorf("first time: %q", in.GetText())
		}
		in.SetText("2026_")
		pressEnter(fm) // the action runs on the next interface turn
	})
	waitUI(t, ui, "renamed", func() bool { return exists(filepath.Join(d, "2026_a.txt")) && len(fm.modals) == 0 })
	ui(func() {
		fm.left.jumpTo("b.txt")
		fm.keys(ctrlKey('K'))
		in := fm.app.GetFocus().(*tview.InputField)
		if in.GetText() != "2026_" {
			t.Errorf("proposed %q", in.GetText())
		}
		closeAll(fm)
	})
	if st := fm.currentState(); strings.Contains(fmt.Sprint(st), "2026_") {
		t.Error("prefix saved in the state")
	}
}

// A right click marks or unmarks the item and puts the cursor on it.
func TestRightClickMarks(t *testing.T) {
	d := t.TempDir()
	for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, filepath.Join(d, f), f, time.Now())
	}
	fm := newTestFM(t)
	p := fm.left
	p.chdir(d, "")
	fm.setActive(fm.right, true)
	screenOf(t, p.table, 60, 12) // lays out the table
	h := p.table.MouseHandler()
	var focused tview.Primitive
	click := func(y int) {
		h(tview.MouseRightClick, tcell.NewEventMouse(10, y, tcell.Button2, 0), func(x tview.Primitive) { focused = x })
	}
	click(3) // row 2 = b.txt (row 0 is "..", at y=1)
	if !p.marked["b.txt"] || focused != p.table {
		t.Fatalf("marks %v", p.marked)
	}
	if row, _ := p.table.GetSelection(); p.entries[row].name != "b.txt" {
		t.Fatal("cursor not on the clicked item")
	}
	click(3)
	if p.marked["b.txt"] {
		t.Fatal("second right click did not unmark")
	}
	click(1) // ".." is never marked
	if len(p.marked) != 0 {
		t.Fatal(".. marked")
	}
}
