package history

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"pbs-win-backup/internal/datalock"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
)

const maxRecords = 200

var mu sync.Mutex

func path() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return dir + string(os.PathSeparator) + "history.json", nil
}

func Load() ([]models.JobRunResult, error) {
	mu.Lock()
	defer mu.Unlock()
	var records []models.JobRunResult
	err := datalock.With("history", func() error {
		p, err := path()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				records = []models.JobRunResult{}
				return nil
			}
			return err
		}
		return json.Unmarshal(data, &records)
	})
	if err != nil {
		return nil, err
	}
	if records == nil {
		return []models.JobRunResult{}, nil
	}
	return records, nil
}

func Append(result models.JobRunResult) error {
	mu.Lock()
	defer mu.Unlock()
	return datalock.With("history", func() error {
		p, err := path()
		if err != nil {
			return err
		}
		records := []models.JobRunResult{}
		if data, readErr := os.ReadFile(p); readErr == nil {
			if err := json.Unmarshal(data, &records); err != nil {
				return fmt.Errorf("history corrupt: %w", err)
			}
		}
		records = append([]models.JobRunResult{result}, records...)
		if len(records) > maxRecords {
			records = records[:maxRecords]
		}
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return err
		}
		return paths.AtomicWriteSensitive(p, data, 0o600)
	})
}

// Clear removes all stored run history.
func Clear() error {
	mu.Lock()
	defer mu.Unlock()
	p, err := path()
	if err != nil {
		return err
	}
	return datalock.With("history", func() error {
		return paths.AtomicWriteSensitive(p, []byte("[]\n"), 0o600)
	})
}
