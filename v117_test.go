package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// The scan finds the same conflicts as before (top-level files, files inside merged
// folders, in source order; type mismatches left to the copy) and the sizes.
func TestScanPlan(t *testing.T) {
	d := t.TempDir()
	src, dst := filepath.Join(d, "src"), filepath.Join(d, "dst")
	t0 := time.Now()
	writeFile(t, filepath.Join(src, "top.txt"), "12345", t0)
	writeFile(t, filepath.Join(src, "dir", "a.txt"), "aa", t0)
	writeFile(t, filepath.Join(src, "dir", "deep", "b.txt"), "bbb", t0)
	writeFile(t, filepath.Join(src, "dir", "new.txt"), "n", t0)
	writeFile(t, filepath.Join(src, "dir", "clash"), "file here", t0) // a folder at the destination
	writeFile(t, filepath.Join(dst, "top.txt"), "old", t0)
	writeFile(t, filepath.Join(dst, "dir", "a.txt"), "old", t0)
	writeFile(t, filepath.Join(dst, "dir", "deep", "b.txt"), "old", t0)
	_ = os.MkdirAll(filepath.Join(dst, "dir", "clash"), 0o755)
	pairs := []copyPair{
		{from: filepath.Join(src, "top.txt"), to: filepath.Join(dst, "top.txt")},
		{from: filepath.Join(src, "dir"), to: filepath.Join(dst, "dir")},
	}
	o := &opCtx{}
	plan := o.scan(pairs, false)
	var names []string
	for _, c := range plan.conflicts {
		r, _ := filepath.Rel(dst, c.dst)
		names = append(names, filepath.ToSlash(r))
		if c.srcInfo == nil || c.dstInfo == nil {
			t.Error("conflict without information")
		}
	}
	sort.Strings(names[1:]) // order inside a folder follows ReadDir (sorted by name)
	if strings.Join(names, ",") != "top.txt,dir/a.txt,dir/deep/b.txt" {
		t.Fatalf("conflicts %v", names)
	}
	if plan.total != 5+2+3+1+9 || pairs[0].size != 5 || pairs[1].size != 15 {
		t.Fatalf("total %d sizes %d %d", plan.total, pairs[0].size, pairs[1].size)
	}
	if !plan.roots[1].scanned || len(plan.roots[1].children) != 4 {
		t.Fatal("tree not kept")
	}
	// findConflicts (used by older callers) gives the same answer and leaves pairs alone
	p2 := []copyPair{{from: pairs[0].from, to: pairs[0].to}}
	if len(o.findConflicts(p2)) != 1 || p2[0].size != 0 {
		t.Error("findConflicts")
	}
}

// The copy uses the tree read by the scan: nothing is read twice. A file created after
// the scan is therefore not copied, while copyAll (no plan) reads the disk itself.
func TestCopyUsesPlan(t *testing.T) {
	d := t.TempDir()
	src := filepath.Join(d, "src")
	writeFile(t, filepath.Join(src, "a", "b", "c.txt"), "c", time.Now())
	for _, withPlan := range []bool{true, false} {
		dst := filepath.Join(d, fmt.Sprintf("dst%v", withPlan))
		pairs := []copyPair{{from: src, to: dst}}
		o := &opCtx{}
		plan := o.scan(pairs, false)
		writeFile(t, filepath.Join(src, "a", "late.txt"), "late", time.Now())
		if withPlan {
			o.copyPlan(pairs, plan)
		} else {
			o.copyAll(pairs)
		}
		if len(o.errs) != 0 || readFile(t, filepath.Join(dst, "a", "b", "c.txt")) != "c" {
			t.Fatalf("copy: %v", o.errs)
		}
		if exists(filepath.Join(dst, "a", "late.txt")) == withPlan {
			t.Errorf("plan=%v: late file copied=%v", withPlan, !withPlan)
		}
		_ = os.Remove(filepath.Join(src, "a", "late.txt"))
	}
}

// A move into an existing folder merges with the plan and removes the source; a move
// whose destination is free on the same volume is not read at all (it is renamed).
func TestMovePlan(t *testing.T) {
	d := t.TempDir()
	src, dst := filepath.Join(d, "src", "dir"), filepath.Join(d, "dst", "dir")
	writeFile(t, filepath.Join(src, "x", "1.txt"), "1", time.Now())
	writeFile(t, filepath.Join(src, "2.txt"), "2", time.Now())
	writeFile(t, filepath.Join(dst, "old.txt"), "o", time.Now())
	pairs := []copyPair{{from: src, to: dst}}
	o := &opCtx{}
	plan := o.scan(pairs, true)
	if !plan.roots[0].scanned {
		t.Fatal("merge not scanned")
	}
	o.movePlan(pairs, plan)
	if len(o.errs) != 0 || exists(src) || readFile(t, filepath.Join(dst, "x", "1.txt")) != "1" || !exists(filepath.Join(dst, "old.txt")) {
		t.Fatalf("merge move: %v", o.errs)
	}
	free := filepath.Join(d, "free")
	pairs = []copyPair{{from: dst, to: free}}
	plan = o.scan(pairs, true)
	if plan.roots[0].scanned || pairs[0].size != 0 {
		t.Fatal("a rename on the same volume was read")
	}
	o.movePlan(pairs, plan)
	if exists(dst) || readFile(t, filepath.Join(free, "2.txt")) != "2" {
		t.Fatal("rename")
	}
	if !sameVolume(d, free) {
		t.Error("sameVolume")
	}
}

// Sorting with the cached lower-case names gives exactly the order of the old
// comparison (lower-case computed at every comparison).
func TestSortKeysSameOrder(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	words := []string{"Alpha", "alpha", "BETA", "beta", "Ärger", "zeta", "Zeta", "file10", "file2", "ÉTÉ", "été"}
	var list []entry
	for i := 0; i < 2000; i++ {
		list = append(list, entry{name: words[r.Intn(len(words))] + fmt.Sprint(r.Intn(5)), isDir: r.Intn(4) == 0,
			size: int64(r.Intn(3)), mod: time.Unix(int64(r.Intn(3)), 0)})
	}
	old := func(a, b entry, mode int, asc bool) bool {
		if a.isDir != b.isDir {
			return a.isDir
		}
		byName := func() bool { return strings.ToLower(a.name) < strings.ToLower(b.name) }
		switch mode {
		case sortSize:
			if a.size != b.size {
				return (a.size < b.size) == asc
			}
		case sortDate:
			if !a.mod.Equal(b.mod) {
				return a.mod.Before(b.mod) == asc
			}
		default:
			if strings.ToLower(a.name) != strings.ToLower(b.name) {
				return byName() == asc
			}
		}
		return byName()
	}
	for mode := range sortLabels {
		for _, asc := range []bool{true, false} {
			a := append([]entry(nil), list...)
			b := append([]entry(nil), list...)
			p := &Panel{sortMode: mode, sortAsc: asc}
			p.sortEntries(a)
			sort.SliceStable(b, func(i, j int) bool { return old(b[i], b[j], mode, asc) })
			for i := range a {
				if a[i].name != b[i].name || a[i].isDir != b[i].isDir || a[i].size != b[i].size {
					t.Fatalf("mode %d asc %v: row %d %q vs %q", mode, asc, i, a[i].name, b[i].name)
				}
			}
		}
	}
}

// The family is computed once and kept; renaming in the flat view resets it.
func TestEntryCaches(t *testing.T) {
	e := entry{name: "Report.PDF"}
	if e.family() != family("Report.PDF") || e.fam == 0 || e.lower() != "report.pdf" {
		t.Fatal("caches")
	}
	e.name = "x.unknownext" // what the rename code does: name, lname and fam together
	e.lname, e.fam = "", 0
	if e.family() != -1 || e.lower() != "x.unknownext" {
		t.Fatal("caches after a rename")
	}
}

// The totals drawn at every frame follow every change (render invalidates them).
func TestPanelTotalsCache(t *testing.T) {
	d := t.TempDir()
	for _, f := range []string{"a.txt", "b.txt"} {
		writeFile(t, filepath.Join(d, f), "12345", time.Now())
	}
	fm := newTestFM(t)
	p := fm.left
	p.chdir(d, "")
	tot := p.totals()
	if tot.files != 2 || tot.size != 10 || tot.items != 2 || tot.mN != 0 {
		t.Fatalf("%+v", *tot)
	}
	p.jumpTo("a.txt")
	fm.setActive(p, true)
	fm.keys(key(tcell.KeyInsert, 0, 0))
	if l := p.selectionLabel(); !strings.Contains(l, "1 selected · 5 B") {
		t.Fatalf("label %q", l)
	}
	// marks changed without a render: the different count is noticed
	p.marked["b.txt"] = true
	if p.totals().mN != 2 {
		t.Fatal("stale totals")
	}
	_ = os.WriteFile(filepath.Join(d, "c.txt"), []byte("1"), 0o644)
	p.reload()
	if tot := p.totals(); tot.files != 3 || tot.size != 11 {
		t.Fatalf("after reload %+v", *tot)
	}
}

// Marked folders are measured (at most sizeWorkers at a time) and the panel shows
// every total once they are done.
func TestFolderSizesPool(t *testing.T) {
	d := t.TempDir()
	for i := 0; i < 12; i++ {
		writeFile(t, filepath.Join(d, fmt.Sprintf("d%02d", i), "f.txt"), strings.Repeat("x", i+1), time.Now())
	}
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(d, "")
		fm.setActive(fm.left, true)
		fm.keys(ctrlKey('A'))
	})
	waitUI(t, ui, "all folders measured", func() bool {
		t := fm.left.totals()
		return t.mN == 12 && !t.mPending && t.mSize == 78
	})
	deadline := time.Now().Add(2 * time.Second)
	for len(sizeSlots) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if cap(sizeSlots) != sizeWorkers || len(sizeSlots) != 0 {
		t.Error("slots not released")
	}
}

// The key bar writes Ctrl as "C" (Ctrl+A → C+a); the summary keeps the full names.
func TestBarKey(t *testing.T) {
	for in, want := range map[string]string{"Ctrl+A": "C+a", "Ctrl+F7": "C+F7", "Ctrl+F2": "C+F2", "Alt+U": "Alt+U", "F4": "F4", "Del": "Del", "Shift+X": "Shift+X"} {
		if got := barKey(in); got != want {
			t.Errorf("%s → %s", in, got)
		}
	}
	fm := newTestFM(t)
	s := screenOf(t, fm.keybar, 200, 1)
	var line strings.Builder
	for x := 0; x < 200; x++ {
		r, _, _, _ := s.GetContent(x, 0)
		line.WriteRune(r)
	}
	if !strings.Contains(line.String(), "C+aSelect") || strings.Contains(line.String(), "Ctrl+") {
		t.Errorf("bar %q", line.String())
	}
	if !strings.Contains(helpText(), "Ctrl+A") {
		t.Error("summary shortened")
	}
}
