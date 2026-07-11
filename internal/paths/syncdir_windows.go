//go:build windows

package paths

import (
	"errors"
	"os"
	"strings"
	"syscall"
)

func syncDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		if isIgnorableDirSyncErr(err) {
			return nil
		}
		return err
	}
	return nil
}

func isIgnorableDirSyncErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "access is denied")
}
