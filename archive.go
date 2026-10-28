package main

// Read access to .zip and .7z archives behind one small interface,
// so preview and extraction share the same (Zip Slip-safe) code.

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
)

type archEntry struct {
	name   string // slash-separated path inside the archive
	isDir  bool
	isLink bool
	size   uint64
	mod    time.Time
	mode   fs.FileMode
	open   func() (io.ReadCloser, error)
}

func archiveKind(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".zip":
		return "ZIP"
	case ".7z":
		return "7z"
	}
	return ""
}

func isArchive(name string) bool { return archiveKind(name) != "" }

// openArchive lists the entries of a .zip or .7z file. The caller must close the closer.
func openArchive(path string) ([]archEntry, io.Closer, error) {
	switch archiveKind(path) {
	case "ZIP":
		r, err := zip.OpenReader(path)
		if err != nil {
			return nil, nil, err
		}
		out := make([]archEntry, 0, len(r.File))
		for _, f := range r.File {
			f := f
			name := zipEntryName(f)
			out = append(out, archEntry{
				name:   name,
				isDir:  f.FileInfo().IsDir() || strings.HasSuffix(name, "/"),
				isLink: f.Mode()&fs.ModeSymlink != 0,
				size:   f.UncompressedSize64,
				mod:    f.Modified,
				mode:   f.Mode(),
				open:   func() (io.ReadCloser, error) { return f.Open() },
			})
		}
		return out, r, nil
	case "7z":
		r, err := sevenzip.OpenReader(path)
		if err != nil {
			return nil, nil, explain7z(err)
		}
		out := make([]archEntry, 0, len(r.File))
		for _, f := range r.File {
			f := f
			out = append(out, archEntry{
				name:   f.Name,
				isDir:  f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/"),
				isLink: f.Mode()&fs.ModeSymlink != 0,
				size:   f.UncompressedSize,
				mod:    f.Modified,
				mode:   f.Mode(),
				open: func() (io.ReadCloser, error) {
					rc, err := f.Open()
					return rc, explain7z(err)
				},
			})
		}
		return out, r, nil
	}
	return nil, nil, errors.New("unsupported archive format")
}

func explain7z(err error) error {
	if err == nil {
		return nil
	}
	var re *sevenzip.ReadError
	if errors.As(err, &re) && re.Encrypted {
		return errors.New("archive is password-protected (not supported)")
	}
	if strings.Contains(strings.ToLower(err.Error()), "password") {
		return errors.New("archive is password-protected (not supported)")
	}
	return err
}

// extract unpacks a whole .zip or .7z into destDir. Existing files are kept unless overwrite is set.
func (o *opCtx) extract(archPath, destDir string, overwrite bool) {
	o.extractFiltered(archPath, destDir, overwrite, func(name string) (string, bool) { return name, true })
}

// extractSelected unpacks only the given items (archive-internal paths, files or folders).
// Each item lands in destDir under its own base name, as when copying files.
func (o *opCtx) extractSelected(archPath string, items []string, destDir string, overwrite bool) {
	o.extractFiltered(archPath, destDir, overwrite, func(name string) (string, bool) {
		for _, it := range items {
			if name == it {
				return path.Base(it), true
			}
			if strings.HasPrefix(name, it+"/") {
				return path.Base(it) + "/" + name[len(it)+1:], true
			}
		}
		return "", false
	})
}

// extractFiltered unpacks the entries for which pick returns true, at the relative path pick returns.
func (o *opCtx) extractFiltered(archPath, destDir string, overwrite bool, pick func(name string) (string, bool)) {
	o.setPhase("Reading archive…")
	all, closer, err := openArchive(archPath)
	if err != nil {
		o.fail(archPath, err)
		return
	}
	defer closer.Close()
	type job struct {
		e   archEntry
		rel string
	}
	var jobs []job
	var total int64
	for _, e := range all {
		if rel, ok := pick(cleanArchName(e.name)); ok && rel != "" {
			jobs = append(jobs, job{e, rel})
			if !e.isDir {
				total += int64(e.size)
			}
		}
	}
	o.total.Store(total)
	o.setPhase("")
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		o.fail(destDir, err)
		return
	}
	if err := os.MkdirAll(destAbs, 0o755); err != nil {
		o.fail(destAbs, err)
		return
	}
	for _, j := range jobs {
		e, rel := j.e, j.rel
		if o.cancelled() {
			return
		}
		target := filepath.Join(destAbs, filepath.FromSlash(rel))
		// Zip Slip guard: nothing may land outside the destination.
		if filepath.IsAbs(filepath.FromSlash(rel)) || filepath.VolumeName(rel) != "" || !isWithin(destAbs, target) {
			o.fail(e.name, errors.New("unsafe path, skipped"))
			continue
		}
		if e.isDir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				o.fail(target, err)
			}
			continue
		}
		if e.isLink {
			o.fail(e.name, errors.New("symbolic link in archive skipped"))
			continue
		}
		if exists(target) && !overwrite {
			o.bytes.Add(int64(e.size)) // skipped, but counts toward the progress
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			o.fail(target, err)
			continue
		}
		o.step(e.name)
		rc, err := e.open()
		if err != nil {
			o.fail(e.name, err)
			if strings.Contains(err.Error(), "password") {
				return // every other entry will fail the same way
			}
			continue
		}
		perm := e.mode.Perm()
		if perm == 0 {
			perm = 0o644
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm|0o200)
		if err != nil {
			rc.Close()
			o.fail(target, err)
			continue
		}
		_, err = io.Copy(out, cancelReader{rc, o})
		rc.Close()
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(target) // no half-written files
			o.fail(target, explain7z(err))
			continue
		}
		if !e.mod.IsZero() {
			_ = os.Chtimes(target, e.mod, e.mod)
		}
	}
}

// archiveSingleRoot reports whether every entry sits under one top-level folder.
func archiveSingleRoot(path string) bool {
	entries, closer, err := openArchive(path)
	if err != nil || len(entries) == 0 {
		return false
	}
	defer closer.Close()
	root := ""
	for _, e := range entries {
		name := strings.TrimPrefix(strings.ReplaceAll(e.name, `\`, "/"), "/")
		first, rest, found := strings.Cut(name, "/")
		if !found && !e.isDir {
			return false // a file at the archive root
		}
		_ = rest
		if root == "" {
			root = first
		} else if !strings.EqualFold(root, first) {
			return false
		}
	}
	return root != ""
}

func previewArchive(path string) string {
	entries, closer, err := openArchive(path)
	if err != nil {
		return "Cannot read archive: " + err.Error()
	}
	defer closer.Close()
	var b strings.Builder
	var total uint64
	for _, e := range entries {
		total += e.size
	}
	fmt.Fprintf(&b, "%s archive: %d entries, %s uncompressed\n\n", archiveKind(path), len(entries), humanSize(int64(total)))
	for i, e := range entries {
		if i >= 1000 {
			fmt.Fprintf(&b, "… and %d more entries\n", len(entries)-i)
			break
		}
		name := e.name
		if e.isDir && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		size := humanSize(int64(e.size))
		if e.isDir {
			size = ""
		}
		fmt.Fprintf(&b, "%10s  %s  %s\n", size, e.mod.Local().Format("2006-01-02 15:04"), name)
	}
	return b.String()
}

// cleanArchName normalises an entry name: forward slashes, no leading or trailing slash.
func cleanArchName(name string) string {
	return strings.Trim(strings.ReplaceAll(name, `\`, "/"), "/")
}

// archChildren lists what is directly inside folder inner ("" = archive root).
// Folders that only exist implicitly (as a path prefix) are synthesised.
func archChildren(all []archEntry, inner string) []entry {
	prefix := ""
	if inner != "" {
		prefix = inner + "/"
	}
	byName := map[string]int{}
	var out []entry
	for _, e := range all {
		name := cleanArchName(e.name)
		if name == "" || name == inner || !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		first, more, isNested := strings.Cut(rest, "/")
		_ = more
		isDir := isNested || e.isDir
		if i, ok := byName[first]; ok {
			if !isNested { // the explicit entry carries the real date
				out[i].mod = e.mod
			}
			continue
		}
		row := entry{name: first, isDir: isDir, inArchive: true}
		if !isNested {
			row.mod = e.mod
			row.isLink = e.isLink
			if !e.isDir {
				row.size = int64(e.size)
			}
		}
		byName[first] = len(out)
		out = append(out, row)
	}
	return out
}

// findArchEntry returns the entry with the given internal path.
func findArchEntry(all []archEntry, name string) (archEntry, bool) {
	for _, e := range all {
		if cleanArchName(e.name) == name {
			return e, true
		}
	}
	return archEntry{}, false
}

// previewArchEntry shows the beginning of one file stored in an archive.
func previewArchEntry(archPath string, e archEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Archive    : %s\n", archPath)
	fmt.Fprintf(&b, "Entry      : %s\n", cleanArchName(e.name))
	fmt.Fprintf(&b, "Size       : %s (%d bytes)\n", humanSize(int64(e.size)), e.size)
	if !e.mod.IsZero() {
		fmt.Fprintf(&b, "Modified   : %s\n", e.mod.Local().Format("2006-01-02 15:04:05"))
	}
	b.WriteString(strings.Repeat("─", 60) + "\n")
	rc, err := e.open()
	if err != nil {
		return b.String() + "Cannot read: " + err.Error()
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, previewBytes))
	if err != nil {
		return b.String() + "Read error: " + explain7z(err).Error()
	}
	if len(data) == 0 {
		return b.String() + "(empty file)"
	}
	text, ok := decodeText(data)
	if !ok {
		return b.String() + "Binary file — first 512 bytes:\n\n" + hexDump(data, 512)
	}
	if e.size > uint64(len(data)) {
		text += fmt.Sprintf("\n\n… preview truncated at %s of %s", humanSize(int64(len(data))), humanSize(int64(e.size)))
	}
	return b.String() + text
}
