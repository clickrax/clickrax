package backupqueue

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pbs-win-backup/internal/backuplock"
	"pbs-win-backup/internal/datalock"
	"pbs-win-backup/internal/paths"
)

const inflightRecentGrace = 2 * time.Minute

var inflightRecentlyActive = inflightRecentlyWritten

// ErrPermanentStart marks queue items that must not be re-enqueued at the head.
var ErrPermanentStart = errors.New("permanent queue start failure")

func inflightPath() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "backup_queue_inflight.json"), nil
}

func writeInflightItem(item Item) error {
	path, err := inflightPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return paths.AtomicWriteSensitive(path, data, 0o644)
}

func markInflight(item Item) error {
	return datalock.With("backup_queue_inflight", func() error {
		return writeInflightItem(item)
	})
}

func clearInflight() error {
	return datalock.With("backup_queue_inflight", func() error {
		path, err := inflightPath()
		if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	})
}

// ReconcileInflight re-enqueues a popped item left in-flight after a crash.
func ReconcileInflight() error {
	return datalock.With("backup_queue_inflight", func() error {
		path, err := inflightPath()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if backupLockHeld() || inflightRecentlyActive(path) {
			return nil
		}
		var item Item
		if err := json.Unmarshal(data, &item); err != nil {
			_ = os.Remove(path)
			RecordDeadLetter(Item{JobName: "corrupt inflight"}, fmt.Errorf("inflight corrupt: %w", err))
			return fmt.Errorf("inflight corrupt: %w", err)
		}
		alreadyQueued, qerr := queueContainsItem(item)
		if qerr != nil {
			return qerr
		}
		if !alreadyQueued {
			if err := EnqueueFront(item); err != nil {
				return err
			}
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	})
}

func queueContainsItem(want Item) (bool, error) {
	items, err := loadItems()
	if err != nil {
		return false, err
	}
	return queueContainsItemLocked(items, want)
}

func queueContainsItemLocked(items []Item, want Item) (bool, error) {
	wantKey := dedupeKey(want)
	for _, it := range items {
		if wantKey != "" && dedupeKey(it) == wantKey {
			return true, nil
		}
		if it.JobID == want.JobID && it.SlotKey == want.SlotKey && it.EnqueuedAt.Equal(want.EnqueuedAt) {
			return true, nil
		}
	}
	return false, nil
}

func inflightRecentlyWritten(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(st.ModTime()) < inflightRecentGrace
}

// ClaimNext durably marks the queue head in-flight before removing it.
func ClaimNext() (Item, bool, error) {
	var item Item
	ok := false
	err := withQueueLock(func(items *[]Item) error {
		if err := reconcileOrphanInflightLocked(items); err != nil {
			return err
		}
		if len(*items) == 0 {
			return nil
		}
		item = (*items)[0]
		if err := writeInflightItem(item); err != nil {
			return err
		}
		*items = (*items)[1:]
		ok = true
		return saveItems(*items)
	})
	return item, ok, err
}

func reconcileOrphanInflightLocked(items *[]Item) error {
	path, err := inflightPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var existing Item
	if err := json.Unmarshal(data, &existing); err != nil {
		_ = os.Remove(path)
		RecordDeadLetter(Item{JobName: "corrupt inflight"}, fmt.Errorf("inflight corrupt: %w", err))
		return fmt.Errorf("inflight corrupt: %w", err)
	}
	if backupLockHeld() {
		return fmt.Errorf("inflight item pending while backup active")
	}
	queued, err := queueContainsItemLocked(*items, existing)
	if err != nil {
		return err
	}
	if !queued {
		*items = append([]Item{existing}, *items...)
		if err := saveItems(*items); err != nil {
			return err
		}
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsPermanentStartError reports whether a start failure should dead-letter instead of requeue.
func IsPermanentStartError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrPermanentStart) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "job not found") ||
		strings.Contains(msg, "задание не найдено") ||
		strings.Contains(msg, "no sources") ||
		strings.Contains(msg, "нет источников") ||
		strings.Contains(msg, "destination or secret unavailable") ||
		strings.Contains(msg, "назначение или secret недоступны") ||
		strings.Contains(msg, "config") && strings.Contains(msg, "load")
}

var backupLockHeld = backuplock.IsHeld

// PermanentStartError wraps a permanent queue start failure.
func PermanentStartError(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(ErrPermanentStart, err)
}
