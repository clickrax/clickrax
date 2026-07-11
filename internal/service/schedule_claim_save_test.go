package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestRecordSlotSuccess_SaveFailure_KeepsClaim(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	job := models.BackupJob{
		ID: "job-save-fail",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "05:00",
		},
	}
	now := time.Date(2026, 7, 7, 5, 0, 0, 0, time.Local)
	if !TryClaimSlot(job, now) {
		t.Fatal("expected claim")
	}
	path, err := scheduleClaimPath(job.ID, slotKey(job, now))
	if err != nil {
		t.Fatal(err)
	}

	statePath, err := scheduleStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte("{"), 0o444); err != nil {
		t.Fatal(err)
	}

	recordSlotSuccess(job, now)

	if _, err := os.Stat(path); err != nil {
		t.Fatal("claim should remain when durable save fails")
	}
}
