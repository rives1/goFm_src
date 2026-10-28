package main

// Automatic refresh: the folders shown in the two panels are watched with the
// operating system's change notifications (ReadDirectoryChangesW on Windows, which
// also works on SMB shares; inotify on Linux). Nothing is polled, so an idle folder
// costs nothing. Bursts of events (e.g. a copy writing hundreds of files) are
// coalesced: a panel is re-read at most once per interval, and the interval grows
// with how long the last listing took, so a large network folder is not re-read
// continuously. Alt+R still refreshes by hand.

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

const minRefreshDelay = 400 * time.Millisecond

type watcher struct {
	fm      *FM
	w       *fsnotify.Watcher // nil when notifications are unavailable
	watched map[string]string // normPath -> path being watched (UI goroutine only)

	mu       sync.Mutex
	dirty    map[string]bool // normPath of folders with changes
	all      bool            // refresh everything (event overflow)
	timerSet bool
	cost     atomic.Int64 // longest recent listing time, in ns
}

func newWatcher(fm *FM) *watcher {
	wt := &watcher{fm: fm, watched: map[string]string{}, dirty: map[string]bool{}}
	if w, err := fsnotify.NewWatcher(); err == nil {
		wt.w = w
		go wt.loop()
	}
	return wt
}

func (wt *watcher) loop() {
	for {
		select {
		case ev, ok := <-wt.w.Events:
			if !ok {
				return
			}
			wt.mark(filepath.Dir(ev.Name), false)
			wt.mark(ev.Name, false) // the watched folder itself (renamed or deleted)
		case _, ok := <-wt.w.Errors:
			if !ok {
				return
			}
			wt.mark("", true) // e.g. buffer overflow: some events were lost
		}
	}
}

// mark records a change and schedules one refresh for the whole burst.
func (wt *watcher) mark(dir string, all bool) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if all {
		wt.all = true
	} else {
		wt.dirty[normPath(dir)] = true
	}
	if !wt.timerSet {
		wt.timerSet = true
		time.AfterFunc(refreshDelay(time.Duration(wt.cost.Load())), wt.flush)
	}
}

// refreshDelay waits at least minRefreshDelay, and twice the last listing time for slow folders.
func refreshDelay(lastLoad time.Duration) time.Duration {
	return max(minRefreshDelay, 2*lastLoad)
}

func (wt *watcher) flush() {
	wt.mu.Lock()
	dirty, all := wt.dirty, wt.all
	wt.dirty, wt.all, wt.timerSet = map[string]bool{}, false, false
	wt.mu.Unlock()
	wt.fm.app.QueueUpdateDraw(func() {
		var cost time.Duration
		for _, p := range []*Panel{wt.fm.left, wt.fm.right} {
			if p.onDisk() && (all || dirty[normPath(p.dir)]) {
				p.reload()
				cost = max(cost, p.lastLoad)
			}
		}
		wt.cost.Store(int64(cost))
	})
}

// syncWatches makes the watched set match the folders shown in the panels.
// It runs on the UI goroutine after every listing.
func (fm *FM) syncWatches() {
	wt := fm.watch
	if wt == nil || wt.w == nil || fm.left == nil || fm.right == nil {
		return
	}
	want := map[string]string{}
	for _, p := range []*Panel{fm.left, fm.right} {
		if p.onDisk() {
			want[normPath(p.dir)] = p.dir
		}
	}
	for key, path := range wt.watched {
		if _, ok := want[key]; !ok {
			_ = wt.w.Remove(path)
			delete(wt.watched, key)
		}
	}
	for key, path := range want {
		if _, ok := wt.watched[key]; !ok {
			if err := wt.w.Add(path); err == nil { // unsupported targets just stay manual (Alt+R)
				wt.watched[key] = path
			}
		}
	}
}

func (wt *watcher) close() {
	if wt != nil && wt.w != nil {
		_ = wt.w.Close()
	}
}
