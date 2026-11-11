//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func shortName(t *testing.T, p string) string {
	in, err := windows.UTF16PtrFromString(p)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 1024)
	n, err := windows.GetShortPathName(in, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) >= len(buf) {
		t.Skipf("no short name for %q: %v", p, err)
	}
	return windows.UTF16ToString(buf[:n])
}

// v1.21: a folder written with 8.3 short names (C:\PROGRA~1) opens the tree down to it,
// under the long names.
func TestTreeShortNames(t *testing.T) {
	root := visibleTempDir(t)
	long := filepath.Join(root, "Long folder name", "inner folder")
	if err := os.MkdirAll(long, 0o755); err != nil {
		t.Fatal(err)
	}
	short := shortName(t, long)
	if short == long {
		t.Skip("8.3 short names are off on this volume")
	}
	if got := longName(short); got != long {
		t.Fatalf("longName(%q) = %q, want %q", short, got, long)
	}
	if got := longName(`C:\no~such\x`); got != `C:\no~such\x` {
		t.Errorf("missing path changed: %q", got)
	}
	fm := newTestFM(t)
	fm.left.chdir(short, "")
	fm.setActive(fm.left, true)
	fm.keys(ctrlKey('T'))
	tv := fm.tree
	if tv == nil {
		t.Fatal("no tree")
	}
	syncTree(tv)
	if tv.selectedPath() != long || fm.right.dir != long {
		t.Fatalf("tree on %q, right panel on %q, want %q", tv.selectedPath(), fm.right.dir, long)
	}
}
