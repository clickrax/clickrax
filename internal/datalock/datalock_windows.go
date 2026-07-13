//go:build windows

package datalock

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"pbs-win-backup/internal/paths"
)

// With runs fn while holding a cross-process lock file in the data directory.
func With(name string, fn func() error) error {
	if !validLockName(name) {
		return fmt.Errorf("invalid data lock name: %s", name)
	}
	path, err := lockPath(name)
	if err != nil {
		return err
	}
	if err := acquire(path); err != nil {
		return err
	}
	defer release(path)
	return fn()
}

func lockPath(name string) (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".lock"), nil
}

func acquire(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_ = clearStale(path)
	for attempt := 0; attempt < 200; attempt++ {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if _, werr := fmt.Fprintf(f, "%d %d\n", os.Getpid(), time.Now().Unix()); werr != nil {
				_ = f.Close()
				_ = os.Remove(path)
				return werr
			}
			if werr := f.Sync(); werr != nil {
				_ = f.Close()
				_ = os.Remove(path)
				return werr
			}
			if werr := f.Close(); werr != nil {
				_ = os.Remove(path)
				return werr
			}
			return nil
		}
		if !os.IsExist(err) {
			return err
		}
		if clearStale(path) {
			continue
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("data lock busy: %s", filepath.Base(path))
}

func release(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	pid, _, err := parseLock(data)
	if err != nil || pid != os.Getpid() {
		return
	}
	_ = os.Remove(path)
}

func clearStale(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return true
	}
	first, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid, ts, err := parseLock(first)
	if err != nil {
		return false
	}
	if !lockExpiredByAgeOrPID(pid, ts, time.Now()) {
		return false
	}
	second, err := os.ReadFile(path)
	if err != nil || !lockContentsMatch(first, second) {
		return false
	}
	pid2, ts2, err := parseLock(second)
	if err != nil || !lockExpiredByAgeOrPID(pid2, ts2, time.Now()) {
		return false
	}
	return os.Remove(path) == nil
}

const lockMaxAge = 25 * time.Hour

func lockExpiredByAgeOrPID(pid int, ts int64, now time.Time) bool {
	if pid <= 0 {
		return true
	}
	if ts > 0 && now.Sub(time.Unix(ts, 0)) > lockMaxAge {
		return true
	}
	if ts <= 0 {
		return true
	}
	return !isProcessAlive(pid)
}

func lockContentsMatch(a, b []byte) bool {
	return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
}

func validLockName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func parseLock(data []byte) (pid int, ts int64, err error) {
	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0, 0, fmt.Errorf("empty lock")
	}
	parts := strings.Fields(s)
	if len(parts) < 1 {
		return 0, 0, fmt.Errorf("invalid lock")
	}
	pid, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	if len(parts) > 1 {
		ts, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, err
		}
	}
	return pid, ts, nil
}

var openProcessHandle = defaultOpenProcessHandle

func defaultOpenProcessHandle(pid int) (uintptr, syscall.Errno) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProcess := kernel32.NewProc("OpenProcess")
	const processQueryLimited = 0x1000
	handle, _, callErr := openProcess.Call(uintptr(processQueryLimited), 0, uintptr(pid))
	var errno syscall.Errno
	if callErr != nil {
		if e, ok := callErr.(syscall.Errno); ok {
			errno = e
		}
	}
	return handle, errno
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, errno := openProcessHandle(pid)
	if handle == 0 {
		return errno == syscall.ERROR_ACCESS_DENIED
	}
	_ = syscall.CloseHandle(syscall.Handle(handle))
	return true
}
