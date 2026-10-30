package main

import (
	"archive/zip"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Tests never write gofm.ini next to the test binary: GOFM_CONFIG points to a file
// that does not exist and may not be created (tests that need one set their own).
func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "gofm-test")
	os.Setenv("GOFM_CONFIG", filepath.Join(dir, "gofm.ini"))
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// pressEnter sends Enter to the focused element (the selected button of a dialog).
func pressEnter(fm *FM) {
	fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
}

func TestParseSettings(t *testing.T) {
	text := "\ufeff# comment\r\n; other comment\r\n[gofm]\r\n" +
		"Editor = \"C:\\Program Files\\Notepad++\\notepad++.exe\"\r\n" +
		"colorscheme=LIGHT\nicons = basic\nshowHidden = true\nsortBy = date\n" +
		"deleteDefault = cancel\nnetworkDelete = off\nnetworkDeleteDefault = Cancel\n" +
		"sortBy =\n"
	s, problems := parseSettings(text)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	want := settings{editor: `C:\Program Files\Notepad++\notepad++.exe`, colorScheme: "light", icons: "basic",
		showHidden: "yes", sortBy: "date", deleteDefault: "cancel", networkDelete: "off", networkDeleteDefault: "cancel"}
	if s != want {
		t.Fatalf("got %+v", s)
	}
	// an editor with arguments keeps its quotes, so that editorCommand can split it
	s, _ = parseSettings(`editor = "C:\VS Code\Code.exe" --reuse-window`)
	if prog, args, _ := editorCommand(s.editor, ""); prog != `C:\VS Code\Code.exe` || len(args) != 1 {
		t.Errorf("editor %q → %q %q", s.editor, prog, args)
	}
	// problems are reported with their line; valid lines still apply
	s, problems = parseSettings("colorScheme = pink\nfoo = 1\nno equal sign\nicons = basic\nshowHidden = maybe")
	if len(problems) != 4 || s.icons != "basic" || s.colorScheme != "" || s.showHidden != "" {
		t.Fatalf("%+v %q", s, problems)
	}
	if !strings.HasPrefix(problems[0], "line 1:") || !strings.Contains(problems[1], `"foo"`) {
		t.Error(problems)
	}
}

// The file written when gofm.ini is missing: every setting documented and commented
// out, so it changes nothing; Windows line endings for Notepad.
func TestDefaultConfigText(t *testing.T) {
	text := defaultConfigText()
	s, problems := parseSettings(text)
	if s != (settings{}) || len(problems) != 0 {
		t.Fatalf("%+v %v", s, problems)
	}
	if strings.Count(text, "\r\n") != strings.Count(text, "\n") {
		t.Error("not CRLF")
	}
	for _, sp := range settingSpecs {
		if !strings.Contains(text, "\n#"+sp.key+" = ") || !strings.Contains(text, "# "+sp.key+":") {
			t.Errorf("%s not documented", sp.key)
		}
		for _, v := range sp.allowed {
			if !strings.Contains(text, v) {
				t.Errorf("%s: value %s not documented", sp.key, v)
			}
		}
	}
	// the commented defaults are the built-in ones
	for _, l := range []string{"#deleteDefault = " + defDeleteDefault, "#networkDelete = " + defNetworkDelete,
		"#networkDeleteDefault = " + defNetworkDeleteDefault, "#colorScheme = navy", "#icons = nerd",
		"#showHidden = no", "#sortBy = name", "#editor = " + "notepad.exe"} {
		if !strings.Contains(text, l) {
			t.Errorf("missing %q", l)
		}
	}
	// the uncommented file must parse to exactly the defaults
	s, problems = parseSettings(strings.ReplaceAll(strings.ReplaceAll(text, "\n#editor = notepad.exe", "\neditor = notepad.exe"), "\n#", "\n"))
	_ = s
	for _, p := range problems { // the explanation lines become invalid, the settings must not
		for _, sp := range settingSpecs {
			if strings.Contains(p, sp.key+" must be") {
				t.Errorf("default of %s invalid: %s", sp.key, p)
			}
		}
	}
}

// gofm.ini and state.json live next to the executable; the environment overrides them.
func TestStorage(t *testing.T) {
	dir, writable := appDirInfo()
	exe, _ := os.Executable()
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	if dir != filepath.Dir(exe) || !writable {
		t.Fatalf("app dir %q writable=%v", dir, writable)
	}
	t.Setenv("GOFM_STATE", "")
	t.Setenv("GOFM_CONFIG", "")
	s := findStorage()
	if s.statePath != filepath.Join(dir, "state.json") || s.configPath != filepath.Join(dir, "gofm.ini") ||
		!s.stateWrite || !s.configWrite || s.readOnlyDir != "" {
		t.Fatalf("%+v", s)
	}
	t.Setenv("GOFM_STATE", "/x/s.json")
	t.Setenv("GOFM_CONFIG", "/x/c.ini")
	if s = findStorage(); s.statePath != "/x/s.json" || s.configPath != "/x/c.ini" {
		t.Fatalf("%+v", s)
	}
}

func TestDirWritable(t *testing.T) {
	dir := t.TempDir()
	if !dirWritable(dir) || dirWritable("") || dirWritable(filepath.Join(dir, "missing")) {
		t.Fatal("dirWritable")
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatal("test file left behind")
	}
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return // chmod does not make a folder read-only there
	}
	_ = os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)
	if dirWritable(dir) {
		t.Error("read-only folder reported writable")
	}
}

// gofm.ini is created, commented, only when missing and only where gofm may write;
// a read-only program folder saves nothing.
func TestLoadSettingsCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gofm.ini")
	if s, msg := loadSettings(storage{configPath: path}); s != (settings{}) || msg != "" {
		t.Fatal(s, msg)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("created in a read-only folder")
	}
	loadSettings(storage{configPath: path, configWrite: true})
	data, err := os.ReadFile(path)
	if err != nil || string(data) != defaultConfigText() {
		t.Fatal("not created", err)
	}
	_ = os.WriteFile(path, []byte("sortBy = size\nbad line\n"), 0o644)
	s, msg := loadSettings(storage{configPath: path, configWrite: true})
	if s.sortBy != "size" || !strings.Contains(msg, "gofm.ini line 2") {
		t.Fatal(s, msg)
	}
	if data, _ := os.ReadFile(path); string(data) != "sortBy = size\nbad line\n" {
		t.Fatal("existing file rewritten")
	}
}

// A read-only program folder: existing files are read, nothing is created or written,
// and the status bar says so.
func TestReadOnlyProgramFolder(t *testing.T) {
	dir := t.TempDir()
	old := appDirInfo
	appDirInfo = func() (string, bool) { return dir, false }
	defer func() { appDirInfo = old }()
	t.Setenv("GOFM_STATE", "")
	t.Setenv("GOFM_CONFIG", "")
	fm := newFM(t.TempDir(), t.TempDir(), savedState{})
	defer fm.watch.close()
	fm.left.chdir(t.TempDir(), "")
	fm.cycleScheme()
	defer applyScheme(0)
	if files, _ := os.ReadDir(dir); len(files) != 0 {
		t.Fatalf("%d files written", len(files))
	}
	if !strings.Contains(fm.notice, "read-only") && !strings.Contains(fm.notice, "Colours") {
		t.Error(fm.notice)
	}
	fm2 := newFM(t.TempDir(), t.TempDir(), savedState{})
	defer fm2.watch.close()
	if !strings.Contains(fm2.notice, "read-only: settings and folders are not saved") {
		t.Errorf("notice %q", fm2.notice)
	}
	// a gofm.ini already there is still read
	_ = os.WriteFile(filepath.Join(dir, "gofm.ini"), []byte("sortBy = date\n"), 0o644)
	fm3 := newFM(t.TempDir(), t.TempDir(), savedState{})
	defer fm3.watch.close()
	if fm3.left.sortMode != sortDate {
		t.Error("existing gofm.ini not read")
	}
}

// Settings of gofm.ini at start: they win over state.json; sort and hidden files apply
// to both panels; the editor is used by F4 unless GOFM_EDITOR is set.
func TestSettingsAtStart(t *testing.T) {
	ini := filepath.Join(t.TempDir(), "gofm.ini")
	_ = os.WriteFile(ini, []byte("colorScheme = dark\nicons = basic\nshowHidden = yes\nsortBy = size\neditor = myeditor\n"), 0o644)
	t.Setenv("GOFM_CONFIG", ini)
	t.Setenv("GOFM_STATE", filepath.Join(t.TempDir(), "state.json"))
	fm := newFM(t.TempDir(), t.TempDir(), savedState{ColorScheme: "light"})
	defer fm.watch.close()
	defer applyScheme(0)
	if schemes[curScheme].name != "dark" || !fm.basicIcons {
		t.Fatal("scheme or icons")
	}
	for _, p := range []*Panel{fm.left, fm.right} {
		if !p.showHidden || sortLabels[p.sortMode] != "size" || p.sortAsc {
			t.Fatal("view settings")
		}
	}
	if fm.cfg.editor != "myeditor" || fm.notice != "" {
		t.Fatal(fm.cfg, fm.notice)
	}
	// without the settings, state.json decides
	_ = os.WriteFile(ini, []byte("# nothing\n"), 0o644)
	fm2 := newFM(t.TempDir(), t.TempDir(), savedState{ColorScheme: "light", BasicIcons: true})
	defer fm2.watch.close()
	if schemes[curScheme].name != "light" || !fm2.basicIcons || fm2.left.showHidden || fm2.left.sortMode != sortName {
		t.Fatal("state.json not used")
	}
	// a problem in the file is shown in the status bar
	_ = os.WriteFile(ini, []byte("sortBy = random\n"), 0o644)
	fm3 := newFM(t.TempDir(), t.TempDir(), savedState{})
	defer fm3.watch.close()
	if !strings.Contains(fm3.notice, "sortBy must be") {
		t.Fatal(fm3.notice)
	}
}

// Alt+R reads gofm.ini again and applies only what changed in the file.
func TestReloadSettings(t *testing.T) {
	ini := filepath.Join(t.TempDir(), "gofm.ini")
	_ = os.WriteFile(ini, []byte("# empty\n"), 0o644)
	t.Setenv("GOFM_CONFIG", ini)
	fm := newTestFM(t)
	defer applyScheme(0)
	_ = os.WriteFile(ini, []byte("colorScheme = light\nshowHidden = yes\neditor = ed2\n"), 0o644)
	fm.keys(key(tcell.KeyRune, 'r', tcell.ModAlt))
	if schemes[curScheme].name != "light" || !fm.left.showHidden || !fm.right.showHidden || fm.cfg.editor != "ed2" {
		t.Fatal("not applied")
	}
	if !strings.Contains(fm.notice, "Settings reloaded") {
		t.Error(fm.notice)
	}
	// changes made with the keys survive a refresh when the file did not change
	fm.cycleScheme()
	fm.keys(key(tcell.KeyRune, 'h', tcell.ModAlt))
	fm.keys(key(tcell.KeyRune, 'r', tcell.ModAlt))
	if schemes[curScheme].name == "light" || fm.active.showHidden {
		t.Fatal("Alt+R undid the keys")
	}
	if fm.notice != "" {
		t.Errorf("notice %q without changes", fm.notice)
	}
	// a missing file is created again on Alt+R
	_ = os.Remove(ini)
	fm.keys(key(tcell.KeyRune, 'r', tcell.ModAlt))
	if _, err := os.Stat(ini); err != nil {
		t.Error("gofm.ini not recreated")
	}
}

// The delete button is selected by default; gofm.ini can select Cancel instead,
// separately for network drives, or turn deleting on network drives off.
func TestDeleteDefaults(t *testing.T) {
	for _, c := range []struct {
		s       settings
		network bool
		want    int
	}{
		{settings{}, false, 0}, {settings{}, true, 0},
		{settings{deleteDefault: "cancel"}, false, 1}, {settings{deleteDefault: "cancel"}, true, 0},
		{settings{networkDeleteDefault: "cancel"}, true, 1}, {settings{networkDeleteDefault: "cancel"}, false, 0},
	} {
		if got := deleteDefaultIndex(c.s, c.network); got != c.want {
			t.Errorf("%+v network=%v: %d", c.s, c.network, got)
		}
	}
	for _, network := range []bool{false, true} {
		_, buttons := deleteQuestion([]string{"a"}, network)
		if len(buttons) != 2 || (buttons[1] != "Cancel" && buttons[1] != "No") {
			t.Errorf("buttons %q", buttons)
		}
	}
	if (settings{}).networkDeleteMode() != "permanent" || (settings{networkDelete: "off"}).networkDeleteMode() != "off" {
		t.Error("networkDelete")
	}
}

// Del: the button selected at first (the one Enter presses) follows gofm.ini, and
// Enter on Cancel deletes nothing.
func TestDeleteEnter(t *testing.T) {
	fm := newTestFM(t)
	p := fm.left
	file := filepath.Join(p.dir, "f.txt")
	_ = os.WriteFile(file, []byte("x"), 0o644)
	p.reload()
	fm.setActive(p, true)
	for _, c := range []struct{ setting, want string }{{"", "Yes"}, {"delete", "Yes"}, {"cancel", "No"}} {
		fm.cfg.deleteDefault = c.setting
		p.jumpTo("f.txt")
		fm.keys(key(tcell.KeyDelete, 0, 0))
		b, ok := fm.app.GetFocus().(*tview.Button)
		if len(fm.modals) != 1 || !ok || b.GetLabel() != c.want {
			t.Fatalf("deleteDefault=%q: selected %v", c.setting, fm.app.GetFocus())
		}
		if c.want == "No" {
			pressEnter(fm)
			if _, err := os.Stat(file); err != nil || len(fm.modals) != 0 {
				t.Fatal("Enter on No deleted, or the dialog stayed open")
			}
		} else {
			closeAll(fm)
		}
	}
}

// Copy and move: folders with the same name at the destination need Merge first;
// Cancel stops the whole operation.
func TestMergeQuestion(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	_ = os.MkdirAll(filepath.Join(src, "docs"), 0o755)
	_ = os.MkdirAll(filepath.Join(dst, "docs"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "a.txt"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(dst, "a.txt"), nil, 0o644)
	_ = os.MkdirAll(filepath.Join(src, "new"), 0o755)
	got := sameNameFolders(
		[]string{filepath.Join(src, "docs"), filepath.Join(src, "a.txt"), filepath.Join(src, "new")},
		[]string{filepath.Join(dst, "docs"), filepath.Join(dst, "a.txt"), filepath.Join(dst, "new")})
	if len(got) != 1 || got[0] != filepath.Join(dst, "docs") {
		t.Fatalf("%q", got)
	}
	text := mergeText(got, dst, "then")
	if !strings.Contains(text, "A folder with the same name") || !strings.Contains(text, "\ndocs\n") || !strings.Contains(text, "Cancel stops") {
		t.Error(text)
	}
	var many []string
	for i := 0; i < 10; i++ {
		many = append(many, filepath.Join(dst, string(rune('a'+i))))
	}
	if text = mergeText(many, dst, ""); !strings.Contains(text, "10 folders") || !strings.Contains(text, "… and 2 more") {
		t.Error(text)
	}

	fm := newTestFM(t)
	started := 0
	fm.askMerge(nil, dst, "", func() { started++ })
	if started != 1 || len(fm.modals) != 0 {
		t.Fatal("no folders: must start at once")
	}
	for _, c := range []struct {
		key  tcell.Key
		want int
	}{{tcell.KeyEnter, 2}, {tcell.KeyEscape, 2}} {
		fm.askMerge(got, dst, "", func() { started++ })
		if len(fm.modals) != 1 {
			t.Fatal("no question")
		}
		fm.app.GetFocus().InputHandler()(tcell.NewEventKey(c.key, 0, 0), func(tview.Primitive) {})
		if started != c.want || len(fm.modals) != 0 {
			t.Fatalf("key %v: started %d", c.key, started)
		}
	}

	// F5 and F6 ask before doing anything
	for _, k := range []tcell.Key{tcell.KeyF5, tcell.KeyF6} {
		fm.left.chdir(src, "")
		fm.right.chdir(dst, "")
		fm.setActive(fm.left, true)
		fm.left.jumpTo("docs")
		fm.keys(key(k, 0, 0))
		if len(fm.modals) != 1 {
			t.Fatalf("%v: no merge question", k)
		}
		fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
		if len(fm.modals) != 0 {
			t.Fatal("still open")
		}
		if _, err := os.Stat(filepath.Join(src, "docs")); err != nil {
			t.Fatal("Cancel moved the folder")
		}
	}
}

func TestArchiveTopFolders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.zip")
	f, _ := os.Create(path)
	w := zip.NewWriter(f)
	for _, n := range []string{"top/", "top/x.txt", "Other/sub/y.txt", "root.txt", "OTHER/z.txt"} {
		_, _ = w.Create(n)
	}
	w.Close()
	f.Close()
	got := archiveTopFolders(path)
	if strings.Join(got, ",") != "top,Other" {
		t.Fatalf("%q", got)
	}
}

// The Windows icon has the sizes Explorer uses: 16, 32, 48 and 256 pixels.
func TestIconSizes(t *testing.T) {
	want := map[string]int{"icon.png": 256, "icon48.png": 48, "icon32.png": 32, "icon16.png": 16}
	for name, size := range want {
		f, err := os.Open("winres/" + name)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(f)
		f.Close()
		if err != nil || cfg.Width != size || cfg.Height != size {
			t.Errorf("%s: %dx%d %v", name, cfg.Width, cfg.Height, err)
		}
	}
	data, _ := os.ReadFile("winres/winres.json")
	for name := range want {
		if !strings.Contains(string(data), `"`+name+`"`) {
			t.Errorf("%s not in winres.json", name)
		}
	}
}
