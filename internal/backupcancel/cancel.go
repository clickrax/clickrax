package backupcancel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"pbs-win-backup/internal/paths"
)

const maxSignalAge = 10 * time.Minute

// CancelSignal records a cross-process cancel request for a running backup job.
type CancelSignal struct {
	JobID       string `json:"job_id"`
	RequestedAt string `json:"requested_at"`
	RequestedBy string `json:"requested_by"`
}

func validateJobID(jobID string) error {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return errEmptyJobID
	}
	if _, err := uuid.Parse(jobID); err != nil {
		return errInvalidJobID
	}
	return nil
}

func requestPath(jobID string) (string, error) {
	if err := validateJobID(jobID); err != nil {
		return "", err
	}
	dir, err := paths.CancelRequestsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, jobID+".json"), nil
}

// Request writes a cancel signal for the service (or another process) to pick up.
func Request(jobID string) error {
	paths.EnsureSharedDataAccess()
	p, err := requestPath(jobID)
	if err != nil {
		return err
	}
	req := CancelSignal{
		JobID:       strings.TrimSpace(jobID),
		RequestedAt: time.Now().UTC().Format(time.RFC3339),
		RequestedBy: "gui",
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func signalAge(p string) (time.Duration, bool) {
	info, err := os.Stat(p)
	if err != nil {
		return 0, false
	}
	return time.Since(info.ModTime()), true
}

// IsRequested reports whether a fresh cancel signal exists for the job.
func IsRequested(jobID string) bool {
	p, err := requestPath(jobID)
	if err != nil {
		return false
	}
	age, ok := signalAge(p)
	if !ok {
		return false
	}
	if age > maxSignalAge {
		_ = os.Remove(p)
		return false
	}
	return true
}

// ReapStale removes expired cancel signals from disk.
func ReapStale(maxAge time.Duration) {
	if maxAge <= 0 {
		maxAge = maxSignalAge
	}
	dir, err := paths.CancelRequestsDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > maxAge {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// Clear removes a cancel signal after it was handled or the job finished.
func Clear(jobID string) {
	p, err := requestPath(jobID)
	if err != nil {
		return
	}
	_ = os.Remove(p)
}
