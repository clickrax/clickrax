package schedule

import (
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestClockDue_DSTSpringForward_MatchesActiveSlot(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("timezone data unavailable")
	}
	job := models.BackupJob{
		ID: "dst-job",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "02:30",
		},
	}
	// Spring-forward day: 02:30 wall time is skipped; clockDue and activeSlot must agree.
	candidates := []time.Time{
		time.Date(2026, 3, 8, 1, 45, 0, 0, loc),
		time.Date(2026, 3, 8, 3, 30, 0, 0, loc),
		time.Date(2026, 3, 8, 3, 35, 0, 0, loc),
		time.Date(2026, 3, 8, 4, 0, 0, 0, loc),
	}
	for _, now := range candidates {
		due := Due(job, now)
		slot, ok := activeSlot(job.Schedule, now)
		if due != ok {
			t.Fatalf("now=%v: Due=%v activeSlot=%v (must match without UTC branch)", now, due, ok)
		}
		if due {
			want := slot.Format("2006-01-02T15:04")
			if got := SlotKey(job, now); got != want {
				t.Fatalf("now=%v: SlotKey=%q want %q", now, got, want)
			}
		}
	}
}
