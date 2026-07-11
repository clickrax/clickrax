//go:build windows

package winattr

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

func captureFileTimes(path string, entry *Entry) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	win, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		entry.MtimeNS = info.ModTime().UTC().UnixNano()
		return
	}
	ctime := windows.Filetime(win.CreationTime)
	atime := windows.Filetime(win.LastAccessTime)
	mtime := windows.Filetime(win.LastWriteTime)
	entry.MtimeNS = mtime.Nanoseconds()
	entry.CtimeNS = ctime.Nanoseconds()
	entry.AtimeNS = atime.Nanoseconds()
}

func applyFileTimes(path string, e Entry) error {
	if !e.HasTimes() {
		return nil
	}
	pathPtr, err := windows.UTF16PtrFromString(normalizeWindowsPath(path))
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(
		pathPtr,
		windows.FILE_WRITE_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	var ctime, atime, mtime *windows.Filetime
	if e.CtimeNS != 0 {
		ft := windows.NsecToFiletime(e.CtimeNS)
		ctime = &ft
	}
	if e.AtimeNS != 0 {
		ft := windows.NsecToFiletime(e.AtimeNS)
		atime = &ft
	}
	if e.MtimeNS != 0 {
		ft := windows.NsecToFiletime(e.MtimeNS)
		mtime = &ft
	}
	return windows.SetFileTime(h, ctime, atime, mtime)
}
