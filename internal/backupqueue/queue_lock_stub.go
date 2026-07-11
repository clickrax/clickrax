//go:build !windows

package backupqueue

import "sync"

var queueMu sync.Mutex

func withQueueLock(fn func(*[]Item) error) error {
	queueMu.Lock()
	defer queueMu.Unlock()

	items, err := loadItems()
	if err != nil {
		return err
	}
	if items == nil {
		items = []Item{}
	}
	return fn(&items)
}

func ClearStaleLock() bool { return true }

func ForceClearLock() bool { return true }
