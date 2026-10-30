package main

// F4 (v1.12): edit the file under the cursor with an external editor. The editor is
// the command in the GOFM_EDITOR environment variable, else the "editor" setting of
// gofm.ini (v1.13), else Notepad on Windows. Examples:
//   GOFM_EDITOR=C:\Program Files\Notepad++\notepad++.exe
//   GOFM_EDITOR="C:\Program Files\Microsoft VS Code\Code.exe" --reuse-window
// gofm does not wait for the editor: it is started detached from gofm's console, so
// it must be a windowed (GUI) editor. Saved changes appear by themselves (automatic
// refresh).

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// editorCommand splits an editor command into the program and its arguments. The
// program may be quoted; an unquoted path with spaces is accepted when it names an
// existing file. An empty spec gives the default editor.
func editorCommand(spec, def string) (prog string, args []string, err error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = def
	}
	if spec == "" {
		return "", nil, errors.New("no editor: set editor in gofm.ini to the path of a windowed editor")
	}
	if strings.HasPrefix(spec, `"`) {
		end := strings.Index(spec[1:], `"`)
		if end < 0 {
			return "", nil, errors.New("editor command: missing closing quote")
		}
		return spec[1 : end+1], strings.Fields(spec[end+2:]), nil
	}
	if st, err := os.Stat(spec); err == nil && !st.IsDir() {
		return spec, nil, nil
	}
	f := strings.Fields(spec)
	return f[0], f[1:], nil
}

func (fm *FM) editFile() {
	p := fm.active
	e := p.current()
	if e == nil || e.isUp || e.isDrive || e.isShare {
		return
	}
	switch {
	case p.arch != nil:
		fm.showMessage("Edit", "Files inside an archive cannot be edited.\nExtract them first (F5 or Alt+U).")
		return
	case e.isDir:
		fm.showMessage("Edit", "F4 edits files, not folders.")
		return
	}
	path := p.fullPath(e.name)
	spec := os.Getenv("GOFM_EDITOR") // the environment wins over gofm.ini
	if spec == "" {
		spec = fm.cfg.editor
	}
	prog, args, err := editorCommand(spec, defaultEditor)
	if err == nil {
		cmd := exec.Command(prog, append(args, path)...)
		cmd.Dir = filepath.Dir(path)
		detach(cmd)
		if err = cmd.Start(); err == nil {
			_ = cmd.Process.Release()
			fm.notice = "Editing " + e.name + " with " + filepath.Base(prog)
			fm.updateStatus()
			return
		}
	}
	fm.showMessage("Edit", "Cannot start the editor "+prog+":\n"+err.Error()+
		"\n\nSet editor in gofm.ini (next to gofm.exe) to the path of a windowed editor.")
}
