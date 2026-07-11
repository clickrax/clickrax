package backupqueue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"pbs-win-backup/internal/datalock"
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/history"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
)

type deadLetterRecord struct {
	Item   Item      `json:"item"`
	LostAt time.Time `json:"lost_at"`
	Reason string    `json:"reason"`
}

func deadLetterPath() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "backup_queue_dead_letter.json"), nil
}

// RecordDeadLetter logs and persists a queue item that could not be re-enqueued.
func RecordDeadLetter(item Item, reason error) {
	msg := fmt.Sprintf("очередь бэкапов: элемент потерян после сбоя re-enqueue (job=%s, trigger=%s): %v",
		item.JobID, item.Trigger, reason)
	eventlog.Error(msg)

	now := time.Now()
	_ = history.Append(models.JobRunResult{
		JobID:      item.JobID,
		JobName:    item.JobName,
		Status:     "error",
		Trigger:    item.Trigger,
		Error:      msg,
		StartedAt:  now,
		FinishedAt: now,
	})
	_ = appendDeadLetter(deadLetterRecord{
		Item:   item,
		LostAt: now,
		Reason: reason.Error(),
	})
}

func appendDeadLetter(rec deadLetterRecord) error {
	return datalock.With("backup_queue_dead_letter", func() error {
		path, err := deadLetterPath()
		if err != nil {
			return err
		}
		var records []deadLetterRecord
		if data, readErr := os.ReadFile(path); readErr == nil && len(data) > 0 {
			if err := json.Unmarshal(data, &records); err != nil {
				return fmt.Errorf("dead letter corrupt: %w", err)
			}
		}
		records = append(records, rec)
		if len(records) > MaxDeadLetterRecords {
			records = records[len(records)-MaxDeadLetterRecords:]
		}
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return err
		}
		return paths.AtomicWriteSensitive(path, data, 0o644)
	})
}

func loadDeadLetters() ([]deadLetterRecord, error) {
	path, err := deadLetterPath()
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
	var records []deadLetterRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}
