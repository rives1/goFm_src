// gofm — a dual-pane file manager for the Windows console.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const version = "1.13"

type FM struct {
	app     *tview.Application
	pages   *tview.Pages
	left    *Panel
	right   *Panel
	active  *Panel
	status  *statusBar
	watch   *watcher
	modals  []string
	focus   map[string]tview.Primitive
	seq     int
	lastDir map[byte]string // last folder visited on each drive, for Shift+<letter>

	notice string // one-shot message in the status bar, cleared by the next key

	basicIcons bool // Alt+I: basic symbols instead of Nerd Font icons

	root, panelsBox *tview.Flex // kept to recolour them (Ctrl+F9)
	keybar          *keyBar

	cfg settings // gofm.ini as last read (start, Alt+R)

	saved     savedState // last state written to disk
	restoring bool       // true while the panels are being set up: no state writes yet

	qs     string // quick-search text typed so far
	qsTime time.Time
	qsMiss bool
}

const quickSearchTimeout = 1500 * time.Millisecond

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "/?") {
		fmt.Println("gofm " + version + " — usage: gofm [left_folder] [right_folder]")
		return
	}
	cwd, _ := os.Getwd()
	l, r := cwd, cwd
	st, haveState := loadState()
	if haveState { // remembered folders; a folder that is gone or does not answer falls back
		l = restoreDir(st.Left, cwd, 3*time.Second)
		r = restoreDir(st.Right, cwd, 3*time.Second)
	}
	if len(os.Args) > 1 {
		l = absOr(os.Args[1], cwd)
	}
	if len(os.Args) > 2 {
		r = absOr(os.Args[2], cwd)
	}
	fm := newFM(l, r, st)
	defer fm.watch.close()
	defer fm.saveState()
	if err := fm.app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func absOr(p, fallback string) string {
	p = expandPath(p)
	if onWindows && isUNCServer(p) {
		return p
	}
	a, err := filepath.Abs(p)
	if err != nil {
		return fallback
	}
	if st, err := os.Stat(a); err != nil || !st.IsDir() {
		return fallback
	}
	return a
}

func newFM(leftDir, rightDir string, st savedState) *FM {
	store := findStorage()
	cfg, cfgProblem := loadSettings(store)
	// gofm.ini wins over what state.json remembers, when the setting is present.
	if cfg.colorScheme != "" {
		st.ColorScheme = cfg.colorScheme
	}
	if cfg.icons != "" {
		st.BasicIcons = cfg.icons == "basic"
	}
	applyScheme(schemeIndex(st.ColorScheme)) // before any primitive is created
	fm := &FM{app: tview.NewApplication(), pages: tview.NewPages(),
		focus: map[string]tview.Primitive{}, lastDir: map[byte]string{}, restoring: true,
		basicIcons: st.BasicIcons, cfg: cfg}
	for k, d := range st.DriveDirs {
		if len(k) == 1 && driveLetter(k+":") != 0 {
			fm.lastDir[driveLetter(k+":")] = d
		}
	}
	fm.watch = newWatcher(fm)
	fm.left = newPanel(fm, leftDir)
	fm.right = newPanel(fm, rightDir)
	for _, p := range []*Panel{fm.left, fm.right} {
		p.applyViewSettings(cfg)
		if err := p.load(""); err != nil {
			p.dir, _ = os.Getwd()
			_ = p.load("")
		}
		if l := driveLetter(p.dir); l != 0 {
			fm.lastDir[l] = p.dir
		}
	}

	fm.status = newStatusBar(fm)
	fm.keybar = newKeyBar(fm)

	fm.panelsBox = tview.NewFlex().
		AddItem(fm.left.table, 0, 1, true).
		AddItem(fm.right.table, 0, 1, false)
	fm.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(fm.panelsBox, 0, 1, true).
		AddItem(fm.status, 1, 0, false).
		AddItem(fm.keybar, 1, 0, false)

	fm.pages.AddPage("main", fm.root, true, true)
	fm.app.SetRoot(fm.pages, true).EnableMouse(true)
	fm.app.SetInputCapture(fm.keys)
	fm.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		// Underline cursor (as the classic Windows console) instead of the terminal's I-beam.
		screen.SetCursorStyle(tcell.CursorStyleBlinkingUnderline)
		return false
	})
	if st.Active == "right" {
		fm.setActive(fm.right, true)
	} else {
		fm.setActive(fm.left, true)
	}
	fm.restoring = false
	fm.syncWatches()
	fm.saveState()
	switch {
	case cfgProblem != "":
		fm.notice = cfgProblem
	case store.readOnlyDir != "":
		fm.notice = "The gofm folder is read-only: settings and folders are not saved"
	}
	return fm
}

// applyViewSettings sets the hidden files and the sort order of gofm.ini, when present.
func (p *Panel) applyViewSettings(s settings) {
	if s.showHidden != "" {
		p.showHidden = s.showHidden == "yes"
	}
	if s.sortBy != "" {
		for i, l := range sortLabels {
			if l == s.sortBy {
				p.sortMode, p.sortAsc = i, defaultAsc(i)
			}
		}
	}
}

// reloadSettings (Alt+R) reads gofm.ini again. Only the settings whose value changed
// in the file are applied, so a refresh keeps what was changed with the keys.
func (fm *FM) reloadSettings() {
	cfg, problem := loadSettings(findStorage())
	old := fm.cfg
	fm.cfg = cfg
	changed := false
	if cfg.colorScheme != old.colorScheme && cfg.colorScheme != "" {
		applyScheme(schemeIndex(cfg.colorScheme))
		fm.recolor()
		changed = true
	}
	if cfg.icons != old.icons && cfg.icons != "" {
		fm.basicIcons = cfg.icons == "basic"
		changed = true
	}
	view := settings{}
	if cfg.showHidden != old.showHidden {
		view.showHidden = cfg.showHidden
	}
	if cfg.sortBy != old.sortBy {
		view.sortBy = cfg.sortBy
	}
	for _, p := range []*Panel{fm.left, fm.right} {
		p.applyViewSettings(view)
	}
	changed = changed || view.showHidden != "" || view.sortBy != "" || cfg != old
	if changed {
		fm.saveState()
	}
	switch {
	case problem != "":
		fm.notice = problem
	case changed:
		fm.notice = "Settings reloaded from " + configName
	}
}

// toggleIcons (Alt+I) switches between Nerd Font icons and basic symbols. The
// choice is remembered in state.json.
func (fm *FM) toggleIcons() {
	fm.basicIcons = !fm.basicIcons
	for _, p := range []*Panel{fm.left, fm.right} {
		p.render()
	}
	if fm.basicIcons {
		fm.notice = "Icons: basic symbols — Alt+I for Nerd Font icons"
	} else {
		fm.notice = "Icons: Nerd Font (needs e.g. Cascadia Mono NF) — Alt+I for basic symbols"
	}
	fm.saveState()
	fm.updateStatus()
}

// cycleScheme (Ctrl+F9) switches to the next colour scheme and remembers it.
func (fm *FM) cycleScheme() {
	applyScheme((curScheme + 1) % len(schemes))
	fm.recolor()
	fm.notice = "Colours: " + schemes[curScheme].name + " — Ctrl+F9 for the next scheme"
	fm.saveState()
	fm.updateStatus()
}

// recolor applies the current scheme to the primitives that already exist; dialogs
// opened later take it from tview's defaults.
func (fm *FM) recolor() {
	for _, b := range []interface{ SetBackgroundColor(tcell.Color) *tview.Box }{fm.pages, fm.root, fm.panelsBox} {
		if b != nil {
			b.SetBackgroundColor(colBackground)
		}
	}
	if fm.status != nil {
		fm.status.SetBackgroundColor(colBarBg)
	}
	if fm.keybar != nil {
		fm.keybar.SetBackgroundColor(colBarBg)
	}
	for _, p := range []*Panel{fm.left, fm.right} {
		p.table.SetBackgroundColor(colBackground)
		p.table.SetTitleColor(colTitle)
		p.render()
	}
	fm.setActive(fm.active, false)
}

func (fm *FM) other() *Panel {
	if fm.active == fm.left {
		return fm.right
	}
	return fm.left
}

// setActive highlights the active panel; focus=true also moves keyboard focus.
func (fm *FM) setActive(p *Panel, focus bool) {
	if fm.active != p {
		fm.resetQuickSearch()
	}
	fm.active = p
	for _, q := range []*Panel{fm.left, fm.right} {
		if q == p {
			q.table.SetBorderColor(colActiveBorder)
			q.table.SetSelectedStyle(tcell.StyleDefault.Background(colCursorBg).Foreground(colCursorFg))
		} else {
			q.table.SetBorderColor(colPanelBorder)
			q.table.SetSelectedStyle(tcell.StyleDefault.Background(colCursorOffBg).Foreground(colCursorOffFg))
		}
	}
	if focus {
		fm.app.SetFocus(p.table)
	}
	fm.updateStatus()
}

// updateStatus is called whenever what the status bar shows may have changed. The
// bar reads the current state when it is drawn, which tview does after every event
// and every QueueUpdateDraw, so nothing has to be stored here.
func (fm *FM) updateStatus() {}

func plural(n int64, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func countReal(es []entry) int {
	n := 0
	for _, e := range es {
		if !e.isUp {
			n++
		}
	}
	return n
}

// ---------- keyboard ----------

// ctrlLetter recognises Ctrl+A…Z. tcell reports it as KeyCtrlA…Z (classic consoles)
// or as the letter key with ModCtrl (Windows Terminal's win32-input-mode, which also
// tells Ctrl+H/I/M apart from Backspace/Tab/Enter).
func ctrlLetter(ev *tcell.EventKey) (byte, bool) {
	k, mod := ev.Key(), ev.Modifiers()
	if mod&tcell.ModCtrl == 0 || mod&tcell.ModAlt != 0 {
		return 0, false
	}
	switch {
	case k >= 'A' && k <= 'Z':
		return byte(k), true
	case k >= tcell.KeyCtrlA && k <= tcell.KeyCtrlZ:
		return byte('A' + k - tcell.KeyCtrlA), true
	case k == tcell.KeyRune && ev.Rune() < 128 && unicode.IsLetter(ev.Rune()):
		return byte(unicode.ToUpper(ev.Rune())), true
	}
	return 0, false
}

// driveKey recognises Shift+A…Z, which arrives as an upper-case ASCII letter.
func driveKey(r rune) (byte, bool) {
	if r >= 'A' && r <= 'Z' {
		return byte(r), true
	}
	return 0, false
}

// altLetter recognises Alt+<letter> (without Ctrl: Ctrl+Alt is AltGr on European keyboards).
func altLetter(ev *tcell.EventKey) (rune, bool) {
	if ev.Key() != tcell.KeyRune || ev.Modifiers()&tcell.ModAlt == 0 || ev.Modifiers()&tcell.ModCtrl != 0 {
		return 0, false
	}
	return unicode.ToLower(ev.Rune()), true
}

func (fm *FM) resetQuickSearch() {
	if fm.qs != "" || fm.qsMiss {
		fm.qs, fm.qsMiss = "", false
		fm.updateStatus()
	}
}

// quickSearch adds r to the typed text and jumps to the first matching name.
func (fm *FM) quickSearch(r rune) {
	if time.Since(fm.qsTime) > quickSearchTimeout {
		fm.qs = ""
	}
	fm.qsTime = time.Now()
	if fm.active.jumpTo(fm.qs + string(r)) {
		fm.qs += string(r)
		fm.qsMiss = false
	} else {
		fm.qsMiss = true // keep the last match; the extra character is ignored
	}
	fm.updateStatus()
}

func (fm *FM) keys(ev *tcell.EventKey) *tcell.EventKey {
	if name, _ := fm.pages.GetFrontPage(); name != "main" {
		return ev
	}
	p := fm.active
	if fm.notice != "" {
		fm.notice = "" // shown until the next key
		fm.updateStatus()
	}

	// Ctrl+Right in the left panel / Ctrl+Left in the right panel: show this location
	// on the other side (the arrow points to the panel that changes).
	if ev.Modifiers()&tcell.ModCtrl != 0 && (ev.Key() == tcell.KeyRight || ev.Key() == tcell.KeyLeft) {
		fm.resetQuickSearch()
		if (ev.Key() == tcell.KeyRight && p == fm.left) || (ev.Key() == tcell.KeyLeft && p == fm.right) {
			fm.syncOther()
		}
		return nil
	}

	// Ctrl+B: flat view; Ctrl+A: mark all, or invert the marks. Other Ctrl+letters are
	// not used (drives moved to Shift+letter).
	if l, ok := ctrlLetter(ev); ok {
		fm.resetQuickSearch()
		switch l {
		case 'B':
			fm.toggleFlat()
		case 'A':
			p.markAllOrInvert()
		}
		return nil
	}
	// Function keys with Ctrl or Alt (v1.12). Any other combination does nothing, so
	// that e.g. Ctrl+F7 never falls through to F7.
	if ev.Key() >= tcell.KeyF1 && ev.Key() <= tcell.KeyF64 && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) != 0 {
		fm.resetQuickSearch()
		ctrl, alt := ev.Modifiers()&tcell.ModCtrl != 0, ev.Modifiers()&tcell.ModAlt != 0
		switch {
		case alt && !ctrl && ev.Key() == tcell.KeyF7:
			fm.searchDialog()
		case ctrl && !alt && ev.Key() == tcell.KeyF7:
			fm.filterDialog()
		case ctrl && !alt && ev.Key() == tcell.KeyF2:
			fm.showKeys()
		case ctrl && !alt && ev.Key() == tcell.KeyF9:
			fm.cycleScheme()
		}
		return nil
	}
	// Alt+<letter>: commands.
	if r, ok := altLetter(ev); ok {
		fm.resetQuickSearch()
		switch r {
		case 'u':
			fm.extract()
		case 'l':
			fm.gotoDialog()
		case 'v':
			if len(listDrives()) == 0 {
				fm.showMessage("Drives", "The drive list is available on Windows only.")
			} else {
				sel := ""
				if l := driveLetter(p.dir); l != 0 {
					sel = string(l) + `:\`
				}
				p.chdir("", sel)
			}
		case 'r':
			fm.reloadSettings() // gofm.ini (v1.13), before the panels are read again
			for _, q := range []*Panel{fm.left, fm.right} {
				q.resetSizes() // a manual refresh also re-measures marked folders
			}
			fm.refreshAll()
		case 'o':
			p.sortMode = (p.sortMode + 1) % len(sortLabels)
			p.sortAsc = defaultAsc(p.sortMode)
			p.reload()
		case 'd':
			p.sortAsc = !p.sortAsc
			p.reload()
		case 'h':
			p.showHidden = !p.showHidden
			p.reload()
		case 'e':
			fm.syncOther()
		case 'i':
			fm.toggleIcons()
		case 'q':
			fm.app.Stop()
		}
		return nil
	}

	// Plain typing: quick search. Space marks unless a search is in progress.
	if ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModAlt|tcell.ModCtrl) == 0 {
		r := ev.Rune()
		if r == ' ' && (fm.qs == "" || time.Since(fm.qsTime) > quickSearchTimeout) {
			fm.resetQuickSearch()
			p.toggleMark()
			return nil
		}
		// Shift+letter switches drive. The terminal delivers it as the upper-case letter
		// (it cannot be told apart from Caps Lock), so quick search uses lower case.
		if l, ok := driveKey(r); ok {
			fm.resetQuickSearch()
			fm.gotoDrive(l)
			return nil
		}
		if unicode.IsPrint(r) {
			fm.quickSearch(r)
		}
		return nil
	}

	switch ev.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if fm.qs != "" {
			rs := []rune(fm.qs)
			fm.qs = string(rs[:len(rs)-1])
			fm.qsTime, fm.qsMiss = time.Now(), false
			if fm.qs != "" {
				p.jumpTo(fm.qs)
			}
			fm.updateStatus()
			return nil
		}
		fm.resetQuickSearch()
		p.goParent()
		return nil
	case tcell.KeyEscape:
		switch {
		case fm.qs != "" || fm.qsMiss:
			fm.resetQuickSearch()
		case p.filter != "":
			p.filter = ""
			p.reload()
		case len(p.marked) > 0:
			p.marked = map[string]bool{}
			p.render()
		}
		fm.updateStatus()
		return nil
	}

	// Every other key ends the quick search.
	fm.resetQuickSearch()
	switch ev.Key() {
	case tcell.KeyTab, tcell.KeyBacktab:
		fm.setActive(fm.other(), true)
	case tcell.KeyLeft:
		p.goParent()
	case tcell.KeyRight:
		p.enterDir()
	case tcell.KeyF12:
		fm.compareFolders()
	case tcell.KeyF2:
		fm.rename()
	case tcell.KeyF3:
		fm.preview()
	case tcell.KeyF5:
		fm.copyOrMove(false)
	case tcell.KeyF6:
		fm.copyOrMove(true)
	case tcell.KeyF7:
		fm.mkdir()
	case tcell.KeyDelete:
		fm.trash()
	case tcell.KeyF4:
		fm.editFile()
	case tcell.KeyF9:
		fm.zip()
	case tcell.KeyF10:
		fm.app.Stop()
	case tcell.KeyInsert:
		p.toggleMark()
	default:
		return ev
	}
	return nil
}

// compareFolders (F12) compares the files listed in the two panels by name, size and
// date, and marks on each side what is worth copying to the other: the files missing on
// the other side and, for files on both sides, the newer copy. Same date but different
// size marks both copies (it cannot tell which one is right). Folders are ignored; the
// comparison uses what the panels list (filter and hidden files setting included).
// Existing marks are replaced.
func (fm *FM) compareFolders() {
	fm.resetQuickSearch()
	for _, p := range []*Panel{fm.left, fm.right} {
		if p.inDriveList() || p.inShareList() {
			fm.showMessage("Compare", "Both panels must show a folder (or the inside of an archive).")
			return
		}
	}
	r := compareSides(fm.left.entries, fm.right.entries, onWindows)
	fm.left.setMarks(r.markL)
	fm.right.setMarks(r.markR)
	fm.notice = r.summary()
	fm.updateStatus()
}

// dateTolerance: FAT/exFAT and many NAS store times with a 2-second resolution.
const dateTolerance = 2 * time.Second

type compareResult struct {
	markL, markR   []string // names to mark on each side
	onlyL, onlyR   int      // files missing on the other side
	newerL, newerR int      // files on both sides, newer here
	sizeOnly       int      // same date (±2 s), different size: marked on both sides
}

func (r compareResult) summary() string {
	if len(r.markL)+len(r.markR) == 0 {
		return "Compare: same files on both sides (name, size, date ±2 s)"
	}
	s := fmt.Sprintf("Compare: left %d only here, %d newer · right %d only here, %d newer",
		r.onlyL, r.newerL, r.onlyR, r.newerR)
	if r.sizeOnly > 0 {
		s += fmt.Sprintf(" · %d same date, different size", r.sizeOnly)
	}
	return s
}

// compareSides compares files (not folders) by name; foldCase makes the name match
// case-insensitive, as file names are on Windows. An unknown date (zero) compares by
// size only.
func compareSides(left, right []entry, foldCase bool) (r compareResult) {
	key := func(s string) string {
		if foldCase {
			return strings.ToLower(s)
		}
		return s
	}
	isFile := func(e entry) bool { return !e.isDir && !e.isUp && !e.isDrive && !e.isShare }
	index := func(es []entry) map[string]entry {
		m := map[string]entry{}
		for _, e := range es {
			if isFile(e) {
				m[key(e.name)] = e
			}
		}
		return m
	}
	L, R := index(left), index(right)
	for _, e := range left {
		if !isFile(e) {
			continue
		}
		o, ok := R[key(e.name)]
		switch {
		case !ok:
			r.onlyL++
			r.markL = append(r.markL, e.name)
		case newer(e, o):
			r.newerL++
			r.markL = append(r.markL, e.name)
		case newer(o, e):
			r.newerR++
			r.markR = append(r.markR, o.name)
		case e.size != o.size:
			r.sizeOnly++
			r.markL = append(r.markL, e.name)
			r.markR = append(r.markR, o.name)
		}
	}
	for _, e := range right {
		if isFile(e) {
			if _, ok := L[key(e.name)]; !ok {
				r.onlyR++
				r.markR = append(r.markR, e.name)
			}
		}
	}
	return r
}

// newer reports whether a was modified more than dateTolerance after b.
func newer(a, b entry) bool {
	if a.mod.IsZero() || b.mod.IsZero() {
		return false
	}
	return a.mod.Sub(b.mod) > dateTolerance
}

func (fm *FM) gotoDrive(l byte) {
	p := fm.active
	drives := listDrives()
	if len(drives) == 0 {
		fm.showMessage("Drives", "Shift+<letter> switches drive on Windows only.\nType lower-case letters to search by name.")
		return
	}
	root := string(l) + `:\`
	found := false
	for _, d := range drives {
		if strings.EqualFold(d, root) {
			found = true
		}
	}
	if !found {
		fm.showMessage("Drives", "Drive "+string(l)+": does not exist.")
		return
	}
	target := root
	if last, ok := fm.lastDir[l]; ok {
		if st, err := os.Stat(last); err == nil && st.IsDir() {
			target = last
		}
	}
	p.chdir(target, "")
}

// syncOther shows the active panel's location in the other panel.
func (fm *FM) syncOther() {
	p, o := fm.active, fm.other()
	sel := ""
	if e := p.current(); e != nil && p.arch == nil {
		sel = e.name // same cursor position, when the item exists on both sides
	}
	if p.arch != nil {
		sel = filepath.Base(p.arch.path)
	}
	o.chdir(p.dir, sel)
	if p.arch != nil {
		o.openArchive(p.arch.path)
		if o.arch != nil {
			o.arch.inner = p.arch.inner
			_ = o.load("")
		}
	}
}

// ---------- dialogs ----------

func (fm *FM) openModal(prim tview.Primitive, w, h int, focus tview.Primitive) string {
	fm.seq++
	name := fmt.Sprintf("modal%d", fm.seq)
	var item tview.Primitive = prim
	if w > 0 {
		item = tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(prim, h, 0, true).
				AddItem(nil, 0, 1, false), w, 0, true).
			AddItem(nil, 0, 1, false)
	}
	fm.pages.AddPage(name, item, true, true)
	fm.modals = append(fm.modals, name)
	if focus == nil {
		focus = prim
	}
	fm.focus[name] = focus
	fm.app.SetFocus(focus)
	return name
}

func (fm *FM) closeModal(name string) {
	if _, ok := fm.focus[name]; !ok {
		return // already closed
	}
	delete(fm.focus, name)
	// RemovePage gives the focus back to the main page, whose layout focuses its first
	// item, the LEFT panel, and that panel's focus handler makes it the active one.
	// Remember the active panel to restore it (bug of 1.9 and earlier: closing any
	// dialog opened from the right panel moved the focus to the left one).
	active := fm.active
	fm.pages.RemovePage(name)
	for i, n := range fm.modals {
		if n == name {
			fm.modals = append(fm.modals[:i], fm.modals[i+1:]...)
			break
		}
	}
	if len(fm.modals) == 0 {
		if active != nil {
			fm.setActive(active, true)
		}
	} else {
		fm.app.SetFocus(fm.focus[fm.modals[len(fm.modals)-1]])
	}
}

func (fm *FM) showMessage(title, text string) {
	var name string
	m := tview.NewModal().SetText(tview.Escape(text)).AddButtons([]string{"OK"}).
		SetDoneFunc(func(int, string) { fm.closeModal(name) })
	m.SetTitle(" " + title + " ")
	name = fm.openModal(m, 0, 0, nil)
}

func (fm *FM) confirm(text string, buttons []string, cb func(string)) {
	fm.confirmDefault(text, buttons, 0, cb)
}

// confirmDefault is confirm with the button selected at first (the one Enter presses).
func (fm *FM) confirmDefault(text string, buttons []string, def int, cb func(string)) {
	var name string
	m := tview.NewModal().SetText(tview.Escape(text)).AddButtons(buttons).SetFocus(def).
		SetDoneFunc(func(_ int, label string) {
			fm.closeModal(name)
			cb(label) // empty label = Esc
		})
	name = fm.openModal(m, 0, 0, nil)
}

func (fm *FM) prompt(title, label, initial string, cb func(string)) {
	fm.promptEx(title, label, initial, false, cb)
}

func (fm *FM) promptEx(title, label, initial string, allowEmpty bool, cb func(string)) {
	var name string
	form := tview.NewForm()
	input := tview.NewInputField().SetLabel(tview.Escape(label)).SetText(initial).SetFieldWidth(0)
	closed := false
	accept := func() {
		if closed {
			return
		}
		closed = true
		v := strings.TrimSpace(input.GetText())
		fm.closeModal(name)
		if v != "" || allowEmpty {
			cb(v)
		}
	}
	// After DoneFunc the Form moves focus to its next button: defer the action until the
	// key has been handled, otherwise focus would land on a closed form.
	// (Esc is already handled by the Form's CancelFunc.)
	input.SetDoneFunc(func(k tcell.Key) {
		if k == tcell.KeyEnter {
			go fm.app.QueueUpdateDraw(accept)
		}
	})
	form.AddFormItem(input).
		AddButton("OK", accept).
		AddButton("Cancel", func() { fm.closeModal(name) }).
		SetCancelFunc(func() { fm.closeModal(name) })
	form.SetBorder(true).SetTitle(" " + title + " ")
	name = fm.openModal(form, 76, 7, nil)
}

// runTask runs fn in the background in a progress dialog with a Cancel button.
// When fn sets o.total, the dialog shows a progress bar, speed and time left.
func (fm *FM) runTask(title string, fn func(o *opCtx), done func(o *opCtx)) {
	o := &opCtx{}
	var name string
	text := tview.NewTextView().SetDynamicColors(false)
	form := tview.NewForm().SetButtonsAlign(tview.AlignCenter)
	cancel := func() {
		o.cancel.Store(true)
		text.SetText(title + "\n\nCancelling…")
	}
	form.AddButton("Cancel", cancel).SetCancelFunc(cancel)
	box := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(text, 0, 1, false).
		AddItem(form, 3, 0, true)
	box.SetBorder(true).SetTitle(" Working ")
	text.SetText(title + "\n\nStarting…")
	name = fm.openModal(box, 72, 12, form)

	stop := make(chan struct{})
	meter := &rateMeter{}
	go func() {
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				fm.app.QueueUpdateDraw(func() {
					if !o.cancelled() {
						text.SetText(progressText(title, o, meter, 66, time.Now()))
					}
				})
			}
		}
	}()
	go func() {
		if runtime.GOOS == "windows" {
			runtime.LockOSThread() // SHFileOperation may show windows: keep a fixed OS thread
			defer runtime.UnlockOSThread()
		}
		fn(o)
		close(stop)
		fm.app.QueueUpdateDraw(func() {
			fm.closeModal(name)
			fm.refreshAll()
			if done != nil {
				done(o)
			}
			fm.reportErrors(o)
		})
	}()
}

// rateMeter measures throughput over the last few seconds, so the estimate follows
// changes in speed (e.g. many small files, then one big one).
type rateMeter struct {
	samples []rateSample
}

type rateSample struct {
	t time.Time
	b int64
}

const rateWindow = 5 * time.Second

func (m *rateMeter) add(now time.Time, b int64) {
	m.samples = append(m.samples, rateSample{now, b})
	for len(m.samples) > 2 && now.Sub(m.samples[0].t) > rateWindow {
		m.samples = m.samples[1:]
	}
}

// rate returns bytes per second, or 0 while there is not enough data yet.
func (m *rateMeter) rate() float64 {
	if len(m.samples) < 2 {
		return 0
	}
	first, last := m.samples[0], m.samples[len(m.samples)-1]
	dt := last.t.Sub(first.t).Seconds()
	if dt < 0.75 {
		return 0
	}
	return float64(last.b-first.b) / dt
}

func progressText(title string, o *opCtx, m *rateMeter, width int, now time.Time) string {
	if ph := loadString(&o.phase); ph != "" {
		return title + "\n\n" + ph
	}
	cur := loadString(&o.cur)
	if r := []rune(cur); len(r) > width {
		cur = "…" + string(r[len(r)-width+1:])
	}
	total, done := o.total.Load(), o.bytes.Load()
	if total <= 0 {
		return fmt.Sprintf("%s\n\n%d files processed\n%s", title, o.files.Load(), cur)
	}
	if done > total {
		done = total
	}
	m.add(now, done)
	frac := float64(done) / float64(total)
	barW := width - 6
	filled := int(frac * float64(barW))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)
	speed, eta := "", "estimating time left…"
	if r := m.rate(); r > 0 {
		speed = humanSize(int64(r)) + "/s · "
		eta = formatDuration(time.Duration(float64(total-done)/r*float64(time.Second))) + " left"
	} else if done == total {
		eta = "finishing…"
	}
	return fmt.Sprintf("%s\n\n%s %3d%%\n%s of %s · %s%s\n\n%s",
		title, bar, int(frac*100), humanSize(done), humanSize(total), speed, eta, cur)
}

func formatDuration(d time.Duration) string {
	s := int(d.Round(time.Second).Seconds())
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func (fm *FM) reportErrors(o *opCtx) {
	if len(o.errs) == 0 {
		if o.cancelled() {
			fm.showMessage("Cancelled", "Operation cancelled.")
		}
		return
	}
	list := o.errs
	more := ""
	if len(list) > 12 {
		more = fmt.Sprintf("\n… and %d more errors", len(list)-12)
		list = list[:12]
	}
	fm.showMessage("Errors", fmt.Sprintf("%d errors:\n\n%s%s", len(o.errs), strings.Join(list, "\n"), more))
}

func (fm *FM) refreshAll() {
	fm.left.reload()
	fm.right.reload()
	fm.updateStatus()
}

// ---------- actions ----------

func describe(items []string) string {
	if len(items) == 1 {
		return "\"" + filepath.Base(filepath.FromSlash(items[0])) + "\""
	}
	return fmt.Sprintf("%d items", len(items))
}

// readOnlyHere explains why file operations are not possible in the current location.
func (fm *FM) readOnlyHere() bool {
	p := fm.active
	switch {
	case p.arch != nil:
		fm.showMessage("Archive", "Archives are read-only here.\nUse F5 or Alt+U to extract the selected items.")
	case p.inShareList():
		fm.showMessage("Shares", "Open a share first (→ or Enter).")
	case p.dir == "":
		fm.showMessage("Drives", "Open a drive first (→ or Enter).")
	default:
		return false
	}
	return true
}

// destFolder is the default destination: the other panel's folder, if it is a real one.
func (fm *FM) destFolder() (string, bool) {
	o := fm.other()
	if !o.onDisk() {
		return "", false
	}
	return o.dir, true
}

func (fm *FM) resolveDest(dest, base string) string {
	dest = expandPath(dest)
	if !filepath.IsAbs(dest) && !strings.HasPrefix(dest, `\\`) {
		dest = filepath.Join(base, dest)
	}
	return dest
}

// askSync asks a question from a background task and waits for the answer (the
// dialog runs on the interface goroutine). An empty answer means Esc.
func (fm *FM) askSync(text string, buttons []string) string {
	answer := make(chan string, 1)
	fm.app.QueueUpdateDraw(func() {
		fm.confirm(text, buttons, func(choice string) { answer <- choice })
	})
	return <-answer
}

// askOverwrite is the single question of extractions: overwrite or skip every
// existing file.
func (fm *FM) askOverwrite(text string, run func(overwrite bool)) {
	fm.confirm(text, []string{"Overwrite", "Skip existing", "Cancel"}, func(choice string) {
		switch choice {
		case "Overwrite":
			run(true)
		case "Skip existing":
			run(false)
		}
	})
}

func (fm *FM) copyOrMove(move bool) {
	src := fm.active
	if src.arch != nil {
		if move {
			fm.showMessage("Archive", "Archives are read-only here: use F5 to extract instead of moving.")
			return
		}
		fm.extractSelection()
		return
	}
	if fm.readOnlyHere() {
		return
	}
	items := src.targets()
	if len(items) == 0 {
		return
	}
	verb := "Copy"
	if move {
		verb = "Move"
	}
	dest, ok := fm.destFolder()
	if !ok {
		fm.showMessage(verb, noDestMsg)
		return
	}
	func(dest string) {
		type pair = copyPair
		var pairs []pair
		if st, err := os.Stat(dest); err == nil && st.IsDir() {
			for _, it := range items {
				pairs = append(pairs, pair{from: it, to: filepath.Join(dest, filepath.Base(it))})
			}
		} else { // the other panel's folder was removed meanwhile
			fm.showMessage("Error", "Destination folder does not exist:\n"+dest)
			fm.other().reload()
			return
		}
		var targets []string
		for _, pr := range pairs {
			targets = append(targets, pr.to)
		}
		if dups := duplicateTargets(targets); len(dups) > 0 { // flat view: same name from different subfolders
			fm.showMessage("Error", fmt.Sprintf("These names are selected more than once (from different subfolders)\n"+
				"and would overwrite each other in one folder:\n%s\n\nUnmark the duplicates and try again.",
				strings.Join(dups[:min(len(dups), 8)], "\n")))
			return
		}
		for _, pr := range pairs {
			if samePath(pr.from, pr.to) {
				fm.showMessage("Error", "Source and destination are the same:\n"+pr.from)
				return
			}
			if fi, err := os.Stat(pr.from); err == nil && fi.IsDir() && isWithin(pr.from, pr.to) {
				fm.showMessage("Error", "Cannot copy a folder into itself:\n"+pr.from)
				return
			}
			if fi, err := os.Lstat(pr.to); err == nil {
				sfi, _ := os.Lstat(pr.from)
				if sfi != nil && sfi.IsDir() != fi.IsDir() {
					fm.showMessage("Error", "An item of a different type with the same name already exists:\n"+pr.to)
					return
				}
			}
		}
		start := func() {
			fm.runTask(verb+" "+describe(items), func(o *opCtx) {
				o.setPhase("Counting files…")
				var total int64
				for i := range pairs {
					pairs[i].size = o.treeSize(pairs[i].from)
					total += pairs[i].size
				}
				o.total.Store(total)
				// Files that already exist at the destination, also inside folders that
				// will be merged: one question each (v1.12), until an "all" answer.
				o.setPhase("Checking the destination…")
				conflicts := o.findConflicts(pairs)
				if o.cancelled() {
					return
				}
				skip, ok := resolveConflicts(conflicts, func(i int) string {
					return fm.askSync(conflictText(conflicts[i], i, len(conflicts), dest), conflictButtons)
				})
				if !ok {
					o.cancel.Store(true)
					return
				}
				o.skip = skip
				o.setPhase("")
				// One shared engine for all the items: small files are copied in parallel
				// across the whole selection, big files one at a time. Skipped files stay
				// where they are; the others go on.
				if move {
					o.moveAll(pairs)
				} else {
					o.copyAll(pairs)
				}
			}, func(*opCtx) { src.marked = map[string]bool{}; src.reload() })
		}
		// Folders with the same name at the destination (v1.13): merge them, or cancel
		// the whole operation. After Merge, existing files are asked one by one.
		var srcDirs, dstDirs []string
		for _, pr := range pairs {
			srcDirs, dstDirs = append(srcDirs, pr.from), append(dstDirs, pr.to)
		}
		fm.askMerge(sameNameFolders(srcDirs, dstDirs), dest, "Files that exist in both are asked about one by one.", start)
	}(dest)
}

// sameNameFolders returns the destinations that already exist as folders, for the
// sources that are folders (links are not folders here: they are copied as links).
func sameNameFolders(from, to []string) []string {
	var out []string
	for i := range from {
		if sfi, err := os.Lstat(from[i]); err != nil || !sfi.IsDir() {
			continue
		}
		if dfi, err := os.Lstat(to[i]); err == nil && dfi.IsDir() {
			out = append(out, to[i])
		}
	}
	return out
}

// mergeText is the question asked when folders with the same name already exist.
func mergeText(folders []string, dest, then string) string {
	var b strings.Builder
	if len(folders) == 1 {
		b.WriteString("A folder with the same name already exists in\n" + dest + ":\n\n")
	} else {
		fmt.Fprintf(&b, "%d folders with the same name already exist in\n%s:\n\n", len(folders), dest)
	}
	for i, f := range folders {
		if i == maxWarnNames {
			fmt.Fprintf(&b, "… and %d more\n", len(folders)-maxWarnNames)
			break
		}
		b.WriteString(filepath.Base(f) + "\n")
	}
	b.WriteString("\nMerge the contents? " + then + "\nCancel stops the whole operation.")
	return b.String()
}

// askMerge runs start at once when no folder exists already, otherwise after Merge;
// Cancel or Esc abort the operation.
func (fm *FM) askMerge(folders []string, dest, then string, start func()) {
	if len(folders) == 0 {
		start()
		return
	}
	fm.confirm(mergeText(folders, dest, then), []string{"Merge", "Cancel"}, func(choice string) {
		if choice == "Merge" {
			start()
		}
	})
}

// noDestMsg explains why F5/F6 cannot run: they always target the other panel's folder.
const noDestMsg = "The destination is the other panel:\nopen a folder on disk there first."

func (fm *FM) rename() {
	if fm.readOnlyHere() {
		return
	}
	p := fm.active
	e := p.current()
	if e == nil || e.isUp {
		return
	}
	old := p.fullPath(e.name)
	fm.prompt("Rename", "New name: ", filepath.Base(e.name), func(name string) {
		if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
			fm.showMessage("Error", "The name cannot contain \\ or / (use F6 to move).")
			return
		}
		if onWindows && strings.ContainsAny(name, `:*?"<>|`) {
			fm.showMessage("Error", "Windows does not allow : * ? \" < > | in names.")
			return
		}
		newRel := filepath.Join(filepath.Dir(e.name), name) // flat view: stays in its subfolder
		nw := p.fullPath(newRel)
		if exists(nw) && !samePath(old, nw) {
			fm.showMessage("Error", "An item named \""+name+"\" already exists.")
			return
		}
		if err := os.Rename(old, nw); err != nil {
			fm.showMessage("Error", err.Error())
			return
		}
		delete(p.marked, e.name)
		if p.flat {
			for i := range p.flatEntries {
				if p.flatEntries[i].name == e.name {
					p.flatEntries[i].name = newRel
				}
			}
		}
		_ = p.load(newRel)
		fm.other().reload()
	})
}

func (fm *FM) mkdir() {
	if fm.readOnlyHere() {
		return
	}
	p := fm.active
	fm.prompt("New folder", "Name: ", "", func(name string) {
		path := fm.resolveDest(name, p.dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			fm.showMessage("Error", err.Error())
			return
		}
		first := strings.Split(filepath.ToSlash(name), "/")[0]
		_ = p.load(first)
		fm.other().reload()
	})
}

// trash is the Del key. On local drives the items go to the Recycle Bin; on network
// drives (UNC paths and mapped drives) they are deleted permanently, after an explicit
// warning (v1.12): Windows would delete them permanently anyway, since network
// folders have no Recycle Bin. F8 no longer deletes (v1.12).
// v1.13: the button selected in the confirmations (Enter) is the delete button, or
// Cancel, as set in gofm.ini (deleteDefault, networkDeleteDefault); networkDelete=off
// disables Del on network drives.
func (fm *FM) trash() {
	if fm.readOnlyHere() {
		return
	}
	p := fm.active
	items := p.targets()
	if len(items) == 0 {
		return
	}
	network := isNetworkLocation(p.dir)
	if network && fm.cfg.networkDeleteMode() == "off" {
		fm.showMessage("Delete", "Deleting on network drives is turned off\n(networkDelete = off in "+configName+").")
		return
	}
	run := func() {
		if network {
			fm.runTask("Deleting "+describe(items)+" permanently", func(o *opCtx) {
				o.deleteAll(items)
			}, func(*opCtx) { p.marked = map[string]bool{}; p.reload() })
			return
		}
		fm.runTask("Moving "+describe(items)+" to the Recycle Bin", func(o *opCtx) {
			o.setPhase("Waiting for Windows…")
			if err := moveToTrash(items); err != nil {
				o.fail("Recycle Bin", err)
			}
		}, func(*opCtx) { p.marked = map[string]bool{}; p.reload() })
	}
	text, buttons := deleteQuestion(items, network)
	yes, deepYes := buttons[0], deepButton(network)
	def := deleteDefaultIndex(fm.cfg, network)
	fm.confirmDefault(text, buttons, def, func(choice string) {
		if choice != yes {
			return
		}
		// Folders with subfolders need a second confirmation. Reading a folder on a
		// slow share must not freeze the interface: check in the background.
		go func() {
			deep := foldersWithSubfolders(items)
			fm.app.QueueUpdateDraw(func() {
				if len(deep) == 0 {
					run()
					return
				}
				fm.confirmDefault(subfolderWarning(deep, network), []string{deepYes, "Cancel"}, def, func(choice string) {
					if choice == deepYes {
						run()
					}
				})
			})
		}()
	})
}

// deleteQuestion is the first confirmation of Del: the delete button comes first,
// Cancel/No second.
func deleteQuestion(items []string, network bool) (string, []string) {
	if network {
		return "Delete " + describe(items) + " PERMANENTLY?\n\nThis is a network drive: the Recycle Bin is not used,\nthe deleted items cannot be restored.",
			[]string{"Delete", "Cancel"}
	}
	return "Move " + describe(items) + " to the Recycle Bin?", []string{"Yes", "No"}
}

// deepButton is the delete button of the second confirmation (folders with subfolders).
func deepButton(network bool) string {
	if network {
		return "Delete permanently"
	}
	return "Move to Recycle Bin"
}

// deleteDefaultIndex is the button selected at first in the delete confirmations:
// 0 = delete (default), 1 = Cancel/No.
func deleteDefaultIndex(s settings, network bool) int {
	if s.deleteDefaultButton(network) == "cancel" {
		return 1
	}
	return 0
}

// foldersWithSubfolders returns the items that are folders (not links) containing at
// least one subfolder. Each folder is read only until the first subfolder is found.
func foldersWithSubfolders(items []string) []string {
	var deep []string
	for _, it := range items {
		if st, err := os.Lstat(it); err != nil || !st.IsDir() {
			continue
		}
		if hasSubfolder(it) {
			deep = append(deep, it)
		}
	}
	return deep
}

func hasSubfolder(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	for {
		list, err := f.ReadDir(256)
		for _, de := range list {
			if de.IsDir() {
				return true
			}
		}
		if err != nil || len(list) == 0 {
			return false
		}
	}
}

// maxWarnNames limits the folder names listed in the subfolder warning.
const maxWarnNames = 8

// subfolderWarning is the text of the second confirmation of F8.
func subfolderWarning(deep []string, permanent bool) string {
	var b strings.Builder
	if len(deep) == 1 {
		b.WriteString("WARNING: this folder contains subfolders:\n\n")
	} else {
		fmt.Fprintf(&b, "WARNING: %d folders contain subfolders:\n\n", len(deep))
	}
	for i, d := range deep {
		if i == maxWarnNames {
			fmt.Fprintf(&b, "… and %d more\n", len(deep)-maxWarnNames)
			break
		}
		b.WriteString(filepath.Base(d) + "\n")
	}
	if permanent {
		b.WriteString("\nThey will be deleted PERMANENTLY with everything inside them.")
	} else {
		b.WriteString("\nThey will go to the Recycle Bin with everything inside them.")
	}
	return b.String()
}

func (fm *FM) zip() {
	if fm.readOnlyHere() {
		return
	}
	p := fm.active
	items := p.targets()
	if len(items) == 0 {
		return
	}
	base := filepath.Base(p.dir)
	if len(items) == 1 {
		base = strings.TrimSuffix(filepath.Base(items[0]), filepath.Ext(items[0]))
	}
	destDir, ok := fm.destFolder()
	if !ok {
		destDir = p.dir
	}
	fm.prompt("Compress to ZIP", "Archive: ", filepath.Join(destDir, base+".zip"), func(zipPath string) {
		zipPath = fm.resolveDest(zipPath, p.dir)
		if !strings.EqualFold(filepath.Ext(zipPath), ".zip") {
			zipPath += ".zip"
		}
		run := func() {
			fm.runTask("Compressing "+describe(items), func(o *opCtx) {
				o.setPhase("Counting files…")
				var total int64
				for _, it := range items {
					total += o.treeSize(it)
				}
				o.total.Store(total)
				o.setPhase("")
				o.zipPaths(items, p.dir, zipPath)
			}, nil)
		}
		if exists(zipPath) {
			fm.confirm("The archive already exists:\n"+zipPath+"\nReplace it?", []string{"Replace", "Cancel"}, func(c string) {
				if c == "Replace" {
					run()
				}
			})
			return
		}
		run()
	})
}

// extract (Alt+U): inside an archive, extracts the selection; on disk, extracts the
// whole archive under the cursor.
func (fm *FM) extract() {
	p := fm.active
	if p.arch != nil {
		fm.extractSelection()
		return
	}
	e := p.current()
	if e == nil || e.isDir || !p.onDisk() || !isArchive(e.name) {
		fm.showMessage("Extract", "Put the cursor on a .zip or .7z file\n(or press → to browse it and pick files).")
		return
	}
	archPath := p.fullPath(e.name)
	destDir, ok := fm.destFolder()
	if !ok {
		destDir = p.dir
	}
	// If the archive already has a single root folder, extract right here (no "project\project").
	def := destDir
	if !archiveSingleRoot(archPath) {
		def = filepath.Join(destDir, strings.TrimSuffix(e.name, filepath.Ext(e.name)))
	}
	fm.prompt("Extract "+archiveKind(e.name), "Extract to: ", def, func(dest string) {
		dest = fm.resolveDest(dest, p.dir)
		run := func(overwrite bool) {
			fm.runTask("Extracting "+e.name, func(o *opCtx) { o.extract(archPath, dest, overwrite) }, nil)
		}
		// Folders with the same name (v1.13): the destination itself, when it is a new
		// folder name that already exists, and the archive's top folders found there.
		var folders []string
		if des, err := os.ReadDir(dest); err == nil && len(des) > 0 && !samePath(dest, destDir) {
			folders = append(folders, dest)
		}
		for _, d := range archiveTopFolders(archPath) {
			if st, err := os.Lstat(filepath.Join(dest, d)); err == nil && st.IsDir() {
				folders = append(folders, filepath.Join(dest, d))
			}
		}
		fm.askMerge(folders, filepath.Dir(dest), "You will then be asked about existing files.", func() {
			if des, err := os.ReadDir(dest); err == nil && len(des) > 0 {
				fm.askOverwrite("The destination folder is not empty.\nWhat about files that already exist?", run)
				return
			}
			run(false)
		})
	})
}

// extractSelection extracts the marked items (or the one under the cursor) of the archive being browsed.
func (fm *FM) extractSelection() {
	p := fm.active
	items := p.selection()
	if len(items) == 0 {
		return
	}
	archPath := p.arch.path
	dest, ok := fm.destFolder()
	if !ok {
		fm.showMessage("Extract", noDestMsg)
		return
	}
	func(dest string) {
		conflicts := 0
		for _, it := range items {
			if exists(filepath.Join(dest, filepath.Base(filepath.FromSlash(it)))) {
				conflicts++
			}
		}
		run := func(overwrite bool) {
			fm.runTask(fmt.Sprintf("Extracting %s from %s", describe(items), filepath.Base(archPath)), func(o *opCtx) {
				o.extractSelected(archPath, items, dest, overwrite)
			}, func(*opCtx) { p.marked = map[string]bool{}; p.render() })
		}
		// Folders of the selection that already exist at the destination (v1.13).
		chosen := map[string]bool{}
		for _, it := range items {
			chosen[it] = true
		}
		var folders []string
		for _, en := range p.entries {
			if !en.isDir || en.isUp || !chosen[p.fullPath(en.name)] {
				continue
			}
			to := filepath.Join(dest, filepath.Base(filepath.FromSlash(en.name)))
			if st, err := os.Lstat(to); err == nil && st.IsDir() {
				folders = append(folders, to)
			}
		}
		fm.askMerge(folders, dest, "You will then be asked about existing files.", func() {
			if conflicts == 0 {
				run(false)
				return
			}
			fm.askOverwrite(fmt.Sprintf("%d items already exist at the destination.\nWhat about files that already exist?", conflicts), run)
		})
	}(dest)
}

func (fm *FM) filterDialog() {
	p := fm.active
	fm.promptEx("Filter", "Show only (text or *.ext, empty = all): ", p.filter, true, func(f string) {
		p.filter = f
		p.reload()
		fm.updateStatus()
	})
}

func (fm *FM) gotoDialog() {
	p := fm.active
	initial := p.dir
	if p.arch != nil {
		initial = p.dir
	}
	fm.prompt("Go to", `Path (also \\server or \\server\share): `, initial, func(path string) {
		path = expandPath(path)
		if onWindows && isUNCServer(path) {
			p.chdir(path, "")
			return
		}
		if !filepath.IsAbs(path) && !strings.HasPrefix(path, `\\`) && p.dir != "" {
			path = filepath.Join(p.dir, path)
		}
		st, err := os.Stat(path)
		if err != nil {
			msg := err.Error()
			if strings.HasPrefix(path, `\\`) {
				msg += "\n\nIf the share needs a password, connect it once with\n\"net use \\\\server\\share\" or in Explorer, then retry."
			}
			fm.showMessage("Error", msg)
			return
		}
		if st.IsDir() {
			p.chdir(path, "")
		} else {
			p.chdir(filepath.Dir(path), filepath.Base(path))
		}
	})
}

func (fm *FM) searchDialog() {
	p := fm.active
	if !p.onDisk() {
		fm.showMessage("Find", "Search works in folders on disk.\nUse Ctrl+F7 to filter the current list.")
		return
	}
	var name string
	form := tview.NewForm()
	form.AddInputField("Name (text or *.ext): ", "", 0, nil, nil).
		AddInputField("Containing text (optional): ", "", 0, nil, nil).
		AddCheckbox("Include hidden: ", false, nil)
	started := false
	start := func() {
		if started {
			return
		}
		started = true
		namePat := strings.TrimSpace(form.GetFormItem(0).(*tview.InputField).GetText())
		content := form.GetFormItem(1).(*tview.InputField).GetText()
		hidden := form.GetFormItem(2).(*tview.Checkbox).IsChecked()
		fm.closeModal(name)
		if namePat == "" && content == "" {
			return
		}
		var hits []searchHit
		root := p.dir
		fm.runTask("Searching "+root, func(o *opCtx) {
			hits = o.search(root, namePat, content, hidden, 2000)
		}, func(o *opCtx) {
			fm.showResults(root, hits)
		})
	}
	for i := 0; i < 2; i++ { // Enter in a text field starts the search right away
		form.GetFormItem(i).(*tview.InputField).SetDoneFunc(func(k tcell.Key) {
			if k == tcell.KeyEnter {
				go fm.app.QueueUpdateDraw(start)
			}
		})
	}
	form.AddButton("Find", start).AddButton("Cancel", func() { fm.closeModal(name) }).
		SetCancelFunc(func() { fm.closeModal(name) })
	form.SetBorder(true).SetTitle(" Find files in " + tview.Escape(p.dir) + " ")
	name = fm.openModal(form, 76, 11, nil)
}

func (fm *FM) showResults(root string, hits []searchHit) {
	if len(hits) == 0 {
		fm.showMessage("Find", "No results.")
		return
	}
	var name string
	list := tview.NewList().ShowSecondaryText(false).SetHighlightFullLine(true)
	for _, h := range hits {
		h := h
		rel, err := filepath.Rel(root, h.path)
		if err != nil {
			rel = h.path
		}
		if h.isDir {
			rel += string(filepath.Separator)
		}
		list.AddItem(tview.Escape(rel), "", 0, func() {
			fm.closeModal(name)
			p := fm.active
			if !p.showHidden {
				if fi, err := os.Lstat(h.path); err == nil {
					if _, hid := fileAttrs(fi); hid || strings.HasPrefix(filepath.Base(h.path), ".") {
						p.showHidden = true
					}
				}
			}
			p.chdir(filepath.Dir(h.path), filepath.Base(h.path))
		})
	}
	list.SetDoneFunc(func() { fm.closeModal(name) })
	list.SetBorder(true).SetTitle(fmt.Sprintf(" %d results — Enter to go there, Esc to close ", len(hits)))
	name = fm.openModal(list, 0, 0, nil)
}

func (fm *FM) preview() {
	p := fm.active
	e := p.current()
	if e == nil || e.isUp || e.isDrive || e.isShare {
		return
	}
	var name string
	o := &opCtx{}
	tv := tview.NewTextView().SetDynamicColors(false).SetWrap(true).SetWordWrap(false).SetScrollable(true)
	tv.SetBorder(true).SetTitle(" " + tview.Escape(e.name) + " — Esc/F3 to close ")
	tv.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyF3 || (ev.Key() == tcell.KeyRune && ev.Rune() == 'q') {
			o.cancel.Store(true)
			fm.closeModal(name)
			return nil
		}
		return ev
	})

	if p.arch != nil {
		inner := p.fullPath(e.name)
		if e.isDir {
			n, size := 0, int64(0)
			for _, ae := range p.arch.entries {
				if strings.HasPrefix(cleanArchName(ae.name), inner+"/") && !ae.isDir {
					n++
					size += int64(ae.size)
				}
			}
			tv.SetText(fmt.Sprintf("Archive    : %s\nFolder     : %s\nContains   : %d files, %s (%d bytes)",
				p.arch.path, inner, n, humanSize(size), size))
		} else if ae, ok := findArchEntry(p.arch.entries, inner); ok {
			tv.SetText(previewArchEntry(p.arch.path, ae))
		}
		name = fm.openModal(tv, 0, 0, nil)
		return
	}

	path := p.fullPath(e.name)
	header, fi, err := previewHeader(path)
	if err != nil {
		fm.showMessage("Error", err.Error())
		return
	}
	if fi.IsDir() {
		tv.SetText(header + "Calculating contents…")
		go func() {
			files, dirs, size := dirStats(path, o)
			if o.cancelled() {
				return
			}
			fm.app.QueueUpdateDraw(func() {
				tv.SetText(header + fmt.Sprintf("Contains: %d files, %d folders\nTotal size: %s (%d bytes)",
					files, dirs, humanSize(size), size))
			})
		}()
	} else {
		tv.SetText(header + previewBody(path, fi))
	}
	name = fm.openModal(tv, 0, 0, nil)
}

// ---------- utilities ----------

var winEnv = regexp.MustCompile(`%([A-Za-z0-9_()]+)%`)

// expandPath expands ~, %VARIABLE% (Windows style) and $VARIABLE, and normalises UNC paths.
func expandPath(p string) string {
	p = strings.Trim(strings.TrimSpace(p), `"`)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			p = home + p[1:]
		}
	}
	p = winEnv.ReplaceAllStringFunc(p, func(m string) string {
		if v, ok := os.LookupEnv(m[1 : len(m)-1]); ok {
			return v
		}
		return m
	})
	if !onWindows {
		return filepath.Clean(os.ExpandEnv(p))
	}
	if _, _, _, ok := uncParts(p); ok {
		// filepath.Clean would mangle a bare \\server, so UNC paths are normalised by hand.
		return normalizeUNC(p)
	}
	if len(p) == 2 && p[1] == ':' {
		p += `\` // "D:" -> "D:\" (otherwise it means the drive's current folder)
	}
	return filepath.Clean(p)
}
