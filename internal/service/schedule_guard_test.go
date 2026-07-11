package service

import (
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestSlotAlreadySucceededAfterRecord(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	job := models.BackupJob{
		ID: "job-a",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "03:00",
		},
	}
	now := time.Date(2026, 7, 7, 3, 5, 0, 0, time.Local)
	if SlotAlreadySucceeded(job, now) {
		t.Fatal("slot should not be marked before success")
	}
	RecordScheduleSuccessForJob(job, now)
	if !SlotAlreadySucceeded(job, now) {
		t.Fatal("slot should be marked after successful run")
	}
}

func TestBeginScheduledRunDedupesConcurrentStarts(t *testing.T) {
	if !BeginScheduledRun("dup-job", "") {
		t.Fatal("first begin should succeed")
	}
	if BeginScheduledRun("dup-job", "") {
		t.Fatal("second begin should be rejected while first is active")
	}
	EndScheduledRun("dup-job", "")
	if !BeginScheduledRun("dup-job", "") {
		t.Fatal("begin should succeed after end")
	}
	EndScheduledRun("dup-job", "")
}
