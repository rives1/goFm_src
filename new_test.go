package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

func TestUNCHelpers(t *testing.T) {
	cases := []struct {
		in, server, share, rest string
		ok                      bool
	}{
		{`\\nas\docs\a\b`, "nas", "docs", `a\b`, true},
		{`//nas/docs`, "nas", "docs", "", true},
		{`\\nas`, "nas", "", "", true},
		{`\\nas\`, "nas", "", "", true},
		{`C:\x`, "", "", "", false},
		{`\\?\C:\x`, "", "", "", false},
	}
	for _, c := range cases {
		s, sh, r, ok := uncParts(c.in)
		if s != c.server || sh != c.share || r != c.rest || ok != c.ok {
			t.Errorf("uncParts(%q) = %q %q %q %v", c.in, s, sh, r, ok)
		}
	}
	if normalizeUNC(`//nas/docs`) != `\\nas\docs\` || normalizeUNC(`\\nas\`) != `\\nas` {
		t.Error("normalizeUNC")
	}
	if !isUNCServer(`\\nas`) || isUNCServer(`\\nas\x`) || !isUNCShareRoot(`\\nas\x\`) {
		t.Error("isUNC*")
	}
}

func TestWindowsParent(t *testing.T) {
	cases := []struct {
		dir, parent, sel string
		ok               bool
	}{
		{`C:\Users\me`, `C:\Users`, "me", true},
		{`C:\Users`, `C:\`, "Users", true},
		{`C:\`, "", `C:\`, true},
		{`\\nas\docs\a\b`, `\\nas\docs\a`, "b", true},
		{`\\nas\docs\a`, `\\nas\docs\`, "a", true},
		{`\\nas\docs\`, `\\nas`, "docs", true},
		{`\\nas`, "", "", true},
		{"", "", "", false},
	}
	for _, c := range cases {
		p, s, ok := windowsParent(c.dir)
		if p != c.parent || s != c.sel || ok != c.ok {
			t.Errorf("windowsParent(%q) = %q %q %v", c.dir, p, s, ok)
		}
	}
	if driveLetter(`d:\x`) != 'D' || driveLetter(`\\nas\x`) != 0 {
		t.Error("driveLetter")
	}
}

func TestTruncateName(t *testing.T) {
	s := truncateName("  a very long report name for the board.pdf", 20)
	if uniseg.StringWidth(s) != 20 || !strings.HasSuffix(s, "….pdf") {
		t.Fatalf("%q", s)
	}
	if truncateName("short.txt", 20) != "short.txt" {
		t.Fatal("short changed")
	}
	if s := truncateName("文件名很长很长很长很长.txt", 12); uniseg.StringWidth(s) > 12 {
		t.Fatalf("wide chars: %q", s)
	}
	if s := truncateName("archive.backup-2026-10-22", 12); uniseg.StringWidth(s) > 12 || !strings.HasSuffix(s, "…") {
		t.Fatalf("long ext: %q", s)
	}
}

func TestShortDate(t *testing.T) {
	now := time.Date(2026, 10, 22, 16, 0, 0, 0, time.Local)
	if got := shortDate(time.Date(2026, 3, 5, 9, 7, 0, 0, time.Local), now); got != "05/03 09:07" {
		t.Error(got)
	}
	if got := shortDate(time.Date(2024, 3, 5, 9, 7, 0, 0, time.Local), now); got != "05/03/2024" {
		t.Error(got)
	}
}

func TestCtrlAltLetter(t *testing.T) {
	// classic console / VT: Ctrl+D arrives as KeyCtrlD
	if l, ok := ctrlLetter(tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModCtrl)); !ok || l != 'D' {
		t.Error("KeyCtrlD")
	}
	// Windows Terminal win32-input-mode: the letter key with ModCtrl (Ctrl+H, I, M distinct)
	for _, k := range []tcell.Key{'H', 'I', 'M'} {
		if l, ok := ctrlLetter(tcell.NewEventKey(k, 0, tcell.ModCtrl)); !ok || l != byte(k) {
			t.Errorf("win32 Ctrl+%c", k)
		}
	}
	// plain Backspace / Tab / Enter must not be read as Ctrl+H/I/M
	for _, k := range []tcell.Key{tcell.KeyBackspace, tcell.KeyTab, tcell.KeyEnter} {
		if _, ok := ctrlLetter(tcell.NewEventKey(k, 0, tcell.ModNone)); ok {
			t.Errorf("key %v taken as Ctrl letter", k)
		}
	}
	// AltGr (Ctrl+Alt) is not a drive shortcut
	if _, ok := ctrlLetter(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModCtrl|tcell.ModAlt)); ok {
		t.Error("AltGr")
	}
	if r, ok := altLetter(tcell.NewEventKey(tcell.KeyRune, 'S', tcell.ModAlt)); !ok || r != 's' {
		t.Error("Alt+S")
	}
	if _, ok := altLetter(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone)); ok {
		t.Error("plain s taken as Alt")
	}
}

func TestRateAndProgress(t *testing.T) {
	o := &opCtx{}
	o.total.Store(100 << 20)
	m := &rateMeter{}
	t0 := time.Now()
	o.bytes.Store(0)
	progressText("Copy", o, m, 66, t0)
	o.bytes.Store(50 << 20)
	txt := progressText("Copy", o, m, 66, t0.Add(5*time.Second)) // 10 MB/s, 50 MB left -> 0:05
	if !strings.Contains(txt, " 50%") || !strings.Contains(txt, "10.0 MB/s") || !strings.Contains(txt, "0:05 left") {
		t.Fatal(txt)
	}
	if formatDuration(3725*time.Second) != "1:02:05" {
		t.Fatal(formatDuration(3725 * time.Second))
	}
}

func TestCopyCountsBytes(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "s", "a.bin"), strings.Repeat("x", 3000))
	write(t, filepath.Join(d, "s", "b.bin"), strings.Repeat("y", 2000))
	o := &opCtx{}
	if o.treeSize(filepath.Join(d, "s")) != 5000 {
		t.Fatal("treeSize")
	}
	o.copyPath(filepath.Join(d, "s"), filepath.Join(d, "t"))
	if o.bytes.Load() != 5000 || o.files.Load() != 2 {
		t.Fatalf("bytes=%d files=%d", o.bytes.Load(), o.files.Load())
	}
}

func TestArchiveBrowseAndSelectiveExtract(t *testing.T) {
	for _, arch := range []string{"testdata/multi.7z", ""} {
		d := t.TempDir()
		if arch == "" { // same structure as a zip
			write(t, filepath.Join(d, "src", "progetto", "città.txt"), "accentato")
			write(t, filepath.Join(d, "src", "progetto", "sub", "big.txt"), "big")
			write(t, filepath.Join(d, "src", "solo.txt"), "S")
			arch = filepath.Join(d, "multi.zip")
			o := &opCtx{}
			o.zipPaths([]string{filepath.Join(d, "src", "progetto"), filepath.Join(d, "src", "solo.txt")}, filepath.Join(d, "src"), arch)
		}
		all, closer, err := openArchive(arch)
		if err != nil {
			t.Fatal(err)
		}
		root := archChildren(all, "")
		names := []string{}
		for _, e := range root {
			names = append(names, e.name)
		}
		if len(root) != 2 {
			t.Fatalf("%s root: %v", arch, names)
		}
		sub := archChildren(all, "progetto")
		if len(sub) != 2 {
			t.Fatalf("%s progetto/: %v", arch, sub)
		}
		closer.Close()

		out := filepath.Join(d, "out")
		os.MkdirAll(out, 0o755)
		o := &opCtx{}
		o.extractSelected(arch, []string{"progetto/sub", "solo.txt"}, out, false)
		if len(o.errs) > 0 {
			t.Fatal(o.errs)
		}
		if !exists(filepath.Join(out, "sub", "big.txt")) || read(t, filepath.Join(out, "solo.txt")) != "S" ||
			exists(filepath.Join(out, "progetto")) || exists(filepath.Join(out, "città.txt")) {
			t.Fatalf("%s: wrong selective extraction", arch)
		}
		if o.total.Load() == 0 || o.bytes.Load() != o.total.Load() {
			t.Fatalf("%s: progress %d/%d", arch, o.bytes.Load(), o.total.Load())
		}
	}
}

func TestPreviewArchEntry(t *testing.T) {
	all, closer, err := openArchive("testdata/single.7z")
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()
	e, ok := findArchEntry(all, "progetto/città.txt")
	if !ok || !strings.Contains(previewArchEntry("x.7z", e), "accentato") {
		t.Fatal("preview of 7z entry")
	}
}

func TestTreeStats(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "a", "b", "x"), strings.Repeat("x", 700))
	write(t, filepath.Join(d, "a", "y"), strings.Repeat("y", 300))
	files, size := (&opCtx{}).treeStats(filepath.Join(d, "a"))
	if files != 2 || size != 1000 {
		t.Fatalf("files=%d size=%d", files, size)
	}
	if plural(1, "file") != "1 file" || plural(3, "file") != "3 files" {
		t.Fatal("plural")
	}
}

func TestOnlyOnOneSide(t *testing.T) {
	L := []entry{{name: "..", isDir: true, isUp: true}, {name: "sub", isDir: true}, {name: "a.txt"}, {name: "Report.PDF"}, {name: "left.bin"}}
	R := []entry{{name: "..", isDir: true, isUp: true}, {name: "other", isDir: true}, {name: "a.txt"}, {name: "report.pdf"}, {name: "right.doc"}}
	l, r := onlyOnOneSide(L, R, true)
	if strings.Join(l, ",") != "left.bin" || strings.Join(r, ",") != "right.doc" {
		t.Fatalf("case-insensitive: %v %v", l, r)
	}
	l, r = onlyOnOneSide(L, R, false)
	if strings.Join(l, ",") != "Report.PDF,left.bin" || strings.Join(r, ",") != "report.pdf,right.doc" {
		t.Fatalf("case-sensitive: %v %v", l, r)
	}
	if l, r := onlyOnOneSide(L, L, true); len(l)+len(r) != 0 {
		t.Fatal("same folder")
	}
}
