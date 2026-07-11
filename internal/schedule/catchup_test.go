package schedule

import (
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestDueCatchUpWithinGrace(t *testing.T) {
	job := models.BackupJob{
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "03:00",
		},
	}
	scheduled := time.Date(2026, 7, 6, 3, 0, 0, 0, time.Local)
	later := scheduled.Add(15 * time.Minute)
	if !Due(job, later) {
		t.Fatal("job should be due within catch-up grace after scheduled minute")
	}
	tooLate := scheduled.Add(2 * time.Hour)
	if Due(job, tooLate) {
		t.Fatal("job should not be due outside catch-up grace")
	}
}

func TestDueExactMinute(t *testing.T) {
	job := models.BackupJob{
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "02:00",
		},
	}
	now := time.Date(2026, 7, 6, 2, 0, 0, 0, time.Local)
	if !Due(job, now) {
		t.Fatal("exact minute should be due")
	}
}
