package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pbs-win-backup/internal/backuplock"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
	schedpkg "pbs-win-backup/internal/schedule"
)

func ShouldRunJob(job models.BackupJob, now time.Time) bool {
	if !schedpkg.Due(job, now) {
		return false
	}
	st, err := loadScheduleState()
	if err != nil {
		return false
	}
	key := slotKey(job, now)
	return st.LastRun[job.ID] != key
}

// MatchesWindow reports whether the job is due at now (without claiming the slot).
func MatchesWindow(job models.BackupJob, now time.Time) bool {
	return schedpkg.Due(job, now)
}

// TryClaimMinute reserves this minute slot before starting backup.
func TryClaimMinute(jobID string, now time.Time) bool {
	return tryClaimScheduleSlot(jobID, schedpkg.MinuteKey(now))
}

// TryClaimMinuteForJob reserves the scheduled slot (preferred over TryClaimMinute).
func TryClaimMinuteForJob(job models.BackupJob, now time.Time) bool {
	return TryClaimSlot(job, now)
}

// ReleaseMinuteClaim frees the minute slot so a failed start can retry within catch-up.
func ReleaseMinuteClaim(jobID string, now time.Time) {
	releaseScheduleClaim(jobID, schedpkg.MinuteKey(now))
}

// ReleaseMinuteClaimForJob frees the scheduled slot after failure.
func ReleaseMinuteClaimForJob(job models.BackupJob, now time.Time) {
	ReleaseSlotClaim(job, now)
}

// RecordScheduleSuccess persists last successful schedule run for the job.
func RecordScheduleSuccess(jobID string, now time.Time) {
	key := schedpkg.MinuteKey(now)
	st, err := loadScheduleState()
	if err != nil {
		return
	}
	if st.LastRun == nil {
		st.LastRun = map[string]string{}
	}
	st.LastRun[jobID] = key
	_ = saveScheduleState(st)
}

func releaseScheduleClaim(jobID, key string) {
	path, err := scheduleClaimPath(jobID, key)
	if err != nil {
		return
	}
	_ = os.Remove(path)
}

func alreadyClaimed(jobID, key string) bool {
	path, err := scheduleClaimPath(jobID, key)
	if err != nil {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	_ = backuplock.ClearStaleFileLock(path)
	_, err = os.Stat(path)
	return err == nil
}

// AlreadyClaimed reports whether this job/minute slot was already taken.
func AlreadyClaimed(jobID, key string) bool {
	return alreadyClaimed(jobID, key)
}

// AlreadyClaimedForJob reports whether the scheduled slot already completed successfully.
func AlreadyClaimedForJob(job models.BackupJob, now time.Time) bool {
	return SlotAlreadySucceeded(job, now)
}

func tryClaimScheduleSlot(jobID, key string) bool {
	path, err := scheduleClaimPath(jobID, key)
	if err != nil {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false
	}
	_ = backuplock.ClearStaleFileLock(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	if _, err := fmt.Fprintf(f, "%d %d\n", os.Getpid(), time.Now().Unix()); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return false
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return false
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return false
	}
	return true
}

func scheduleClaimPath(jobID, key string) (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	safeKey := strings.ReplaceAll(key, ":", "-")
	return filepath.Join(dir, fmt.Sprintf("schedule_claim_%s_%s.lock", jobID, safeKey)), nil
}

// shouldRunJob kept for unit-style callers with daily schedule only.
func shouldRunJob(jobID string, scheduleTime string, now time.Time) bool {
	return ShouldRunJob(models.BackupJob{
		ID: jobID,
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    scheduleTime,
		},
	}, now)
}
