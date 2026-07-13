package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"pbs-win-backup/internal/backuplock"
	"pbs-win-backup/internal/datalock"
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
	schedpkg "pbs-win-backup/internal/schedule"
)

const scheduleNudgeFile = "schedule_nudge"

// ClearStaleScheduleClaims removes schedule claim locks left by crashed processes.
func ClearStaleScheduleClaims() {
	dir, err := paths.DataDir()
	if err != nil {
		return
	}
	matches, err := filepath.Glob(filepath.Join(dir, "schedule_claim_*.lock"))
	if err != nil {
		return
	}
	for _, p := range matches {
		_ = backuplock.ClearStaleFileLock(p)
	}
}

func slotKey(job models.BackupJob, now time.Time) string {
	return schedpkg.SlotKey(job, now)
}

// ClearScheduleClaims removes claim locks and persisted slot state for a job
// so schedule edits take effect immediately (including catch-up re-runs).
func ClearScheduleClaims(jobID string) {
	dir, err := paths.DataDir()
	if err != nil {
		return
	}
	pattern := filepath.Join(dir, fmt.Sprintf("schedule_claim_%s_*.lock", jobID))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}
	for _, p := range matches {
		_ = os.Remove(p)
	}
	_ = datalock.With("schedule_state", func() error {
		st := scheduleState{LastRun: map[string]string{}}
		p, err := scheduleStatePath()
		if err != nil {
			return err
		}
		if data, readErr := os.ReadFile(p); readErr == nil {
			if err := json.Unmarshal(data, &st); err != nil {
				return err
			}
		}
		if st.LastRun == nil {
			st.LastRun = map[string]string{}
		}
		delete(st.LastRun, jobID)
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		return paths.AtomicWriteSensitive(p, data, 0o600)
	})
}

// NudgeScheduler signals a running service to re-check schedule without restart.
func NudgeScheduler() error {
	dir, err := paths.DataDir()
	if err != nil {
		return err
	}
	p := filepath.Join(dir, scheduleNudgeFile)
	return os.WriteFile(p, []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0o644)
}

// ConsumeScheduleNudge reports whether an external nudge was pending (and clears it).
func ConsumeScheduleNudge() bool {
	dir, err := paths.DataDir()
	if err != nil {
		return false
	}
	p := filepath.Join(dir, scheduleNudgeFile)
	if _, err := os.Stat(p); err != nil {
		return false
	}
	_ = os.Remove(p)
	return true
}

func tryClaimSlot(job models.BackupJob, now time.Time) bool {
	return tryClaimScheduleSlot(job.ID, slotKey(job, now))
}

func releaseSlotClaim(job models.BackupJob, now time.Time) {
	releaseScheduleClaim(job.ID, slotKey(job, now))
}

func alreadyClaimedSlot(job models.BackupJob, now time.Time) bool {
	return alreadyClaimed(job.ID, slotKey(job, now))
}

func recordSlotSuccess(job models.BackupJob, now time.Time) {
	key := slotKey(job, now)
	if err := datalock.With("schedule_state", func() error {
		st := scheduleState{LastRun: map[string]string{}}
		p, err := scheduleStatePath()
		if err != nil {
			return err
		}
		if data, readErr := os.ReadFile(p); readErr == nil {
			if err := json.Unmarshal(data, &st); err != nil {
				return err
			}
		}
		if st.LastRun == nil {
			st.LastRun = map[string]string{}
		}
		st.LastRun[job.ID] = key
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		return paths.AtomicWriteSensitive(p, data, 0o600)
	}); err != nil {
		eventlog.Error("schedule_state save failed for " + job.ID + ": " + err.Error())
		return
	}
	releaseScheduleClaim(job.ID, key)
}

// ReleaseSlotClaimByKey frees a slot claim using the persisted slot key from the queue item.
func ReleaseSlotClaimByKey(jobID, key string) {
	if jobID == "" || key == "" {
		return
	}
	releaseScheduleClaim(jobID, key)
}

// TryClaimSlot reserves the scheduled slot before starting backup.
func TryClaimSlot(job models.BackupJob, now time.Time) bool {
	return tryClaimSlot(job, now)
}

// ReleaseSlotClaim frees the slot after a failed run so catch-up can retry.
func ReleaseSlotClaim(job models.BackupJob, now time.Time) {
	releaseSlotClaim(job, now)
}

// AlreadyClaimedSlot reports whether the scheduled slot is taken.
func AlreadyClaimedSlot(job models.BackupJob, now time.Time) bool {
	return alreadyClaimedSlot(job, now)
}

// RecordScheduleSlotHandledForJob marks the scheduled slot handled (success, error, or cancel)
// so catch-up does not re-trigger every few seconds.
func RecordScheduleSlotHandledForJob(job models.BackupJob, now time.Time) {
	recordSlotSuccess(job, now)
}

// RecordScheduleSuccessForJob persists successful run for the scheduled slot.
func RecordScheduleSuccessForJob(job models.BackupJob, now time.Time) {
	RecordScheduleSlotHandledForJob(job, now)
}
