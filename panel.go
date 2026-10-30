package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// entry is one row of a panel.
type entry struct {
	name      string
	isDir     bool
	isLink    bool
	isUp      bool      // the ".." row
	isDrive   bool      // a drive (C:\, D:\ ...)
	drive     driveType // kind of drive, for isDrive rows
	isShare   bool      // a network share in a \\server listing
	inArchive bool      // an item inside a .zip/.7z being browsed
	hidden    bool
	size      int64
	mod       time.Time
	attrs     string
	remark    string
}

const (
	sortName = iota
	sortSize
	sortDate
)

var sortLabels = []string{"name", "size", "date"}

// defaultAsc is the direction a sort starts with when chosen with Alt+O: names A→Z,
// sizes and dates largest/newest first. Alt+D reverses it (v1.12).
func defaultAsc(mode int) bool { return mode == sortName }

// sortArrow is shown next to the sort type in the panel title: ↑ ascending (A→Z,
// small→large, old→new), ↓ descending.
func sortArrow(asc bool) string {
	if asc {
		return "↑"
	}
	return "↓"
}

// Columns of a row: mark ("*"), icon, name, size, date. tview separates columns
// with one space, so the mark and the icon take 4 cells and the name column gets
// width - leadW - sizeColW - dateColW - 2.
const (
	colMark  = 0
	colIcon  = 1
	colName  = 2
	colSize  = 3
	colDate  = 4
	leadW    = 4 // mark + space + icon + space
	sizeColW = 9
	dateColW = 11
)

// archView is the state of a panel that is browsing inside an archive.
type archView struct {
	path    string // the archive file
	entries []archEntry
	closer  io.Closer
	inner   string // current folder inside the archive, "" = root, slash-separated
}

// Panel is one of the two file panels.
type Panel struct {
	fm         *FM
	table      *panelView
	dir        string // "" = drive list; \\server = share list; else a folder (contains the archive when arch != nil)
	arch       *archView
	entries    []entry
	marked     map[string]bool
	filter     string
	showHidden bool
	sortMode   int
	sortAsc    bool                // direction of sortMode (Alt+D)
	width      int                 // inner width at the last render
	sizes      map[string]*dirSize // recursive totals of marked folders, by full path
	sizesLoc   string              // location the sizes belong to
	lastLoad   time.Duration       // how long the last listing took (paces auto-refresh)

	flat        bool    // flat view (Ctrl+B): all files of dir and its subfolders
	flatEntries []entry // the flat listing, names relative to dir
	flatTrunc   bool    // the listing stopped at maxFlatEntries
	flatRescan  *opCtx  // background rescan in progress

	diskFree  uint64    // space available on the volume (bottom border)
	diskTotal uint64    // size of the volume; 0 = unknown
	diskKey   string    // folder the space was measured for
	diskAt    time.Time // when it was measured
	diskBusy  bool
}

// panelView re-renders the rows when the panel width changes, so long names are
// shortened and the size and date columns always stay visible.
type panelView struct {
	*tview.Table
	p *Panel
}

func (v *panelView) Draw(screen tcell.Screen) {
	if _, _, w, _ := v.GetInnerRect(); w != v.p.width {
		v.p.width = w
		v.p.render()
	}
	v.p.styleCursorIcon()
	v.Table.Draw(screen)
	// Files of the folder and free space of the volume on the bottom border, right-aligned.
	files, size := v.p.folderTotals()
	x, y, w, h := v.GetRect()
	if d := bottomLabel(w-4, files, size, v.p.listsFiles(), v.p.diskFree, v.p.diskTotal); d != "" {
		lw := len([]rune(d))
		tview.Print(screen, tview.Escape(d), x+w-lw-2, y+h-1, lw, tview.AlignLeft, colText)
	}
	// Selection total on the top border, right-aligned, on the same line as the path.
	if label := v.p.selectionLabel(); label != "" {
		x, y, w, _ := v.GetRect()
		lw := len([]rune(label))
		if lw+4 <= w {
			tview.Print(screen, tview.Escape(label), x+w-lw-2, y, lw, tview.AlignLeft, colMarked)
		}
	}
}

// selectionLabel is the cumulative total of the marked items, e.g.
// " 3 selected · 1.9 MB · 3 files ", or "" when nothing is marked.
func (p *Panel) selectionLabel() string {
	n, files, size, pending := p.markedSummary()
	if n == 0 {
		return ""
	}
	s := fmt.Sprintf(" %d selected · %s · %s", n, humanSize(size), plural(files, "file"))
	if pending {
		s += " · counting…"
	}
	return s + " "
}

func newPanel(fm *FM, dir string) *Panel {
	p := &Panel{fm: fm, dir: dir, marked: map[string]bool{}, sizes: map[string]*dirSize{}, sortAsc: true}
	t := tview.NewTable().SetSelectable(true, false)
	t.SetBorder(true).SetTitleAlign(tview.AlignLeft)
	t.SetSelectedFunc(func(row, col int) { p.enter() })
	t.SetSelectionChangedFunc(func(row, col int) { fm.updateStatus() })
	t.SetFocusFunc(func() { fm.setActive(p, false) })
	p.table = &panelView{Table: t, p: p}
	return p
}

func (p *Panel) inShareList() bool { return p.arch == nil && onWindows && isUNCServer(p.dir) }
func (p *Panel) inDriveList() bool { return p.arch == nil && p.dir == "" }

// onDisk reports whether the panel shows a real folder (file operations are possible).
func (p *Panel) onDisk() bool { return p.arch == nil && p.dir != "" && !p.inShareList() }

// locKey identifies the current location (folder, archive folder, drive or share list).
func (p *Panel) locKey() string {
	if p.arch != nil {
		return p.arch.path + "|" + p.arch.inner
	}
	return p.dir
}

// load re-reads the location and tries to put the cursor on selectName.
func (p *Panel) load(selectName string) error {
	start := time.Now()
	defer func() { p.lastLoad = time.Since(start) }()
	var list []entry
	switch {
	case p.arch != nil:
		list = archChildren(p.arch.entries, p.arch.inner)
	case p.flat:
		list = append([]entry(nil), p.flatEntries...)
	case p.dir == "":
		for _, d := range listDrives() {
			list = append(list, entry{name: d, isDir: true, isDrive: true, drive: driveKind(d)})
		}
	case p.inShareList():
		shares, err := listShares(p.dir)
		if err != nil {
			return err
		}
		for _, s := range shares {
			if s.special && !p.showHidden {
				continue
			}
			list = append(list, entry{name: s.name, isDir: true, isShare: true, hidden: s.special, remark: s.remark})
		}
	default:
		des, err := os.ReadDir(p.dir)
		if err != nil {
			return err
		}
		for _, de := range des {
			info, err := de.Info()
			if err != nil {
				continue // file vanished meanwhile
			}
			e := entry{name: de.Name(), size: info.Size(), mod: info.ModTime(), isDir: info.IsDir()}
			if info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
				e.isLink = true
				if st, err := os.Stat(filepath.Join(p.dir, e.name)); err == nil && st.IsDir() {
					e.isDir = true
				}
			}
			var winHidden bool
			e.attrs, winHidden = fileAttrs(info)
			e.hidden = winHidden || strings.HasPrefix(e.name, ".")
			if e.hidden && !p.showHidden {
				continue
			}
			list = append(list, e)
		}
	}
	if p.filter != "" && !p.inDriveList() {
		kept := list[:0]
		for _, e := range list {
			if matchName(e.name, p.filter) {
				kept = append(kept, e)
			}
		}
		list = kept
	}
	p.sortEntries(list)
	if p.hasParent() {
		list = append([]entry{{name: "..", isDir: true, isUp: true}}, list...)
	}
	p.entries = list

	// Forget marks on entries that are no longer listed.
	keep := map[string]bool{}
	for _, e := range list {
		if p.marked[e.name] {
			keep[e.name] = true
		}
	}
	p.marked = keep
	p.followTree(p.sizesLoc != p.locKey())
	if p.sizesLoc != p.locKey() { // new location: forget folder totals
		p.resetSizes()
		p.sizesLoc = p.locKey()
	}
	for _, e := range list {
		if p.marked[e.name] {
			p.ensureSize(e)
		}
	}
	defer p.fm.syncWatches()
	defer p.updateDisk()
	defer p.fm.saveState()

	p.render()
	row := 0
	for i, e := range list {
		if strings.EqualFold(e.name, selectName) {
			row = i
			if e.name == selectName {
				break
			}
		}
	}
	p.table.Select(0, 0)
	p.table.ScrollToBeginning()
	p.table.Select(row, 0)
	return nil
}

// followTree keeps the tree view (Ctrl+T) in step with the right panel: a new folder
// there is shown in the tree; a re-read of the same folder re-reads its branch, so
// folders created or deleted in the right panel appear in the tree.
func (p *Panel) followTree(moved bool) {
	if p.fm == nil || p.fm.tree == nil || p != p.fm.right || !p.onDisk() || p.flat {
		return
	}
	t := p.fm.tree
	if moved {
		t.quiet = true
		t.reveal(p.dir)
		t.quiet = false
	} else {
		t.refreshPath(p.dir)
	}
}

// sortEntries sorts a listing: folders first, then by the panel's sort type and
// direction; equal keys fall back to the name, always A→Z.
func (p *Panel) sortEntries(list []entry) {
	sort.SliceStable(list, func(i, j int) bool {
		return entryLess(list[i], list[j], p.sortMode, p.sortAsc)
	})
}

func entryLess(a, b entry, mode int, asc bool) bool {
	if a.isUp != b.isUp {
		return a.isUp
	}
	if a.isDir != b.isDir {
		return a.isDir // folders first
	}
	byName := func() bool { return strings.ToLower(a.name) < strings.ToLower(b.name) }
	switch mode {
	case sortSize:
		if a.size != b.size {
			return (a.size < b.size) == asc
		}
	case sortDate:
		if !a.mod.Equal(b.mod) {
			return a.mod.Before(b.mod) == asc
		}
	default:
		if strings.ToLower(a.name) != strings.ToLower(b.name) {
			return byName() == asc
		}
	}
	return byName()
}

// styleCursorIcon gives the icon on the cursor row its colours. The table paints
// the whole cursor row in one text colour, which suits the active panel (several
// family colours are unreadable on its cyan cursor); in the inactive panel the
// icon keeps its family colour on the dark cursor.
func (p *Panel) styleCursorIcon() {
	row, _ := p.table.GetSelection()
	if row < 0 || row >= len(p.entries) || row >= p.table.GetRowCount() {
		return
	}
	st := tcell.StyleDefault.Background(colCursorOffBg).Foreground(iconColor(p.entries[row]))
	if p.fm != nil && p.fm.active == p {
		st = tcell.StyleDefault.Background(colCursorBg).Foreground(colCursorFg)
	}
	p.table.GetCell(row, colIcon).SetSelectedStyle(st)
}

func (p *Panel) render() {
	t := p.table
	row, _ := t.GetSelection()
	rowOff, colOff := t.GetOffset()
	t.Clear()
	nameW := 0 // 0 = no limit (width not known yet)
	if p.width > 0 {
		nameW = max(6, p.width-leadW-sizeColW-dateColW-2)
	}
	basic := p.fm != nil && p.fm.basicIcons
	now := time.Now()
	for i, e := range p.entries {
		color, attr := entryStyle(e, p.marked[e.name])
		mark := " "
		if p.marked[e.name] {
			mark = "*"
		}
		name := e.name
		if e.isLink {
			name += " ~"
		}
		size := humanSize(e.size)
		date := ""
		switch {
		case e.isUp:
			size = "UP"
		case e.isDrive:
			size = e.drive.label()
		case e.isShare:
			size = "SHARE"
		case e.isDir:
			size = "<DIR>"
			if p.marked[e.name] { // marked folder: recursive total, "…" while measuring
				size = "…"
				if ds, ok := p.sizes[p.fullPath(e.name)]; ok && ds.done {
					size = humanSize(ds.size)
				}
			}
		}
		if !e.mod.IsZero() {
			date = shortDate(e.mod, now)
		}
		text := name
		if nameW > 0 {
			text = truncateName(text, nameW)
		}
		t.SetCell(i, colMark, tview.NewTableCell(mark).SetTextColor(color).SetAttributes(attr))
		t.SetCell(i, colIcon, tview.NewTableCell(string(iconFor(e, basic))).SetTextColor(iconColor(e)))
		t.SetCell(i, colName, tview.NewTableCell(tview.Escape(text)).
			SetTextColor(color).SetAttributes(attr).SetExpansion(1))
		t.SetCell(i, colSize, tview.NewTableCell(fmt.Sprintf("%*s", sizeColW, size)).SetTextColor(color))
		t.SetCell(i, colDate, tview.NewTableCell(fmt.Sprintf("%-*s", dateColW, date)).SetTextColor(color))
	}
	if row >= len(p.entries) {
		row = len(p.entries) - 1
	}
	if row >= 0 {
		t.Select(row, 0)
	}
	t.SetOffset(rowOff, colOff)
	p.updateTitle()
}

// shortDate: "22/10 16:37" for the current year, "22/10/2024" otherwise.
func shortDate(t, now time.Time) string {
	t = t.Local()
	if t.Year() == now.Year() {
		return t.Format("02/01 15:04")
	}
	return t.Format("02/01/2006")
}

// truncateName shortens s to at most w cells, keeping a short extension visible:
// "a very long report name.pdf" -> "a very lo….pdf".
func truncateName(s string, w int) string {
	if uniseg.StringWidth(s) <= w {
		return s
	}
	ext := filepath.Ext(s)
	extW := uniseg.StringWidth(ext)
	if ext == s || extW > 6 || w < extW+4 {
		ext, extW = "", 0
	}
	head := strings.TrimSuffix(s, ext)
	limit := w - extW - 1 // one cell for the ellipsis
	var b strings.Builder
	used := 0
	g := uniseg.NewGraphemes(head)
	for g.Next() {
		cw := g.Width()
		if used+cw > limit {
			break
		}
		b.WriteString(g.Str())
		used += cw
	}
	return b.String() + "…" + ext
}

func (p *Panel) location() string {
	switch {
	case p.arch != nil:
		loc := p.arch.path
		if p.arch.inner != "" {
			loc += string(filepath.Separator) + filepath.FromSlash(p.arch.inner)
		}
		return loc
	case p.dir == "":
		return "Drives"
	case p.inShareList():
		return p.dir + "  (shares)"
	}
	return p.dir
}

func (p *Panel) updateTitle() {
	title := p.location()
	extra := []string{}
	if p.filter != "" {
		extra = append(extra, "filter: "+p.filter)
	}
	extra = append(extra, "sort: "+sortLabels[p.sortMode]+" "+sortArrow(p.sortAsc))
	if p.showHidden {
		extra = append(extra, "hidden shown")
	}
	if p.flat {
		f := "flat view"
		if p.flatTrunc {
			f += ", first " + fmt.Sprint(maxFlatEntries)
		}
		if p.flatRescan != nil {
			f += ", updating…"
		}
		extra = append([]string{f}, extra...)
	}
	if len(extra) > 0 {
		title += "  [" + strings.Join(extra, ", ") + "]"
	}
	// Trim long paths on the left, leaving room for the selection total on the right.
	_, _, w, _ := p.table.GetInnerRect()
	if label := p.selectionLabel(); label != "" && len([]rune(label))+4 <= w+2 {
		w -= len([]rune(label)) + 1
	}
	if w > 10 && len([]rune(title)) > w-4 {
		r := []rune(title)
		title = "…" + string(r[len(r)-(w-5):])
	}
	p.table.SetTitle(" " + tview.Escape(title) + " ")
}

func (p *Panel) current() *entry {
	row, _ := p.table.GetSelection()
	if row < 0 || row >= len(p.entries) {
		return nil
	}
	return &p.entries[row]
}

// fullPath of an entry: a disk path, or an archive-internal path when browsing an archive.
func (p *Panel) fullPath(name string) string {
	switch {
	case p.arch != nil:
		if p.arch.inner == "" {
			return name
		}
		return p.arch.inner + "/" + name
	case p.dir == "":
		return name
	case p.inShareList():
		return p.dir + `\` + name + `\`
	}
	return filepath.Join(p.dir, name)
}

// selection returns the marked entries' full paths or, if none is marked, the one under the cursor.
func (p *Panel) selection() []string {
	var out []string
	for _, e := range p.entries {
		if p.marked[e.name] {
			out = append(out, p.fullPath(e.name))
		}
	}
	if len(out) == 0 {
		if e := p.current(); e != nil && !e.isUp && !e.isDrive && !e.isShare {
			out = append(out, p.fullPath(e.name))
		}
	}
	return out
}

// targets returns disk paths to operate on (nil outside real folders).
func (p *Panel) targets() []string {
	if !p.onDisk() {
		return nil
	}
	return p.selection()
}

// enter is the Enter action: open folders, previews files inside archives, opens other files.
func (p *Panel) enter() {
	e := p.current()
	if e == nil {
		return
	}
	switch {
	case e.isDir || e.isUp:
		p.enterDir()
	case e.inArchive:
		p.fm.preview()
	default:
		if err := openWithDefault(p.fullPath(e.name)); err != nil {
			p.fm.showMessage("Error", "Cannot open file:\n"+err.Error())
		}
	}
}

// enterDir is the Right-arrow action: folders, drives, shares, "..", and .zip/.7z archives.
func (p *Panel) enterDir() {
	e := p.current()
	if e == nil {
		return
	}
	switch {
	case e.isUp:
		p.goParent()
	case p.arch != nil && e.isDir:
		p.arch.inner = p.fullPath(e.name)
		p.filter, p.marked = "", map[string]bool{}
		_ = p.load("")
		p.fm.updateStatus()
	case e.isDrive:
		p.chdir(e.name, "")
	case e.isDir:
		p.chdir(p.fullPath(e.name), "")
	case p.arch == nil && p.onDisk() && isArchive(e.name):
		p.openArchive(p.fullPath(e.name))
	}
}

func (p *Panel) openArchive(archPath string) {
	entries, closer, err := openArchive(archPath)
	if err != nil {
		p.fm.showMessage("Error", "Cannot open archive:\n"+err.Error())
		return
	}
	p.exitFlat()
	p.arch = &archView{path: archPath, entries: entries, closer: closer}
	p.filter, p.marked = "", map[string]bool{}
	_ = p.load("")
	p.fm.updateStatus()
}

func (p *Panel) closeArchive() {
	if p.arch != nil {
		p.arch.closer.Close()
		p.arch = nil
	}
}

// chdir moves to a disk location ("" = drives, \\server = shares); on error it stays where it was.
func (p *Panel) chdir(dir, selectName string) {
	if err := p.tryChdir(dir, selectName); err != nil {
		what := "folder"
		if isUNCServer(dir) {
			what = "share list"
		}
		p.fm.showMessage("Error", "Cannot open "+what+":\n"+err.Error())
	}
}

// tryChdir is chdir that returns the error instead of showing it (the tree view
// reports it in the status bar).
func (p *Panel) tryChdir(dir, selectName string) (err error) {
	if onWindows {
		dir = normalizeUNC(dir)
	}
	old, oldFilter, oldArch, oldMarked := p.dir, p.filter, p.arch, p.marked
	oldFlat, oldFlatEntries, oldTrunc := p.flat, p.flatEntries, p.flatTrunc
	p.exitFlat() // any move to another location ends the flat view
	p.dir, p.filter, p.arch = dir, "", nil
	p.marked = map[string]bool{}
	if err = p.load(selectName); err != nil {
		p.dir, p.filter, p.arch, p.marked = old, oldFilter, oldArch, oldMarked
		p.flat, p.flatEntries, p.flatTrunc = oldFlat, oldFlatEntries, oldTrunc
		_ = p.load("")
	} else if oldArch != nil {
		oldArch.closer.Close()
	}
	if l := driveLetter(p.dir); l != 0 && p.arch == nil {
		p.fm.lastDir[l] = p.dir
	}
	p.fm.updateStatus()
	return err
}

// parentLoc returns where Left/Backspace leads and which name to select there.
func (p *Panel) parentLoc() (string, string, bool) {
	if onWindows {
		return windowsParent(p.dir)
	}
	parent := filepath.Dir(p.dir)
	if p.dir == "" || parent == p.dir {
		return "", "", false
	}
	return parent, filepath.Base(p.dir), true
}

func (p *Panel) hasParent() bool {
	if p.arch != nil {
		return true
	}
	parent, _, ok := p.parentLoc()
	if ok && parent == "" && len(listDrives()) == 0 {
		return false
	}
	return ok
}

func (p *Panel) goParent() {
	if p.arch != nil {
		p.filter, p.marked = "", map[string]bool{}
		if p.arch.inner != "" {
			base := path.Base(p.arch.inner)
			p.arch.inner = path.Dir(p.arch.inner)
			if p.arch.inner == "." {
				p.arch.inner = ""
			}
			_ = p.load(base)
		} else {
			name := filepath.Base(p.arch.path)
			p.closeArchive()
			_ = p.load(name)
		}
		p.fm.updateStatus()
		return
	}
	if !p.hasParent() {
		return
	}
	parent, sel, _ := p.parentLoc()
	p.chdir(parent, sel)
}

func (p *Panel) toggleMark() {
	e := p.current()
	if e != nil && !e.isUp && !e.isDrive && !e.isShare {
		if p.marked[e.name] {
			delete(p.marked, e.name)
			p.dropSize(*e)
		} else {
			p.marked[e.name] = true
			p.ensureSize(*e)
		}
		row, _ := p.table.GetSelection()
		p.render()
		if row+1 < len(p.entries) {
			row++
		}
		p.table.Select(row, 0)
	}
	p.fm.updateStatus()
}

func (p *Panel) invertMarks() {
	for _, e := range p.entries {
		if e.isUp || e.isDrive || e.isShare {
			continue
		}
		if p.marked[e.name] {
			delete(p.marked, e.name)
			p.dropSize(e)
		} else {
			p.marked[e.name] = true
			p.ensureSize(e)
		}
	}
	p.render()
	p.fm.updateStatus()
}

// setMarks replaces the marks with the given names (and drops folder measurements).
func (p *Panel) setMarks(names []string) {
	p.resetSizes()
	p.marked = map[string]bool{}
	for _, n := range names {
		p.marked[n] = true
	}
	p.render()
}

// jumpTo moves the cursor to the first name starting with prefix (or, failing that,
// containing it). It returns false when nothing matches.
func (p *Panel) jumpTo(prefix string) bool {
	q := strings.ToLower(prefix)
	for pass := 0; pass < 2; pass++ {
		for i, e := range p.entries {
			if e.isUp {
				continue
			}
			n := strings.ToLower(e.name)
			base := strings.ToLower(filepath.Base(e.name)) // flat view: match the file name, not the folder
			if (pass == 0 && strings.HasPrefix(base, q)) || (pass == 1 && strings.Contains(n, q)) {
				p.table.Select(i, 0)
				return true
			}
		}
	}
	return false
}

// reload re-reads keeping the cursor on the same item. If that item is gone (moved,
// deleted, renamed elsewhere) the cursor goes to the item that followed it, or to the
// previous one at the end of the list. The scroll position is kept.
func (p *Panel) reload() {
	old := p.entries
	row, _ := p.table.GetSelection()
	rowOff, colOff := p.table.GetOffset()
	name := ""
	if e := p.current(); e != nil {
		name = e.name
	}
	if err := p.load(name); err == nil {
		p.table.SetOffset(rowOff, colOff)
		p.table.Select(nextSurvivor(old, row, p.entries), 0)
		if p.flat {
			p.rescanFlat() // the tree may have changed below the root as well
		}
		return
	}
	// The location may be gone: walk up to the first one that loads.
	for {
		parent, _, ok := p.parentLoc()
		if !ok {
			break
		}
		p.dir = parent
		if p.load("") == nil {
			return
		}
	}
	_ = p.load("")
}

// nextSurvivor returns the row, in the new listing, for the cursor that was on old[row]:
// the same item if it still exists, otherwise the first following item that still
// exists, otherwise the nearest preceding one.
func nextSurvivor(old []entry, row int, now []entry) int {
	idx := make(map[string]int, len(now))
	for i, e := range now {
		idx[e.name] = i
	}
	if row < 0 || row >= len(old) {
		return 0
	}
	for i := row; i < len(old); i++ {
		if j, ok := idx[old[i].name]; ok {
			return j
		}
	}
	for i := row - 1; i >= 0; i-- {
		if j, ok := idx[old[i].name]; ok {
			return j
		}
	}
	return 0
}

// markAllOrInvert is Ctrl+A (v1.12): with nothing marked it marks every file and
// folder; otherwise it inverts the marks.
func (p *Panel) markAllOrInvert() {
	if len(p.marked) > 0 {
		p.invertMarks()
		return
	}
	for _, e := range p.entries {
		if !e.isUp && !e.isDrive && !e.isShare {
			p.marked[e.name] = true
			p.ensureSize(e)
		}
	}
	p.render()
	p.fm.updateStatus()
}

// dirSize is the recursive total of one marked folder. A disk folder is measured
// in the background; done turns true when the result has arrived.
type dirSize struct {
	files, size int64
	done        bool
	o           *opCtx
}

func (p *Panel) resetSizes() {
	for _, ds := range p.sizes {
		ds.o.cancel.Store(true)
	}
	p.sizes = map[string]*dirSize{}
}

// dropSize forgets a folder that has been unmarked, stopping its measurement.
func (p *Panel) dropSize(e entry) {
	key := p.fullPath(e.name)
	if ds, ok := p.sizes[key]; ok {
		ds.o.cancel.Store(true)
		delete(p.sizes, key)
	}
}

// ensureSize starts measuring a marked folder, unless it is already known.
func (p *Panel) ensureSize(e entry) {
	if !e.isDir || e.isUp || e.isDrive || e.isShare {
		return
	}
	key := p.fullPath(e.name)
	if _, ok := p.sizes[key]; ok {
		return
	}
	ds := &dirSize{o: &opCtx{}}
	p.sizes[key] = ds
	if p.arch != nil { // archive listing is in memory: instant
		for _, ae := range p.arch.entries {
			if !ae.isDir && strings.HasPrefix(cleanArchName(ae.name), key+"/") {
				ds.files++
				ds.size += int64(ae.size)
			}
		}
		ds.done = true
		return
	}
	go func() {
		files, size := ds.o.treeStats(key)
		if ds.o.cancelled() {
			return
		}
		p.fm.app.QueueUpdateDraw(func() {
			if p.sizes[key] == ds { // still marked, same listing
				ds.files, ds.size, ds.done = files, size, true
				p.render() // size column and top-border total
				p.fm.updateStatus()
			}
		})
	}()
}

// markedSummary returns how many entries are marked and the cumulative number of files
// and bytes they contain (folders counted recursively). pending is true while some
// folder is still being measured; the totals are then partial.
func (p *Panel) markedSummary() (n int, files, size int64, pending bool) {
	for _, e := range p.entries {
		if !p.marked[e.name] {
			continue
		}
		n++
		if !e.isDir {
			files++
			size += e.size
			continue
		}
		if ds, ok := p.sizes[p.fullPath(e.name)]; ok && ds.done {
			files += ds.files
			size += ds.size
		} else {
			pending = true
		}
	}
	return
}

func matchName(name, pattern string) bool {
	n, p := strings.ToLower(name), strings.ToLower(pattern)
	if strings.ContainsAny(p, "*?[") {
		ok, err := filepath.Match(p, n)
		return err == nil && ok
	}
	return strings.Contains(n, p)
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// updateDisk measures, in the background, the free and total space of the volume the
// panel is on. Drive and share lists show nothing. A folder is measured again at most
// every two seconds (listings are re-read often during copies).
func (p *Panel) updateDisk() {
	key := ""
	if p.arch != nil || p.onDisk() {
		key = p.dir
	}
	if key == "" {
		p.diskFree, p.diskTotal, p.diskKey = 0, 0, ""
		return
	}
	if p.diskBusy || (key == p.diskKey && time.Since(p.diskAt) < 2*time.Second) {
		return
	}
	p.diskBusy = true
	go func() {
		free, total, err := diskSpace(key)
		p.fm.app.QueueUpdateDraw(func() {
			p.diskBusy = false
			p.diskKey, p.diskAt = key, time.Now()
			if err != nil {
				free, total = 0, 0
			}
			p.diskFree, p.diskTotal = free, total
			if p.dir != key { // the panel moved while measuring
				p.updateDisk()
			}
		})
	}()
}

// listsFiles reports whether the panel lists files (folder, flat view or archive),
// as opposed to the drive or share list.
func (p *Panel) listsFiles() bool { return p.arch != nil || p.onDisk() }

// folderTotals counts the files listed in the panel and their total size
// (folders are not included; in the flat view, every listed file).
func (p *Panel) folderTotals() (files, size int64) {
	for _, e := range p.entries {
		if !e.isDir && !e.isUp {
			files++
			size += e.size
		}
	}
	return
}

// compactSize: 28.5G, 512M, 900K, 12B — the short form used for disk space.
func compactSize(n uint64) string {
	s := humanSize(int64(n))
	s = strings.TrimSuffix(s, "B")
	return strings.ReplaceAll(strings.TrimSpace(s), " ", "")
}

// bottomLabel builds " 12 files 3.4 MB │ 28.5G free (11%) " for at most width cells:
// the free-space part is dropped first, then the folder part.
func bottomLabel(width int, files, size int64, listsFiles bool, free, total uint64) string {
	var folder, disk string
	if listsFiles {
		folder = plural(files, "file") + " " + humanSize(size)
	}
	if total > 0 {
		disk = fmt.Sprintf("%s free (%d%%)", compactSize(free), free*100/total)
	}
	for _, parts := range [][]string{{folder, disk}, {folder}, {disk}} {
		var keep []string
		for _, s := range parts {
			if s != "" {
				keep = append(keep, s)
			}
		}
		if len(keep) == 0 {
			continue
		}
		label := " " + strings.Join(keep, " │ ") + " "
		if len([]rune(label)) <= width {
			return label
		}
	}
	return ""
}
