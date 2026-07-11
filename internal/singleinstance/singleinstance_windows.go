//go:build windows

package singleinstance

import (
	"fmt"
	"syscall"

	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/i18nconfig"

	"golang.org/x/sys/windows"
)

const mutexName = "Global\\PbsWinBackupSingleInstance_v1"

var mutexHandle windows.Handle

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
