package main

// Remembered folders: the location of each panel, the active panel and the last
// folder used on each drive are saved in the user's configuration folder
// (%APPDATA%\gofm\state.json on Windows) whenever a panel changes location and at
// exit, and restored at the next start. Folders given on the command line win.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type savedState struct {
	Version   int               `json:"version"`
	Left      string            `json:"left"`
	Right     string            `json:"right"`
	Active    string            `json:"active"` // "left" or "right"
	DriveDirs map[string]string `json:"driveDirs,omitempty"`
	// BasicIcons: basic symbols instead of Nerd Font icons (Alt+I). Absent = false,
	// so state files written by older versions keep the default.
	BasicIcons bool `json:"basicIcons,omitempty"`
}

// statePath can be overridden with GOFM_STATE (tests, portable use).
func statePath() string {
	if p := os.Getenv("GOFM_STATE"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "gofm", "state.json")
}

func loadState() (savedState, bool) {
	var st savedState
	path := statePath()
	if path == "" {
		return st, false
	}
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &st) != nil {
		return savedState{}, false
	}
	return st, true
}

// panelHome is the location to remember for a panel: the folder that contains an
// archive being browsed, the root of a flat view, "" for the drive list, \\server
// for a share list.
func (p *Panel) panelHome() string { return p.dir }

func (fm *FM) currentState() savedState {
	st := savedState{Version: 1, Left: fm.left.panelHome(), Right: fm.right.panelHome(),
		Active: "left", DriveDirs: map[string]string{}, BasicIcons: fm.basicIcons}
	if fm.active == fm.right {
		st.Active = "right"
	}
	for l, d := range fm.lastDir {
		st.DriveDirs[string(l)] = d
	}
	return st
}

// saveState writes the state atomically (temporary file + rename). Errors are
// ignored: remembering folders is a convenience.
func (fm *FM) saveState() {
	if fm.left == nil || fm.right == nil || fm.restoring {
		return
	}
	st := fm.currentState()
	if st.Left == fm.saved.Left && st.Right == fm.saved.Right && st.Active == fm.saved.Active &&
		st.BasicIcons == fm.saved.BasicIcons && len(st.DriveDirs) == len(fm.saved.DriveDirs) && fm.saved.Version != 0 {
		same := true
		for k, v := range st.DriveDirs {
			if fm.saved.DriveDirs[k] != v {
				same = false
			}
		}
		if same {
			return // nothing changed: no write
		}
	}
	path := statePath()
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) != nil || os.Rename(tmp, path) != nil {
		_ = os.Remove(tmp)
		return
	}
	fm.saved = st
}

// restoreDir decides where a remembered location can be reopened. A network folder
// that does not answer within the timeout is not waited for. A local folder that no
// longer exists is replaced by its nearest existing parent.
func restoreDir(dir, fallback string, timeout time.Duration) string {
	switch {
	case dir == "":
		if onWindows {
			return "" // drive list
		}
		return fallback
	case onWindows && isUNCServer(dir):
		return dir // share list: if it cannot be read, the panel falls back by itself
	}
	ok := make(chan bool, 1)
	go func() {
		st, err := os.Stat(dir)
		ok <- err == nil && st.IsDir()
	}()
	select {
	case good := <-ok:
		if good {
			return dir
		}
	case <-time.After(timeout):
		return fallback // slow or unreachable (e.g. VPN not connected)
	}
	if _, _, _, isUNC := uncParts(dir); isUNC && onWindows {
		return fallback
	}
	for d := filepath.Dir(dir); d != dir; d, dir = filepath.Dir(d), d {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
	}
	return fallback
}
