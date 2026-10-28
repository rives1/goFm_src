package main

// Key summary (Alt+F1): every shortcut that is NOT already shown in the key bar at the
// bottom of the screen. A test checks that the two lists never overlap.

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// keybarKeys are the shortcuts of the key bar, in order.
var keybarKeys = [][2]string{
	{"F2", "Ren"}, {"F3", "View"}, {"F5", "Copy"}, {"F6", "Move"},
	{"F7", "MkDir"}, {"F8", "Trash"}, {"F9", "Zip"}, {"F10", "Quit"}, {"F12", "Compare"},
	{"Ctrl+B", "Flat"}, {"Alt+S", "Find"}, {"Alt+F", "Filter"}, {"Alt+U", "Extract"}, {"Alt+V", "Drives"},
	{"Alt+I", "Icons"}, {"Shift+X", "Drive X:"},
}

type helpSection struct {
	title string
	keys  [][2]string // shortcut, description
}

var helpSections = []helpSection{
	{"Navigation", [][2]string{
		{"↑ ↓", "move the cursor (also PgUp PgDn Home End)"},
		{"→", "open a folder, drive, share or .zip/.7z archive"},
		{"←", "parent folder, or leave the archive"},
		{"Backspace", "parent folder; in a search: delete a character"},
		{"Enter", "open a folder or file; inside an archive: preview"},
		{"Tab / Shift+Tab", "switch panel"},
		{"Ctrl+→", "(left panel) same location in the right panel"},
		{"Ctrl+←", "(right panel) same location in the left panel"},
		{"Alt+E", "same location in the other panel"},
		{"Alt+L", `go to a path: C:\x, %USERPROFILE%, ~, \\server\share`},
		{"Mouse click", "select a row and activate its panel"},
	}},
	{"Search and selection", [][2]string{
		{"a–z 0–9 …", "quick search by name (Shift+letter = drive)"},
		{"Space / Ins", "mark or unmark, then move down"},
		{"*", "nothing marked: mark all; otherwise unmark all"},
		{"Alt+A", "invert the marks"},
		{"Esc", "end search, then clear filter, then marks"},
	}},
	{"Files", [][2]string{
		{"Del", "move to the Recycle Bin (same as F8)"},
	}},
	{"View", [][2]string{
		{"Alt+O", "sort by name → size → date"},
		{"Alt+H", "show/hide hidden files and admin shares"},
		{"Alt+R", "refresh both panels, re-measure marked folders"},
	}},
	{"Program", [][2]string{
		{"Alt+F1", "this summary"},
		{"Alt+Q", "quit (same as F10)"},
	}},
}

// helpText is the content of the summary, with tview colour tags.
func helpText() string {
	w := 0
	for _, s := range helpSections {
		for _, k := range s.keys {
			w = max(w, len([]rune(k[0])))
		}
	}
	var b strings.Builder
	for i, s := range helpSections {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "[#FFE14D::b]%s[-::-]\n", s.title)
		for _, k := range s.keys {
			pad := strings.Repeat(" ", w-len([]rune(k[0])))
			fmt.Fprintf(&b, "  [#8FD3FF]%s[-]%s  %s\n", tview.Escape(k[0]), pad, tview.Escape(k[1]))
		}
	}
	b.WriteString("\n[#7F8FB4]The keys in the bar at the bottom are not repeated here.[-]")
	return b.String()
}

// showKeys opens the key summary (Alt+F1). Esc, Enter, F1, Alt+F1 or q close it.
func (fm *FM) showKeys() {
	var name string
	text := helpText()
	tv := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(false).SetText(text)
	tv.SetBorder(true).SetTitle(" Keys — Esc to close ")
	tv.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Key() == tcell.KeyEscape, ev.Key() == tcell.KeyEnter, ev.Key() == tcell.KeyF1,
			ev.Key() == tcell.KeyRune && ev.Rune() == 'q':
			fm.closeModal(name)
			return nil
		}
		return ev
	})
	lines := strings.Split(text, "\n")
	w := 0
	for _, l := range lines {
		w = max(w, tview.TaggedStringWidth(l))
	}
	h := len(lines) + 2
	if _, _, _, sh := fm.pages.GetRect(); sh > 0 && h > sh {
		h = sh // the text scrolls with the arrows
		tv.SetTitle(" Keys — ↑↓ to scroll, Esc to close ")
	}
	name = fm.openModal(tv, w+4, h, nil)
}
