package main

import (
	"sort"
	"testing"
	"time"

	"github.com/rivo/tview"
)

// v1.20: the sort of pointers gives the order of the old sort of entries
// (sort.SliceStable with entryLess), also on a large listing with equal keys,
// names that differ only in case and folders mixed with files.
func TestSortPointersSameOrder(t *testing.T) {
	list := bigListing(5000)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	for i := range list {
		switch i % 7 {
		case 0:
			list[i].isDir = true
		case 1:
			list[i].size = 42 // many equal sizes
		case 2:
			list[i].mod = t0 // many equal dates
		case 3:
			list[i].name = "Same.txt" // equal names: the stable order decides
		case 4:
			list[i].name = "same.TXT"
		}
		list[i].lname = ""
	}
	list[17] = entry{name: "..", isDir: true, isUp: true}
	for mode := range sortLabels {
		for _, asc := range []bool{true, false} {
			a := append([]entry(nil), list...)
			b := append([]entry(nil), list...)
			p := &Panel{sortMode: mode, sortAsc: asc}
			p.sortEntries(a)
			for i := range b {
				b[i].lower()
			}
			sort.SliceStable(b, func(i, j int) bool { return entryLess(&b[i], &b[j], mode, asc) })
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("mode %d asc %v: row %d %+v vs %+v", mode, asc, i, a[i], b[i])
				}
			}
			if !a[0].isUp {
				t.Fatalf("mode %d asc %v: \"..\" is not first", mode, asc)
			}
		}
	}
}

// v1.20: escapeName gives what tview.Escape gives, without the regular expression
// for names without "[".
func TestEscapeName(t *testing.T) {
	for _, s := range []string{"", "report.pdf", "a]b", "[x].txt", "[red]", "a[b", "[[", "dir\\[1] copia", "日本[語]"} {
		if got, want := escapeName(s), tview.Escape(s); got != want {
			t.Errorf("escapeName(%q) = %q, want %q", s, got, want)
		}
	}
}
