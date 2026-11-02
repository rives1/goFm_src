package main

// Ctrl+R (v1.18): capitalize the first letter of every word of the names of the marked
// items (files and folders), or of the item under the cursor. A word starts at the
// beginning of the name and after a space, "_" or "-"; the other letters stay as they
// are ("rapporto_IVA-q3.pdf" → "Rapporto_IVA-Q3.pdf"). The extension of a file is left
// alone ("foto.JPG" stays ".JPG"). A dialog shows the new names first; an item whose
// new name already exists is skipped, as with Ctrl+K. A change of case only is a
// normal rename, also on Windows, where the old and the new name are the same file.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// isWordSep reports whether r separates words for Ctrl+R.
func isWordSep(r rune) bool { return r == ' ' || r == '_' || r == '-' }

// titleWords upper-cases the first letter of each word of s.
func titleWords(s string) string {
	var b strings.Builder
	start := true
	for _, r := range s {
		if start {
			r = unicode.ToUpper(r)
		}
		b.WriteRune(r)
		start = isWordSep(r)
	}
	return b.String()
}

// capitalizedName is the new name of an item for Ctrl+R: the words of the name are
// capitalized, the extension of a file is kept as it is. A name that starts with a
// dot and has no other one (".profile") has no extension.
func capitalizedName(name string, isDir bool) string {
	if isDir {
		return titleWords(name)
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem == "" {
		stem, ext = name, ""
	}
	return titleWords(stem) + ext
}

// renameEach renames each item (a name relative to dir, which may contain a subfolder
// in the flat view) to newBase(its base name) in the same folder. Items whose name
// does not change are left out; existing names are skipped, except the item itself
// (a change of case on a file system that ignores case); other errors go to o.
func (o *opCtx) renameEach(dir string, names []string, newBase func(rel string) string) prefixResult {
	r := prefixResult{renamed: map[string]string{}}
	for _, rel := range names {
		if o.cancelled() {
			break
		}
		nb := newBase(rel)
		if nb == filepath.Base(rel) {
			continue
		}
		newRel := filepath.Join(filepath.Dir(rel), nb)
		from, to := filepath.Join(dir, rel), filepath.Join(dir, newRel)
		if tfi, err := os.Lstat(to); err == nil {
			if ffi, ferr := os.Lstat(from); ferr != nil || !os.SameFile(ffi, tfi) {
				r.skipped = append(r.skipped, rel)
				continue
			}
		}
		if err := os.Rename(from, to); err != nil {
			o.fail(from, err)
			continue
		}
		r.renamed[rel] = newRel
		o.step(from)
	}
	return r
}

// capitalizePreview is the text of the Ctrl+R dialog for the names that change.
func capitalizePreview(changes [][2]string, unchanged int) string {
	var b strings.Builder
	if len(changes) == 1 {
		b.WriteString("Capitalize the words of this name?\n\n")
	} else {
		fmt.Fprintf(&b, "Capitalize the words of %d names?\n\n", len(changes))
	}
	for i, c := range changes {
		if i == maxPreview {
			fmt.Fprintf(&b, "… and %d more\n", len(changes)-maxPreview)
			break
		}
		b.WriteString(c[0] + " → " + c[1] + "\n")
	}
	if unchanged > 0 {
		fmt.Fprintf(&b, "\n%s already capitalized: left as they are.", plural(int64(unchanged), "name"))
	}
	b.WriteString("\nA name that already exists is skipped.")
	return b.String()
}

// capitalizeNames is Ctrl+R.
func (fm *FM) capitalizeNames() {
	if fm.readOnlyHere() {
		return
	}
	p := fm.active
	names := p.selectedNames()
	if len(names) == 0 {
		return
	}
	isDir := map[string]bool{}
	for _, e := range p.entries {
		isDir[e.name] = e.isDir
	}
	newBase := func(rel string) string { return capitalizedName(filepath.Base(rel), isDir[rel]) }
	var changes [][2]string
	for _, n := range names {
		if nb := newBase(n); nb != filepath.Base(n) {
			changes = append(changes, [2]string{filepath.Base(n), nb})
		}
	}
	if len(changes) == 0 {
		fm.notice = "Ctrl+R: the words of " + describe(names) + " already start with capitals"
		fm.updateStatus()
		return
	}
	cursor := ""
	if e := p.current(); e != nil {
		cursor = e.name
	}
	fm.confirm(capitalizePreview(changes, len(names)-len(changes)), []string{"Rename", "Cancel"}, func(c string) {
		if c != "Rename" {
			return
		}
		var res prefixResult
		fm.runTask("Capitalizing "+describe(names), func(o *opCtx) {
			res = o.renameEach(p.dir, names, newBase)
		}, func(*opCtx) {
			fm.afterRename(p, cursor, res, len(changes), "Capitalize")
		})
	})
}
