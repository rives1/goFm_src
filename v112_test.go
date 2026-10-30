package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// forEachScheme runs f once per colour scheme, then restores the main scheme.
func forEachScheme(t *testing.T, f func(t *testing.T)) {
	t.Helper()
	defer applyScheme(0)
	for i, s := range schemes {
		applyScheme(i)
		t.Run(s.name, f)
	}
}

// Every scheme is readable: text 4.5:1 (WCAG AA), borders and other graphics 3:1.
func TestSchemesReadable(t *testing.T) {
	names := map[string]bool{}
	for _, s := range schemes {
		if names[s.name] {
			t.Errorf("scheme %q twice", s.name)
		}
		names[s.name] = true
	}
	if schemes[0].name != "navy" || len(schemes) != 3 {
		t.Fatal("navy must be the main scheme, three in all")
	}
	forEachScheme(t, func(t *testing.T) {
		pairs := []struct {
			what   string
			fg, bg tcell.Color
			min    float64
		}{
			{"text", colText, colBackground, 4.5},
			{"title", colTitle, colBackground, 4.5},
			{"folder", colFolder, colBackground, 4.5},
			{"mark", colMarked, colBackground, 4.5},
			{"mark on the inactive cursor", colMarked, colCursorOffBg, 3},
			{"cursor", colCursorFg, colCursorBg, 4.5},
			{"inactive cursor", colCursorOffFg, colCursorOffBg, 4.5},
			{"status text", colText, colBarBg, 4.5},
			{"status columns", colDim, colBarBg, 3},
			{"key names", colKeyFg, colKeyBg, 4.5},
			{"notice", colNoticeFg, colNoticeBg, 4.5},
			{"form labels", colLabel, colBackground, 4.5},
			{"dialog text", colText, colContrastBg, 4.5},
			{"error", colError, colBarBg, 4.5},
			{"active border", colActiveBorder, colBackground, 3},
			{"inactive border", colPanelBorder, colBackground, 3},
			{"search badge", colCursorFg, colMarked, 4.5},
		}
		for _, p := range pairs {
			if r := contrast(p.fg, p.bg); r < p.min {
				t.Errorf("%s: %06X on %06X = %.2f, want %.1f", p.what, p.fg.Hex(), p.bg.Hex(), r, p.min)
			}
		}
	})
	if schemeIndex("light") != 2 || schemeIndex("nonsense") != 0 || schemeIndex("") != 0 {
		t.Error("schemeIndex")
	}
}

func TestDriveKinds(t *testing.T) {
	want := map[driveType]iconKind{driveLocal: kindDrive, driveNetwork: kindDriveNet, driveRemovable: kindDriveUSB, driveOptical: kindDriveCD}
	labels := map[string]bool{}
	for d, k := range want {
		e := entry{name: `X:\`, isDir: true, isDrive: true, drive: d}
		if entryKind(e) != k {
			t.Errorf("drive %d: kind %d, want %d", d, entryKind(e), k)
		}
		if l := d.label(); uniseg.StringWidth(l) > sizeColW || labels[l] {
			t.Errorf("label %q too long or repeated", l)
		}
		labels[d.label()] = true
	}
	if isNetworkLocation("") || (!onWindows && isNetworkLocation("/mnt/x")) {
		t.Error("isNetworkLocation")
	}
}

func TestSortDirection(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	list := []entry{
		{name: "b.txt", size: 1, mod: t0.Add(2 * time.Hour)},
		{name: "A.txt", size: 3, mod: t0},
		{name: "dir", isDir: true},
		{name: "c.txt", size: 2, mod: t0.Add(time.Hour)},
		{name: "..", isDir: true, isUp: true},
	}
	order := func(mode int, asc bool) string {
		l := append([]entry(nil), list...)
		p := &Panel{sortMode: mode, sortAsc: asc}
		p.sortEntries(l)
		var n []string
		for _, e := range l {
			n = append(n, e.name)
		}
		return strings.Join(n, " ")
	}
	cases := []struct {
		mode int
		asc  bool
		want string
	}{
		{sortName, true, ".. dir A.txt b.txt c.txt"},
		{sortName, false, ".. dir c.txt b.txt A.txt"},
		{sortSize, false, ".. dir A.txt c.txt b.txt"},
		{sortSize, true, ".. dir b.txt c.txt A.txt"},
		{sortDate, false, ".. dir b.txt c.txt A.txt"},
		{sortDate, true, ".. dir A.txt c.txt b.txt"},
	}
	for _, c := range cases {
		if got := order(c.mode, c.asc); got != c.want {
			t.Errorf("%s %v: %s, want %s", sortLabels[c.mode], c.asc, got, c.want)
		}
	}
	if !defaultAsc(sortName) || defaultAsc(sortSize) || defaultAsc(sortDate) || sortArrow(true) != "↑" || sortArrow(false) != "↓" {
		t.Error("defaults and arrows")
	}
}

func key(k tcell.Key, r rune, mod tcell.ModMask) *tcell.EventKey { return tcell.NewEventKey(k, r, mod) }

func closeAll(fm *FM) {
	for len(fm.modals) > 0 {
		fm.closeModal(fm.modals[len(fm.modals)-1])
	}
}

func TestKeysV112(t *testing.T) {
	fm := newTestFM(t)
	p := fm.left
	for _, n := range []string{"a.txt", "b.txt"} {
		_ = os.WriteFile(filepath.Join(p.dir, n), []byte("x"), 0o644)
	}
	_ = os.Mkdir(filepath.Join(p.dir, "sub"), 0o755)
	p.reload()
	fm.setActive(p, true)
	p.table.SetRect(0, 0, 200, 20) // wide enough for the whole title
	title := func() string { p.updateTitle(); return p.table.GetTitle() }
	if !strings.Contains(title(), "sort: name ↑") {
		t.Errorf("title %q", title())
	}

	// Ctrl+A: all, then invert
	fm.keys(key(tcell.KeyCtrlA, 0, tcell.ModCtrl))
	if len(p.marked) != 3 {
		t.Fatalf("Ctrl+A marked %d", len(p.marked))
	}
	p.marked = map[string]bool{"a.txt": true}
	fm.keys(key(tcell.KeyCtrlA, 0, tcell.ModCtrl))
	if len(p.marked) != 2 || p.marked["a.txt"] || !p.marked["b.txt"] || !p.marked["sub"] {
		t.Fatalf("Ctrl+A did not invert: %v", p.marked)
	}
	p.marked = map[string]bool{}
	// '*' and Alt+A do nothing any more
	fm.keys(key(tcell.KeyRune, '*', 0))
	fm.keys(key(tcell.KeyRune, 'a', tcell.ModAlt))
	if len(p.marked) != 0 {
		t.Error("* or Alt+A still mark")
	}
	fm.resetQuickSearch()

	// Alt+O / Alt+D and the arrow in the title
	fm.keys(key(tcell.KeyRune, 'd', tcell.ModAlt))
	if p.sortAsc || !strings.Contains(title(), "sort: name ↓") {
		t.Errorf("Alt+D: %q", title())
	}
	fm.keys(key(tcell.KeyRune, 'o', tcell.ModAlt))
	if p.sortMode != sortSize || p.sortAsc || !strings.Contains(title(), "sort: size ↓") {
		t.Errorf("Alt+O: %q", title())
	}
	fm.keys(key(tcell.KeyRune, 'd', tcell.ModAlt))
	if !strings.Contains(title(), "sort: size ↑") {
		t.Errorf("Alt+D on size: %q", title())
	}

	// keys that open a dialog, and keys that must not
	opens := []struct {
		what string
		ev   *tcell.EventKey
	}{
		{"Alt+F7 find", key(tcell.KeyF7, 0, tcell.ModAlt)},
		{"Ctrl+F7 filter", key(tcell.KeyF7, 0, tcell.ModCtrl)},
		{"Ctrl+F2 keys", key(tcell.KeyF2, 0, tcell.ModCtrl)},
		{"Del", nil},
		{"F7 new folder", key(tcell.KeyF7, 0, 0)},
	}
	for _, o := range opens {
		if o.ev == nil { // Del needs a file under the cursor
			p.jumpTo("a.txt")
			o.ev = key(tcell.KeyDelete, 0, 0)
		}
		fm.keys(o.ev)
		if len(fm.modals) != 1 {
			t.Errorf("%s: %d dialogs", o.what, len(fm.modals))
		}
		closeAll(fm)
	}
	for _, ev := range []*tcell.EventKey{
		key(tcell.KeyF8, 0, 0), key(tcell.KeyF1, 0, tcell.ModAlt), key(tcell.KeyRune, 's', tcell.ModAlt),
		key(tcell.KeyRune, 'f', tcell.ModAlt), key(tcell.KeyF7, 0, tcell.ModCtrl|tcell.ModAlt), key(tcell.KeyF5, 0, tcell.ModCtrl),
	} {
		fm.keys(ev)
		if len(fm.modals) != 0 {
			t.Errorf("%v opened a dialog", ev.Name())
			closeAll(fm)
		}
	}
	if fm.active != p {
		t.Error("active panel changed")
	}
}

// The key bar shows the items in order of priority while they fit, never the keys
// removed from it, and a click runs the key.
func TestKeyBar(t *testing.T) {
	for _, w := range []int{0, 10, 40, 80, 120, 400} {
		slots := keybarLayout(w)
		for i, s := range slots {
			if s.item != i {
				t.Errorf("width %d: item %d out of order", w, s.item)
			}
			if s.x+s.w > w {
				t.Errorf("width %d: item %d does not fit", w, s.item)
			}
			if i > 0 && s.x != slots[i-1].x+slots[i-1].w+1 {
				t.Errorf("width %d: spacing", w)
			}
		}
		if w == 400 && len(slots) != len(keybarItems) {
			t.Errorf("wide bar shows %d of %d", len(slots), len(keybarItems))
		}
		if w == 80 && (len(slots) < 8 || len(slots) == len(keybarItems)) {
			t.Errorf("80 columns: %d items", len(slots))
		}
	}
	for _, it := range keybarItems {
		switch it.key {
		case "F2", "F3", "F5", "F7", "F10", "F8", "Alt+S", "Alt+F", "*":
			t.Errorf("%s must not be in the bar", it.key)
		}
	}
	if keybarItems[0].key != "F4" || itemAt(80, 0) != -1 || itemAt(80, 1) != 0 || itemAt(80, 6) != 0 || itemAt(80, 7) != -1 || itemAt(80, 8) != 1 {
		t.Error("itemAt")
	}

	fm := newTestFM(t)
	p := fm.left
	fm.setActive(p, true)
	fm.keybar.SetRect(0, 20, 400, 1)
	click := func(label string) {
		for _, s := range keybarLayout(400) {
			if keybarItems[s.item].label == label {
				ev := tcell.NewEventMouse(s.x+1, 20, tcell.Button1, 0)
				fm.keybar.MouseHandler()(tview.MouseLeftClick, ev, func(tview.Primitive) { t.Error("focus moved") })
				return
			}
		}
		t.Fatalf("%s not in the bar", label)
	}
	click("Reverse")
	if p.sortAsc {
		t.Error("click on Alt+D Reverse")
	}
	click("Keys")
	if len(fm.modals) != 1 {
		t.Error("click on Ctrl+F2 Keys")
	}
	closeAll(fm)
	click("Colours")
	if curScheme != 1 {
		t.Error("click on Ctrl+F9")
	}
	applyScheme(0)
	click("Drive X:") // only a hint
	if len(fm.modals) != 0 || fm.active != p {
		t.Error("Shift+X hint")
	}
}

func TestColourSchemeSwitch(t *testing.T) {
	fm := newTestFM(t)
	defer applyScheme(0)
	for i := 1; i <= 3; i++ {
		fm.keys(key(tcell.KeyF9, 0, tcell.ModCtrl))
		want := i % 3
		if curScheme != want || fm.left.table.GetBackgroundColor() != schemes[want].background ||
			fm.keybar.GetBackgroundColor() != schemes[want].barBg {
			t.Fatalf("step %d: scheme %d", i, curScheme)
		}
		st, _ := loadState()
		if (want == 0 && st.ColorScheme != "") || (want > 0 && st.ColorScheme != schemes[want].name) {
			t.Fatalf("saved %q", st.ColorScheme)
		}
		if !strings.Contains(fm.notice, schemes[want].name) {
			t.Errorf("notice %q", fm.notice)
		}
	}
	// a new FM starts with the saved scheme
	applyScheme(0)
	fm2 := newFM(t.TempDir(), t.TempDir(), savedState{ColorScheme: "light"})
	defer fm2.watch.close()
	if curScheme != 2 || fm2.left.table.GetBackgroundColor() != schemes[2].background {
		t.Error("saved scheme not applied at start")
	}
}

func TestEditorCommand(t *testing.T) {
	d := t.TempDir()
	spaced := filepath.Join(d, "my editor.exe")
	_ = os.WriteFile(spaced, nil, 0o755)
	cases := []struct {
		spec, def, prog string
		args            []string
		err             bool
	}{
		{"", "notepad.exe", "notepad.exe", nil, false},
		{"  ", "", "", nil, true},
		{`"C:\Program Files\Code\Code.exe" --reuse-window`, "", `C:\Program Files\Code\Code.exe`, []string{"--reuse-window"}, false},
		{`"C:\x\ed.exe"`, "", `C:\x\ed.exe`, nil, false},
		{`"C:\x\ed.exe`, "", "", nil, true},
		{spaced, "", spaced, nil, false},
		{"code --wait", "", "code", []string{"--wait"}, false},
	}
	for _, c := range cases {
		prog, args, err := editorCommand(c.spec, c.def)
		if (err != nil) != c.err || prog != c.prog || strings.Join(args, "|") != strings.Join(c.args, "|") {
			t.Errorf("%q: %q %q %v", c.spec, prog, args, err)
		}
	}
}

// F4 starts the editor with the file under the cursor and does not wait for it.
func TestEditFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the editor")
	}
	fm := newTestFM(t)
	p := fm.left
	file := filepath.Join(p.dir, "note.txt")
	_ = os.WriteFile(file, []byte("x"), 0o644)
	out := filepath.Join(t.TempDir(), "args")
	script := filepath.Join(t.TempDir(), "ed.sh")
	_ = os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+out+"\n"), 0o755)
	t.Setenv("GOFM_EDITOR", script+" --flag")
	p.reload()
	fm.setActive(p, true)
	p.jumpTo("note")
	fm.keys(key(tcell.KeyF4, 0, 0))
	deadline := time.Now().Add(5 * time.Second)
	var got []byte
	for time.Now().Before(deadline) {
		if got, _ = os.ReadFile(out); len(got) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if string(got) != "--flag\n"+file+"\n" {
		t.Fatalf("editor got %q", got)
	}
	if !strings.Contains(fm.notice, "Editing note.txt") || len(fm.modals) != 0 {
		t.Errorf("notice %q, %d dialogs", fm.notice, len(fm.modals))
	}
	// folders, "..", and a missing editor
	p.jumpTo("..")
	fm.keys(key(tcell.KeyF4, 0, 0))
	if len(fm.modals) != 0 {
		t.Error("F4 on ..")
	}
	t.Setenv("GOFM_EDITOR", filepath.Join(t.TempDir(), "missing-editor"))
	p.jumpTo("note")
	fm.keys(key(tcell.KeyF4, 0, 0))
	if len(fm.modals) != 1 {
		t.Error("no message for a missing editor")
	}
}

func writeFile(t *testing.T, path, content string, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(path, mod, mod)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return "<missing>"
	}
	return string(b)
}

// Conflicts at any depth: skipped files stay as they are, the others are copied or
// moved; a moved folder keeps only the skipped files.
func TestConflicts(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	for _, move := range []bool{false, true} {
		d := t.TempDir()
		src, dst := filepath.Join(d, "src"), filepath.Join(d, "dst")
		writeFile(t, filepath.Join(src, "top.txt"), "new top", t0.Add(time.Hour))
		writeFile(t, filepath.Join(src, "solo.txt"), "solo", t0)
		writeFile(t, filepath.Join(src, "dir", "a.txt"), "new a", t0)
		writeFile(t, filepath.Join(src, "dir", "deep", "b.txt"), "new b", t0)
		writeFile(t, filepath.Join(src, "dir", "c.txt"), "c only in source", t0)
		writeFile(t, filepath.Join(dst, "top.txt"), "old top", t0)
		writeFile(t, filepath.Join(dst, "dir", "a.txt"), "old a", t0)
		writeFile(t, filepath.Join(dst, "dir", "deep", "b.txt"), "old b", t0)
		pairs := []copyPair{
			{from: filepath.Join(src, "top.txt"), to: filepath.Join(dst, "top.txt")},
			{from: filepath.Join(src, "solo.txt"), to: filepath.Join(dst, "solo.txt")},
			{from: filepath.Join(src, "dir"), to: filepath.Join(dst, "dir")},
		}
		o := &opCtx{}
		cs := o.findConflicts(pairs)
		var names []string
		for _, c := range cs {
			r, _ := filepath.Rel(dst, c.dst)
			names = append(names, filepath.ToSlash(r))
		}
		if strings.Join(names, ",") != "top.txt,dir/a.txt,dir/deep/b.txt" {
			t.Fatalf("conflicts %v", names)
		}
		txt := conflictText(cs[0], 0, 3, dst)
		if !strings.Contains(txt, "(1 of 3)") || !strings.Contains(txt, "\ntop.txt\n") || !strings.Contains(txt, "(newer)") {
			t.Errorf("text %q", txt)
		}
		// Overwrite top.txt, Skip a.txt, then Overwrite b.txt
		answers := []string{conflictOverwrite, conflictSkip, conflictOverwrite}
		skip, ok := resolveConflicts(cs, func(i int) string { return answers[i] })
		if !ok || len(skip) != 1 || !skip[filepath.Join(src, "dir", "a.txt")] {
			t.Fatalf("skip %v", skip)
		}
		o.skip = skip
		if move {
			o.moveAll(pairs)
		} else {
			o.copyAll(pairs)
		}
		if len(o.errs) != 0 {
			t.Fatalf("errors: %v", o.errs)
		}
		for path, want := range map[string]string{
			"dst/top.txt": "new top", "dst/solo.txt": "solo", "dst/dir/a.txt": "old a",
			"dst/dir/deep/b.txt": "new b", "dst/dir/c.txt": "c only in source",
		} {
			if got := readFile(t, filepath.Join(d, path)); got != want {
				t.Errorf("move=%v %s = %q, want %q", move, path, got, want)
			}
		}
		left := map[string]string{"src/dir/a.txt": "new a"} // the skipped file stays in the source
		if !move {
			left = map[string]string{"src/top.txt": "new top", "src/dir/a.txt": "new a", "src/dir/deep/b.txt": "new b"}
		} else {
			for _, gone := range []string{"src/top.txt", "src/solo.txt", "src/dir/c.txt", "src/dir/deep"} {
				if _, err := os.Lstat(filepath.Join(d, gone)); err == nil {
					t.Errorf("move: %s still in the source", gone)
				}
			}
		}
		for path, want := range left {
			if got := readFile(t, filepath.Join(d, path)); got != want {
				t.Errorf("move=%v %s = %q, want %q", move, path, got, want)
			}
		}
		if o.bytes.Load() < 0 {
			t.Error("progress")
		}
	}
}

func TestResolveConflicts(t *testing.T) {
	cs := make([]fileConflict, 5)
	for i := range cs {
		cs[i].src = string(rune('a' + i))
	}
	run := func(answers ...string) (string, bool, int) {
		asked := 0
		skip, ok := resolveConflicts(cs, func(i int) string { asked++; return answers[asked-1] })
		var s []string
		for _, c := range cs {
			if skip[c.src] {
				s = append(s, c.src)
			}
		}
		return strings.Join(s, ""), ok, asked
	}
	if s, ok, n := run(conflictSkip, conflictSkipAll); !ok || s != "abcde" || n != 2 {
		t.Errorf("skip all: %q %v %d", s, ok, n)
	}
	if s, ok, n := run(conflictSkip, conflictOverwriteAll); !ok || s != "a" || n != 2 {
		t.Errorf("overwrite all: %q %v %d", s, ok, n)
	}
	if _, ok, n := run(conflictSkip, conflictCancel); ok || n != 2 {
		t.Error("cancel")
	}
	if _, ok, _ := run(""); ok {
		t.Error("Esc must cancel")
	}
	if conflictButtons[0] != conflictSkip {
		t.Error("the default button must not overwrite")
	}
}

func TestDeletePermanently(t *testing.T) {
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "x", "ro", "f.txt"), "x", time.Now())
	_ = os.Chmod(filepath.Join(d, "x", "ro", "f.txt"), 0o444)
	_ = os.Chmod(filepath.Join(d, "x", "ro"), 0o555) // read-only folder: plain RemoveAll fails on Linux
	writeFile(t, filepath.Join(d, "y.txt"), "y", time.Now())
	o := &opCtx{}
	o.deleteAll([]string{filepath.Join(d, "x"), filepath.Join(d, "y.txt")})
	if len(o.errs) != 0 {
		t.Fatalf("errors %v", o.errs)
	}
	if des, _ := os.ReadDir(d); len(des) != 0 {
		t.Errorf("left: %v", des)
	}
	w := subfolderWarning([]string{"a"}, true)
	if !strings.Contains(w, "PERMANENTLY") {
		t.Errorf("warning %q", w)
	}
}
