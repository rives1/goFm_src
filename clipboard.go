package main

// gofm's own clipboard (v1.16): Ctrl+O copies and Ctrl+I cuts the marked items (or
// the one under the cursor); Ctrl+P pastes them into the folder of the active panel,
// with every check and question of F5/F6 (items onto themselves, folders with the same
// name, existing files). After pasting cut items the clipboard is emptied; copied items
// can be pasted again. The clipboard is not Windows' one and is lost at exit.
// Ctrl+C/X/V are not used: terminals keep Ctrl+V for pasting text.
//
// Also here: the right mouse button marks or unmarks the item (v1.16).

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// clipboard holds full paths of copied or cut items.
type clipboard struct {
	items []string
	cut   bool
}

// clipboardPut is Ctrl+O (cut=false) and Ctrl+I (cut=true).
func (fm *FM) clipboardPut(cut bool) {
	if fm.readOnlyHere() {
		return
	}
	items := fm.active.targets()
	if len(items) == 0 {
		return
	}
	fm.clip = &clipboard{items: items, cut: cut}
	if cut {
		fm.notice = fmt.Sprintf("%s cut — Ctrl+P moves them to the folder of the active panel", describe(items))
	} else {
		fm.notice = fmt.Sprintf("%s copied — Ctrl+P pastes them into the folder of the active panel", describe(items))
	}
	fm.updateStatus()
}

// paste is Ctrl+P.
func (fm *FM) paste() {
	if fm.clip == nil || len(fm.clip.items) == 0 {
		fm.notice = "Nothing to paste: copy with Ctrl+O or cut with Ctrl+I first"
		fm.updateStatus()
		return
	}
	if fm.readOnlyHere() {
		return
	}
	p := fm.active
	var items []string
	for _, it := range fm.clip.items { // items deleted or moved meanwhile are dropped
		if _, err := os.Lstat(it); err == nil {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		fm.clip = nil
		fm.showMessage("Paste", "The copied items no longer exist.")
		return
	}
	cut := fm.clip.cut
	fm.transfer(items, p.dir, cut, func() {
		if cut {
			fm.clip = nil // cut items are pasted once
		}
		p.reload()
	})
}

// rightClickMark marks or unmarks the row under the pointer and puts the cursor on it.
func (v *panelView) rightClickMark(event *tcell.EventMouse) bool {
	x, y := event.Position()
	if !v.InRect(x, y) {
		return false
	}
	row, _ := v.CellAt(x, y)
	p := v.p
	if row < 0 || row >= len(p.entries) {
		return false
	}
	p.flipMark(row)
	p.render()
	p.table.Select(row, 0)
	if p.fm != nil {
		p.fm.updateStatus()
	}
	return true
}

// rightClick handles the right button for a panel's MouseHandler.
func (v *panelView) rightClick(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) bool {
	if action != tview.MouseRightClick {
		return false
	}
	if v.rightClickMark(event) {
		setFocus(v)
		return true
	}
	return false
}
