package service

import (
	"sync"
	"time"

	"pbs-win-backup/internal/models"
)

var activeScheduled sync.Map // runKey -> struct{}

func scheduledRunKey(jobID, slotKey string) string {
	if slotKey != "" {
		return jobID + "|" + slotKey
	}
	return jobID
}

// BeginScheduledRun returns false if this job slot already has a scheduled run in progress.
func BeginScheduledRun(jobID, slotKey string) bool {
	key := scheduledRunKey(jobID, slotKey)
	_, loaded := activeScheduled.LoadOrStore(key, struct{}{})
	return !loaded
}

// EndScheduledRun marks the scheduled run finished (success or failure).
func EndScheduledRun(jobID, slotKey string) {
	activeScheduled.Delete(scheduledRunKey(jobID, slotKey))
}

// SlotAlreadySucceeded reports whether this schedule slot completed successfully.
func SlotAlreadySucceeded(job models.BackupJob, now time.Time) bool {
	key := slotKey(job, now)
	st, err := loadScheduleState()
	if err != nil {
		return false
	}
	return st.LastRun[job.ID] == key
}
