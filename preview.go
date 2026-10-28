package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

const previewBytes = 512 << 10 // show at most 512 KB of text

// previewHeader describes the file: type, size, dates, attributes.
func previewHeader(path string) (string, fs.FileInfo, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return "", nil, err
	}
	var b strings.Builder
	kind := "File"
	switch {
	case fi.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0:
		kind = "Link"
		if t, err := os.Readlink(path); err == nil {
			kind += " → " + t
		}
	case fi.IsDir():
		kind = "Folder"
	}
	fmt.Fprintf(&b, "Path       : %s\n", path)
	fmt.Fprintf(&b, "Type       : %s\n", kind)
	if !fi.IsDir() {
		fmt.Fprintf(&b, "Size       : %s (%d bytes)\n", humanSize(fi.Size()), fi.Size())
	}
	fmt.Fprintf(&b, "Modified   : %s\n", fi.ModTime().Format("2006-01-02 15:04:05"))
	if t, ok := createdTime(fi); ok {
		fmt.Fprintf(&b, "Created    : %s\n", t.Format("2006-01-02 15:04:05"))
	}
	if a, _ := fileAttrs(fi); a != "" {
		fmt.Fprintf(&b, "Attributes : %s   (R=read-only H=hidden S=system A=archive)\n", a)
	}
	b.WriteString(strings.Repeat("─", 60) + "\n")
	return b.String(), fi, nil
}

// previewBody returns what to show for a file.
func previewBody(path string, fi fs.FileInfo) string {
	if isArchive(path) {
		return previewArchive(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "Cannot read: " + err.Error()
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, previewBytes))
	if err != nil {
		return "Read error: " + err.Error()
	}
	if len(data) == 0 {
		return "(empty file)"
	}
	text, ok := decodeText(data)
	if !ok {
		return "Binary file — first 512 bytes:\n\n" + hexDump(data, 512)
	}
	if fi.Size() > int64(len(data)) {
		text += fmt.Sprintf("\n\n… preview truncated at %s of %s", humanSize(int64(len(data))), humanSize(fi.Size()))
	}
	return text
}

// decodeText handles UTF-8, UTF-16 (with BOM) and, as a fallback, Windows-1252.
func decodeText(data []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		s, err := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder().Bytes(data)
		return string(s), err == nil
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		s, err := unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder().Bytes(data)
		return string(s), err == nil
	}
	sample := data
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return "", false
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	// If the preview was truncated, a multi-byte character cut in half must not fail the UTF-8 test.
	trimmed := data
	for i := len(data) - 1; len(data) >= previewBytes && i >= 0 && i >= len(data)-3; i-- {
		if utf8.RuneStart(data[i]) {
			if data[i] >= 0xC0 && !utf8.FullRune(data[i:]) {
				trimmed = data[:i] // incomplete multi-byte sequence from the cut
			}
			break
		}
	}
	if utf8.Valid(trimmed) {
		return strings.ReplaceAll(string(trimmed), "\r\n", "\n"), true
	}
	s, err := charmap.Windows1252.NewDecoder().Bytes(data)
	if err != nil {
		return "", false
	}
	return strings.ReplaceAll(string(s), "\r\n", "\n"), true
}

func hexDump(data []byte, max int) string {
	if len(data) > max {
		data = data[:max]
	}
	var b strings.Builder
	for i := 0; i < len(data); i += 16 {
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		fmt.Fprintf(&b, "%08X  ", i)
		for j := i; j < i+16; j++ {
			if j < end {
				fmt.Fprintf(&b, "%02X ", data[j])
			} else {
				b.WriteString("   ")
			}
		}
		b.WriteString(" ")
		for _, c := range data[i:end] {
			if c >= 32 && c < 127 {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// dirStats counts files and total size of a folder (cancellable).
func dirStats(root string, o *opCtx) (files, dirs int, size int64) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if o.cancelled() {
			return filepath.SkipAll
		}
		if err != nil || path == root {
			return nil
		}
		if d.IsDir() {
			dirs++
			return nil
		}
		files++
		if info, err := d.Info(); err == nil {
			size += info.Size()
		}
		return nil
	})
	return
}
