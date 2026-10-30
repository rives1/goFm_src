package main

// Status bar: the details of the item under the cursor in fixed-width columns
// (size, date, attributes, item count) and a name column that takes the rest of
// the line — the same rule as the panels. A one-shot notice (e.g. after F12)
// replaces the line until the next key; the quick-search text sits before the name.

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

const (
	stSizeW  = 9  // "1023.9 MB"
	stDateW  = 16 // "26/10/2026 16:25"
	stAttrW  = 4  // "RHSA"
	stItemsW = 11 // "99999 items"
	stSep    = " │ "
)

type statusBar struct {
	*tview.Box
	fm *FM
}

func newStatusBar(fm *FM) *statusBar {
	return &statusBar{Box: tview.NewBox().SetBackgroundColor(colBarBg), fm: fm}
}

// statusFields are the texts shown for the item under the cursor.
type statusFields struct {
	name, size, date, attrs, items string
}

func (fm *FM) statusFieldsFor(p *Panel) statusFields {
	f := statusFields{items: fmt.Sprintf("%d items", countReal(p.entries))}
	e := p.current()
	if e == nil || e.isUp {
		return f
	}
	f.name = e.name
	if e.remark != "" {
		f.name += " — " + e.remark
	}
	switch {
	case e.isDrive, e.isShare:
	case e.isDir:
		f.size = "<DIR>"
		if ds, ok := p.sizes[p.fullPath(e.name)]; ok && ds.done && p.marked[e.name] {
			f.size = humanSize(ds.size)
		}
	default:
		f.size = humanSize(e.size)
	}
	if !e.mod.IsZero() {
		f.date = e.mod.Local().Format("02/01/2006 15:04")
	}
	f.attrs = e.attrs
	return f
}

// layoutStatus returns the name column (shortened to fit) and the fixed right part
// of a status line of the given width; prefixW cells are already taken on the left.
func layoutStatus(width, prefixW int, f statusFields) (name, right string) {
	right = fmt.Sprintf("%*s%s%-*s%s%-*s%s%*s",
		stSizeW, f.size, stSep, stDateW, f.date, stSep, stAttrW, f.attrs, stSep, stItemsW, f.items)
	nameW := width - prefixW - uniseg.StringWidth(right) - len([]rune(stSep)) - 2 // 1-cell margin each side
	if nameW < 1 {
		return "", right
	}
	name = truncateName(f.name, nameW)
	name += strings.Repeat(" ", nameW-uniseg.StringWidth(name))
	return name, right
}

func (b *statusBar) Draw(screen tcell.Screen) {
	b.Box.DrawForSubclass(screen, b)
	x, y, w, _ := b.GetInnerRect()
	fm := b.fm
	if fm.active == nil {
		return
	}
	if fm.notice != "" {
		tview.Print(screen, "["+tagColor(colNoticeFg)+":"+tagColor(colNoticeBg)+"] "+tview.Escape(fm.notice)+" [-:-]", x+1, y, w-1, tview.AlignLeft, colText)
		return
	}
	col := x + 1
	prefix := ""
	if fm.qs != "" || fm.qsMiss {
		prefix = " Search: " + fm.qs + " "
		tview.Print(screen, tview.Escape(prefix), col, y, w, tview.AlignLeft, colCursorFg)
		for i := 0; i < uniseg.StringWidth(prefix); i++ { // badge background
			r, comb, st, _ := screen.GetContent(col+i, y)
			screen.SetContent(col+i, y, r, comb, st.Background(colMarked).Foreground(colCursorFg))
		}
		if fm.qsMiss {
			prefix += "no match "
			tview.Print(screen, "no match", col+uniseg.StringWidth(prefix)-9, y, 9, tview.AlignLeft, colError)
		}
		col += uniseg.StringWidth(prefix) + 1
	}
	f := statusFields{}
	if fm.tree != nil && fm.active == fm.left {
		f.name = fm.tree.selectedPath() // the folder under the tree cursor
	} else {
		f = fm.statusFieldsFor(fm.active)
	}
	name, right := layoutStatus(w, col-x, f)
	tview.Print(screen, tview.Escape(name), col, y, w, tview.AlignLeft, colText)
	rw := uniseg.StringWidth(right)
	tview.Print(screen, tview.Escape(right), x+w-rw-1, y, rw, tview.AlignLeft, colDim)
}
