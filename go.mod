module gofm

go 1.24.0

toolchain go1.27.3

require (
	github.com/bodgit/sevenzip v1.6.0
	github.com/fsnotify/fsnotify v1.8.0
	github.com/gdamore/tcell/v2 v2.13.10
	github.com/rivo/tview v0.42.0
	github.com/rivo/uniseg v0.4.7
	golang.org/x/sys v0.38.0
	golang.org/x/text v0.31.0
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/tc-hib/go-winres v0.3.3 // indirect
	github.com/tc-hib/winres v0.2.1 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	github.com/urfave/cli/v2 v2.25.7 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go4.org v0.0.0-20200411211856-f5505b9728dd // indirect
	golang.org/x/image v0.15.0 // indirect
	golang.org/x/term v0.37.0 // indirect
)

// go4.org/syncutil and go4.org/readerutil (used by the 7z reader) are vendored:
// the go4.org module would pull in a large, unrelated dependency graph.
replace go4.org => ./third_party/go4

tool github.com/tc-hib/go-winres
