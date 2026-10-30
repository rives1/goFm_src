//go:build windows

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	procSHFileOperationW = shell32.NewProc("SHFileOperationW")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procGetOEMCP         = kernel32.NewProc("GetOEMCP")
)

// listDrives returns the mounted drives, e.g. ["C:\", "D:\"].
func listDrives() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var out []string
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) != 0 {
			out = append(out, string(rune('A'+i))+`:\`)
		}
	}
	return out
}

// fileAttrs returns the Windows attributes (R H S A) and whether the file is hidden.
func fileAttrs(fi fs.FileInfo) (string, bool) {
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return "", false
	}
	a := d.FileAttributes
	flag := func(bit uint32, c byte) byte {
		if a&bit != 0 {
			return c
		}
		return '-'
	}
	s := string([]byte{
		flag(windows.FILE_ATTRIBUTE_READONLY, 'R'),
		flag(windows.FILE_ATTRIBUTE_HIDDEN, 'H'),
		flag(windows.FILE_ATTRIBUTE_SYSTEM, 'S'),
		flag(windows.FILE_ATTRIBUTE_ARCHIVE, 'A'),
	})
	return s, a&(windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_SYSTEM) != 0
}

func createdTime(fi fs.FileInfo) (time.Time, bool) {
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, d.CreationTime.Nanoseconds()), true
}

// openWithDefault opens the file with its associated program (like a double-click in Explorer).
func openWithDefault(path string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(path))
	return windows.ShellExecute(0, verb, file, nil, dir, windows.SW_SHOWNORMAL)
}

// SHFILEOPSTRUCTW, 64-bit layout (amd64/arm64). On 386 the struct is packed and this layout is wrong.
type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

const (
	foDelete           = 0x0003
	fofSilent          = 0x0004
	fofNoConfirmation  = 0x0010
	fofAllowUndo       = 0x0040 // = Recycle Bin
	fofNoErrorUI       = 0x0400
	fofWantNukeWarning = 0x4000 // warn if a file would be destroyed instead of recycled
)

// moveToTrash moves the paths to the Windows Recycle Bin.
func moveToTrash(paths []string) error {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		return errors.New("Recycle Bin supported on 64-bit Windows only")
	}
	var buf []uint16
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		buf = append(buf, utf16.Encode([]rune(abs))...)
		buf = append(buf, 0)
	}
	buf = append(buf, 0) // pFrom needs a double terminator
	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &buf[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI | fofWantNukeWarning,
	}
	ret, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("SHFileOperation returned code 0x%X", ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return errors.New("operation aborted (some items may not have been recycled)")
	}
	return nil
}

// oemDecoder returns a decoder for the system OEM code page (CP850 for Italian).
func oemDecoder() *encoding.Decoder {
	cp, _, _ := procGetOEMCP.Call()
	switch cp {
	case 850:
		return charmap.CodePage850.NewDecoder()
	case 852:
		return charmap.CodePage852.NewDecoder()
	case 858:
		return charmap.CodePage858.NewDecoder()
	case 866:
		return charmap.CodePage866.NewDecoder()
	default:
		return charmap.CodePage437.NewDecoder()
	}
}

var (
	netapi32         = windows.NewLazySystemDLL("netapi32.dll")
	procNetShareEnum = netapi32.NewProc("NetShareEnum")
)

// SHARE_INFO_1 (64-bit layout).
type shareInfo1 struct {
	netname *uint16
	typ     uint32
	remark  *uint16
}

const (
	stypeMask    = 0x000000FF
	stypeDisk    = 0x00000000
	stypeSpecial = 0x80000000 // C$, ADMIN$, IPC$ ...
)

// listShares returns the disk shares published by \\server.
func listShares(server string) ([]shareEntry, error) {
	srv, err := windows.UTF16PtrFromString(server)
	if err != nil {
		return nil, err
	}
	var buf *byte
	var read, total, resume uint32
	ret, _, _ := procNetShareEnum.Call(
		uintptr(unsafe.Pointer(srv)), 1, uintptr(unsafe.Pointer(&buf)),
		0xFFFFFFFF, // MAX_PREFERRED_LENGTH: everything in one call
		uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&resume)))
	if buf != nil {
		defer windows.NetApiBufferFree(buf)
	}
	if ret != 0 && ret != uintptr(windows.ERROR_MORE_DATA) {
		return nil, fmt.Errorf("cannot list shares of %s: %w", server, syscall.Errno(ret))
	}
	if buf == nil || read == 0 {
		return nil, nil
	}
	var out []shareEntry
	for _, it := range unsafe.Slice((*shareInfo1)(unsafe.Pointer(buf)), read) {
		if it.typ&stypeMask != stypeDisk {
			continue // printers, IPC, devices
		}
		out = append(out, shareEntry{
			name:    windows.UTF16PtrToString(it.netname),
			remark:  windows.UTF16PtrToString(it.remark),
			special: it.typ&stypeSpecial != 0,
		})
	}
	return out, nil
}

// ---------- file copy with CopyFileEx ----------
//
// CopyFileEx is the copy engine used by Explorer and robocopy. Compared with a
// read/write loop it lets Windows pick the best I/O for the medium and, where the
// server and protocol support it, offload the copy: between two folders of the same
// SMB server (server-side copy) or of the same storage array (ODX) the data does
// not travel through this PC. It also copies attributes and dates in one go.

var (
	procCopyFileExW = kernel32.NewProc("CopyFileExW")
	copyCallback    = syscall.NewCallback(copyProgress) // created once: callbacks are a limited resource
	copyStates      sync.Map                            // id -> *copyState, one per running copy
	copySeq         atomic.Uintptr
)

const (
	copyFileNoBuffering = 0x00001000 // COPY_FILE_NO_BUFFERING
	progressContinue    = 0          // PROGRESS_CONTINUE
	progressCancel      = 1          // PROGRESS_CANCEL: Windows stops and deletes the partial file
	errRequestAborted   = syscall.Errno(1235)
	unbufferedMin       = 256 << 20 // from this size on, bypass the system cache (as robocopy /J)
)

type copyState struct {
	o    *opCtx
	last int64
}

// copyProgress is the LPPROGRESS_ROUTINE. All arguments are register-sized on 64-bit
// Windows (LARGE_INTEGER is passed by value as one 64-bit value).
func copyProgress(totalSize, transferred, streamSize, streamTransferred, streamNumber, reason, hSrc, hDst, data uintptr) uintptr {
	v, ok := copyStates.Load(data)
	if !ok {
		return progressContinue
	}
	st := v.(*copyState)
	t := int64(transferred)
	st.o.bytes.Add(t - st.last)
	st.last = t
	if st.o.cancelled() {
		return progressCancel
	}
	return progressContinue
}

// longPath adds the \\?\ prefix so CopyFileEx accepts paths longer than 260 characters.
func longPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil || strings.HasPrefix(abs, `\\?\`) {
		return p
	}
	if strings.HasPrefix(abs, `\\`) {
		return `\\?\UNC\` + abs[2:]
	}
	return `\\?\` + abs
}

func copyFileNative(o *opCtx, src, dst string, fi fs.FileInfo) error {
	s16, err := windows.UTF16PtrFromString(longPath(src))
	if err != nil {
		return err
	}
	d16, err := windows.UTF16PtrFromString(longPath(dst))
	if err != nil {
		return err
	}
	id := copySeq.Add(1)
	st := &copyState{o: o}
	copyStates.Store(id, st)
	defer copyStates.Delete(id)
	var flags uintptr
	if fi.Size() >= unbufferedMin {
		flags |= copyFileNoBuffering
	}
	r, _, e := procCopyFileExW.Call(uintptr(unsafe.Pointer(s16)), uintptr(unsafe.Pointer(d16)),
		copyCallback, id, 0, flags)
	if r == 0 {
		if errors.Is(e, errRequestAborted) || o.cancelled() {
			return errCancelled
		}
		return e
	}
	if st.last < fi.Size() {
		o.bytes.Add(fi.Size() - st.last) // in case the last callback was skipped
	}
	return nil
}

// diskSpace returns the bytes available to this user and the total size of the
// volume holding path (local drive or network share).
func diskSpace(path string) (free, total uint64, err error) {
	if !strings.HasSuffix(path, `\`) {
		path += `\` // required for UNC paths, harmless for local ones
	}
	p16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var totalFree uint64
	err = windows.GetDiskFreeSpaceEx(p16, &free, &total, &totalFree)
	return free, total, err
}

// ---------- kinds of drive (v1.12) ----------

const ioctlStorageQueryProperty = 0x002D1400 // CTL_CODE(IOCTL_STORAGE_BASE, 0x500, METHOD_BUFFERED, FILE_ANY_ACCESS)

// Bus types (STORAGE_BUS_TYPE) of external drives that Windows reports as fixed.
const (
	busType1394 = 0x4
	busTypeUsb  = 0x7
	busTypeSd   = 0xC
	busTypeMmc  = 0xD
)

// driveKind classifies a drive root such as `C:\`. GetDriveType tells network,
// removable and optical drives apart; USB disks are reported as fixed, so for fixed
// drives the bus type is asked to the storage driver (no administrator rights needed:
// the volume is opened with no access rights, only for the query).
func driveKind(root string) driveType {
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return driveLocal
	}
	switch windows.GetDriveType(p) {
	case windows.DRIVE_REMOTE:
		return driveNetwork
	case windows.DRIVE_REMOVABLE:
		return driveRemovable
	case windows.DRIVE_CDROM:
		return driveOptical
	case windows.DRIVE_FIXED:
		switch busType(root) {
		case busTypeUsb, busType1394, busTypeSd, busTypeMmc:
			return driveRemovable
		}
	}
	return driveLocal
}

// busType returns the STORAGE_BUS_TYPE of the disk behind a drive root, or 0.
func busType(root string) uint32 {
	if len(root) < 2 {
		return 0
	}
	vol, err := windows.UTF16PtrFromString(`\\.\` + root[:2])
	if err != nil {
		return 0
	}
	h, err := windows.CreateFile(vol, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	// STORAGE_PROPERTY_QUERY{PropertyId: StorageDeviceProperty, QueryType: PropertyStandardQuery}
	var query [12]byte
	var out [1024]byte // STORAGE_DEVICE_DESCRIPTOR followed by its strings
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlStorageQueryProperty, &query[0], uint32(len(query)),
		&out[0], uint32(len(out)), &n, nil); err != nil || n < 32 {
		return 0
	}
	return *(*uint32)(unsafe.Pointer(&out[28])) // STORAGE_DEVICE_DESCRIPTOR.BusType
}

// ---------- external editor (v1.12) ----------

const defaultEditor = "notepad.exe"

// detach starts a program without gofm's console (DETACHED_PROCESS), so a console
// program (e.g. code.cmd) cannot write over gofm's screen.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008}
}
