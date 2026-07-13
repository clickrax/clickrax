package status

import (
	"encoding/json"
	"os"
	"time"

	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
)

func ReadLastStatus() (models.LastStatus, error) {
	path, err := paths.LastStatusPath()
	if err != nil {
		return models.LastStatus{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return models.LastStatus{}, nil
		}
		return models.LastStatus{}, err
	}
	var s models.LastStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return models.LastStatus{}, err
	}
	return s, nil
}

func WriteLastStatus(s models.LastStatus) error {
	path, err := paths.LastStatusPath()
	if err != nil {
		return err
	}
	if s.Status != "ok" && s.Status != "warning" {
		if prev, err := ReadLastStatus(); err == nil && prev.LastSuccess != "" {
			s.LastSuccess = prev.LastSuccess
		}
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return paths.AtomicWrite(path, data, 0o644)
}

func FromJobResult(result models.JobRunResult, hostname string) models.LastStatus {
	status := result.Status
	lastSuccess := ""
	if status == "ok" || status == "warning" {
		lastSuccess = result.FinishedAt.Format(time.RFC3339)
	}
	return models.LastStatus{
		Hostname:         hostname,
		JobName:          result.JobName,
		LastRun:          result.FinishedAt.Format(time.RFC3339),
		LastSuccess:      lastSuccess,
		Status:           status,
		BackupType:       result.BackupType,
		DurationSec:      result.DurationSec,
		BytesTransferred: result.BytesTransferred,
		BytesReused:      result.BytesReused,
		Snapshot:         result.Snapshot,
		Error:            result.Error,
	}
}
