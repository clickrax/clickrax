//go:build windows

package singleinstance

import (
	"fmt"
	"syscall"
	"unsafe"

	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/i18nconfig"

	"golang.org/x/sys/windows"
)

const mutexName = "Global\\PbsWinBackupSingleInstance_v1"

var mutexHandle windows.Handle

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows  = user32.NewProc("EnumWindows")
	procShowWindow   = user32.NewProc("ShowWindow")
	procSetForeground = user32.NewProc("SetForegroundWindow")
	procGetWindowText = user32.NewProc("GetWindowTextW")
	procIsWindowVisible = user32.NewProc("IsWindowVisible")
)

const swRestore = 9

type enumWindowsData struct {
	prefix string
	found  windows.Handle
}

func ActivateExisting() bool {
	data := enumWindowsData{prefix: branding.Title}
	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		d := (*enumWindowsData)(unsafe.Pointer(lParam))
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		var buf [256]uint16
		procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		title := syscall.UTF16ToString(buf[:])
		if len(title) >= len(d.prefix) && title[:len(d.prefix)] == d.prefix {
			d.found = windows.Handle(hwnd)
			return 0
		}
		return 1
	})
	procEnumWindows.Call(cb, uintptr(unsafe.Pointer(&data)))
	if data.found == 0 {
		return false
	}
	procShowWindow.Call(uintptr(data.found), swRestore)
	procSetForeground.Call(uintptr(data.found))
	return true
}

func Acquire() error {
	name, err := syscall.UTF16PtrFromString(mutexName)
	if err != nil {
		return err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		return fmt.Errorf("mutex: %w", err)
	}
	mutexHandle = h
	err = windows.GetLastError()
	if err == windows.ERROR_ALREADY_EXISTS {
		ActivateExisting()
		return i18nconfig.FromConfig().Ef("singleinstance.already_running", map[string]string{"app": branding.Name})
	}
	return nil
}

func Release() {
	if mutexHandle != 0 {
		windows.CloseHandle(mutexHandle)
		mutexHandle = 0
	}
}
