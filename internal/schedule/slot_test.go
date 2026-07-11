package schedule

import (
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestSlotKeyUsesScheduledMinuteNotCurrent(t *testing.T) {
	job := models.BackupJob{
		ID: "j1",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "08:10",
		},
	}
	scheduled := time.Date(2026, 7, 7, 8, 10, 0, 0, time.Local)
	catchUp := time.Date(2026, 7, 7, 8, 15, 0, 0, time.Local)
	if got, want := SlotKey(job, scheduled), "2026-07-07T08:10"; got != want {
		t.Fatalf("scheduled: got %q want %q", got, want)
	}
	if got, want := SlotKey(job, catchUp), "2026-07-07T08:10"; got != want {
		t.Fatalf("catch-up: got %q want %q", got, want)
	}
}

func TestDueCatchUpSharesSlotKey(t *testing.T) {
	job := models.BackupJob{
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "03:00",
		},
	}
	scheduled := time.Date(2026, 7, 6, 3, 0, 0, 0, time.Local)
	later := scheduled.Add(10 * time.Minute)
	if SlotKey(job, scheduled) != SlotKey(job, later) {
		t.Fatal("catch-up should share slot key with scheduled minute")
	}
}
