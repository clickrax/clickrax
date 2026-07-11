package service

import (
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestSlotKeyWrapperMatchesSchedulePackage(t *testing.T) {
	job := models.BackupJob{
		ID: "job-1",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "09:30",
		},
	}
	now := time.Date(2026, 7, 7, 9, 45, 0, 0, time.Local)
	if slotKey(job, now) != "2026-07-07T09:30" {
		t.Fatalf("unexpected slot key: %s", slotKey(job, now))
	}
}
