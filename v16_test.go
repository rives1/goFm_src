package main

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

func names(es ...string) []entry {
	var out []entry
	for _, n := range es {
		out = append(out, entry{name: n})
	}
	return out
}

func TestNextSurvivor(t *testing.T) {
	old := names("..", "a", "b", "c", "d")
	cases := []struct {
		row  int
		now  []entry
		want int
	}{
		{2, names("..", "a", "b", "c", "d"), 2}, // nothing removed: same item
		{2, names("..", "a", "c", "d"), 2},      // b removed: cursor on c (the next one)
		{2, names("..", "a", "d"), 2},           // b and c removed: on d
		{4, names("..", "a", "b", "c"), 3},      // last item removed: on the previous one
		{3, names("..", "a"), 1},                // c and everything after removed: on a
		{2, names("..", "x", "a", "c", "d"), 3}, // a new file appeared before: still follows c
		{1, names(".."), 0},                     // all removed
	}
	for _, c := range cases {
		if got := nextSurvivor(old, c.row, c.now); got != c.want {
			t.Errorf("row %d now %v: got %d want %d", c.row, c.now, got, c.want)
		}
	}
}

func TestLayoutStatusFixedColumns(t *testing.T) {
	f := statusFields{name: "a very long report name for the board meeting.pdf", size: "1.2 MB",
		date: "26/10/2026 16:25", attrs: "---A", items: "12 items"}
	for _, w := range []int{80, 120, 200} {
		name, right := layoutStatus(w, 1, f)
		if got := 1 + uniseg.StringWidth(name) + len([]rune(stSep)) + uniseg.StringWidth(right) + 2; got != w {
			t.Errorf("width %d: line is %d cells", w, got)
		}
		if !strings.HasSuffix(strings.TrimSpace(name), ".pdf") && w < 120 {
			t.Errorf("width %d: extension lost: %q", w, name)
		}
		// the fixed part never changes with the width
		_, r2 := layoutStatus(300, 1, f)
		if right != r2 {
			t.Errorf("fixed columns depend on the width")
		}
	}
	if !strings.Contains(func() string { _, r := layoutStatus(100, 1, f); return r }(), "1.2 MB │ 26/10/2026 16:25 │ ---A │    12 items") {
		t.Error("fixed columns layout")
	}
}

func TestFileColoursNeverLookMarked(t *testing.T) {
	mr, mg, mb := colMarked.RGB()
	for _, ft := range fileTypes {
		r, g, b := ft.color.RGB()
		d := (r-mr)*(r-mr) + (g-mg)*(g-mg) + (b-mb)*(b-mb)
		if d < 120*120 {
			t.Errorf("type %q colour too close to the mark colour", ft.exts)
		}
	}
	for _, e := range []entry{{name: "x.pdf"}, {name: "x.zip"}, {name: "dir", isDir: true}, {name: ".h", hidden: true}, {name: "plain"}} {
		if c, _ := entryStyle(e, false); c == colMarked {
			t.Errorf("%s unmarked uses the mark colour", e.name)
		}
		if c, _ := entryStyle(e, true); c != colMarked {
			t.Errorf("%s marked does not use the mark colour", e.name)
		}
	}
	if c, _ := entryStyle(entry{name: "REPORT.PDF"}, false); c != fileTypes[family("x.pdf")].color {
		t.Error("extension match must ignore case")
	}
}

func TestRefreshDelay(t *testing.T) {
	if refreshDelay(0) != minRefreshDelay || refreshDelay(time.Second) != 2*time.Second {
		t.Fatal("refreshDelay")
	}
	_ = tcell.ColorDefault
}

func TestFileTypeColoursDistinct(t *testing.T) {
	for i, a := range fileTypes {
		for _, b := range fileTypes[i+1:] {
			r1, g1, b1 := a.color.RGB()
			r2, g2, b2 := b.color.RGB()
			if d := (r1-r2)*(r1-r2) + (g1-g2)*(g1-g2) + (b1-b2)*(b1-b2); d < 60*60 {
				t.Errorf("%q and %q look alike", a.exts[:8], b.exts[:8])
			}
		}
	}
}
