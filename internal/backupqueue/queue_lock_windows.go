//go:build windows

package backupqueue

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"pbs-win-backup/internal/backuplock"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/paths"
)

var queueLockMu sync.Mutex

func withQueueLock(fn func(*[]Item) error) error {
	queueLockMu.Lock()
	defer queueLockMu.Unlock()

	path, err := acquireQueueLock()
	if err != nil {
		return err
	}
	defer releaseQueueLock(path)

	items, err := loadItems()
	if err != nil {
		return err
	}
	if items == nil {
		items = []Item{}
	}
	if err := fn(&items); err != nil {
		return err
	}
	return nil
}

func acquireQueueLock() (string, error) {
	path, err := queueLockPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	_ = backuplock.ClearStaleFileLock(path)

	for attempt := 0; attempt < 100; attempt++ {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if werr := writeQueueLockFile(f); werr != nil {
				_ = os.Remove(path)
				return "", i18nconfig.FromConfig().Ewrap("lock.queue_write_failed", map[string]string{"err": werr.Error()}, werr)
			}
			return path, nil
		}
		if !os.IsExist(err) {
			return "", err
		}

		if isLegacyQueueLock(path) || backuplock.ClearStaleFileLock(path) {
			continue
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", i18n.E("lock.queue_busy", nil)
}

func writeQueueLockFile(f *os.File) error {
	if _, err := fmt.Fprintf(f, "%d %d\n", os.Getpid(), time.Now().Unix()); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}

// isLegacyQueueLock detects old queue locks that used placeholder "1" instead of a real PID.
func isLegacyQueueLock(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(data)
	if s == "1\n" || s == "1\r\n" || s == "1" {
		_ = os.Remove(path)
		return true
	}
	return false
}

func releaseQueueLock(path string) {
	if path == "" {
		return
	}
	_ = backuplock.ReleaseOwnedFileLock(path)
}

// ClearStaleLock removes a stale backup_queue.lock left after a crash.
func ClearStaleLock() bool {
	path, err := queueLockPath()
	if err != nil {
		return false
	}
	if isLegacyQueueLock(path) {
		return true
	}
	return backuplock.ClearStaleFileLock(path)
}

// ForceClearLock removes backup_queue.lock unconditionally.
func ForceClearLock() bool {
	path, err := queueLockPath()
	if err != nil {
		return false
	}
	return backuplock.ForceClearFileLock(path)
}

func queueLockPath() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "backup_queue.lock"), nil
}
