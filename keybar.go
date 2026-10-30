package main

// Key bar (bottom line), v1.12: the shortcuts are shown in order of priority, as many
// as fit the width of the window. Clicking a shortcut with the mouse runs it, exactly
// as if the key had been pressed.

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// keyItem is one shortcut of the bar. ev builds the key event that a click sends; nil
// means the item is only a hint (Shift+X stands for 26 keys).
type keyItem struct {
	key, label string
	ev         func() *tcell.EventKey
}

func fkey(k tcell.Key, mod tcell.ModMask) func() *tcell.EventKey {
	return func() *tcell.EventKey { return tcell.NewEventKey(k, 0, mod) }
}

func altKey(r rune) func() *tcell.EventKey {
	return func() *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModAlt) }
}

// keybarItems in order of priority: operations first. F2, F3, F5, F7 and F10 are
// never shown (user's choice, v1.12); they are in the key summary (Ctrl+F2).
var keybarItems = []keyItem{
	{"F4", "Edit", fkey(tcell.KeyF4, 0)},
	{"F6", "Move", fkey(tcell.KeyF6, 0)},
	{"Del", "Delete", fkey(tcell.KeyDelete, 0)},
	{"F9", "Zip", fkey(tcell.KeyF9, 0)},
	{"F12", "Compare", fkey(tcell.KeyF12, 0)},
	{"Ctrl+A", "Select", fkey(tcell.KeyCtrlA, tcell.ModCtrl)},
	{"Ctrl+B", "Flat", fkey(tcell.KeyCtrlB, tcell.ModCtrl)},
	{"Alt+F7", "Find", fkey(tcell.KeyF7, tcell.ModAlt)},
	{"Ctrl+F7", "Filter", fkey(tcell.KeyF7, tcell.ModCtrl)},
	{"Alt+U", "Extract", altKey('u')},
	{"Alt+V", "Drives", altKey('v')},
	{"Ctrl+F2", "Keys", fkey(tcell.KeyF2, tcell.ModCtrl)},
	{"Alt+O", "Sort", altKey('o')},
	{"Alt+D", "Reverse", altKey('d')},
	{"Alt+H", "Hidden", altKey('h')},
	{"Ctrl+F9", "Colours", fkey(tcell.KeyF9, tcell.ModCtrl)},
	{"Alt+I", "Icons", altKey('i')},
	{"Shift+X", "Drive X:", nil},
	{"Alt+L", "Go to", altKey('l')},
}

// keySlot is where an item is drawn: cells [x, x+w) from the left edge of the bar.
type keySlot struct {
	item, x, w int
}

// keybarLayout places the items from the left, in order of priority, while they fit
// in width cells (one cell of margin on the left, one space after each item).
func keybarLayout(width int) []keySlot {
	var slots []keySlot
	x := 1
	for i, it := range keybarItems {
		w := uniseg.StringWidth(it.key) + uniseg.StringWidth(it.label)
		if x+w > width {
			break
		}
		slots = append(slots, keySlot{i, x, w})
		x += w + 1
	}
	return slots
}

type keyBar struct {
	*tview.Box
	fm *FM
}

func newKeyBar(fm *FM) *keyBar {
	return &keyBar{Box: tview.NewBox().SetBackgroundColor(colBarBg), fm: fm}
}

func (b *keyBar) Draw(screen tcell.Screen) {
	b.Box.DrawForSubclass(screen, b)
	x, y, w, _ := b.GetInnerRect()
	keyStyle := tcell.StyleDefault.Foreground(colKeyFg).Background(colKeyBg)
	labelStyle := tcell.StyleDefault.Foreground(colText).Background(colBarBg)
	for _, s := range keybarLayout(w) {
		it := keybarItems[s.item]
		cx := x + s.x
		for _, r := range it.key {
			screen.SetContent(cx, y, r, nil, keyStyle)
			cx++
		}
		for _, r := range it.label {
			screen.SetContent(cx, y, r, nil, labelStyle)
			cx++
		}
	}
}

// itemAt returns the item under column col of a bar of the given width, or -1.
func itemAt(width, col int) int {
	for _, s := range keybarLayout(width) {
		if col >= s.x && col < s.x+s.w {
			return s.item
		}
	}
	return -1
}

func (b *keyBar) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	return b.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		if !b.InRect(event.Position()) {
			return false, nil
		}
		if action == tview.MouseLeftClick {
			mx, _ := event.Position()
			x, _, w, _ := b.GetInnerRect()
			if i := itemAt(w, mx-x); i >= 0 && keybarItems[i].ev != nil {
				b.fm.keys(keybarItems[i].ev()) // the panel keeps the focus
			}
		}
		return true, nil
	})
}
