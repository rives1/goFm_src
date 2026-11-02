package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// cursorName is the name under the cursor of a panel.
func cursorName(p *Panel) string {
	if e := p.current(); e != nil {
		return e.name
	}
	return ""
}

func TestFirstRow(t *testing.T) {
	up := entry{name: "..", isDir: true, isUp: true}
	list := []entry{up, {name: "Dir"}, {name: "dir"}, {name: "file.txt"}}
	for sel, want := range map[string]int{"": 1, "missing": 1, "dir": 2, "DIR": 2, "file.txt": 3, "FILE.TXT": 3} {
		if got := firstRow(list, sel); got != want {
			t.Errorf("%q: row %d, want %d", sel, got, want)
		}
	}
	if firstRow([]entry{up}, "") != 0 || firstRow([]entry{{name: "C:\\"}}, "") != 0 || firstRow(nil, "") != 0 {
		t.Error("empty folder or drive list")
	}
}

// Entering a folder, opening an archive, closing the flat view and closing the tree
// put the cursor on the first item; going up keeps it on the folder left.
func TestCursorOnFirstItem(t *testing.T) {
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "sub", "b.txt"), "b", time.Now())
	writeFile(t, filepath.Join(d, "sub", "deep", "c.txt"), "c", time.Now())
	writeFile(t, filepath.Join(d, "a.txt"), "a", time.Now())
	_ = os.Mkdir(filepath.Join(d, "empty"), 0o755)
	fm := newTestFM(t)
	p := fm.left
	fm.setActive(p, true)
	p.chdir(d, "")
	if cursorName(p) != "empty" { // folders first: empty, sub, then a.txt
		t.Fatalf("chdir: cursor on %q", cursorName(p))
	}
	p.jumpTo("sub")
	p.enterDir()
	if cursorName(p) != "deep" {
		t.Fatalf("→: cursor on %q", cursorName(p))
	}
	p.goParent()
	if cursorName(p) != "sub" {
		t.Fatalf("←: cursor on %q, want the folder left", cursorName(p))
	}
	p.jumpTo("empty")
	p.enterDir()
	if cursorName(p) != ".." {
		t.Fatalf("empty folder: cursor on %q", cursorName(p))
	}
	p.goParent()
	// flat view: open and close
	fm.toggleFlatSync(t, p)
	if !p.flat {
		t.Fatal("flat view not opened")
	}
	p.table.Select(len(p.entries)-1, 0)
	fm.keys(ctrlKey('B'))
	if p.flat || cursorName(p) != "empty" {
		t.Fatalf("flat view closed: cursor on %q", cursorName(p))
	}
	// tree: close puts the left panel on its first item
	p.jumpTo("sub")
	fm.keys(ctrlKey('T'))
	if fm.tree == nil {
		t.Fatal("tree not opened")
	}
	fm.keys(ctrlKey('T'))
	if fm.tree != nil || cursorName(fm.left) == ".." {
		t.Fatalf("tree closed: cursor on %q", cursorName(fm.left))
	}
}

// toggleFlatSync opens the flat view of p and waits for it (the scan runs in a task).
func (fm *FM) toggleFlatSync(t *testing.T, p *Panel) {
	t.Helper()
	root := p.dir
	list, trunc := (&opCtx{}).flatScan(root, p.showHidden)
	p.flat, p.flatEntries, p.flatTrunc = true, list, trunc
	p.marked, p.filter = map[string]bool{}, ""
	_ = p.load("")
	if cursorName(p) == ".." {
		t.Fatalf("flat view: cursor on ..")
	}
}

func TestDoubleClick(t *testing.T) {
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "sub", "x.txt"), "x", time.Now())
	writeFile(t, filepath.Join(d, "a.txt"), "a", time.Now())
	fm := newTestFM(t)
	p := fm.left
	p.chdir(d, "")
	fm.setActive(fm.right, true)
	screenOf(t, p.table, 60, 12)
	h := p.table.MouseHandler()
	var focused tview.Primitive
	dbl := func(y int) {
		h(tview.MouseLeftDoubleClick, tcell.NewEventMouse(10, y, tcell.Button1, 0), func(x tview.Primitive) { focused = x })
	}
	p.table.Select(2, 0) // a.txt (row 0 "..", row 1 "sub")
	dbl(2)               // double click on "sub" while the cursor is elsewhere: only moves there
	if p.dir != d || cursorName(p) != "sub" || focused != p.table || fm.active != p {
		t.Fatalf("double click on another row: dir %q cursor %q", p.dir, cursorName(p))
	}
	dbl(2) // now on the cursor row: Enter
	if p.dir != filepath.Join(d, "sub") || cursorName(p) != "x.txt" {
		t.Fatalf("double click = Enter: dir %q cursor %q", p.dir, cursorName(p))
	}
	screenOf(t, p.table, 60, 12)
	p.table.Select(0, 0)
	dbl(1) // ".."
	if p.dir != d || cursorName(p) != "sub" {
		t.Fatalf("double click on ..: dir %q cursor %q", p.dir, cursorName(p))
	}
	dbl(11) // below the list: nothing
	if p.dir != d {
		t.Fatal("double click below the list")
	}
}

func TestCapitalize(t *testing.T) {
	cases := map[[2]string]string{
		{"rapporto_IVA-q3.pdf", "f"}: "Rapporto_IVA-Q3.pdf",
		{"my file name.txt", "f"}:    "My File Name.txt",
		{"foto.JPG", "f"}:            "Foto.JPG",
		{"archive.tar.gz", "f"}:      "Archive.tar.gz",
		{".profile", "f"}:            ".profile",
		{"a--b__c  d", "f"}:          "A--B__C  D",
		{"3d_model-v2.stl", "f"}:     "3d_Model-V2.stl",
		{"èlite dir", "d"}:           "Èlite Dir",
		{"some dir.v2", "d"}:         "Some Dir.v2",
		{"Already Ok.txt", "f"}:      "Already Ok.txt",
		{"x.y", "f"}:                 "X.y",
	}
	for in, want := range cases {
		if got := capitalizedName(in[0], in[1] == "d"); got != want {
			t.Errorf("%q: %q, want %q", in[0], got, want)
		}
	}
}

func TestRenameEach(t *testing.T) {
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "one two.txt"), "1", time.Now())
	writeFile(t, filepath.Join(d, "taken.txt"), "2", time.Now())
	writeFile(t, filepath.Join(d, "Taken.txt"), "3", time.Now()) // a different file: skipped
	writeFile(t, filepath.Join(d, "sub", "deep name.md"), "4", time.Now())
	writeFile(t, filepath.Join(d, "same.txt"), "5", time.Now())
	// a second name for the same file, as a case-insensitive file system sees "Same.txt"
	if err := os.Link(filepath.Join(d, "same.txt"), filepath.Join(d, "Same.txt")); err != nil {
		t.Skip("no hard links:", err)
	}
	o := &opCtx{}
	names := []string{"one two.txt", "taken.txt", filepath.Join("sub", "deep name.md"), "same.txt", "Already.txt"}
	r := o.renameEach(d, names, func(rel string) string { return capitalizedName(filepath.Base(rel), false) })
	if len(o.errs) != 0 {
		t.Fatalf("errors %v", o.errs)
	}
	want := map[string]string{"one two.txt": "One Two.txt", filepath.Join("sub", "deep name.md"): filepath.Join("sub", "Deep Name.md"), "same.txt": "Same.txt"}
	for k, v := range want {
		if r.renamed[k] != v {
			t.Errorf("%q → %q, want %q", k, r.renamed[k], v)
		}
	}
	if len(r.renamed) != 3 || len(r.skipped) != 1 || r.skipped[0] != "taken.txt" {
		t.Fatalf("renamed %v skipped %v", r.renamed, r.skipped)
	}
	if readFile(t, filepath.Join(d, "One Two.txt")) != "1" || readFile(t, filepath.Join(d, "Taken.txt")) != "3" {
		t.Error("contents")
	}
}

func TestCapitalizeDialog(t *testing.T) {
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "my_file-name.txt"), "1", time.Now())
	writeFile(t, filepath.Join(d, "Already Ok.txt"), "2", time.Now())
	_ = os.Mkdir(filepath.Join(d, "some dir"), 0o755)
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(d, "")
		fm.setActive(fm.left, true)
		fm.keys(ctrlKey('A'))
		fm.keys(ctrlKey('R'))
	})
	waitUI(t, ui, "dialog", func() bool { return len(fm.modals) == 1 })
	ui(func() {
		text := fm.app.GetFocus()
		_ = text
		pressEnter(fm) // Rename (the default button)
	})
	waitUI(t, ui, "renamed", func() bool {
		return exists(filepath.Join(d, "My_File-Name.txt")) && exists(filepath.Join(d, "Some Dir")) && len(fm.modals) == 0
	})
	ui(func() {
		if !exists(filepath.Join(d, "Already Ok.txt")) || len(fm.left.marked) != 0 || !strings.Contains(fm.notice, "Renamed 2 items") {
			t.Errorf("after Ctrl+R: notice %q marks %v", fm.notice, fm.left.marked)
		}
		// nothing to change: only a notice
		fm.left.jumpTo("Already Ok.txt")
		fm.keys(ctrlKey('R'))
		if len(fm.modals) != 0 || !strings.Contains(fm.notice, "already start with capitals") {
			t.Errorf("nothing to change: notice %q", fm.notice)
		}
	})
	txt := capitalizePreview([][2]string{{"a b", "A B"}}, 2)
	if !strings.Contains(txt, "this name?") || !strings.Contains(txt, "a b → A B") || !strings.Contains(txt, "2 names already capitalized") {
		t.Errorf("preview %q", txt)
	}
}

func TestResolveDuplicates(t *testing.T) {
	pr := func(from, to string) copyPair { return copyPair{from: from, to: to} }
	pairs := []copyPair{
		pr("/s/a/x.txt", "/d/x.txt"), pr("/s/y.txt", "/d/y.txt"),
		pr("/s/b/x.txt", "/d/x.txt"), pr("/s/c/x.txt", "/d/x.txt"), pr("/s/b/y.txt", "/d/y.txt"),
	}
	run := func(answers ...string) (string, bool, int) {
		asked := 0
		var qs []dupQuestion
		kept, ok := resolveDuplicates(pairs, func(q dupQuestion) string { qs = append(qs, q); asked++; return answers[asked-1] })
		var s []string
		for _, k := range kept {
			s = append(s, k.from)
		}
		if asked > 0 && (qs[0].n != 3 || qs[0].i != 0 || qs[0].chosen.from != "/s/a/x.txt" || qs[0].dup.from != "/s/b/x.txt") {
			t.Errorf("first question %+v", qs[0])
		}
		return strings.Join(s, ","), ok, asked
	}
	if s, ok, n := run(conflictSkip, conflictSkip, conflictSkip); !ok || s != "/s/a/x.txt,/s/y.txt" || n != 3 {
		t.Errorf("skip: %q %v %d", s, ok, n)
	}
	if s, ok, n := run(conflictOverwrite, conflictSkip, conflictOverwrite); !ok || s != "/s/b/x.txt,/s/b/y.txt" || n != 3 {
		t.Errorf("overwrite/skip: %q %v %d", s, ok, n)
	}
	if s, ok, n := run(conflictOverwriteAll); !ok || s != "/s/c/x.txt,/s/b/y.txt" || n != 1 {
		t.Errorf("overwrite all: %q %v %d", s, ok, n)
	}
	if s, ok, n := run(conflictSkipAll); !ok || s != "/s/a/x.txt,/s/y.txt" || n != 1 {
		t.Errorf("skip all: %q %v %d", s, ok, n)
	}
	if _, ok, _ := run(conflictSkip, ""); ok {
		t.Error("Esc must cancel")
	}
	if kept, ok := resolveDuplicates(pairs[:2], func(dupQuestion) string { t.Error("asked"); return "" }); !ok || len(kept) != 2 {
		t.Error("no duplicates")
	}
}

func TestDupText(t *testing.T) {
	d := t.TempDir()
	old, now := time.Now().Add(-time.Hour), time.Now()
	writeFile(t, filepath.Join(d, "src", "a", "util.go"), "aa", old)
	writeFile(t, filepath.Join(d, "src", "b", "deep", "util.go"), "bbbb", now)
	q := dupQuestion{
		chosen: copyPair{from: filepath.Join(d, "src", "a", "util.go"), to: "/d/util.go"},
		dup:    copyPair{from: filepath.Join(d, "src", "b", "deep", "util.go"), to: "/d/util.go"},
		i:      0, n: 2,
	}
	txt := dupText(q)
	for _, want := range []string{"(1 of 2)", "\nutil.go\n", "Chosen:   " + filepath.Join("a", "util.go") + " · 2 B",
		"This one: " + filepath.Join("b", "deep", "util.go") + " · 4 B", "(newer)", "Overwrite takes this one"} {
		if !strings.Contains(txt, want) {
			t.Errorf("missing %q in %q", want, txt)
		}
	}
	if strings.Count(txt, "(newer)") != 1 {
		t.Error("only the newer copy is marked")
	}
}

// Flat view, F5 with two files of the same name: the question replaces the refusal.
func TestFlatDuplicatesCopy(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, "a", "x.txt"), "from a", time.Now())
	writeFile(t, filepath.Join(src, "b", "x.txt"), "from b", time.Now())
	writeFile(t, filepath.Join(src, "a", "y.txt"), "y", time.Now())
	for _, answer := range []string{"skip", "overwrite", "cancel"} {
		_ = os.RemoveAll(dst)
		_ = os.Mkdir(dst, 0o755)
		fm := newTestFM(t)
		ui := runUI(t, fm)
		ui(func() {
			fm.left.chdir(src, "")
			fm.right.chdir(dst, "")
			fm.setActive(fm.left, true)
			fm.toggleFlatSync(t, fm.left)
			fm.keys(ctrlKey('A'))
			fm.keys(key(tcell.KeyF5, 0, 0))
		})
		waitUI(t, ui, "duplicate question", func() bool { return len(fm.modals) == 1 })
		ui(func() {
			switch answer {
			case "skip":
				pressEnter(fm) // Skip is the default button
			case "overwrite": // Skip, Skip all, Overwrite: two Tabs
				for i := 0; i < 2; i++ {
					fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, 0), func(tview.Primitive) {})
				}
				pressEnter(fm)
			case "cancel":
				fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
			}
		})
		if answer == "cancel" {
			waitUI(t, ui, "cancelled", func() bool { return len(fm.modals) == 0 })
			time.Sleep(100 * time.Millisecond)
			if des, _ := os.ReadDir(dst); len(des) != 0 {
				t.Errorf("cancel: copied %v", des)
			}
			continue
		}
		waitUI(t, ui, "copied "+answer, func() bool {
			return exists(filepath.Join(dst, "x.txt")) && exists(filepath.Join(dst, "y.txt")) && len(fm.modals) == 0
		})
		want := map[string]string{"skip": "from a", "overwrite": "from b"}[answer]
		if got := readFile(t, filepath.Join(dst, "x.txt")); got != want {
			t.Errorf("%s: x.txt = %q, want %q", answer, got, want)
		}
	}
}

func TestSwapPanels(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(a, "a1.txt"), "1", time.Now())
	writeFile(t, filepath.Join(a, "a2.txt"), "2", time.Now())
	writeFile(t, filepath.Join(b, "b1.txt"), "3", time.Now())
	fm := newTestFM(t)
	L, R := fm.left, fm.right
	L.chdir(a, "a2.txt")
	L.marked["a1.txt"] = true
	L.sortAsc = false
	R.chdir(b, "")
	R.filter = "b"
	fm.setActive(L, true)
	fm.keys(ctrlKey('U'))
	if fm.left != R || fm.right != L {
		t.Fatal("panels not swapped")
	}
	if fm.active != fm.left || fm.app.GetFocus() != fm.left.table || fm.left.table.GetBorderColor() != colActiveBorder {
		t.Fatal("the highlight must stay on the left side")
	}
	if fm.left.dir != b || fm.right.dir != a || cursorName(fm.right) != "a2.txt" || !fm.right.marked["a1.txt"] || fm.right.sortAsc || fm.left.filter != "b" {
		t.Fatal("state not carried")
	}
	items := fm.panelsBox.GetItemCount()
	if items != 2 || fm.panelsBox.GetItem(0) != fm.left.table || fm.panelsBox.GetItem(1) != fm.right.table {
		t.Fatal("layout")
	}
	st, _ := loadState()
	if st.Left != b || st.Right != a {
		t.Errorf("state.json: %q %q", st.Left, st.Right)
	}
	if !strings.Contains(fm.notice, "swapped") {
		t.Errorf("notice %q", fm.notice)
	}
	fm.keys(ctrlKey('U'))
	if fm.left != L || fm.active != L {
		t.Fatal("swap back")
	}
	// refused in the tree view: on the tree a notice, in the right panel a message
	fm.keys(ctrlKey('T'))
	fm.keys(ctrlKey('U'))
	if fm.tree == nil || len(fm.modals) != 0 || !strings.Contains(fm.notice, "Ctrl+T back to two panels") {
		t.Errorf("Ctrl+U on the tree: notice %q", fm.notice)
	}
	fm.setActive(fm.right, true)
	fm.keys(ctrlKey('U'))
	if fm.right != R || len(fm.modals) != 1 {
		t.Error("Ctrl+U in the right panel of the tree view")
	}
}

// A paused copy starts no new file and stops inside a file; Resume finishes it;
// Cancel works during a pause.
func TestPauseEngine(t *testing.T) {
	d := t.TempDir()
	for i := 0; i < 6; i++ {
		writeFile(t, filepath.Join(d, "src", string(rune('a'+i))+".txt"), strings.Repeat("x", 1000), time.Now())
	}
	o := &opCtx{}
	o.paused.Store(true)
	done := make(chan struct{})
	go func() { o.copyPath(filepath.Join(d, "src"), filepath.Join(d, "dst")); close(done) }()
	time.Sleep(300 * time.Millisecond)
	if des, _ := os.ReadDir(filepath.Join(d, "dst")); len(des) != 0 || o.bytes.Load() != 0 {
		t.Fatalf("copied during the pause: %d files, %d bytes", len(des), o.bytes.Load())
	}
	o.paused.Store(false)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("not resumed")
	}
	if des, _ := os.ReadDir(filepath.Join(d, "dst")); len(des) != 6 {
		t.Fatalf("%d files after Resume", len(des))
	}
	// cancel while paused
	o2 := &opCtx{}
	o2.paused.Store(true)
	done2 := make(chan struct{})
	go func() { o2.copyPath(filepath.Join(d, "src"), filepath.Join(d, "dst2")); close(done2) }()
	time.Sleep(150 * time.Millisecond)
	o2.cancel.Store(true)
	select {
	case <-done2:
	case <-time.After(5 * time.Second):
		t.Fatal("Cancel does not work during a pause")
	}
	// inside a file: the reader waits
	o3 := &opCtx{}
	r := cancelReader{strings.NewReader("abc"), o3}
	o3.paused.Store(true)
	var n atomic.Int64
	go func() { b := make([]byte, 3); k, _ := r.Read(b); n.Store(int64(k)) }()
	time.Sleep(200 * time.Millisecond)
	if n.Load() != 0 {
		t.Fatal("read during the pause")
	}
	o3.paused.Store(false)
	time.Sleep(200 * time.Millisecond)
	if n.Load() != 3 {
		t.Fatal("read not resumed")
	}
}

func TestPauseText(t *testing.T) {
	o := &opCtx{}
	o.total.Store(1000)
	o.bytes.Store(400)
	m := &rateMeter{}
	t0 := time.Now()
	progressText("Copy", o, m, 66, t0)
	o.bytes.Store(500)
	if txt := progressText("Copy", o, m, 66, t0.Add(time.Second)); !strings.Contains(txt, "left") {
		t.Fatalf("running: %q", txt)
	}
	o.paused.Store(true)
	txt := progressText("Copy", o, m, 66, t0.Add(2*time.Second))
	if !strings.Contains(txt, "PAUSED") || strings.Contains(txt, "/s") || len(m.samples) != 0 {
		t.Fatalf("paused: %q, %d samples", txt, len(m.samples))
	}
	o.phase.Store("Reading the folders…")
	if txt := progressText("Copy", o, m, 66, t0); !strings.Contains(txt, "PAUSED") {
		t.Fatalf("paused in a phase: %q", txt)
	}
}

// The progress dialog of a copy has Pause, which becomes Resume; other tasks do not.
func TestPauseButton(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(src, string(rune('a'+i))+".txt"), "x", time.Now())
	}
	slow := copyOneFile
	copyOneFile = func(o *opCtx, s, d string, fi os.FileInfo) error {
		time.Sleep(40 * time.Millisecond)
		return slow(o, s, d, fi)
	}
	defer func() { copyOneFile = slow }()
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(src, "")
		fm.right.chdir(dst, "")
		fm.setActive(fm.left, true)
		fm.keys(ctrlKey('A'))
		fm.keys(key(tcell.KeyF5, 0, 0))
	})
	var form *tview.Form
	waitUI(t, ui, "progress dialog", func() bool {
		if len(fm.modals) != 1 {
			return false
		}
		f, ok := fm.focus[fm.modals[0]].(*tview.Form)
		form = f
		return ok
	})
	ui(func() {
		if form.GetButtonCount() != 2 || form.GetButton(0).GetLabel() != pauseLabel || form.GetButton(1).GetLabel() != "Cancel" {
			t.Fatalf("buttons %d %q", form.GetButtonCount(), form.GetButton(0).GetLabel())
		}
		pressEnter(fm) // Pause has the focus
		if form.GetButton(0).GetLabel() != resumeLabel {
			t.Fatal("Pause did not become Resume")
		}
	})
	countDst := func() int { des, _ := os.ReadDir(dst); return len(des) }
	time.Sleep(150 * time.Millisecond)
	before := countDst()
	time.Sleep(400 * time.Millisecond)
	if after := countDst(); after != before || after == 20 {
		t.Fatalf("copying during the pause: %d → %d", before, after)
	}
	ui(func() { pressEnter(fm) }) // Resume
	waitUI(t, ui, "copy finished", func() bool { return countDst() == 20 && len(fm.modals) == 0 })
	// a task without Pause (a delete) keeps the single Cancel button
	var only int
	ui(func() {
		fm.runTask("Test", func(o *opCtx) { time.Sleep(300 * time.Millisecond) }, nil)
		f := fm.focus[fm.modals[0]].(*tview.Form)
		only = f.GetButtonCount()
	})
	if only != 1 {
		t.Errorf("non-pausable task: %d buttons", only)
	}
}

// New keys are in the summary and in the key bar.
func TestKeysV118(t *testing.T) {
	text := helpText()
	for _, k := range []string{"Ctrl+R", "Ctrl+U", "Double click"} {
		if !strings.Contains(text, "]"+k+"[") {
			t.Errorf("%s missing from the summary", k)
		}
	}
	bar := map[string]bool{}
	for _, it := range keybarItems {
		bar[it.key] = true
	}
	if !bar["Ctrl+R"] || !bar["Ctrl+U"] {
		t.Error("key bar")
	}
}
