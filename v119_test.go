package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestColoursSwapped(t *testing.T) {
	want := map[string][2]int32{ // media, programs
		"navy": {0x5AF78E, 0xFF6B6B}, "dark": {0x5AF78E, 0xFF6B6B}, "light": {0x1A7F37, 0xD80838},
	}
	forEachScheme(t, func(t *testing.T) {
		w := want[schemes[curScheme].name]
		if c, _ := entryStyle(entry{name: "film.mp4"}, false); c != rgb(w[0]) {
			t.Errorf("media: %06X, want %06X", c.Hex(), w[0])
		}
		if c, _ := entryStyle(entry{name: "setup.exe"}, false); c != rgb(w[1]) {
			t.Errorf("programs: %06X, want %06X", c.Hex(), w[1])
		}
	})
}

func TestHelpVersion(t *testing.T) {
	if !strings.Contains(helpText(), "gofm "+version+"[") {
		t.Error("version missing from the summary")
	}
	fm := newTestFM(t)
	fm.showKeys()
	if len(fm.modals) != 1 {
		t.Fatal("summary not open")
	}
	if title := fm.focus[fm.modals[0]].(interface{ GetTitle() string }).GetTitle(); !strings.Contains(title, "gofm "+version) {
		t.Errorf("title %q", title)
	}
	// the version of the program is the one of the changelog, so the summary is current
	log, err := os.ReadFile("CHANGELOG.md")
	if err != nil || !strings.Contains(string(log), "\n## "+version+" ") {
		t.Error("CHANGELOG.md has no section for the version")
	}
}

// No ARM build since v1.19: script, workflows and install script name gofm.exe only.
func TestNoARMBuild(t *testing.T) {
	for _, f := range []string{"scripts/build-release.sh", ".github/workflows/ci.yml", ".github/workflows/release.yml", "windows-terminal/install-gofm.ps1"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "gofm-arm64") || strings.Contains(string(b), "GOARCH=arm64") || strings.Contains(string(b), "amd64,arm64") {
			t.Errorf("%s still builds or installs for ARM64", f)
		}
	}
}

func TestPaneHeight(t *testing.T) {
	for waiting, want := range map[int]int{0: 7, 1: 7, 2: 8, 3: 9, 10: 9} {
		if got := paneHeight(waiting); got != want {
			t.Errorf("%d waiting: %d, want %d", waiting, got, want)
		}
	}
}

// slowCopies makes each file copy take d and records when each copy starts.
func slowCopies(t *testing.T, d time.Duration) *[]string {
	var mu sync.Mutex
	var log []string
	orig := copyOneFile
	copyOneFile = func(o *opCtx, s, dst string, fi fs.FileInfo) error {
		mu.Lock()
		log = append(log, filepath.Base(filepath.Dir(dst)))
		mu.Unlock()
		time.Sleep(d)
		return orig(o, s, dst, fi)
	}
	t.Cleanup(func() { copyOneFile = orig })
	return &log
}

func paneShown(fm *FM) bool {
	for i := 0; i < fm.root.GetItemCount(); i++ {
		if fm.qpane != nil && fm.root.GetItem(i) == fm.qpane.box {
			return true
		}
	}
	return false
}

func fill(t *testing.T, dir string, n int) {
	for i := 0; i < n; i++ {
		writeFile(t, filepath.Join(dir, string(rune('a'+i))+".txt"), "x", time.Now())
	}
}

// Two copies run one after the other in the pane, and the panels stay usable.
func TestQueueSequential(t *testing.T) {
	src, d1, d2 := t.TempDir(), t.TempDir(), t.TempDir()
	fill(t, src, 6)
	log := slowCopies(t, 30*time.Millisecond)
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(src, "")
		fm.right.chdir(d1, "")
		fm.setActive(fm.left, true)
		fm.keys(ctrlKey('A'))
		fm.keys(key(tcell.KeyF5, 0, 0))
		if len(fm.left.marked) != 0 {
			t.Error("marks not cleared when queued")
		}
		fm.right.chdir(d2, "")
		fm.keys(ctrlKey('A'))
		fm.keys(key(tcell.KeyF5, 0, 0))
		if len(fm.queue.jobs) != 2 || !paneShown(fm) || len(fm.modals) != 0 {
			t.Fatalf("%d jobs, pane %v, %d dialogs", len(fm.queue.jobs), paneShown(fm), len(fm.modals))
		}
		if !strings.Contains(fm.notice, "Queued:") {
			t.Errorf("notice %q", fm.notice)
		}
		// the panels still answer
		fm.left.table.Select(1, 0)
		fm.left.table.InputHandler()(key(tcell.KeyDown, 0, 0), func(tview.Primitive) {})
	})
	ui(func() {
		if row, _ := fm.left.table.GetSelection(); row != 2 {
			t.Errorf("cursor did not move while copying: row %d", row)
		}
		fm.refreshQueue()
		head := fm.qpane.head.GetText(false)
		list := fm.qpane.list.GetCell(0, 0).Text
		if !strings.Contains(head, "▶ Copy 6 items → "+d1) || !strings.Contains(list, "2. Copy 6 items → "+d2) || !strings.Contains(list, "queued") {
			t.Errorf("pane: %q / %q", head, list)
		}
	})
	waitUI(t, ui, "both copies", func() bool { return len(fm.queue.jobs) == 0 })
	ui(func() {
		if paneShown(fm) {
			t.Error("pane still shown")
		}
		if !strings.Contains(fm.notice, "Done: Copy 6 items → "+d2) {
			t.Errorf("notice %q", fm.notice)
		}
	})
	des1, _ := os.ReadDir(d1)
	des2, _ := os.ReadDir(d2)
	if len(des1) != 6 || len(des2) != 6 {
		t.Fatalf("copied %d and %d", len(des1), len(des2))
	}
	seen2 := false
	for _, dir := range *log {
		if dir == filepath.Base(d2) {
			seen2 = true
		} else if seen2 {
			t.Fatal("the second copy started before the first ended")
		}
	}
}

// The questions of a job come when it starts: the second copy of the same file finds
// the file written by the first one.
func TestQueueQuestionsAtStart(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, "a.txt"), "new", time.Now())
	slowCopies(t, 200*time.Millisecond)
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(src, "a.txt")
		fm.right.chdir(dst, "")
		fm.setActive(fm.left, true)
		fm.keys(key(tcell.KeyF5, 0, 0))
		fm.left.jumpTo("a.txt")
		fm.keys(key(tcell.KeyF5, 0, 0))
		if len(fm.modals) != 0 {
			t.Fatal("a question was asked when queuing")
		}
	})
	waitUI(t, ui, "conflict question of the second job", func() bool {
		return len(fm.modals) == 1 && len(fm.queue.jobs) == 1
	})
	ui(func() {
		fm.refreshQueue()
		if !strings.Contains(fm.qpane.head.GetText(false), "Waiting for your answer") {
			t.Errorf("pane: %q", fm.qpane.head.GetText(false))
		}
		pressEnter(fm) // Skip
	})
	waitUI(t, ui, "queue empty", func() bool { return len(fm.queue.jobs) == 0 && len(fm.modals) == 0 })
	// Cancel at a question ends the job and the next one starts
	ui(func() {
		fm.left.jumpTo("a.txt")
		fm.keys(key(tcell.KeyF5, 0, 0)) // asks at once (nothing before it)
	})
	waitUI(t, ui, "question", func() bool { return len(fm.modals) == 1 })
	ui(func() {
		// a question is a dialog, so the keys go to it: queue the next copy directly
		fm.transfer([]string{filepath.Join(src, "a.txt")}, dst, false, func() {})
		if len(fm.queue.jobs) != 2 {
			t.Fatalf("%d jobs", len(fm.queue.jobs))
		}
		fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
	})
	waitUI(t, ui, "next job asks", func() bool { return len(fm.queue.jobs) == 1 && len(fm.modals) == 1 })
	ui(func() {
		fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
	})
	waitUI(t, ui, "queue empty again", func() bool { return len(fm.queue.jobs) == 0 })
}

// F8, Pause/Resume, Remove and Cancel in the pane.
func TestQueuePane(t *testing.T) {
	src, d1, d2, d3 := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	fill(t, src, 8)
	slowCopies(t, 60*time.Millisecond)
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.keys(key(tcell.KeyF8, 0, 0))
		if !strings.Contains(fm.notice, "no operations") {
			t.Errorf("F8 without jobs: %q", fm.notice)
		}
		fm.left.chdir(src, "")
		fm.setActive(fm.left, true)
		for _, d := range []string{d1, d2, d3} {
			fm.right.chdir(d, "")
			fm.keys(ctrlKey('A'))
			fm.keys(key(tcell.KeyF5, 0, 0))
		}
		if len(fm.queue.jobs) != 3 || fm.qpane.height != paneHeight(2) {
			t.Fatalf("%d jobs, height %d", len(fm.queue.jobs), fm.qpane.height)
		}
		fm.keys(key(tcell.KeyF8, 0, 0))
		if !fm.queueHasFocus() || fm.app.GetFocus() != fm.qpane.list {
			t.Fatal("F8 does not focus the list")
		}
		// letters go to the pane, not to the quick search
		fm.keys(key(tcell.KeyRune, 'x', 0))
		if fm.qs != "" {
			t.Error("quick search while in the pane")
		}
		fm.qpane.list.Select(0, 0) // job 2 (to d2)
		fm.queueRemove()
		if len(fm.queue.jobs) != 2 || fm.queue.jobs[1].title != "Copy 8 items → "+d3 {
			t.Fatalf("remove: %d jobs", len(fm.queue.jobs))
		}
	})
	waitUI(t, ui, "first job running", func() bool { j := fm.currentJob(); return j != nil && j.o != nil && j.o.files.Load() > 0 })
	ui(func() {
		fm.queuePause()
		if !fm.currentJob().o.paused.Load() || fm.qpane.form.GetButton(0).GetLabel() != resumeLabel {
			t.Fatal("Pause")
		}
	})
	time.Sleep(150 * time.Millisecond)
	n1 := len(mustRead(d1))
	time.Sleep(400 * time.Millisecond)
	if n2 := len(mustRead(d1)); n2 != n1 || n2 == 8 {
		t.Fatalf("copying while paused: %d → %d", n1, n2)
	}
	ui(func() {
		if !strings.Contains(fm.qpane.head.GetText(false), "PAUSED") {
			t.Errorf("pane while paused: %q", fm.qpane.head.GetText(false))
		}
		fm.queueCancel() // cancel while paused
	})
	waitUI(t, ui, "second job (to d3) running", func() bool {
		j := fm.currentJob()
		return j != nil && strings.HasSuffix(j.title, d3)
	})
	ui(func() {
		if len(mustRead(d1)) == 8 {
			t.Error("the cancelled copy finished anyway")
		}
		// Esc goes back to the panels
		fm.app.SetFocus(fm.qpane.form)
		fm.keys(key(tcell.KeyEscape, 0, 0))
		if fm.queueHasFocus() || fm.app.GetFocus() != fm.active.table {
			t.Error("Esc does not leave the pane")
		}
	})
	waitUI(t, ui, "all done", func() bool { return len(fm.queue.jobs) == 0 })
	if len(mustRead(d2)) != 0 || len(mustRead(d3)) != 8 {
		t.Errorf("removed job ran, or last job incomplete: %d %d", len(mustRead(d2)), len(mustRead(d3)))
	}
}

func mustRead(d string) []os.DirEntry {
	des, _ := os.ReadDir(d)
	return des
}

// Quitting with jobs asks first; Quit cancels the running job and then stops.
func TestQuitWithQueue(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	fill(t, src, 8)
	slowCopies(t, 80*time.Millisecond)
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(src, "")
		fm.right.chdir(dst, "")
		fm.setActive(fm.left, true)
		fm.keys(ctrlKey('A'))
		fm.keys(key(tcell.KeyF5, 0, 0))
	})
	waitUI(t, ui, "running", func() bool { j := fm.currentJob(); return j != nil && j.o != nil })
	ui(func() {
		fm.keys(ctrlKey('W'))
		if fm.quitting || len(fm.modals) != 1 {
			t.Fatal("no question before quitting")
		}
		pressEnter(fm) // Cancel (the default): keep working
		if fm.quitting || len(fm.queue.jobs) != 1 {
			t.Fatal("Cancel quit anyway")
		}
		fm.keys(ctrlKey('W'))
		fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, 0), func(tview.Primitive) {})
		pressEnter(fm) // Quit
	})
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) && !quitting(fm) {
		time.Sleep(20 * time.Millisecond)
	}
	if !quitting(fm) {
		t.Fatal("did not quit")
	}
	if len(mustRead(dst)) == 8 {
		t.Error("the running copy was not cancelled")
	}
}

func quitting(fm *FM) bool {
	done := make(chan bool, 1)
	go func() { fm.app.QueueUpdate(func() { done <- fm.quitting }) }()
	select {
	case q := <-done:
		return q
	case <-time.After(200 * time.Millisecond):
		return true // the application has stopped
	}
}

// With the focus in the pane, Ctrl+F2 and quitting still work; closing the summary
// gives the focus back to the pane.
func TestPaneGlobalKeys(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	fill(t, src, 8)
	slowCopies(t, 100*time.Millisecond)
	fm := newTestFM(t)
	ui := runUI(t, fm)
	ui(func() {
		fm.left.chdir(src, "")
		fm.right.chdir(dst, "")
		fm.setActive(fm.left, true)
		fm.keys(ctrlKey('A'))
		fm.keys(key(tcell.KeyF5, 0, 0))
		fm.keys(key(tcell.KeyF8, 0, 0))
		if !fm.queueHasFocus() {
			t.Fatal("F8")
		}
		fm.keys(key(tcell.KeyF2, 0, tcell.ModCtrl))
		if len(fm.modals) != 1 {
			t.Fatal("Ctrl+F2 from the pane")
		}
		fm.app.GetFocus().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
		if len(fm.modals) != 0 || !fm.queueHasFocus() {
			t.Errorf("after the summary: %d dialogs, focus in the pane %v", len(fm.modals), fm.queueHasFocus())
		}
		fm.keys(key(tcell.KeyRune, 'q', tcell.ModAlt))
		if len(fm.modals) != 1 {
			t.Fatal("Alt+Q from the pane")
		}
		pressEnter(fm) // Cancel
	})
	waitUI(t, ui, "done", func() bool { return len(fm.queue.jobs) == 0 })
	ui(func() {
		if fm.app.GetFocus() != fm.active.table {
			t.Error("the focus did not go back to the panel when the pane closed")
		}
	})
}
