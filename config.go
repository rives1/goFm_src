package main

// Portable files and user settings (v1.13).
//
// gofm keeps its two files next to the executable, so the program folder can be
// copied anywhere (a USB stick, another PC) together with its settings:
//
//   gofm.ini    settings written by the user (gofm creates it, commented, if missing,
//               and never rewrites it afterwards)
//   state.json  what gofm remembers by itself: folders, active panel, icons, colours
//
// If the program folder is not writable (e.g. C:\Program Files) nothing is saved:
// existing files are read, none is created or updated, and every start uses the
// defaults plus whatever those files already contain. There is no migration from the
// %APPDATA%\gofm\state.json of 1.12 and earlier.
//
// GOFM_STATE and GOFM_CONFIG override the two paths (tests, special set-ups);
// GOFM_EDITOR overrides the editor of gofm.ini.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	configName = "gofm.ini"
	stateName  = "state.json"
)

// appDir is the folder of the running executable ("" if unknown).
func appDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return filepath.Dir(exe)
}

// dirWritable tells whether a file can be created in dir: the only reliable test on
// Windows, where folder permissions (ACLs) are not visible in the file mode.
func dirWritable(dir string) bool {
	if dir == "" {
		return false
	}
	f, err := os.CreateTemp(dir, ".gofm-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	_ = os.Remove(name)
	return true
}

// storage says where the two files are and whether they may be written.
type storage struct {
	statePath, configPath string
	stateWrite            bool // state.json may be created/updated
	configWrite           bool // gofm.ini may be created when missing
	readOnlyDir           string
}

// appDirInfo is computed once: the program folder does not change while gofm runs.
var appDirInfo = sync.OnceValues(func() (string, bool) {
	d := appDir()
	return d, dirWritable(d)
})

func findStorage() storage {
	var s storage
	dir, writable := appDirInfo()
	if !writable {
		s.readOnlyDir = dir
	}
	if p := os.Getenv("GOFM_STATE"); p != "" {
		s.statePath, s.stateWrite = p, true
	} else if dir != "" {
		s.statePath, s.stateWrite = filepath.Join(dir, stateName), writable
	}
	if p := os.Getenv("GOFM_CONFIG"); p != "" {
		s.configPath, s.configWrite = p, true
	} else if dir != "" {
		s.configPath, s.configWrite = filepath.Join(dir, configName), writable
	}
	return s
}

// ---------- settings ----------

// settings are the values of gofm.ini. An empty string means "not set": the built-in
// default, or for colorScheme and icons the choice remembered in state.json.
type settings struct {
	editor               string
	colorScheme          string // navy | dark | light
	icons                string // nerd | basic
	showHidden           string // yes | no
	sortBy               string // name | size | date
	deleteDefault        string // delete | cancel: default button when deleting (local drives)
	networkDelete        string // permanent | off: Del on network drives
	networkDeleteDefault string // delete | cancel: default button when deleting on network drives
}

// Built-in defaults of the settings that always have a value.
const (
	defDeleteDefault        = "delete"
	defNetworkDelete        = "permanent"
	defNetworkDeleteDefault = "delete"
)

// settingSpec describes one key: the allowed values (nil = free text).
type settingSpec struct {
	key     string
	allowed []string
	field   func(*settings) *string
}

var settingSpecs = []settingSpec{
	{"editor", nil, func(s *settings) *string { return &s.editor }},
	{"colorScheme", []string{"navy", "dark", "light"}, func(s *settings) *string { return &s.colorScheme }},
	{"icons", []string{"nerd", "basic"}, func(s *settings) *string { return &s.icons }},
	{"showHidden", []string{"yes", "no"}, func(s *settings) *string { return &s.showHidden }},
	{"sortBy", []string{"name", "size", "date"}, func(s *settings) *string { return &s.sortBy }},
	{"deleteDefault", []string{"delete", "cancel"}, func(s *settings) *string { return &s.deleteDefault }},
	{"networkDelete", []string{"permanent", "off"}, func(s *settings) *string { return &s.networkDelete }},
	{"networkDeleteDefault", []string{"delete", "cancel"}, func(s *settings) *string { return &s.networkDeleteDefault }},
}

// boolWords accepts the usual spellings for yes/no.
var boolWords = map[string]string{"yes": "yes", "true": "yes", "on": "yes", "1": "yes",
	"no": "no", "false": "no", "off": "no", "0": "no"}

// parseSettings reads the text of gofm.ini: "key = value" lines, comments starting
// with # or ;, optional [sections] (ignored), keys and values not case-sensitive
// (except the editor command), values optionally in double quotes. Unknown keys and
// invalid values are reported and ignored; the other lines still apply.
func parseSettings(text string) (s settings, problems []string) {
	text = strings.TrimPrefix(text, "\ufeff") // Notepad may save with a BOM
	for n, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || line[0] == '#' || line[0] == ';' || line[0] == '[' {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			problems = append(problems, fmt.Sprintf("line %d: expected key = value", n+1))
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' && strings.Count(value, `"`) == 2 {
			value = value[1 : len(value)-1] // "C:\...\x.exe" alone; an editor with arguments keeps its quotes
		}
		var spec *settingSpec
		for i := range settingSpecs {
			if strings.EqualFold(settingSpecs[i].key, key) {
				spec = &settingSpecs[i]
			}
		}
		if spec == nil {
			problems = append(problems, fmt.Sprintf("line %d: unknown setting %q", n+1, key))
			continue
		}
		if value == "" {
			continue // "key =" means not set
		}
		if spec.allowed != nil {
			v := strings.ToLower(value)
			if spec.key == "showHidden" {
				v = boolWords[v]
			}
			valid := false
			for _, a := range spec.allowed {
				valid = valid || v == a
			}
			if !valid {
				problems = append(problems, fmt.Sprintf("line %d: %s must be %s", n+1, spec.key, strings.Join(spec.allowed, " or ")))
				continue
			}
			value = v
		}
		*spec.field(&s) = value
	}
	return s, problems
}

// deleteDefaultButton is the deleteDefault or networkDeleteDefault in effect.
func (s settings) deleteDefaultButton(network bool) string {
	if network {
		return orDefault(s.networkDeleteDefault, defNetworkDeleteDefault)
	}
	return orDefault(s.deleteDefault, defDeleteDefault)
}

func (s settings) networkDeleteMode() string { return orDefault(s.networkDelete, defNetworkDelete) }

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// loadSettings reads gofm.ini; when it is missing and may be written, it is created
// with every setting commented out. Problems are returned as one short notice.
func loadSettings(st storage) (settings, string) {
	if st.configPath == "" {
		return settings{}, ""
	}
	data, err := os.ReadFile(st.configPath)
	if os.IsNotExist(err) {
		if st.configWrite {
			_ = os.WriteFile(st.configPath, []byte(defaultConfigText()), 0o644)
		}
		return settings{}, ""
	}
	if err != nil {
		return settings{}, configName + ": " + err.Error()
	}
	s, problems := parseSettings(string(data))
	if len(problems) == 0 {
		return s, ""
	}
	msg := configName + " " + problems[0]
	if len(problems) > 1 {
		msg += fmt.Sprintf(" (and %d more problems)", len(problems)-1)
	}
	return s, msg + ": ignored"
}

// defaultConfigText is the gofm.ini written when the file is missing. Lines use the
// Windows line ending so that every editor shows them correctly.
func defaultConfigText() string {
	return strings.ReplaceAll(`# gofm settings (version `+version+`)
#
# This file is read when gofm starts and again with Alt+R. Edit it with any text
# editor, save it, then press Alt+R in gofm (or restart it).
#
# Syntax
#   key = value       one setting per line; spaces around "=" are optional
#   # or ;            a comment: the rest of the line is ignored
#   key =             (no value) the same as a commented line: the default is used
#   Keys and values are not case-sensitive. Values may be written in double quotes.
#   An unknown key or an invalid value is reported in the status bar and ignored;
#   the other lines still apply.
#
# Every setting below is commented out, so gofm uses its defaults. To change one,
# remove the "#" at the start of its line and edit the value.
#
# On Alt+R only the settings whose value changed in this file are applied, so a
# refresh does not undo what you changed with the keys (Alt+H, Alt+O, Ctrl+F9, Alt+I).
#
# This file and state.json live next to gofm.exe. gofm never rewrites this file;
# state.json is gofm's own memory (folders, icons, colours) and needs no editing.
# If the folder of gofm.exe is read-only, gofm saves nothing.


# ---------------------------------------------------------------------------
# editor: program used by F4 to edit the file under the cursor.
#   Default: notepad.exe
#   Write the path of the program, optionally followed by arguments; the file name
#   is added at the end. A path with spaces may be written with or without quotes;
#   with arguments it needs quotes. The editor must have its own window (gofm does
#   not wait for it): console editors such as vim or nano do not work.
#   The environment variable GOFM_EDITOR, if set, wins over this setting.
#   Examples:
#     editor = C:\Program Files\Notepad++\notepad++.exe
#     editor = "C:\Program Files\Microsoft VS Code\Code.exe" --reuse-window
#editor = notepad.exe


# ---------------------------------------------------------------------------
# colorScheme: colours of the interface.
#   navy   blue background, the classic look (default)
#   dark   dark grey background
#   light  white background
#   Not set: the last scheme chosen with Ctrl+F9 (remembered in state.json).
#   Set: this scheme at every start; Ctrl+F9 still changes it until the next start.
#colorScheme = navy


# ---------------------------------------------------------------------------
# icons: symbols in front of the file names.
#   nerd   Nerd Font icons (default); they need a Nerd Font in the terminal,
#          e.g. Cascadia Mono NF, otherwise they appear as empty boxes
#   basic  simple symbols that every font has
#   Not set: the last choice made with Alt+I (remembered in state.json).
#   Set: this choice at every start; Alt+I still changes it until the next start.
#icons = nerd


# ---------------------------------------------------------------------------
# showHidden: show hidden and system files in both panels at start.
#   no   hidden files are not listed (default)
#   yes  hidden files are listed, dimmed
#   (also accepted: true/false, on/off, 1/0). Alt+H toggles it in each panel.
#showHidden = no


# ---------------------------------------------------------------------------
# sortBy: sort order of both panels at start.
#   name   by name, A to Z (default)
#   size   by size, largest first
#   date   by date, newest first
#   Folders are always listed before files. Alt+O changes the order in a panel,
#   Alt+D reverses it.
#sortBy = name


# ---------------------------------------------------------------------------
# deleteDefault: the button selected when the Del key asks for confirmation on a
# local drive (the items go to the Recycle Bin, so they can be restored).
#   delete  Enter confirms the deletion (default)
#   cancel  Enter cancels; you must move to the delete button first
#   It also applies to the second warning shown for folders that contain subfolders.
#deleteDefault = delete


# ---------------------------------------------------------------------------
# networkDelete: what the Del key does on network drives (\\server\share paths and
# mapped drive letters). Network folders have no Recycle Bin.
#   permanent  the items are deleted PERMANENTLY after a confirmation that says so
#              (default)
#   off        Del does nothing on network drives except show a message; delete
#              there with Windows Explorer
#networkDelete = permanent


# ---------------------------------------------------------------------------
# networkDeleteDefault: the button selected when Del asks for confirmation on a
# network drive, where the deletion is PERMANENT and cannot be undone.
#   delete  Enter confirms the deletion (default)
#   cancel  Enter cancels; safer, since one Enter too many deletes for good
#   It also applies to the second warning for folders that contain subfolders.
#networkDeleteDefault = delete
`, "\n", "\r\n")
}
