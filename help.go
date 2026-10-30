package main

// Key summary (Ctrl+F2, v1.12): every shortcut. The key bar shows only as many as
// fit, so the summary lists them all (a test checks that every key bar item is here).

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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
		{"Alt+V", "drive list"},
		{"Shift+letter", "go to that drive (last folder used there)"},
		{"Mouse click", "select a row; key bar: run the key; scroll bar: jump"},
	}},
	{"Search and selection", [][2]string{
		{"a–z 0–9 …", "quick search by name (lower-case letters)"},
		{"Space / Ins", "mark or unmark, then move down"},
		{"Shift+arrows", "move and (un)mark; also Shift+PgUp PgDn Home End"},
		{"Ctrl+A", "nothing marked: mark all; otherwise invert"},
		{"Esc", "end search, then clear filter, then marks"},
		{"Alt+F7", "find files by name and/or content"},
		{"Ctrl+F7", "filter the panel (text or *.ext)"},
	}},
	{"Files", [][2]string{
		{"F2", "rename"},
		{"F3", "view / properties"},
		{"F4", "edit with the external editor (gofm.ini)"},
		{"F5", "copy to the other panel (in an archive: extract)"},
		{"F6", "move to the other panel"},
		{"F7", "new folder"},
		{"Ctrl+K", "add a prefix to the names of the marked items"},
		{"Del", "delete: Recycle Bin; permanent on network drives"},
		{"F9", "compress to ZIP"},
		{"Alt+U", "extract an archive"},
		{"F12", "compare the panels (name, size, date)"},
	}},
	{"View", [][2]string{
		{"Ctrl+B", "flat view of the folder tree"},
		{"Ctrl+T", "tree on the left, folder contents on the right"},
		{"Alt+O", "sort by name → size → date"},
		{"Alt+D", "reverse the sort direction (↑ ↓ in the title)"},
		{"Alt+H", "show/hide hidden files and admin shares"},
		{"Alt+R", "refresh, re-measure marked folders, re-read gofm.ini"},
		{"Alt+I", "icons: Nerd Font ↔ basic symbols"},
		{"Ctrl+F9", "next colour scheme: navy → dark → light"},
	}},
	{"Program", [][2]string{
		{"Ctrl+F2", "this summary"},
		{"F10 / Alt+Q", "quit"},
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
		fmt.Fprintf(&b, "[%s::b]%s[-::-]\n", tagColor(colMarked), s.title)
		for _, k := range s.keys {
			pad := strings.Repeat(" ", w-len([]rune(k[0])))
			fmt.Fprintf(&b, "  [%s]%s[-]%s  %s\n", tagColor(colLabel), tview.Escape(k[0]), pad, tview.Escape(k[1]))
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// showKeys opens the key summary (Ctrl+F2). Esc, Enter, F2 or q close it.
func (fm *FM) showKeys() {
	var name string
	text := helpText()
	tv := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(false).SetText(text)
	tv.SetBorder(true).SetTitle(" Keys — Esc to close ")
	tv.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Key() == tcell.KeyEscape, ev.Key() == tcell.KeyEnter, ev.Key() == tcell.KeyF2,
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
