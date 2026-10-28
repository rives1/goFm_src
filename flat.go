package main

// Flat view (Ctrl+B): every file of the current folder and of all its subfolders
// in one list, with names relative to the folder ("sub\report.pdf"). The usual
// operations work on it because each entry's full path is folder + relative name.
// The view ends with Ctrl+B again or when the panel moves to another location.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// maxFlatEntries caps the list so that a flat view of a whole drive stays usable.
var maxFlatEntries = 200000 // a variable so tests can lower it

// flatScan lists the files under root, recursively. Folders are walked but not listed;
// links are not followed; hidden files and folders are skipped unless showHidden.
func (o *opCtx) flatScan(root string, showHidden bool) (list []entry, truncated bool) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if o.cancelled() {
			return filepath.SkipAll
		}
		if err != nil || path == root {
			return nil // unreadable folders are skipped silently
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		attrs, winHidden := fileAttrs(info)
		hidden := winHidden || strings.HasPrefix(d.Name(), ".")
		if hidden && !showHidden {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		isLink := d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0
		if isLink { // a link or junction to a folder is not a file
			if st, err := os.Stat(path); err == nil && st.IsDir() {
				return nil
			}
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		list = append(list, entry{name: rel, size: info.Size(), mod: info.ModTime(),
			attrs: attrs, hidden: hidden, isLink: isLink})
		if n := o.files.Add(1); n%500 == 0 {
			o.cur.Store(filepath.Dir(path))
		}
		if len(list) >= maxFlatEntries {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	return
}

// toggleFlat is Ctrl+B: build the flat view of the active panel's folder, or leave it.
func (fm *FM) toggleFlat() {
	p := fm.active
	if p.flat {
		sel := ""
		if e := p.current(); e != nil && !e.isUp { // cursor on the top-level item that contains it
			sel = strings.SplitN(e.name, string(filepath.Separator), 2)[0]
		}
		p.exitFlat()
		_ = p.load(sel)
		return
	}
	if !p.onDisk() {
		fm.showMessage("Flat view", "The flat view works in folders on disk.")
		return
	}
	root, hidden := p.dir, p.showHidden
	var list []entry
	var trunc bool
	fm.runTask("Flat view of "+root, func(o *opCtx) {
		o.setPhase("")
		list, trunc = o.flatScan(root, hidden)
	}, func(o *opCtx) {
		if o.cancelled() || p.dir != root || p.arch != nil {
			return
		}
		p.flat, p.flatEntries, p.flatTrunc = true, list, trunc
		p.marked, p.filter = map[string]bool{}, ""
		_ = p.load("")
		fm.notice = fmt.Sprintf("Flat view: %s in %s and its subfolders — Ctrl+B to go back",
			plural(int64(len(list)), "file"), filepath.Base(root))
		if trunc {
			fm.notice = fmt.Sprintf("Flat view: only the first %d files are listed — Ctrl+B to go back", maxFlatEntries)
		}
	})
}

// exitFlat leaves the flat view (any running rescan is cancelled).
func (p *Panel) exitFlat() {
	if p.flatRescan != nil {
		p.flatRescan.cancel.Store(true)
		p.flatRescan = nil
	}
	if p.flat {
		p.flat, p.flatEntries, p.flatTrunc = false, nil, false
		p.marked = map[string]bool{}
	}
}

// rescanFlat re-reads the tree in the background after a change (copy, delete,
// Alt+R, a change notification on the root folder). The current list stays on screen
// until the new one is ready; the cursor then follows the usual rule (nextSurvivor).
func (p *Panel) rescanFlat() {
	if p.flatRescan != nil {
		p.flatRescan.cancel.Store(true)
	}
	o := &opCtx{}
	p.flatRescan = o
	root, hidden := p.dir, p.showHidden
	go func() {
		list, trunc := o.flatScan(root, hidden)
		if o.cancelled() {
			return
		}
		p.fm.app.QueueUpdateDraw(func() {
			if !p.flat || p.flatRescan != o || p.dir != root {
				return
			}
			p.flatRescan = nil
			old := p.entries
			row, _ := p.table.GetSelection()
			rowOff, colOff := p.table.GetOffset()
			name := ""
			if e := p.current(); e != nil {
				name = e.name
			}
			p.flatEntries, p.flatTrunc = list, trunc
			_ = p.load(name)
			p.table.SetOffset(rowOff, colOff)
			p.table.Select(nextSurvivor(old, row, p.entries), 0)
		})
	}()
}

// duplicateTargets returns the destination names used by more than one source:
// in the flat view two files from different subfolders can share a name and would
// overwrite each other when copied into one folder.
func duplicateTargets(targets []string) []string {
	seen := map[string]int{}
	var dups []string
	for _, t := range targets {
		k := normPath(t)
		seen[k]++
		if seen[k] == 2 {
			dups = append(dups, filepath.Base(t))
		}
	}
	return dups
}
