package backupqueue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/paths"
)

type alreadyQueuedError struct{}

func (alreadyQueuedError) Error() string {
	return i18nconfig.FromConfig().T("job.already_queued")
}

// ErrAlreadyQueued is returned when the same scheduled slot is already in the queue.
var ErrAlreadyQueued = alreadyQueuedError{}

// Item is a pending backup run stored in the shared queue file.
type Item struct {
	JobID       string    `json:"job_id"`
	ForceFull   bool      `json:"force_full"`
	ScheduledAt time.Time `json:"scheduled_at,omitempty"`
	SlotKey     string    `json:"slot_key,omitempty"`
	Trigger     string    `json:"trigger"`
	EnqueuedAt  time.Time `json:"enqueued_at"`
	JobName     string    `json:"job_name,omitempty"`
	FromQueue   bool      `json:"from_queue,omitempty"`
}

func queuePath() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "backup_queue.json"), nil
}

func loadItems() ([]Item, error) {
	path, err := queuePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func saveItems(items []Item) error {
	path, err := queuePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return paths.AtomicWriteSensitive(path, data, 0o644)
}

func dedupeKey(item Item) string {
	if item.SlotKey != "" {
		return item.JobID + "|" + item.SlotKey
	}
	return ""
}

// ListBestEffort reads the queue without the cross-process lock (UI display only).
func ListBestEffort() []Item {
	items, err := loadItems()
	if err != nil || len(items) == 0 {
		return nil
	}
	return items
}

// List returns a copy of queued items (oldest first).
func List() ([]Item, error) {
	var out []Item
	err := withQueueLock(func(items *[]Item) error {
		out = append([]Item(nil), (*items)...)
		return nil
	})
	return out, err
}

// Len returns queue length.
func Len() (int, error) {
	n := 0
	err := withQueueLock(func(items *[]Item) error {
		n = len(*items)
		return nil
	})
	return n, err
}

// Contains reports whether an item with the same dedupe key is already queued.
func Contains(item Item) (bool, error) {
	key := dedupeKey(item)
	if key == "" {
		return false, nil
	}
	found := false
	err := withQueueLock(func(items *[]Item) error {
		for _, it := range *items {
			if dedupeKey(it) == key {
				found = true
				return nil
			}
		}
		return nil
	})
	return found, err
}

// Enqueue adds an item to the tail. Returns 1-based position in queue.
func Enqueue(item Item) (int, error) {
	if item.EnqueuedAt.IsZero() {
		item.EnqueuedAt = time.Now()
	}
	pos := 0
	err := withQueueLock(func(items *[]Item) error {
		if len(*items) >= MaxQueueItems {
			return ErrQueueFull
		}
		key := dedupeKey(item)
		if key != "" {
			for _, it := range *items {
				if dedupeKey(it) == key {
					return ErrAlreadyQueued
				}
			}
		}
		*items = append(*items, item)
		pos = len(*items)
		return saveItems(*items)
	})
	return pos, err
}

// EnqueueFront puts an item back at the head (used when start failed after pop).
func EnqueueFront(item Item) error {
	return withQueueLock(func(items *[]Item) error {
		*items = append([]Item{item}, *items...)
		return saveItems(*items)
	})
}

// PopNext removes and returns the oldest item.
func PopNext() (Item, bool, error) {
	var item Item
	ok := false
	err := withQueueLock(func(items *[]Item) error {
		if len(*items) == 0 {
			return nil
		}
		item = (*items)[0]
		*items = (*items)[1:]
		ok = true
		return saveItems(*items)
	})
	return item, ok, err
}

// RemoveJob drops all queued items for a deleted job.
func RemoveJob(jobID string) error {
	return withQueueLock(func(items *[]Item) error {
		out := (*items)[:0]
		for _, it := range *items {
			if it.JobID != jobID {
				out = append(out, it)
			}
		}
		*items = out
		return saveItems(*items)
	})
}
