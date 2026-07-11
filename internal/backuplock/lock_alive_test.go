//go:build windows

package backuplock

import (
	"syscall"
	"testing"
)

func TestIsProcessAlive_AccessDenied_ReturnsTrue(t *testing.T) {
	restore := SetOpenProcessHandleHook(func(pid int) (uintptr, syscall.Errno) {
		return 0, syscall.ERROR_ACCESS_DENIED
	})
	defer restore()

	if !IsProcessAlive(12345) {
		t.Fatal("ACCESS_DENIED should be treated as alive process")
	}
}
