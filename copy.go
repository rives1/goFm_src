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
	o      *opCtx
	jobs   chan copyJob
	wg     sync.WaitGroup
	big    chan struct{} // one token: only one big file at a time
	mu     sync.Mutex
	failed map[int]bool // top-level items with at least one error
	dirs   []dirTime    // folder dates, applied after their content is written
}

func newCopier(o *opCtx) *copier {
	c := &copier{o: o, jobs: make(chan copyJob, 64), big: make(chan struct{}, 1), failed: map[int]bool{}}
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
	o := c.o
	if o.cancelled() {
		c.fail(root, src, errCancelled)
		return
	}
	fi, err := os.Lstat(src)
	if err != nil {
		c.fail(root, src, err)
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
		if err := os.MkdirAll(dst, fi.Mode().Perm()|0o700); err != nil {
			c.fail(root, dst, err)
			return
		}
		des, err := os.ReadDir(src)
		if err != nil {
			c.fail(root, src, err)
			return
		}
		for _, de := range des {
			c.add(root, filepath.Join(src, de.Name()), filepath.Join(dst, de.Name()))
		}
		c.dirs = append(c.dirs, dirTime{dst, fi.ModTime()})
	case mode.IsRegular():
		c.jobs <- copyJob{root, src, dst, fi}
	default:
		c.fail(root, src, errors.New("unsupported file type"))
	}
}

// finish waits for the workers and returns the top-level items that had errors.
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
func (o *opCtx) copyAll(pairs []copyPair) {
	c := newCopier(o)
	for i, p := range pairs {
		c.add(i, p.from, p.to)
	}
	c.finish()
}

// moveAll renames what it can (same drive: instant) and copies the rest with one
// shared worker pool; a source is deleted only if all of it was copied.
func (o *opCtx) moveAll(pairs []copyPair) {
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
		c.add(i, p.from, p.to)
		copied = append(copied, i)
	}
	failed := c.finish()
	for _, i := range copied {
		src := pairs[i].from
		if o.cancelled() || failed[i] {
			o.fail(src, errors.New("source NOT deleted because the copy is incomplete"))
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
