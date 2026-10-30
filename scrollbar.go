package main

// Scroll bar (v1.15): when a list is longer than its box, the right border of the
// panel (and of the tree) shows a thumb: its length is the visible share of the list,
// its position the place of the visible rows in it. A click on the right border, or a
// drag along it, moves the cursor to that point of the list.
//
// Shift+movement (v1.15): Shift+↑/↓ invert the mark of the item under the cursor and
// move (Total Commander style: going back unmarks); Shift+PgUp/PgDn/Home/End invert
// every row passed, without the row where the cursor lands, unless it is the first or
// the last of the list (Shift+End marks down to the end). ".." is never marked.

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// scrollThumb is the thumb on a track of height h for a list of n rows whose first
// visible row is off: start (0-based, inside the track) and length; ok is false when
// the list fits.
func scrollThumb(n, h, off int) (start, length int, ok bool) {
	if h <= 0 || n <= h {
		return 0, 0, false
	}
	length = max(1, (h*h+n/2)/n)
	maxOff := n - h
	off = min(max(off, 0), maxOff)
	start = (off*(h-length) + maxOff/2) / maxOff
	return start, length, true
}

// scrollTarget is the row that a click at position y of a track of height h points
// to, in a list of n rows.
func scrollTarget(n, h, y int) int {
	if h <= 1 || n <= 1 {
		return 0
	}
	y = min(max(y, 0), h-1)
	return (y*(n-1) + (h-1)/2) / (h - 1)
}

// drawScrollbar paints the thumb on the right border of a table with a border.
func drawScrollbar(screen tcell.Screen, t *tview.Table) {
	x, y, w, h := t.GetRect()
	inner := h - 2
	off, _ := t.GetOffset()
	start, length, ok := scrollThumb(t.GetRowCount(), inner, off)
	if !ok || w < 3 {
		return
	}
	st := tcell.StyleDefault.Background(colBackground).Foreground(t.GetBorderColor())
	for i := 0; i < length; i++ {
		screen.SetContent(x+w-1, y+1+start+i, '█', nil, st)
	}
}

// scrollbarMouse handles a left click or drag on the right border of a table: the
// cursor goes to the matching row. dragging is the state of a drag in progress.
func scrollbarMouse(t *tview.Table, action tview.MouseAction, event *tcell.EventMouse, dragging *bool) bool {
	x, y, w, h := t.GetRect()
	mx, my := event.Position()
	inner := h - 2
	onBar := mx == x+w-1 && my > y && my < y+h-1
	switch {
	case action == tview.MouseLeftDown && onBar:
		if _, _, ok := scrollThumb(t.GetRowCount(), inner, 0); !ok {
			return false
		}
		*dragging = true
	case action == tview.MouseMove && *dragging && event.Buttons()&tcell.Button1 != 0:
	case action == tview.MouseLeftUp && *dragging:
		*dragging = false
		return true
	case action == tview.MouseLeftClick && (onBar || *dragging):
		return true
	default:
		*dragging = false
		return false
	}
	row := scrollTarget(t.GetRowCount(), inner, my-y-1)
	t.Select(row, 0)
	return true
}

// MouseHandler of a panel: the scroll bar first, then the table.
func (v *panelView) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		if scrollbarMouse(v.Table, action, event, &v.dragging) {
			setFocus(v)
			return true, v
		}
		return v.Table.MouseHandler()(action, event, setFocus)
	}
}

// scrollTable is a table with the scroll bar (the tree view).
type scrollTable struct {
	*tview.Table
	dragging bool
}

func (s *scrollTable) Draw(screen tcell.Screen) {
	s.Table.Draw(screen)
	drawScrollbar(screen, s.Table)
}

func (s *scrollTable) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		if scrollbarMouse(s.Table, action, event, &s.dragging) {
			setFocus(s)
			return true, s
		}
		return s.Table.MouseHandler()(action, event, setFocus)
	}
}

// markable reports whether an entry can be marked.
func markable(e entry) bool { return !e.isUp && !e.isDrive && !e.isShare }

// flipMark inverts the mark of one row.
func (p *Panel) flipMark(row int) {
	if row < 0 || row >= len(p.entries) || !markable(p.entries[row]) {
		return
	}
	e := p.entries[row]
	if p.marked[e.name] {
		delete(p.marked, e.name)
		p.dropSize(e)
	} else {
		p.marked[e.name] = true
		p.ensureSize(e)
	}
}

// shiftRows are the rows whose mark Shift+movement inverts, going from row from to
// row to in a list of n rows; single is true for Shift+↑/↓.
func shiftRows(from, to, n int, single bool) []int {
	if n == 0 {
		return nil
	}
	if single || from == to {
		return []int{from} // the item under the cursor (also at the edge, where it cannot move)
	}
	var rows []int
	step := 1
	if to < from {
		step = -1
	}
	for r := from; r != to; r += step {
		rows = append(rows, r)
	}
	if to == 0 || to == n-1 {
		rows = append(rows, to) // Shift+Home / Shift+End reach the edge: include it
	}
	return rows
}

// shiftMove is Shift+↑ ↓ PgUp PgDn Home End.
func (p *Panel) shiftMove(k tcell.Key) {
	n := len(p.entries)
	if n == 0 {
		return
	}
	row, _ := p.table.GetSelection()
	_, _, _, page := p.table.GetInnerRect()
	page = max(page, 1)
	to := row
	switch k {
	case tcell.KeyUp:
		to = row - 1
	case tcell.KeyDown:
		to = row + 1
	case tcell.KeyPgUp:
		to = row - page
	case tcell.KeyPgDn:
		to = row + page
	case tcell.KeyHome:
		to = 0
	case tcell.KeyEnd:
		to = n - 1
	}
	to = min(max(to, 0), n-1)
	single := k == tcell.KeyUp || k == tcell.KeyDown
	for _, r := range shiftRows(row, to, n, single) {
		p.flipMark(r)
	}
	p.render()
	p.table.Select(to, 0)
	p.fm.updateStatus()
}

// isShiftMove recognises the Shift+movement keys.
func isShiftMove(ev *tcell.EventKey) bool {
	if ev.Modifiers()&(tcell.ModShift|tcell.ModCtrl|tcell.ModAlt) != tcell.ModShift {
		return false
	}
	switch ev.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
		return true
	}
	return false
}
