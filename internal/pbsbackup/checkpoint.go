package pbsbackup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
)

type Checkpoint struct {
	JobID            string    `json:"job_id"`
	JobName          string    `json:"job_name"`
	Phase            string    `json:"phase"`
	Trigger          string    `json:"trigger,omitempty"`
	BackupTime       int64     `json:"backup_time,omitempty"`
	NewChunks        uint64    `json:"new_chunks"`
	ReusedChunks     uint64    `json:"reused_chunks"`
	BytesTransferred int64     `json:"bytes_transferred"`
	BytesReused      int64     `json:"bytes_reused"`
	FilesDone        int64     `json:"files_done"`
	FilesTotal       int64     `json:"files_total"`
	FilesSkipped     int64     `json:"files_skipped"`
	Percent          float64   `json:"percent"`
	StartedAt        time.Time `json:"started_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Error            string    `json:"error,omitempty"`
}

func checkpointPath(jobID string) (string, error) {
	dir, err := paths.CheckpointsDir()
	if err != nil {
		return "", err
	}
	return dir + string(os.PathSeparator) + jobID + ".json", nil
}

func SaveCheckpoint(cp Checkpoint) error {
	cp.UpdatedAt = time.Now().UTC()
	p, err := checkpointPath(cp.JobID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return paths.AtomicWrite(p, data, 0o644)
}

func LoadCheckpoint(jobID string) (*Checkpoint, error) {
	p, err := checkpointPath(jobID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

func ClearCheckpoint(jobID string) error {
	p, err := checkpointPath(jobID)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

const interruptedCheckpointMaxAge = 7 * 24 * time.Hour

// ListInterruptedCheckpoints returns unfinished backups (cancelled, error, or killed mid-run).
func ListInterruptedCheckpoints() ([]Checkpoint, error) {
	dir, err := paths.CheckpointsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	cutoff := time.Now().UTC().Add(-interruptedCheckpointMaxAge)
	var out []Checkpoint
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var cp Checkpoint
		if json.Unmarshal(data, &cp) != nil {
			continue
		}
		if cp.Phase == string(models.PhaseDone) {
			continue
		}
		if cp.UpdatedAt.IsZero() {
			cp.UpdatedAt = cp.StartedAt
		}
		if cp.UpdatedAt.Before(cutoff) {
			continue
		}
		out = append(out, cp)
	}
	slices.SortFunc(out, func(a, b Checkpoint) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return out, nil
}

// ListActiveCheckpoints returns checkpoints updated within maxAge (service/GUI progress).
func ListActiveCheckpoints(maxAge time.Duration) ([]Checkpoint, error) {
	dir, err := paths.CheckpointsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	cutoff := time.Now().UTC().Add(-maxAge)
	var out []Checkpoint
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var cp Checkpoint
		if json.Unmarshal(data, &cp) != nil {
			continue
		}
		if cp.UpdatedAt.IsZero() {
			cp.UpdatedAt = cp.StartedAt
		}
		if cp.Phase == string(models.PhaseDone) || cp.Phase == string(models.PhaseError) || cp.Phase == string(models.PhaseCancelled) {
			continue
		}
		if cp.UpdatedAt.Before(cutoff) {
			continue
		}
		out = append(out, cp)
	}
	return out, nil
}
