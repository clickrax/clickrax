//go:build windows

package backuplock

import "syscall"

// IsProcessAlive reports whether pid refers to a running process.
func IsProcessAlive(pid int) bool {
	return isProcessAlive(pid)
}

// SetOpenProcessHandleHook replaces OpenProcess for tests. Returns restore func.
func SetOpenProcessHandleHook(fn func(int) (uintptr, syscall.Errno)) func() {
	old := openProcessHandle
	openProcessHandle = fn
	return func() { openProcessHandle = old }
}
