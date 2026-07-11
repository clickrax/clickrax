package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pbs-win-backup/internal/models"
	schedpkg "pbs-win-backup/internal/schedule"
)

func TestCatchUp_StaleClaimLock_SlotRerun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	job := models.BackupJob{
		ID: "job-stale",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "03:00",
		},
	}
	now := time.Date(2026, 7, 7, 3, 15, 0, 0, time.Local)
	if !schedpkg.Due(job, now) {
		t.Fatal("job should be due within catch-up grace after scheduled minute")
	}

	key := slotKey(job, now)
	safeKey := strings.ReplaceAll(key, ":", "-")
	claimPath := filepath.Join(progData, fmt.Sprintf("schedule_claim_%s_%s.lock", job.ID, safeKey))
	if err := os.WriteFile(claimPath, []byte("999999 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ClearStaleScheduleClaims()

	if _, err := os.Stat(claimPath); !os.IsNotExist(err) {
		t.Fatal("startup reaper should remove stale claim lock")
	}
	if !TryClaimSlot(job, now) {
		t.Fatal("catch-up slot should be claimable after stale lock is reaped")
	}
}

func TestRecordSlotSuccess_RemovesClaimLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	job := models.BackupJob{
		ID: "job-ok",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "04:00",
		},
	}
	now := time.Date(2026, 7, 7, 4, 0, 0, 0, time.Local)
	if !TryClaimSlot(job, now) {
		t.Fatal("expected claim to succeed")
	}
	path, err := scheduleClaimPath(job.ID, slotKey(job, now))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("claim lock should exist before success record")
	}

	RecordScheduleSlotHandledForJob(job, now)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("claim lock should be removed after slot handled")
	}
}
