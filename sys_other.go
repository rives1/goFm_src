//go:build !windows

// Linux/macOS implementations: they let the program build and be tested
// outside Windows. Windows remains the main target.

package main

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

func listDrives() []string { return nil }

func fileAttrs(fi fs.FileInfo) (string, bool) { return "", false }

func createdTime(fi fs.FileInfo) (time.Time, bool) { return time.Time{}, false }

func openWithDefault(path string) error {
	cmd := "xdg-open"
	if runtime.GOOS == "darwin" {
		cmd = "open"
	}
	return exec.Command(cmd, path).Start()
}

// moveToTrash follows (a simplified form of) the freedesktop Trash spec.
func moveToTrash(paths []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	files := filepath.Join(home, ".local", "share", "Trash", "files")
	info := filepath.Join(home, ".local", "share", "Trash", "info")
	if err := os.MkdirAll(files, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(info, 0o700); err != nil {
		return err
	}
	for _, p := range paths {
		abs, _ := filepath.Abs(p)
		base := filepath.Base(abs)
		name := base
		for i := 2; exists(filepath.Join(files, name)); i++ {
			name = fmt.Sprintf("%s.%d", base, i)
		}
		ti := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
			(&url.URL{Path: abs}).EscapedPath(), time.Now().Format("2006-01-02T15:04:05"))
		if err := os.WriteFile(filepath.Join(info, name+".trashinfo"), []byte(ti), 0o600); err != nil {
			return err
		}
		if err := os.Rename(abs, filepath.Join(files, name)); err != nil {
			os.Remove(filepath.Join(info, name+".trashinfo"))
			return fmt.Errorf("%s: %w", abs, err)
		}
	}
	return nil
}

func oemDecoder() *encoding.Decoder { return charmap.CodePage437.NewDecoder() }

func listShares(server string) ([]shareEntry, error) {
	return nil, fmt.Errorf("listing network shares is available on Windows only")
}

// copyFileNative copies one file; outside Windows it is the portable Go copy.
func copyFileNative(o *opCtx, src, dst string, fi fs.FileInfo) error {
	return copyFileGo(o, src, dst, fi)
}

// diskSpace returns the bytes available to this user and the total size of the
// file system holding path.
func diskSpace(path string) (free, total uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	return st.Bavail * uint64(st.Bsize), st.Blocks * uint64(st.Bsize), nil
}
