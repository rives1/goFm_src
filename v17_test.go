package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestFlatScan(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "a.txt"), "a")
	write(t, filepath.Join(d, "sub", "b.pdf"), "bb")
	write(t, filepath.Join(d, "sub", "deep", "c.go"), "ccc")
	write(t, filepath.Join(d, ".hidden", "x.txt"), "x")
	write(t, filepath.Join(d, "sub", ".h.txt"), "h")
	os.MkdirAll(filepath.Join(d, "empty"), 0o755)
	// A link to a folder must not be listed (creating links may need privileges on Windows).
	_ = os.Symlink(filepath.Join(d, "sub"), filepath.Join(d, "linkdir"))
	names := func(es []entry) []string {
		var n []string
		for _, e := range es {
			n = append(n, filepath.ToSlash(e.name))
		}
		sort.Strings(n)
		return n
	}
	list, trunc := (&opCtx{}).flatScan(d, false)
	if got := strings.Join(names(list), ","); got != "a.txt,sub/b.pdf,sub/deep/c.go" || trunc {
		t.Fatalf("visible: %s (trunc %v)", got, trunc)
	}
	list, _ = (&opCtx{}).flatScan(d, true)
	if got := strings.Join(names(list), ","); got != ".hidden/x.txt,a.txt,sub/.h.txt,sub/b.pdf,sub/deep/c.go" {
		t.Fatalf("with hidden: %s", got)
	}
	for _, e := range list {
		if e.name == filepath.Join("sub", "deep", "c.go") && e.size != 3 {
			t.Fatal("size")
		}
	}
	old := maxFlatEntries
	maxFlatEntries = 2
	defer func() { maxFlatEntries = old }()
	if list, trunc = (&opCtx{}).flatScan(d, false); len(list) != 2 || !trunc {
		t.Fatalf("limit: %d %v", len(list), trunc)
	}
}

func TestDuplicateTargets(t *testing.T) {
	dups := duplicateTargets([]string{"/d/a.txt", "/d/b.txt", "/d/a.txt", "/d/c", "/d/a.txt"})
	if strings.Join(dups, ",") != "a.txt" {
		t.Fatal(dups)
	}
	if len(duplicateTargets([]string{"/d/x", "/d/y"})) != 0 {
		t.Fatal("false duplicate")
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "state.json")
	t.Setenv("GOFM_STATE", path)
	fm := &FM{left: &Panel{dir: "/left/dir"}, right: &Panel{dir: "/right/dir"}, lastDir: map[byte]string{'D': `D:\work`}}
	fm.active = fm.right
	fm.saveState()
	st, ok := loadState()
	if !ok || st.Left != "/left/dir" || st.Right != "/right/dir" || st.Active != "right" || st.DriveDirs["D"] != `D:\work` {
		t.Fatalf("%+v %v", st, ok)
	}
	info1, _ := os.Stat(path)
	time.Sleep(20 * time.Millisecond)
	fm.saveState() // unchanged: must not rewrite
	if info2, _ := os.Stat(path); !info2.ModTime().Equal(info1.ModTime()) {
		t.Fatal("unchanged state rewritten")
	}
	fm.restoring = true
	fm.left.dir = "/elsewhere"
	fm.saveState()
	if st, _ := loadState(); st.Left != "/left/dir" {
		t.Fatal("saved while restoring")
	}
}

func TestRestoreDir(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "a", "b"), 0o755)
	if got := restoreDir(filepath.Join(d, "a", "b"), "/fb", time.Second); got != filepath.Join(d, "a", "b") {
		t.Fatal(got)
	}
	if got := restoreDir(filepath.Join(d, "a", "gone", "deeper"), "/fb", time.Second); got != filepath.Join(d, "a") {
		t.Fatalf("nearest parent: %s", got)
	}
	if got := restoreDir("", "/fb", time.Second); got != "/fb" && !onWindows {
		t.Fatalf("drive list outside Windows: %s", got)
	}
}

func TestDiskSpace(t *testing.T) {
	free, total, err := diskSpace(t.TempDir())
	if err != nil || total == 0 || free > total {
		t.Fatal(free, total, err)
	}
	if got := bottomLabel(80, 12, 3565158, true, 30601641984, 270582939648); got != " 12 files 3.4 MB │ 28.5G free (11%) " {
		t.Fatal(got)
	}
	if got := bottomLabel(80, 1, 10, true, 0, 0); got != " 1 file 10 B " { // space unknown
		t.Fatal(got)
	}
	if got := bottomLabel(80, 0, 0, false, 5<<30, 10<<30); got != " 5.0G free (50%) " { // drive list
		t.Fatal(got)
	}
	if got := bottomLabel(20, 12, 3565158, true, 30601641984, 270582939648); got != " 12 files 3.4 MB " { // narrow
		t.Fatal(got)
	}
	if got := bottomLabel(8, 12, 3565158, true, 1, 2); got != "" {
		t.Fatal(got)
	}
	if compactSize(512) != "512" || compactSize(900<<10) != "900.0K" {
		t.Fatal(compactSize(512), compactSize(900<<10))
	}
}

func TestDriveKey(t *testing.T) {
	if l, ok := driveKey('D'); !ok || l != 'D' {
		t.Fatal("D")
	}
	for _, r := range []rune{'d', 'à', 'À', '1', '*', ' '} {
		if _, ok := driveKey(r); ok {
			t.Fatalf("%q taken as a drive", r)
		}
	}
}
