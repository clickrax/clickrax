//go:build windows

package winutil

import (
	"fmt"
	"syscall"
	"unsafe"

	"pbs-win-backup/internal/i18n"

	"golang.org/x/sys/windows"
)

const (
	seeMaskNoCloseProcess = 0x00000040
	swShow                = 5
	errorCancelled        = 1223
)

type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     uintptr
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	dummy        uintptr
	hProcess     uintptr
}

func RunElevated(exe, parameters string) (uint32, error) {
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	params, _ := syscall.UTF16PtrFromString(parameters)

	sei := shellExecuteInfo{
		cbSize:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		fMask:        seeMaskNoCloseProcess,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		nShow:        swShow,
	}

	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecuteEx := shell32.NewProc("ShellExecuteExW")
	ret, _, callErr := shellExecuteEx.Call(uintptr(unsafe.Pointer(&sei)))
	if ret == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errorCancelled {
			return 1, i18n.New("").E("platform.elevation_cancelled")
		}
		if callErr != nil && callErr != syscall.Errno(0) {
			return 1, callErr
		}
		return 1, i18n.New("").E("platform.elevation_failed")
	}
	if sei.hProcess == 0 {
		return 0, nil
	}
	defer windows.CloseHandle(windows.Handle(sei.hProcess))

	event, waitErr := windows.WaitForSingleObject(windows.Handle(sei.hProcess), windows.INFINITE)
	if waitErr != nil {
		return 1, waitErr
	}
	if event != windows.WAIT_OBJECT_0 {
		return 1, i18n.New("").Ef("platform.wait_exit_code", map[string]string{"code": fmt.Sprintf("%d", event)})
	}

	var code uint32
	if err := windows.GetExitCodeProcess(windows.Handle(sei.hProcess), &code); err != nil {
		return 1, err
	}
	return code, nil
}
