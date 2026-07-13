//go:build windows

package backuplock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/paths"
)

var ownLockMu sync.Mutex
var ownLockRefs int
var lockHeartbeatStop chan struct{}

const lockHeartbeatInterval = 30 * time.Minute

type Lock struct {
	path string
}

func lockPath() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "backup.lock"), nil
}

func clearLegacyLock() {
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		return
	}
	for _, sub := range []string{branding.ExeName, "pbs-win-backup"} {
		legacy := filepath.Join(dir, sub, "backup.lock")
		data, err := os.ReadFile(legacy)
		if err != nil {
			_ = os.Remove(legacy)
			continue
		}
		info, err := parseLockContent(data)
		if err != nil || lockExpired(info, time.Now(), isProcessAlive) {
			_ = os.Remove(legacy)
		}
	}
}

func readLockAt(path string) (lockInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return lockInfo{}, err
	}
	return parseLockContent(data)
}

func tryRemoveStaleLock(path string) bool {
	first, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	info, err := parseLockContent(first)
	if err == nil && !lockExpired(info, time.Now(), isProcessAlive) {
		return false
	}
	second, err := os.ReadFile(path)
	if err != nil || !lockContentsMatch(first, second) {
		return false
	}
	info2, err := parseLockContent(second)
	if err != nil || !lockExpired(info2, time.Now(), isProcessAlive) {
		return false
	}
	return os.Remove(path) == nil
}

func writeLockFile(f *os.File) error {
	if _, err := fmt.Fprintf(f, "%d %d\n", os.Getpid(), time.Now().Unix()); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}

func touchLockFile(path string) {
	info, err := readLockAt(path)
	if err != nil || info.pid != os.Getpid() {
		return
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	_ = writeLockFile(f)
}

func startLockHeartbeat(path string) {
	ownLockMu.Lock()
	if lockHeartbeatStop != nil {
		ownLockMu.Unlock()
		return
	}
	stop := make(chan struct{})
	lockHeartbeatStop = stop
	ownLockMu.Unlock()
	go func() {
		ticker := time.NewTicker(lockHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				touchLockFile(path)
			}
		}
	}()
}

func stopLockHeartbeat() {
	ownLockMu.Lock()
	if lockHeartbeatStop != nil {
		close(lockHeartbeatStop)
		lockHeartbeatStop = nil
	}
	ownLockMu.Unlock()
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, errno := openProcessHandle(pid)
	if handle == 0 {
		// ACCESS_DENIED means the process exists but this token cannot query it.
		return errno == syscall.ERROR_ACCESS_DENIED
	}
	_ = syscall.CloseHandle(syscall.Handle(handle))
	return true
}

var openProcessHandle = defaultOpenProcessHandle

func defaultOpenProcessHandle(pid int) (uintptr, syscall.Errno) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProcess := kernel32.NewProc("OpenProcess")
	const processQueryLimited = 0x1000
	handle, _, err := openProcess.Call(uintptr(processQueryLimited), 0, uintptr(pid))
	var errno syscall.Errno
	if err != nil {
		if e, ok := err.(syscall.Errno); ok {
			errno = e
		}
	}
	return handle, errno
}

// IsHeld reports whether a non-stale backup lock is currently held by a live process.
func IsHeld() bool {
	path, err := lockPath()
	if err != nil {
		return false
	}
	info, err := readLockAt(path)
	if err != nil {
		return false
	}
	return !lockExpired(info, time.Now(), isProcessAlive)
}

// ClearStale removes a stale lock file. Returns true when no live backup lock is held.
func ClearStale() bool {
	path, err := lockPath()
	if err != nil {
		return false
	}
	clearLegacyLock()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return true
	}
	return tryRemoveStaleLock(path)
}

// ForceClearOwn removes backup.lock owned by this process (orphaned after crash).
func ForceClearOwn() bool {
	path, err := lockPath()
	if err != nil {
		return false
	}
	info, err := readLockAt(path)
	if err != nil || info.pid != os.Getpid() {
		return false
	}
	return os.Remove(path) == nil
}

// Acquire creates an exclusive backup lock for this process.
func Acquire() (*Lock, error) {
	clearLegacyLock()
	path, err := lockPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 3; attempt++ {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if werr := writeLockFile(f); werr != nil {
				_ = os.Remove(path)
				return nil, fmt.Errorf("%s", i18n.L("lock.backup_write_failed", map[string]string{"err": werr.Error()}))
			}
			startLockHeartbeat(path)
			return &Lock{path: path}, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}

		info, readErr := readLockAt(path)
		if readErr == nil && info.pid == os.Getpid() {
			ownLockMu.Lock()
			ownLockRefs++
			ownLockMu.Unlock()
			startLockHeartbeat(path)
			return &Lock{path: path}, nil
		}
		if readErr != nil || lockExpired(info, time.Now(), isProcessAlive) {
			if !tryRemoveStaleLock(path) {
				return nil, i18n.E("lock.stale_clear_failed", nil)
			}
			continue
		}
		return nil, i18n.E("lock.held_by_process", map[string]string{"pid": strconv.Itoa(info.pid)})
	}
	return nil, i18n.E("lock.acquire_failed", nil)
}

func (l *Lock) Release() {
	if l == nil || l.path == "" {
		return
	}
	ownLockMu.Lock()
	if ownLockRefs > 1 {
		ownLockRefs--
		ownLockMu.Unlock()
		return
	}
	ownLockRefs = 0
	ownLockMu.Unlock()

	info, err := readLockAt(l.path)
	if err == nil && info.pid == os.Getpid() {
		_ = os.Remove(l.path)
	}
	stopLockHeartbeat()
}

// ClearStaleFileLock removes a stale lock file at an arbitrary path (e.g. backup_queue.lock).
func ClearStaleFileLock(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return true
	}
	return tryRemoveStaleLock(path)
}

// ForceClearFileLock removes lock file at path only when owned by this process or stale.
func ForceClearFileLock(path string) bool {
	if path == "" {
		return false
	}
	if ReleaseOwnedFileLock(path) {
		return true
	}
	return ClearStaleFileLock(path)
}

// ReleaseOwnedFileLock removes path when the lock file is owned by this process.
func ReleaseOwnedFileLock(path string) bool {
	if path == "" {
		return false
	}
	info, err := readLockAt(path)
	if err != nil || info.pid != os.Getpid() {
		return false
	}
	return os.Remove(path) == nil
}
