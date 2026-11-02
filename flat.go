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
		p.exitFlat()
		_ = p.load("") // v1.18: cursor on the first item (it was the item containing the file)
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

// dupQuestion is one question about files of the selection that would land on the
// same name in the destination (flat view, v1.18): chosen is the file that goes there
// so far, dup the next one with that name; this is question i of n.
type dupQuestion struct {
	chosen, dup copyPair
	i, n        int
}

// resolveDuplicates keeps one file per destination name. The first file with a name
// goes there; for each later one ask answers like a conflict: Skip keeps the chosen
// file, Overwrite takes this one instead, Skip all and Overwrite all decide the rest,
// Cancel or Esc ("") cancel the whole operation (ok = false). The files left out are
// neither copied nor moved. The order of the items is kept.
func resolveDuplicates(pairs []copyPair, ask func(q dupQuestion) string) (kept []copyPair, ok bool) {
	count := map[string]int{}
	for _, pr := range pairs {
		count[normPath(pr.to)]++
	}
	n := 0
	for _, c := range count {
		n += c - 1
	}
	pos := map[string]int{}
	all, i := "", 0
	for _, pr := range pairs {
		k := normPath(pr.to)
		j, seen := pos[k]
		if !seen {
			pos[k] = len(kept)
			kept = append(kept, pr)
			continue
		}
		answer := all
		if answer == "" {
			answer = ask(dupQuestion{kept[j], pr, i, n})
		}
		i++
		switch answer {
		case conflictSkip:
		case conflictSkipAll:
			all = conflictSkip
		case conflictOverwrite:
			kept[j] = pr
		case conflictOverwriteAll:
			all = conflictOverwrite
			kept[j] = pr
		default:
			return nil, false
		}
	}
	return kept, true
}

// commonDir is the deepest folder that contains both paths.
func commonDir(a, b string) string {
	da, db := filepath.Dir(a), filepath.Dir(b)
	for !isWithin(da, db) && !samePath(da, db) {
		up := filepath.Dir(da)
		if up == da {
			return ""
		}
		da = up
	}
	return da
}

// dupText is the question about two selected files with the same name: the paths
// are shown from their common folder, with size and date of both.
func dupText(q dupQuestion) string {
	base := commonDir(q.chosen.from, q.dup.from)
	rel := func(p string) string {
		if r, err := filepath.Rel(base, p); err == nil && base != "" {
			return r
		}
		return p
	}
	info := func(p string, other string) string {
		fi, err := os.Lstat(p)
		if err != nil {
			return rel(p)
		}
		s := rel(p) + " · " + humanSize(fi.Size()) + " · " + fi.ModTime().Local().Format("02/01/2006 15:04")
		if oi, err := os.Lstat(other); err == nil && fi.ModTime().Sub(oi.ModTime()) > dateTolerance {
			s += " (newer)"
		}
		return s
	}
	return fmt.Sprintf("Selected files with the same name (%d of %d):\n\n%s\n\nChosen:   %s\nThis one: %s\n\nSkip keeps the chosen file; Overwrite takes this one instead.",
		q.i+1, q.n, filepath.Base(q.dup.to), info(q.chosen.from, q.dup.from), info(q.dup.from, q.chosen.from))
}
