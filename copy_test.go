package main

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeCopy replaces the real file copy to measure how many copies run at once.
type fakeCopy struct {
	mu                 sync.Mutex
	running, runBig    int
	maxRunning, maxBig int
	calls              atomic.Int64
}

func (f *fakeCopy) copy(o *opCtx, src, dst string, fi fs.FileInfo) error {
	f.mu.Lock()
	f.running++
	big := fi.Size() >= bigFileLimit
	if big {
		f.runBig++
	}
	f.maxRunning = max(f.maxRunning, f.running)
	f.maxBig = max(f.maxBig, f.runBig)
	f.mu.Unlock()
	time.Sleep(20 * time.Millisecond)
	f.mu.Lock()
	f.running--
	if big {
		f.runBig--
	}
	f.mu.Unlock()
	f.calls.Add(1)
	o.bytes.Add(fi.Size())
	return os.WriteFile(dst, nil, 0o644)
}

func withFakeCopy(t *testing.T) *fakeCopy {
	f := &fakeCopy{}
	old := copyOneFile
	copyOneFile = f.copy
	t.Cleanup(func() { copyOneFile = old })
	return f
}

func TestCopyRunsSmallFilesInParallelAcrossItems(t *testing.T) {
	f := withFakeCopy(t)
	d := t.TempDir()
	var pairs []copyPair
	for i := 0; i < 12; i++ { // 12 separate top-level files, as when marking files in a panel
		name := filepath.Join(d, "src", string(rune('a'+i))+".txt")
		write(t, name, "x")
		pairs = append(pairs, copyPair{from: name, to: filepath.Join(d, "dst", filepath.Base(name))})
	}
	os.MkdirAll(filepath.Join(d, "dst"), 0o755)
	o := &opCtx{}
	start := time.Now()
	o.copyAll(pairs)
	if len(o.errs) > 0 || f.calls.Load() != 12 {
		t.Fatal(o.errs, f.calls.Load())
	}
	if f.maxRunning != copyWorkers {
		t.Fatalf("max parallel copies = %d, want %d", f.maxRunning, copyWorkers)
	}
	if el := time.Since(start); el > 150*time.Millisecond { // 12×20 ms sequential = 240 ms
		t.Fatalf("not parallel: %v", el)
	}
}

func TestCopyBigFilesOneAtATime(t *testing.T) {
	f := withFakeCopy(t)
	d := t.TempDir()
	src := filepath.Join(d, "src")
	os.MkdirAll(src, 0o755)
	for i := 0; i < 3; i++ { // sparse files: big size, no disk space
		p := filepath.Join(src, "big"+string(rune('0'+i)))
		fh, _ := os.Create(p)
		fh.Truncate(bigFileLimit + 1)
		fh.Close()
	}
	for i := 0; i < 6; i++ {
		write(t, filepath.Join(src, "small"+string(rune('0'+i))), "s")
	}
	o := &opCtx{}
	o.copyPath(src, filepath.Join(d, "dst"))
	if len(o.errs) > 0 || f.calls.Load() != 9 {
		t.Fatal(o.errs, f.calls.Load())
	}
	if f.maxBig != 1 {
		t.Fatalf("big files copied together: %d", f.maxBig)
	}
	if f.maxRunning < 2 {
		t.Fatalf("small files not parallel: %d", f.maxRunning)
	}
}

func TestCopyCancelStopsQueue(t *testing.T) {
	f := withFakeCopy(t)
	d := t.TempDir()
	for i := 0; i < 40; i++ {
		write(t, filepath.Join(d, "src", "f"+string(rune('A'+i))), "x")
	}
	o := &opCtx{}
	go func() { time.Sleep(30 * time.Millisecond); o.cancel.Store(true) }()
	o.copyPath(filepath.Join(d, "src"), filepath.Join(d, "dst"))
	if n := f.calls.Load(); n >= 40 {
		t.Fatalf("cancel ignored: %d copies", n)
	}
	if len(o.errs) != 0 {
		t.Fatalf("cancellation reported as errors: %v", o.errs)
	}
}

func TestMoveKeepsSourceWhenCopyIncomplete(t *testing.T) {
	if onWindows {
		t.Skip("relies on a Unix socket file as an item that cannot be copied")
	}
	d := t.TempDir()
	write(t, filepath.Join(d, "src", "x", "ok.txt"), "ok")
	l, err := net.Listen("unix", filepath.Join(d, "src", "x", "sock")) // a type that cannot be copied
	if err != nil {
		t.Skip("unix sockets unavailable:", err)
	}
	defer l.Close()
	write(t, filepath.Join(d, "dst", "x", "old.txt"), "old") // existing folder: merge, so copy + delete
	write(t, filepath.Join(d, "src", "y", "a.txt"), "a")
	write(t, filepath.Join(d, "dst", "y", "b.txt"), "b")
	o := &opCtx{}
	o.moveAll([]copyPair{
		{from: filepath.Join(d, "src", "x"), to: filepath.Join(d, "dst", "x")},
		{from: filepath.Join(d, "src", "y"), to: filepath.Join(d, "dst", "y")},
	})
	if !exists(filepath.Join(d, "src", "x", "ok.txt")) {
		t.Fatal("source x deleted although its copy was incomplete")
	}
	if exists(filepath.Join(d, "src", "y")) || !exists(filepath.Join(d, "dst", "y", "a.txt")) {
		t.Fatal("complete item y not moved")
	}
	joined := strings.Join(o.errs, "\n")
	if !strings.Contains(joined, "unsupported file type") || !strings.Contains(joined, "source NOT deleted") {
		t.Fatal(o.errs)
	}
}

func TestCopyRealFilesParallelContent(t *testing.T) {
	d := t.TempDir()
	for i := 0; i < 30; i++ {
		write(t, filepath.Join(d, "src", "sub", "f"+string(rune('a'+i%26))+string(rune('0'+i/26))), strings.Repeat("z", i*1000))
	}
	o := &opCtx{}
	o.copyPath(filepath.Join(d, "src"), filepath.Join(d, "dst"))
	if len(o.errs) > 0 || o.files.Load() != 30 || o.bytes.Load() != 435000 {
		t.Fatal(o.errs, o.files.Load(), o.bytes.Load())
	}
	for i := 0; i < 30; i++ {
		n := "f" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		if len(read(t, filepath.Join(d, "dst", "sub", n))) != i*1000 {
			t.Fatal("content", n)
		}
	}
}
