package service

import (
	"pbs-win-backup/internal/backupqueue"
	"pbs-win-backup/internal/models"
	"time"
)

// ReleaseLaunchFailure clears scheduled-run markers when a queue item cannot start.
func ReleaseLaunchFailure(item backupqueue.Item, job models.BackupJob, scheduledAt time.Time) {
	EndScheduledRun(item.JobID, item.SlotKey)
	if !scheduledAt.IsZero() {
		ReleaseSlotClaim(job, scheduledAt)
	} else if item.SlotKey != "" {
		ReleaseSlotClaimByKey(item.JobID, item.SlotKey)
	}
}

// RecordScheduleOutcome updates slot claims after a backup finishes.
func RecordScheduleOutcome(job models.BackupJob, scheduledAt time.Time, status string) {
	if scheduledAt.IsZero() {
		return
	}
	if status == "ok" || status == "warning" {
		RecordScheduleSlotHandledForJob(job, scheduledAt)
	} else {
		ReleaseSlotClaim(job, scheduledAt)
	}
}
