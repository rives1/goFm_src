package main

// Copy engine: walks the sources in order, creating folders as it goes, and hands
// the files to a small pool of workers. Small files are copied several at a time
// (on a network each file costs some round trips, which then overlap); big files
// go one at a time, because parallel big streams do not add speed on a single
// link or disk and make a spinning disk seek.
// Each file is copied by copyFileNative: CopyFileEx on Windows, a Go loop elsewhere.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	copyWorkers  = 4        // files copied at the same time
	bigFileLimit = 64 << 20 // files from this size on are copied one at a time
)

// copyOneFile copies a single file; tests replace it to observe the scheduling.
var copyOneFile = copyFileNative

type copyJob struct {
	root     int // index of the top-level item this file belongs to
	src, dst string
	fi       fs.FileInfo
}

type dirTime struct {
	path string
	mod  time.Time
}

type copier struct {
	o       *opCtx
	jobs    chan copyJob
	wg      sync.WaitGroup
	big     chan struct{} // one token: only one big file at a time
	mu      sync.Mutex
	failed  map[int]bool // top-level items with at least one error
	skipped map[int]bool // top-level items with files skipped on a conflict
	dirs    []dirTime    // folder dates, applied after their content is written
}

func newCopier(o *opCtx) *copier {
	c := &copier{o: o, jobs: make(chan copyJob, 64), big: make(chan struct{}, 1),
		failed: map[int]bool{}, skipped: map[int]bool{}}
	for i := 0; i < copyWorkers; i++ {
		c.wg.Add(1)
		go c.worker()
	}
	return c
}

func (c *copier) fail(root int, path string, err error) {
	c.mu.Lock()
	c.failed[root] = true
	c.mu.Unlock()
	c.o.fail(path, err)
}

func (c *copier) worker() {
	defer c.wg.Done()
	for j := range c.jobs {
		if c.o.cancelled() {
			c.fail(j.root, j.src, errCancelled)
			continue // drain the queue
		}
		if j.fi.Size() >= bigFileLimit {
			c.big <- struct{}{}
		}
		c.o.step(j.src)
		err := copyOneFile(c.o, j.src, j.dst, j.fi)
		if j.fi.Size() >= bigFileLimit {
			<-c.big
		}
		if err != nil {
			_ = os.Remove(j.dst) // no half-written files (CopyFileEx already removes them on cancel)
			c.fail(j.root, j.src, err)
		}
	}
}

// add walks src and queues its files for copying to dst. root identifies the
// top-level item, so a move knows which sources were copied completely.
func (c *copier) add(root int, src, dst string) {
	c.addNode(root, &scanNode{src: src, dst: dst})
}

// addNode is add for an item already read by the scan (v1.17): its information and
// children are used instead of reading the disk again; what the scan did not read is
// read here.
func (c *copier) addNode(root int, n *scanNode) {
	o := c.o
	src, dst := n.src, n.dst
	if o.cancelled() {
		c.fail(root, src, errCancelled)
		return
	}
	fi := n.fi
	if fi == nil {
		var err error
		if fi, err = os.Lstat(src); err != nil {
			c.fail(root, src, err)
			return
		}
	}
	if o.skip[src] && !fi.IsDir() { // a conflict answered "Skip": the existing file stays
		o.bytes.Add(fi.Size()) // it counts toward the progress
		c.mu.Lock()
		c.skipped[root] = true
		c.mu.Unlock()
		return
	}
	mode := fi.Mode()
	switch {
	case mode&fs.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err == nil {
			_ = os.Remove(dst)
			err = os.Symlink(target, dst)
		}
		if err != nil {
			// No privilege to create links (common on Windows): copy the content if it is a file.
			if st, serr := os.Stat(src); serr == nil && st.Mode().IsRegular() {
				c.jobs <- copyJob{root, src, dst, st}
				return
			}
			c.fail(root, src, fmt.Errorf("link not copied: %w", err))
		}
	case mode&fs.ModeIrregular != 0:
		// Windows junctions / mount points: not followed, to avoid loops.
		if st, err := os.Stat(src); err == nil && st.Mode().IsRegular() {
			c.jobs <- copyJob{root, src, dst, st}
			return
		}
		c.fail(root, src, errors.New("folder link (junction) skipped"))
	case fi.IsDir():
		// A folder known to exist is not created again; a new one is created with one
		// call (MkdirAll would check every parent first: one round trip each).
		if !(n.dstInfo != nil && n.dstInfo.IsDir()) {
			if err := os.Mkdir(dst, fi.Mode().Perm()|0o700); err != nil && !os.IsExist(err) {
				if err := os.MkdirAll(dst, fi.Mode().Perm()|0o700); err != nil {
					c.fail(root, dst, err)
					return
				}
			}
		}
		children := n.children
		if !n.scanned {
			des, err := os.ReadDir(src)
			if err != nil {
				c.fail(root, src, err)
				return
			}
			for _, de := range des {
				children = append(children, &scanNode{src: filepath.Join(src, de.Name()), dst: filepath.Join(dst, de.Name())})
			}
		}
		for _, ch := range children {
			c.addNode(root, ch)
		}
		c.dirs = append(c.dirs, dirTime{dst, fi.ModTime()})
	case mode.IsRegular():
		c.jobs <- copyJob{root, src, dst, fi}
	default:
		c.fail(root, src, errors.New("unsupported file type"))
	}
}

// finish waits for the workers and returns the top-level items that had errors.
// (Called from one goroutine; c.skipped is complete once add has returned.)
func (c *copier) finish() map[int]bool {
	close(c.jobs)
	c.wg.Wait()
	for i := len(c.dirs) - 1; i >= 0; i-- { // innermost folders first
		_ = os.Chtimes(c.dirs[i].path, c.dirs[i].mod, c.dirs[i].mod)
	}
	return c.failed
}

// copyPair is one top-level item of a copy or move.
type copyPair struct {
	from, to string
	size     int64
}

// copyAll copies every pair with one shared worker pool.
func (o *opCtx) copyAll(pairs []copyPair) { o.copyPlan(pairs, nil) }

// copyPlan copies with the tree of a scan (nil: the engine reads everything).
func (o *opCtx) copyPlan(pairs []copyPair, plan *copyPlan) {
	c := newCopier(o)
	for i, p := range pairs {
		c.addNode(i, planRoot(plan, i, p))
	}
	c.finish()
}

// planRoot is the scanned root of pair i, or a node to be read by the engine.
func planRoot(plan *copyPlan, i int, p copyPair) *scanNode {
	if plan != nil && i < len(plan.roots) && plan.roots[i] != nil {
		return plan.roots[i]
	}
	return &scanNode{src: p.from, dst: p.to}
}

// moveAll renames what it can (same drive: instant) and copies the rest with one
// shared worker pool; a source is deleted only if all of it was copied.
func (o *opCtx) moveAll(pairs []copyPair) { o.movePlan(pairs, nil) }

// movePlan is moveAll with the tree of a scan.
func (o *opCtx) movePlan(pairs []copyPair, plan *copyPlan) {
	c := newCopier(o)
	var copied []int
	for i, p := range pairs {
		if o.cancelled() {
			break
		}
		si, err := os.Lstat(p.from)
		if err != nil {
			o.fail(p.from, err)
			continue
		}
		if o.skip[p.from] && !si.IsDir() { // a conflict answered "Skip": nothing to move
			o.bytes.Add(p.size)
			continue
		}
		di, derr := os.Lstat(p.to)
		merge := derr == nil && di.IsDir() && si.IsDir()
		if !merge {
			if err := os.Rename(p.from, p.to); err == nil {
				o.step(p.from)
				o.bytes.Add(p.size)
				continue
			}
		}
		// Different drive or folder merge: copy, delete the source afterwards.
		c.addNode(i, planRoot(plan, i, p))
		copied = append(copied, i)
	}
	failed := c.finish()
	for _, i := range copied {
		src := pairs[i].from
		if o.cancelled() || failed[i] {
			o.fail(src, errors.New("source NOT deleted because the copy is incomplete"))
			continue
		}
		if c.skipped[i] { // keep the skipped files where they are, delete the rest
			if err := removeExcept(src, o.skip); err != nil {
				o.fail(src, fmt.Errorf("copied, but could not delete the source: %w", err))
			}
			continue
		}
		if err := os.RemoveAll(src); err != nil {
			o.fail(src, fmt.Errorf("copied, but could not delete the source: %w", err))
		}
	}
}

// copyPath and movePath handle a single item (kept for callers and tests).
func (o *opCtx) copyPath(src, dst string) { o.copyAll([]copyPair{{from: src, to: dst}}) }
func (o *opCtx) movePath(src, dst string) { o.moveAll([]copyPair{{from: src, to: dst}}) }

// copyFileGo is the portable file copy: 1 MB buffer, byte counting and cancellation
// through cancelReader, permissions and modification time preserved.
func copyFileGo(o *opCtx, src, dst string, fi fs.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode().Perm()|0o200)
	if err != nil {
		return err
	}
	_, err = io.CopyBuffer(out, cancelReader{in, o}, make([]byte, 1<<20))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	_ = os.Chtimes(dst, fi.ModTime(), fi.ModTime())
	return nil
}

// removeExcept deletes the tree at root except the files in keep (and the folders
// that still contain them): what is left of a moved folder after some files were
// skipped. Children are removed before their parents; a folder that is not empty
// stays.
func removeExcept(root string, keep map[string]bool) error {
	var paths []string
	var dirs = map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, p)
		if d.IsDir() {
			dirs[p] = true
		}
		return nil
	})
	for i := len(paths) - 1; i >= 0; i-- {
		p := paths[i]
		switch {
		case keep[p]:
		case dirs[p]:
			_ = os.Remove(p) // fails, and stays, if a kept file is inside
		default:
			if rerr := os.Remove(p); rerr != nil && err == nil {
				err = rerr
			}
		}
	}
	return err
}

// fileConflict is a file of a copy or move that already exists at the destination.
type fileConflict struct {
	src, dst         string
	srcInfo, dstInfo fs.FileInfo
}

// Answers to a conflict, in the order of the buttons: the default (first) one never
// overwrites.
const (
	conflictSkip         = "Skip"
	conflictSkipAll      = "Skip all"
	conflictOverwrite    = "Overwrite"
	conflictOverwriteAll = "Overwrite all"
	conflictCancel       = "Cancel"
)

var conflictButtons = []string{conflictSkip, conflictSkipAll, conflictOverwrite, conflictOverwriteAll, conflictCancel}

// resolveConflicts asks about each conflict, until an "all" answer decides the rest.
// It returns the source files to skip; ok is false when the operation is cancelled
// (Cancel, or Esc: an empty answer).
func resolveConflicts(cs []fileConflict, ask func(i int) string) (skip map[string]bool, ok bool) {
	skip = map[string]bool{}
	all := ""
	for i, c := range cs {
		answer := all
		if answer == "" {
			answer = ask(i)
		}
		switch answer {
		case conflictSkip:
			skip[c.src] = true
		case conflictSkipAll:
			skip[c.src] = true
			all = conflictSkip
		case conflictOverwrite:
		case conflictOverwriteAll:
			all = conflictOverwrite
		default:
			return nil, false
		}
	}
	return skip, true
}

// conflictText describes conflict i of n; destRoot is the destination folder, so the
// name is shown relative to it.
func conflictText(c fileConflict, i, n int, destRoot string) string {
	name := c.dst
	if rel, err := filepath.Rel(destRoot, c.dst); err == nil {
		name = rel
	}
	side := func(fi fs.FileInfo, newer bool) string {
		s := humanSize(fi.Size()) + " · " + fi.ModTime().Local().Format("02/01/2006 15:04")
		if newer {
			s += " (newer)"
		}
		return s
	}
	ns, ds := c.srcInfo.ModTime(), c.dstInfo.ModTime()
	return fmt.Sprintf("A file with this name already exists (%d of %d):\n\n%s\n\nNew:      %s\nExisting: %s",
		i+1, n, name, side(c.srcInfo, ns.Sub(ds) > dateTolerance), side(c.dstInfo, ds.Sub(ns) > dateTolerance))
}
