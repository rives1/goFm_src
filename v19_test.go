package main

import (
	"math"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// Every icon is one cell wide, distinct within its set; Nerd Font icons are in the
// Private Use Area (they need the font), basic symbols are not.
func TestIconSets(t *testing.T) {
	for name, set := range map[string][numKinds]rune{"nerd": nerdIcons, "basic": basicIcons} {
		seen := map[rune]iconKind{}
		for k, r := range set {
			if r == 0 {
				t.Errorf("%s: kind %d has no icon", name, k)
				continue
			}
			if w := uniseg.StringWidth(string(r)); w != 1 {
				t.Errorf("%s: icon %U is %d cells wide", name, r, w)
			}
			if o, dup := seen[r]; dup {
				t.Errorf("%s: kinds %d and %d share icon %U", name, o, k, r)
			}
			seen[r] = iconKind(k)
			pua := r >= 0xE000 && r <= 0xF8FF
			if name == "nerd" && !pua || name == "basic" && pua {
				t.Errorf("%s: icon %U in the wrong range", name, r)
			}
		}
	}
}

// One icon per family: the seven families have seven different kinds.
func TestFamiliesHaveOwnIcon(t *testing.T) {
	kinds := map[iconKind]bool{}
	for _, ft := range fileTypes {
		if ft.kind == kindFile || ft.kind >= kindFolder || kinds[ft.kind] {
			t.Errorf("family %q: bad or repeated kind %d", ft.exts[:8], ft.kind)
		}
		kinds[ft.kind] = true
	}
	if len(kinds) != 7 {
		t.Errorf("%d families, want 7", len(kinds))
	}
}

func TestEntryKind(t *testing.T) {
	cases := []struct {
		e    entry
		want iconKind
	}{
		{entry{name: "..", isDir: true, isUp: true}, kindUp},
		{entry{name: `C:\`, isDir: true, isDrive: true}, kindDrive},
		{entry{name: "public", isDir: true, isShare: true}, kindShare},
		{entry{name: "src", isDir: true}, kindFolder},
		{entry{name: "backup.zip", isDir: true}, kindFolder}, // a folder named like an archive
		{entry{name: "A.ZIP"}, kindArchive},
		{entry{name: "setup.exe"}, kindProgram},
		{entry{name: "photo.jpg"}, kindImage},
		{entry{name: "report.pdf"}, kindDocument},
		{entry{name: "song.mp3"}, kindMedia},
		{entry{name: `src\deep\util.go`}, kindCode}, // flat view name
		{entry{name: "kernel32.dll"}, kindSystem},
		{entry{name: "README"}, kindFile},
		{entry{name: "data.unknownext"}, kindFile},
	}
	for _, c := range cases {
		if got := entryKind(c.e); got != c.want {
			t.Errorf("%q: kind %d, want %d", c.e.name, got, c.want)
		}
	}
}

// The icon keeps the family colour when the item is marked, and never uses the
// mark colour; hidden items are dimmed.
func TestIconColour(t *testing.T) {
	forEachScheme(t, func(t *testing.T) {
		for _, ft := range fileTypes {
			e := entry{name: "x" + strings.Fields(ft.exts)[0]}
			if iconColor(e) != ft.color {
				t.Errorf("%s: icon colour is not the family colour", e.name)
			}
		}
		for _, e := range []entry{{name: "x.pdf"}, {name: "d", isDir: true}, {name: "plain"}, {name: ".h", hidden: true}} {
			if iconColor(e) == colMarked {
				t.Errorf("%s: icon uses the mark colour", e.name)
			}
		}
		if iconColor(entry{name: "x.exe", hidden: true}) != colDim {
			t.Error("hidden items: icon not dimmed")
		}
	})
}

func luminance(c tcell.Color) float64 {
	r, g, b := c.RGB()
	lin := func(v int32) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// contrast is the WCAG contrast ratio (1 to 21).
func contrast(a, b tcell.Color) float64 {
	la, lb := luminance(a), luminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

// Icons must be readable wherever they are drawn in their own colour: on the panel
// background (4.5:1, as text) and on the inactive cursor row (3:1, the WCAG minimum
// for graphics). On the active cursor row they use the cursor text colour, because
// the family colours are not readable there.
func TestIconContrast(t *testing.T) {
	forEachScheme(t, func(t *testing.T) {
		colours := []tcell.Color{colText, colDim, colFolder}
		for _, ft := range fileTypes {
			colours = append(colours, ft.color)
		}
		for _, c := range colours {
			if r := contrast(c, colBackground); r < 4.5 {
				t.Errorf("%06X on the background: %.2f", c.Hex(), r)
			}
			if r := contrast(c, colCursorOffBg); r < 3 {
				t.Errorf("%06X on the inactive cursor: %.2f", c.Hex(), r)
			}
		}
		if r := contrast(colCursorFg, colCursorBg); r < 4.5 {
			t.Errorf("cursor text on the active cursor: %.2f", r)
		}
		low := 0
		for _, ft := range fileTypes {
			if contrast(ft.color, colCursorBg) < 3 {
				low++
			}
		}
		if low == 0 {
			t.Log("all family colours are now readable on the active cursor: the exception could go")
		}
	})
}

// The row is laid out as mark, icon, name, size, date, and fits the panel width.
func TestRowLayoutWithIcons(t *testing.T) {
	fm := &FM{}
	p := newPanel(fm, t.TempDir())
	fm.left, fm.right, fm.active = p, newPanel(fm, t.TempDir()), p
	p.entries = []entry{
		{name: "..", isDir: true, isUp: true},
		{name: "src", isDir: true},
		{name: "a very long report name for the board meeting.pdf", size: 1234},
	}
	p.marked["src"] = true
	for _, w := range []int{40, 60, 100} {
		p.width = w
		p.render()
		for i := range p.entries {
			cells := 0
			for c := colMark; c <= colDate; c++ {
				cells += uniseg.StringWidth(p.table.GetCell(i, c).Text)
			}
			if cells+4 > w { // + the four column separators
				t.Errorf("width %d row %d: %d cells", w, i, cells+4)
			}
		}
		if got := p.table.GetCell(1, colName).Text; got != "src" {
			t.Errorf("folder name %q: the separator prefix must be gone", got)
		}
		if p.table.GetCell(1, colMark).Text != "*" || p.table.GetCell(2, colMark).Text != " " {
			t.Error("mark column")
		}
		if !strings.HasSuffix(p.table.GetCell(2, colName).Text, ".pdf") {
			t.Errorf("width %d: extension lost", w)
		}
	}
	if p.table.GetCell(1, colIcon).Text != string(nerdIcons[kindFolder]) {
		t.Error("default set must be Nerd Font")
	}
	fm.basicIcons = true
	p.render()
	if p.table.GetCell(1, colIcon).Text != "►" || p.table.GetCell(0, colIcon).Text != "↑" {
		t.Error("basic symbols")
	}
	if fg := cellFg(p.table.GetCell(1, colIcon)); fg != colFolder {
		t.Errorf("marked folder: icon colour %06X, want the folder colour", fg.Hex())
	}
	if fg := cellFg(p.table.GetCell(1, colName)); fg != colMarked {
		t.Error("marked folder: the name is still yellow")
	}
	// cursor row: cursor text colour in the active panel, family colour otherwise
	p.table.Select(2, 0)
	p.styleCursorIcon()
	if fg, bg, _ := p.table.GetCell(2, colIcon).SelectedStyle.Decompose(); fg != colCursorFg || bg != colCursorBg {
		t.Error("active cursor row")
	}
	fm.active = fm.right
	p.styleCursorIcon()
	if fg, bg, _ := p.table.GetCell(2, colIcon).SelectedStyle.Decompose(); fg != fileTypes[family("x.pdf")].color || bg != colCursorOffBg {
		t.Error("inactive cursor row")
	}
}

// cellFg is the text colour of a cell, wherever tview keeps it.
func cellFg(c *tview.TableCell) tcell.Color {
	if c.Style != tcell.StyleDefault {
		fg, _, _ := c.Style.Decompose()
		return fg
	}
	return c.Color
}

func TestIconsRemembered(t *testing.T) {
	path := t.TempDir() + "/state.json"
	t.Setenv("GOFM_STATE", path)
	fm := &FM{left: &Panel{dir: "/l"}, right: &Panel{dir: "/r"}, lastDir: map[byte]string{}}
	fm.active = fm.left
	fm.basicIcons = true
	fm.saveState()
	st, ok := loadState()
	if !ok || !st.BasicIcons {
		t.Fatal("basicIcons not saved")
	}
	fm.basicIcons = false
	fm.saveState() // only the icon set changed: it must still be written
	if st, _ := loadState(); st.BasicIcons {
		t.Fatal("icon change not saved")
	}
}
