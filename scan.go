package main

// Preparation of a copy or move (v1.17): ONE walk of the sources does what took three
// before (counting the bytes, finding the files that already exist, and the copy's
// own walk). Each source folder is read once; when the same folder exists at the
// destination it is read once too, and its names are compared in memory, instead of
// one stat per file on each side: on a network every stat is a round trip, so this
// is where a copy into an existing tree spent its time. The copy engine then uses the
// tree built here (folders not read here, e.g. after a failed rename, are read by the
// engine itself).

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// scanNode is an item of the copy: its source information and, for a folder read by
// the scan, its children. dstInfo is what exists at the destination (nil = nothing).
type scanNode struct {
	src, dst string
	fi       fs.FileInfo // Lstat-like information of the source; nil = not known
	dstInfo  fs.FileInfo
	children []*scanNode
	scanned  bool // children read
}

// copyPlan is the result of the scan: one root per pair (same order), the files
// that already exist, the total of the regular files.
type copyPlan struct {
	roots     []*scanNode
	conflicts []fileConflict
	total     int64
}

// nameKey compares names the way the file system does (case-insensitive on Windows).
func nameKey(name string) string {
	if onWindows {
		return strings.ToLower(name)
	}
	return name
}

// scan prepares pairs: sizes (pairs[i].size), total, conflicts in source order. For a
// move, an item whose destination does not exist and that is on the same volume will
// simply be renamed: it is not read at all.
func (o *opCtx) scan(pairs []copyPair, move bool) *copyPlan {
	plan := &copyPlan{roots: make([]*scanNode, len(pairs))}
	for i := range pairs {
		if o.cancelled() {
			break
		}
		n := &scanNode{src: pairs[i].from, dst: pairs[i].to}
		plan.roots[i] = n
		fi, err := os.Lstat(n.src)
		if err != nil {
			continue // the engine reports it
		}
		n.fi = fi
		if di, err := os.Lstat(n.dst); err == nil {
			n.dstInfo = di
		}
		if move && n.dstInfo == nil && sameVolume(n.src, filepath.Dir(n.dst)) {
			continue // an instant rename: nothing to read
		}
		before := plan.total
		o.scanNode(n, plan)
		pairs[i].size = plan.total - before
	}
	return plan
}

func (o *opCtx) scanNode(n *scanNode, plan *copyPlan) {
	o.waitIfPaused()
	if o.cancelled() || n.fi == nil {
		return
	}
	switch {
	case n.fi.IsDir():
		des, err := os.ReadDir(n.src)
		if err != nil {
			return // not scanned: the engine reads it again and reports the error
		}
		var existing map[string]fs.DirEntry
		if n.dstInfo != nil && n.dstInfo.IsDir() {
			if dd, err := os.ReadDir(n.dst); err == nil {
				existing = make(map[string]fs.DirEntry, len(dd))
				for _, d := range dd {
					existing[nameKey(d.Name())] = d
				}
			}
		}
		n.children = make([]*scanNode, 0, len(des))
		for _, de := range des {
			c := &scanNode{src: filepath.Join(n.src, de.Name()), dst: filepath.Join(n.dst, de.Name())}
			if info, err := de.Info(); err == nil {
				c.fi = info
			}
			if d, ok := existing[nameKey(de.Name())]; ok {
				if info, err := d.Info(); err == nil {
					c.dstInfo = info
				}
			}
			o.scanNode(c, plan)
			n.children = append(n.children, c)
		}
		n.scanned = true
	default:
		if n.fi.Mode().IsRegular() {
			plan.total += n.fi.Size()
		}
		if n.dstInfo != nil && !n.dstInfo.IsDir() {
			plan.conflicts = append(plan.conflicts, fileConflict{n.src, n.dst, n.fi, n.dstInfo})
		}
	}
}

// findConflicts lists, in order, the files that would overwrite an existing file:
// top-level files, and files inside folders that will be merged. Items of different
// types (file against folder) are left to the copy, which reports them.
func (o *opCtx) findConflicts(pairs []copyPair) []fileConflict {
	return o.scan(append([]copyPair(nil), pairs...), false).conflicts
}
