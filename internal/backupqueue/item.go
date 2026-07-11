package backupqueue

import (
	"time"

	"pbs-win-backup/internal/models"
	schedpkg "pbs-win-backup/internal/schedule"
)

// ItemFromJob builds a queue entry for manual or scheduled backup.
func ItemFromJob(job models.BackupJob, forceFull bool, scheduledAt time.Time) Item {
	trigger := "manual"
	slotKey := ""
	if !scheduledAt.IsZero() {
		trigger = "scheduled"
		slotKey = schedpkg.SlotKey(job, scheduledAt)
	}
	return Item{
		JobID:       job.ID,
		ForceFull:   forceFull,
		ScheduledAt: scheduledAt,
		SlotKey:     slotKey,
		Trigger:     trigger,
		EnqueuedAt:  time.Now(),
		JobName:     job.Name,
	}
}
