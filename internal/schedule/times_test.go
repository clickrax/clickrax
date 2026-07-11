package schedule

import (
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestDueMultipleTimesPerDay(t *testing.T) {
	job := models.BackupJob{
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Times:   []string{"08:00", "20:00"},
		},
	}
	at8 := time.Date(2026, 7, 7, 8, 0, 0, 0, time.Local)
	at20 := time.Date(2026, 7, 7, 20, 0, 0, 0, time.Local)
	atNoon := time.Date(2026, 7, 7, 12, 0, 0, 0, time.Local)
	if !Due(job, at8) {
		t.Fatal("08:00 should be due")
	}
	if !Due(job, at20) {
		t.Fatal("20:00 should be due")
	}
	if Due(job, atNoon) {
		t.Fatal("12:00 should not be due")
	}
}

func TestSlotKeyDifferentTimesSameDay(t *testing.T) {
	job := models.BackupJob{
		ID: "j1",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Times:   []string{"08:00", "20:00"},
		},
	}
	morning := time.Date(2026, 7, 7, 8, 5, 0, 0, time.Local)
	evening := time.Date(2026, 7, 7, 20, 3, 0, 0, time.Local)
	if got, want := SlotKey(job, morning), "2026-07-07T08:00"; got != want {
		t.Fatalf("morning slot: got %q want %q", got, want)
	}
	if got, want := SlotKey(job, evening), "2026-07-07T20:00"; got != want {
		t.Fatalf("evening slot: got %q want %q", got, want)
	}
}

func TestListUpcomingMultipleTimes(t *testing.T) {
	now := time.Date(2026, 7, 7, 9, 0, 0, 0, time.Local)
	jobs := []models.BackupJob{{
		ID:   "j1",
		Name: "test",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Times:   []string{"08:00", "20:00"},
		},
	}}
	up := ListUpcoming(jobs, now, 5)
	if len(up) < 2 {
		t.Fatalf("expected at least 2 upcoming runs, got %d", len(up))
	}
	if up[0].RunAt != "2026-07-07 20:00" {
		t.Fatalf("first upcoming: got %q", up[0].RunAt)
	}
}

func TestSyncTimesFromLegacyTime(t *testing.T) {
	s := models.Schedule{Time: "08:30"}
	SyncTimes(&s)
	if len(s.Times) != 1 || s.Times[0] != "08:30" {
		t.Fatalf("times: %#v", s.Times)
	}
}
