package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

var errCancelled = errors.New("operation cancelled")

// opCtx travels with a long operation: cancellation, progress, collected errors.
// The worker goroutine writes the counters; the UI goroutine only reads them.
type opCtx struct {
	cancel atomic.Bool
	errs   []string // written by the workers (under errMu), read after they finish
	errMu  sync.Mutex
	total  atomic.Int64    // bytes expected (0 = unknown)
	bytes  atomic.Int64    // bytes processed so far
	files  atomic.Int64    // files processed so far
	cur    atomic.Value    // string: current file or status line
	phase  atomic.Value    // string: e.g. "Counting files…"
	skip   map[string]bool // source files not to copy (conflicts answered "Skip"); set before copying
}

func (o *opCtx) cancelled() bool { return o.cancel.Load() }

func (o *opCtx) fail(path string, err error) {
	if errors.Is(err, errCancelled) {
		return
	}
	o.errMu.Lock() // copy workers report errors concurrently
	o.errs = append(o.errs, fmt.Sprintf("%s: %v", path, err))
	o.errMu.Unlock()
}

func (o *opCtx) step(path string) {
	o.files.Add(1)
	o.cur.Store(path)
}

func (o *opCtx) setPhase(s string) { o.phase.Store(s) }

func loadString(v *atomic.Value) string {
	s, _ := v.Load().(string)
	return s
}

// cancelReader stops a copy when the operation is cancelled and counts bytes for the progress bar.
type cancelReader struct {
	r io.Reader
	o *opCtx
}

func (c cancelReader) Read(b []byte) (int, error) {
	if c.o.cancelled() {
		return 0, errCancelled
	}
	n, err := c.r.Read(b)
	c.o.bytes.Add(int64(n))
	return n, err
}

// treeSize returns the total size of regular files under path (links are not followed).
func (o *opCtx) treeSize(path string) int64 {
	_, size := o.treeStats(path)
	return size
}

// treeStats counts the regular files under path and their total size (links are not followed).
func (o *opCtx) treeStats(path string) (files, size int64) {
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if o.cancelled() {
			return filepath.SkipAll
		}
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				files++
				size += info.Size()
			}
		}
		return nil
	})
	return
}

// ---------- paths ----------

func normPath(p string) string {
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

func samePath(a, b string) bool { return normPath(a) == normPath(b) }

// isWithin reports whether child equals parent or lies inside it.
func isWithin(parent, child string) bool {
	p, c := normPath(parent), normPath(child)
	if p == c {
		return true
	}
	sep := string(filepath.Separator)
	if !strings.HasSuffix(p, sep) {
		p += sep
	}
	return strings.HasPrefix(c, p)
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// ---------- zip ----------

func (o *opCtx) zipPaths(sources []string, baseDir, zipPath string) {
	f, err := os.Create(zipPath)
	if err != nil {
		o.fail(zipPath, err)
		return
	}
	zw := zip.NewWriter(f)
	for _, src := range sources {
		_ = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
			if o.cancelled() {
				return filepath.SkipAll
			}
			if err != nil {
				o.fail(path, err)
				return nil
			}
			if samePath(path, zipPath) {
				return nil // never put the archive inside itself
			}
			if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
				o.fail(path, errors.New("link not added to the archive"))
				return nil
			}
			info, err := d.Info()
			if err != nil {
				o.fail(path, err)
				return nil
			}
			rel, err := filepath.Rel(baseDir, path)
			if err != nil {
				o.fail(path, err)
				return nil
			}
			hdr, err := zip.FileInfoHeader(info)
			if err != nil {
				o.fail(path, err)
				return nil
			}
			hdr.Name = filepath.ToSlash(rel)
			if d.IsDir() {
				hdr.Name += "/"
				hdr.Method = zip.Store
			} else {
				hdr.Method = zip.Deflate
			}
			w, err := zw.CreateHeader(hdr)
			if err != nil {
				o.fail(path, err)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			o.step(path)
			in, err := os.Open(path)
			if err != nil {
				o.fail(path, err)
				return nil
			}
			_, err = io.Copy(w, cancelReader{in, o})
			in.Close()
			if err != nil {
				o.fail(path, err)
			}
			return nil
		})
	}
	err = zw.Close()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		o.fail(zipPath, err)
	}
	if err != nil || o.cancelled() {
		_ = os.Remove(zipPath)
	}
}

// zipEntryName decodes names written by old Windows tools in the OEM
// code page (e.g. CP850 in Italy) instead of UTF-8.
func zipEntryName(f *zip.File) string {
	if f.NonUTF8 && !utf8.ValidString(f.Name) {
		if s, err := oemDecoder().String(f.Name); err == nil {
			return s
		}
	}
	return f.Name
}

// ---------- search ----------

type searchHit struct {
	path  string
	isDir bool
}

func (o *opCtx) search(root, namePat, content string, includeHidden bool, limit int) []searchHit {
	var hits []searchHit
	needle := bytes.ToLower([]byte(content))
	lastTick := time.Now()
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if o.cancelled() || len(hits) >= limit {
			return filepath.SkipAll
		}
		if err != nil {
			return nil // folders without permission: ignore silently
		}
		if path != root && !includeHidden {
			if info, err := d.Info(); err == nil {
				if _, h := fileAttrs(info); h || strings.HasPrefix(d.Name(), ".") {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}
		if time.Since(lastTick) > 150*time.Millisecond {
			lastTick = time.Now()
			o.cur.Store(fmt.Sprintf("%d found — %s", len(hits), path))
		}
		if path == root {
			return nil
		}
		if namePat != "" && !matchName(d.Name(), namePat) {
			return nil
		}
		if len(needle) > 0 {
			if !d.Type().IsRegular() || !fileContains(path, needle, o) {
				return nil
			}
		}
		hits = append(hits, searchHit{path: path, isDir: d.IsDir()})
		return nil
	})
	return hits
}

// fileContains looks for needle (already lower-case) chunk by chunk.
func fileContains(path string, needle []byte, o *opCtx) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	const chunk = 256 << 10
	buf := make([]byte, chunk+len(needle))
	carry := 0
	for {
		if o.cancelled() {
			return false
		}
		n, err := f.Read(buf[carry:])
		if n > 0 {
			window := buf[:carry+n]
			if bytes.Contains(bytes.ToLower(window), needle) {
				return true
			}
			keep := len(needle) - 1
			if keep > len(window) {
				keep = len(window)
			}
			copy(buf, window[len(window)-keep:])
			carry = keep
		}
		if err != nil {
			return false
		}
	}
}

// deleteAll deletes items permanently (Del on a network drive, v1.12).
func (o *opCtx) deleteAll(items []string) {
	o.setPhase("Deleting…")
	for _, it := range items {
		if o.cancelled() {
			return
		}
		o.step(it)
		if err := removeTree(it); err != nil {
			o.fail(it, err)
		}
	}
}

// removeTree is os.RemoveAll that also removes read-only files: on Windows the
// read-only attribute makes the deletion fail, so it is cleared and the deletion
// retried. Links and junctions are removed, never followed.
func removeTree(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, werr error) error {
		if werr == nil && d.Type()&fs.ModeSymlink == 0 {
			if d.IsDir() {
				_ = os.Chmod(p, 0o777)
			} else {
				_ = os.Chmod(p, 0o666)
			}
		}
		return nil
	})
	return os.RemoveAll(path)
}
