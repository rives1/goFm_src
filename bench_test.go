package main

// Benchmarks (v1.17): the costs that grow with the size of a listing (sort, render,
// totals drawn at every frame) and the preparation of a copy into an existing tree
// (reading the folders, finding the files that already exist).
//
//   go test -run XXX -bench . -benchmem

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// bigListing is a flat-view-like listing of n files in nested folders.
func bigListing(n int) []entry {
	out := make([]entry, n)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	exts := []string{".go", ".txt", ".PDF", ".jpg", ".zip", ".exe", ".md", ".bin"}
	for i := range out {
		out[i] = entry{name: fmt.Sprintf("Dir%03d%cSub%02d%cFile_%06d%s", i%997, filepath.Separator, i%31, filepath.Separator, (i*7919)%n, exts[i%len(exts)]),
			size: int64(i * 37 % 100000), mod: t0.Add(time.Duration(i*13%86400) * time.Second)}
	}
	return out
}

func BenchmarkSortEntries200k(b *testing.B) {
	base := bigListing(200_000)
	p := &Panel{}
	for _, mode := range []int{sortName, sortSize} {
		b.Run(sortLabels[mode], func(b *testing.B) {
			p.sortMode, p.sortAsc = mode, defaultAsc(mode)
			for i := 0; i < b.N; i++ {
				list := append([]entry(nil), base...)
				p.sortEntries(list)
			}
		})
	}
}

func benchPanel(b *testing.B, n, marked int) *Panel {
	b.Helper()
	p := newPanel(nil, "")
	p.entries = bigListing(n)
	p.width = 80
	for i := 0; i < marked; i++ {
		p.marked[p.entries[i*(n/marked)].name] = true
	}
	return p
}

func BenchmarkRender20k(b *testing.B) {
	p := benchPanel(b, 20_000, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.render()
	}
}

// One frame of a panel: the table, the totals on the borders, the scroll bar.
func BenchmarkDrawFrame200k(b *testing.B) {
	p := benchPanel(b, 200_000, 1000)
	p.render()
	s := tcell.NewSimulationScreen("UTF-8")
	_ = s.Init()
	s.SetSize(100, 40)
	p.table.SetRect(0, 0, 100, 40)
	p.table.Draw(s) // the first frame adapts the rows to the width
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.table.Draw(s)
	}
}

// copyFixture: a source tree of dirs×files files and a destination holding a copy of
// half of them, as for F5 into an existing folder.
func copyFixture(b *testing.B, dirs, files int) []copyPair {
	b.Helper()
	root := b.TempDir()
	src, dst := filepath.Join(root, "src", "data"), filepath.Join(root, "dst", "data")
	for d := 0; d < dirs; d++ {
		for _, base := range []string{src, dst} {
			_ = os.MkdirAll(filepath.Join(base, fmt.Sprintf("d%03d", d)), 0o755)
		}
		for f := 0; f < files; f++ {
			name := filepath.Join(fmt.Sprintf("d%03d", d), fmt.Sprintf("f%04d.txt", f))
			_ = os.WriteFile(filepath.Join(src, name), []byte("x"), 0o644)
			if f%2 == 0 {
				_ = os.WriteFile(filepath.Join(dst, name), []byte("y"), 0o644)
			}
		}
	}
	return []copyPair{{from: src, to: dst}}
}

// The preparation of F5/F6 before the first byte is copied.
func BenchmarkPrepareCopy(b *testing.B) {
	pairs := copyFixture(b, 50, 200) // 10 000 files, 5 000 already at the destination
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o := &opCtx{}
		prepareCopy(o, pairs, false)
	}
}

// prepareCopy is what F5/F6 do before copying (v1.17: one scan).
func prepareCopy(o *opCtx, pairs []copyPair, move bool) int {
	return len(o.scan(pairs, move).conflicts)
}
