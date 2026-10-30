package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
)

func TestCompareSizeAndDate(t *testing.T) {
	t0 := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	L := []entry{
		{name: "same.txt", size: 10, mod: t0},
		{name: "fat.txt", size: 10, mod: t0.Add(2 * time.Second)}, // FAT rounding: same
		{name: "newleft.txt", size: 10, mod: t0.Add(3 * time.Second)},
		{name: "newright.txt", size: 99, mod: t0},
		{name: "clash.txt", size: 10, mod: t0},
		{name: "dst.txt", size: 10, mod: t0.Add(time.Hour)}, // 1 h is a real difference
		{name: "nodate.txt", size: 10},
		{name: "nodate2.txt", size: 10},
		{name: "only.txt", size: 1, mod: t0},
		{name: "dir", isDir: true, mod: t0.Add(time.Hour)},
	}
	R := []entry{
		{name: "SAME.TXT", size: 10, mod: t0},
		{name: "fat.txt", size: 10, mod: t0},
		{name: "newleft.txt", size: 10, mod: t0},
		{name: "newright.txt", size: 10, mod: t0.Add(time.Minute)},
		{name: "clash.txt", size: 11, mod: t0.Add(time.Second)},
		{name: "dst.txt", size: 10, mod: t0},
		{name: "nodate.txt", size: 10, mod: t0},
		{name: "nodate2.txt", size: 12, mod: t0},
		{name: "dir", isDir: true, mod: t0},
		{name: "onlyright.txt", size: 1, mod: t0},
	}
	r := compareSides(L, R, true)
	if got := strings.Join(r.markL, ","); got != "newleft.txt,clash.txt,dst.txt,nodate2.txt,only.txt" {
		t.Errorf("left marks: %s", got)
	}
	if got := strings.Join(r.markR, ","); got != "newright.txt,clash.txt,nodate2.txt,onlyright.txt" {
		t.Errorf("right marks: %s", got)
	}
	if r.onlyL != 1 || r.onlyR != 1 || r.newerL != 2 || r.newerR != 1 || r.sizeOnly != 2 {
		t.Errorf("counts: %+v", r)
	}
	if s := r.summary(); s != "Compare: left 1 only here, 2 newer · right 1 only here, 1 newer · 2 same date, different size" {
		t.Errorf("summary: %s", s)
	}
	if s := compareSides(L[:1], R[:1], true).summary(); !strings.Contains(s, "same files") {
		t.Errorf("summary when equal: %s", s)
	}
	// case-sensitive names: SAME.TXT and same.txt are different files
	if r := compareSides(L[:1], R[:1], false); r.onlyL != 1 || r.onlyR != 1 {
		t.Errorf("case-sensitive: %+v", r)
	}
}

func TestFoldersWithSubfolders(t *testing.T) {
	d := t.TempDir()
	mk := func(p string) string {
		p = filepath.Join(d, p)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	flat := mk("flat")
	_ = os.WriteFile(filepath.Join(flat, "a.txt"), []byte("x"), 0o644)
	deep := mk("deep/sub/subsub")
	deep = filepath.Dir(filepath.Dir(deep))
	empty := mk("empty")
	file := filepath.Join(d, "file.txt")
	_ = os.WriteFile(file, []byte("x"), 0o644)
	// many files before the subfolder: the folder is read in batches
	big := mk("big")
	for i := 0; i < 600; i++ {
		_ = os.WriteFile(filepath.Join(big, "f"+strings.Repeat("0", 3)+string(rune('a'+i%26))+time.Duration(i).String()), nil, 0o644)
	}
	mk("big/zz-sub")
	got := foldersWithSubfolders([]string{flat, deep, empty, file, big, filepath.Join(d, "missing")})
	if len(got) != 2 || got[0] != deep || got[1] != big {
		t.Fatalf("got %v", got)
	}
	if err := os.Symlink(deep, filepath.Join(d, "link")); err == nil {
		if got := foldersWithSubfolders([]string{filepath.Join(d, "link")}); len(got) != 0 {
			t.Error("a link to a folder is not a folder to warn about")
		}
	}
	w := subfolderWarning([]string{deep}, false)
	if !strings.HasPrefix(w, "WARNING: this folder contains subfolders:") || !strings.Contains(w, "\ndeep\n") {
		t.Errorf("single: %q", w)
	}
	var many []string
	for i := 0; i < 11; i++ {
		many = append(many, filepath.Join(d, "f"+string(rune('a'+i))))
	}
	w = subfolderWarning(many, false)
	if !strings.HasPrefix(w, "WARNING: 11 folders contain subfolders:") || !strings.Contains(w, "… and 3 more") || strings.Contains(w, "\nfi\n") {
		t.Errorf("many: %q", w)
	}
}

// v1.12: the summary lists every shortcut, those of the key bar included.
func TestHelpListsAllKeys(t *testing.T) {
	text := helpText()
	for _, it := range keybarItems {
		k := it.key
		if k == "Shift+X" {
			k = "Shift+letter"
		}
		if !strings.Contains(text, "]"+k+"[") && !strings.Contains(text, "]"+k+" /") {
			t.Errorf("%s missing from the summary", k)
		}
	}
	for _, want := range []string{"F2", "F3", "F5", "F7", "F10 / Alt+Q", "Ctrl+→", "Ctrl+←", "Alt+L", "Alt+E", "Alt+R", "Ctrl+F2", "Ctrl+F9", "Alt+D"} {
		if !strings.Contains(text, "]"+want+"[") {
			t.Errorf("%s missing from the summary", want)
		}
	}
	for _, gone := range []string{"Alt+F1", "Alt+S", "Alt+F[", "]*[", "Alt+A", "F8"} {
		if strings.Contains(text, gone) {
			t.Errorf("%q is no longer a shortcut", gone)
		}
	}
}

func newTestFM(t *testing.T) *FM {
	t.Helper()
	t.Setenv("GOFM_STATE", filepath.Join(t.TempDir(), "state.json"))
	fm := newFM(t.TempDir(), t.TempDir(), savedState{})
	t.Cleanup(fm.watch.close)
	return fm
}

// Closing a dialog keeps the panel that was active (bug of 1.9: it went to the left one).
func TestDialogKeepsActivePanel(t *testing.T) {
	fm := newTestFM(t)
	for _, p := range []*Panel{fm.right, fm.left, fm.right} {
		fm.setActive(p, true)
		fm.showMessage("Test", "text")
		fm.showKeys()
		if len(fm.modals) != 2 {
			t.Fatalf("%d dialogs", len(fm.modals))
		}
		fm.closeModal(fm.modals[1])
		if fm.app.GetFocus() == p.table {
			t.Fatal("focus on the panel with a dialog still open")
		}
		fm.closeModal(fm.modals[0])
		if fm.active != p || fm.app.GetFocus() != p.table {
			t.Fatalf("active panel changed: left=%v", fm.active == fm.left)
		}
		if p.table.GetBorderColor() != colActiveBorder || fm.other().table.GetBorderColor() != colPanelBorder {
			t.Fatal("border colours")
		}
	}
}

func TestHelpTextFits(t *testing.T) {
	for _, l := range strings.Split(helpText(), "\n") {
		if w := tview.TaggedStringWidth(l); w > 76 {
			t.Errorf("line of %d cells: %q", w, l)
		}
	}
}
