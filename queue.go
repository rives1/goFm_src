package main

// Operations queue (v1.19). Copies, moves and pastes (F5, F6, Ctrl+P) no longer block
// the interface: each one becomes a job in a queue, run one after the other, while the
// panels stay usable. The pane at the bottom (above the status bar, shown only while
// there are jobs) displays the running job with its progress and the jobs waiting.
//
// A job asks its questions (items onto themselves, files with the same name in the
// flat view, folders with the same name, files that already exist) when its turn
// comes, because the jobs before it may change the folders it works on; meanwhile the
// pane says "waiting for your answer".
//
// F8 moves the focus to the pane: ↑↓ choose a waiting job, Pause/Resume and Cancel act
// on the running job, Remove takes the chosen waiting job out of the queue; Esc or F8
// go back to the panels. Everything also works with the mouse.

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type jobState int

const (
	jobQueued  jobState = iota // waiting for its turn
	jobAsking                  // its turn has come: checks and questions
	jobRunning                 // copying or moving
)

// job is one queued operation.
type job struct {
	id       int
	title    string // e.g. `Copy 3 items → D:\backup`
	state    jobState
	start    func(j *job) // runs on the interface goroutine when the job's turn comes
	o        *opCtx       // the running work (nil before)
	meter    *rateMeter
	cancel   bool          // Cancel pressed before the work started
	stopped  chan struct{} // closed when the background work has returned
	finished bool
	fm       *FM
}

// opQueue is the list of jobs: jobs[0] is the current one once it has started.
type opQueue struct {
	jobs   []*job
	nextID int
	tick   chan struct{} // closed to stop the pane refresh
}

// enqueue adds a job and starts it at once if nothing else is running.
func (fm *FM) enqueue(title string, start func(j *job)) *job {
	q := &fm.queue
	q.nextID++
	j := &job{id: q.nextID, title: title, start: start, fm: fm, stopped: make(chan struct{})}
	q.jobs = append(q.jobs, j)
	if len(q.jobs) > 1 {
		fm.notice = fmt.Sprintf("Queued: %s (%d before it) — F8 shows the operations", title, len(q.jobs)-1)
		fm.updateStatus()
	}
	fm.showQueue()
	fm.startNext()
	return j
}

// startNext starts the first job if it has not started yet.
func (fm *FM) startNext() {
	q := &fm.queue
	if len(q.jobs) == 0 {
		fm.showQueue()
		return
	}
	j := q.jobs[0]
	if j.state != jobQueued {
		return
	}
	j.state = jobAsking
	fm.refreshQueue()
	j.start(j)
}

// finish ends a job however it ended (done, cancelled at a question, nothing to do)
// and starts the next one. It may be called more than once.
func (j *job) finish() {
	if j.finished {
		return
	}
	j.finished = true
	fm := j.fm
	q := &fm.queue
	for i, x := range q.jobs {
		if x == j {
			q.jobs = append(q.jobs[:i], q.jobs[i+1:]...)
			break
		}
	}
	select {
	case <-j.stopped:
	default:
		close(j.stopped)
	}
	fm.showQueue()
	fm.startNext()
}

// run starts the work of a job in the background; done runs on the interface
// goroutine when it has returned, then the next job starts.
func (j *job) run(title string, fn func(o *opCtx), done func(o *opCtx)) {
	fm := j.fm
	j.title, j.state = title, jobRunning
	j.o, j.meter = &opCtx{}, &rateMeter{}
	if j.cancel {
		j.o.cancel.Store(true)
	}
	o := j.o
	fm.refreshQueue()
	go func() {
		if runtime.GOOS == "windows" {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
		}
		if !o.cancelled() {
			fn(o)
		}
		fm.app.QueueUpdateDraw(func() {
			fm.refreshAll()
			if done != nil {
				done(o)
			}
			fm.jobReport(j)
			j.finish()
		})
	}()
}

// jobReport shows the errors of a finished job; a cancelled job without errors only
// gets a notice (a dialog would interrupt whatever the user is doing now).
func (fm *FM) jobReport(j *job) {
	o := j.o
	if len(o.errs) == 0 {
		if o.cancelled() {
			fm.notice = "Cancelled: " + j.title
		} else {
			fm.notice = "Done: " + j.title
		}
		fm.updateStatus()
		return
	}
	fm.reportErrors(o)
}

// ---------- pane ----------

// maxQueueRows is the number of waiting jobs visible at once in the pane.
const maxQueueRows = 3

type queuePane struct {
	box    *tview.Flex
	head   *tview.TextView // the current job: title, progress, current file
	list   *tview.Table    // the waiting jobs
	form   *tview.Form     // Pause/Resume, Cancel, Remove
	height int
}

// Buttons of the pane.
const (
	pauseLabel  = "Pause"
	resumeLabel = "Resume"
	cancelLabel = "Cancel"
	removeLabel = "Remove"
)

func (fm *FM) newQueuePane() *queuePane {
	qp := &queuePane{
		head: tview.NewTextView().SetDynamicColors(false).SetWrap(false),
		list: tview.NewTable().SetSelectable(true, false),
		form: tview.NewForm().SetHorizontal(true).SetButtonsAlign(tview.AlignLeft),
	}
	qp.form.SetBorderPadding(0, 0, 0, 0)
	qp.form.AddButton(pauseLabel, fm.queuePause).
		AddButton(cancelLabel, fm.queueCancel).
		AddButton(removeLabel, fm.queueRemove)
	qp.box = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(qp.head, 3, 0, false).
		AddItem(qp.list, 0, 1, false).
		AddItem(qp.form, 1, 0, false)
	qp.box.SetBorder(true).SetTitle(" Operations — F8 ").SetTitleAlign(tview.AlignLeft)
	qp.recolor()
	return qp
}

func (qp *queuePane) recolor() {
	qp.box.SetBackgroundColor(colBackground)
	qp.box.SetBorderColor(colPanelBorder)
	qp.box.SetTitleColor(colTitle)
	qp.head.SetBackgroundColor(colBackground)
	qp.head.SetTextColor(colText)
	qp.list.SetBackgroundColor(colBackground)
	qp.list.SetSelectedStyle(tcell.StyleDefault.Background(colCursorBg).Foreground(colCursorFg))
	qp.form.SetBackgroundColor(colBackground)
	qp.form.SetButtonBackgroundColor(colContrastBg)
	qp.form.SetButtonTextColor(colText)
}

// paneHeight is the height of the pane for n waiting jobs: border, 3 lines for the
// current job, the waiting jobs (at least one line), the buttons.
func paneHeight(waiting int) int {
	return 2 + 3 + min(max(waiting, 1), maxQueueRows) + 1
}

// showQueue shows the pane when there are jobs and hides it otherwise; it rebuilds
// the main layout only when that changes.
func (fm *FM) showQueue() {
	if fm.root == nil {
		return
	}
	q := &fm.queue
	visible := len(q.jobs) > 0
	if fm.qpane == nil {
		fm.qpane = fm.newQueuePane()
	}
	h := 0
	if visible {
		h = paneHeight(len(q.jobs) - 1)
	}
	if h != fm.qpane.height {
		hadFocus := fm.queueHasFocus()
		fm.qpane.height = h
		fm.root.Clear().AddItem(fm.panelsBox, 0, 1, true)
		if visible {
			fm.root.AddItem(fm.qpane.box, h, 0, false)
		}
		fm.root.AddItem(fm.status, 1, 0, false).AddItem(fm.keybar, 1, 0, false)
		if hadFocus && !visible {
			fm.setActive(fm.active, true)
		}
	}
	if visible && q.tick == nil {
		stop := make(chan struct{})
		q.tick = stop
		go func() {
			t := time.NewTicker(250 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					fm.app.QueueUpdateDraw(fm.refreshQueue)
				}
			}
		}()
	}
	if !visible && q.tick != nil {
		close(q.tick)
		q.tick = nil
	}
	fm.refreshQueue()
}

// refreshQueue redraws the content of the pane.
func (fm *FM) refreshQueue() {
	qp, q := fm.qpane, &fm.queue
	if qp == nil || len(q.jobs) == 0 {
		return
	}
	_, _, w, _ := qp.head.GetInnerRect()
	if w <= 0 {
		w = 80
	}
	qp.head.SetText(jobLines(q.jobs[0], w, time.Now()))
	sel, _ := qp.list.GetSelection()
	qp.list.Clear()
	for i, j := range q.jobs[1:] {
		qp.list.SetCell(i, 0, tview.NewTableCell(tview.Escape(fmt.Sprintf("%d. %s — queued", i+2, j.title))).
			SetTextColor(colDim).SetExpansion(1))
	}
	if len(q.jobs) == 1 {
		qp.list.SetCell(0, 0, tview.NewTableCell("No other operations queued").SetTextColor(colDim).SetSelectable(false))
	}
	if sel >= qp.list.GetRowCount() {
		sel = qp.list.GetRowCount() - 1
	}
	if sel >= 0 {
		qp.list.Select(sel, 0)
	}
	label := pauseLabel
	if cur := q.jobs[0]; cur.o != nil && cur.o.paused.Load() {
		label = resumeLabel
	}
	qp.form.GetButton(0).SetLabel(label)
	more := ""
	if n := len(q.jobs) - 1 - maxQueueRows; n > 0 {
		more = fmt.Sprintf(" (%d waiting, ↑↓ to see them)", len(q.jobs)-1)
	}
	qp.box.SetTitle(" Operations — F8" + more + " ")
	if fm.queueHasFocus() {
		qp.box.SetBorderColor(colActiveBorder)
	} else {
		qp.box.SetBorderColor(colPanelBorder)
	}
}

// jobLines are the 3 lines of the current job.
func jobLines(j *job, width int, now time.Time) string {
	first := "▶ " + j.title
	switch {
	case j.state == jobAsking:
		return first + "\n  waiting for your answer (or checking the folders)…\n"
	case j.o == nil:
		return first + "\n\n"
	}
	o := j.o
	if o.paused.Load() {
		first += " — PAUSED"
	}
	if o.cancelled() {
		return first + "\n  cancelling…\n"
	}
	cur := loadString(&o.cur)
	if r := []rune(cur); len(r) > width-2 && width > 3 {
		cur = "…" + string(r[len(r)-width+3:])
	}
	if ph := loadString(&o.phase); ph != "" {
		return first + "\n  " + ph + "\n  " + cur
	}
	total, done := o.total.Load(), o.bytes.Load()
	if total <= 0 {
		return fmt.Sprintf("%s\n  %d files processed\n  %s", first, o.files.Load(), cur)
	}
	if done > total {
		done = total
	}
	if o.paused.Load() {
		j.meter.samples = nil
	} else {
		j.meter.add(now, done)
	}
	frac := float64(done) / float64(total)
	stats := fmt.Sprintf(" %3d%%  %s of %s", int(frac*100), humanSize(done), humanSize(total))
	switch r := j.meter.rate(); {
	case o.paused.Load():
		stats += " · PAUSED — Resume to go on"
	case r > 0:
		stats += " · " + humanSize(int64(r)) + "/s · " +
			formatDuration(time.Duration(float64(total-done)/r*float64(time.Second))) + " left"
	case done == total:
		stats += " · finishing…"
	}
	barW := max(10, min(60, width-2-len([]rune(stats))))
	filled := int(frac * float64(barW))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)
	return first + "\n  " + bar + stats + "\n  " + cur
}

// ---------- pane commands ----------

func (fm *FM) currentJob() *job {
	if len(fm.queue.jobs) == 0 {
		return nil
	}
	return fm.queue.jobs[0]
}

// queuePause pauses or resumes the running job.
func (fm *FM) queuePause() {
	if j := fm.currentJob(); j != nil && j.o != nil && !j.o.cancelled() {
		j.o.paused.Store(!j.o.paused.Load())
	}
	fm.refreshQueue()
}

// queueCancel cancels the current job (also while paused); a job still asking its
// questions is cancelled as soon as its work would start.
func (fm *FM) queueCancel() {
	j := fm.currentJob()
	if j == nil {
		return
	}
	if j.o != nil {
		j.o.cancel.Store(true)
	} else {
		j.cancel = true
	}
	fm.refreshQueue()
}

// queueRemove takes the chosen waiting job out of the queue.
func (fm *FM) queueRemove() {
	q := &fm.queue
	if len(q.jobs) < 2 || fm.qpane == nil {
		return
	}
	row, _ := fm.qpane.list.GetSelection()
	i := row + 1
	if row < 0 || i >= len(q.jobs) {
		return
	}
	j := q.jobs[i]
	q.jobs = append(q.jobs[:i], q.jobs[i+1:]...)
	j.finished = true
	fm.notice = "Removed from the queue: " + j.title
	fm.showQueue()
}

// queueHasFocus reports whether the focus is in the pane.
func (fm *FM) queueHasFocus() bool {
	if fm.qpane == nil {
		return false
	}
	f := fm.app.GetFocus()
	if f == nil {
		return false
	}
	return f == fm.qpane.list || f == fm.qpane.form || fm.qpane.form.HasFocus() || fm.qpane.box.HasFocus()
}

// focusQueue is F8: to the pane and back.
func (fm *FM) focusQueue() {
	if fm.queueHasFocus() {
		fm.setActive(fm.active, true)
		fm.refreshQueue()
		return
	}
	if len(fm.queue.jobs) == 0 {
		fm.notice = "F8: no operations are running or queued"
		fm.updateStatus()
		return
	}
	if len(fm.queue.jobs) > 1 {
		fm.app.SetFocus(fm.qpane.list)
	} else {
		fm.app.SetFocus(fm.qpane.form)
	}
	fm.refreshQueue()
}

// paneGlobalKey: keys that work also with the focus in the pane — the key summary
// (Ctrl+F2) and quitting (Ctrl+W, Alt+Q).
func paneGlobalKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyF2 && ev.Modifiers()&tcell.ModCtrl != 0 {
		return true
	}
	if l, ok := ctrlLetter(ev); ok && l == 'W' {
		return true
	}
	return ev.Key() == tcell.KeyRune && (ev.Rune() == 'q' || ev.Rune() == 'Q') && ev.Modifiers()&tcell.ModAlt != 0
}

// queueKeys handles the keys while the pane has the focus: F8 and Esc go back to
// the panels, Tab and Shift+Tab move between the list and the buttons; the rest goes
// to the list or the buttons.
func (fm *FM) queueKeys(ev *tcell.EventKey) *tcell.EventKey {
	switch ev.Key() {
	case tcell.KeyF8, tcell.KeyEscape:
		fm.setActive(fm.active, true)
		fm.refreshQueue()
		return nil
	case tcell.KeyTab, tcell.KeyBacktab:
		if fm.app.GetFocus() == fm.qpane.list {
			fm.app.SetFocus(fm.qpane.form)
			return nil
		}
		if ev.Key() == tcell.KeyBacktab && fm.qpane.form.HasFocus() {
			if idx, _ := fm.qpane.form.GetFocusedItemIndex(); idx == 0 || idx == -1 {
				fm.app.SetFocus(fm.qpane.list)
				return nil
			}
		}
	}
	return ev
}

// quitQueue asks before quitting while jobs are running or queued; on Quit the
// running job is cancelled (its half-written file is removed) and gofm stops when
// it has returned, or after 5 seconds at most.
func (fm *FM) quitQueue() bool {
	q := &fm.queue
	if len(q.jobs) == 0 {
		return false
	}
	text := fmt.Sprintf("%s running or queued:\nquit anyway? The running one is cancelled.", plural(int64(len(q.jobs)), "operation"))
	if len(q.jobs) == 1 {
		text = "An operation is running:\nquit anyway? It is cancelled."
	}
	fm.confirm(text, []string{"Cancel", "Quit"}, func(c string) {
		if c != "Quit" {
			return
		}
		cur := q.jobs[0]
		q.jobs = q.jobs[:1]
		fm.queueCancel()
		if cur.o == nil { // still asking: nothing runs in the background
			fm.quitting = true
			fm.app.Stop()
			return
		}
		go func() {
			select {
			case <-cur.stopped:
			case <-time.After(5 * time.Second):
			}
			fm.app.QueueUpdate(func() {
				fm.quitting = true
				fm.app.Stop()
			})
		}()
	})
	return true
}
