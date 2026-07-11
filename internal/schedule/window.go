package schedule

import (
	"time"

	"pbs-win-backup/internal/models"
)

// MatchesWindow returns true if now is within the scheduled window (exact minute or catch-up grace).
func MatchesWindow(job models.BackupJob, now time.Time) bool {
	return Due(job, now)
}

func weekdayAllowed(days []int, wd time.Weekday) bool {
	uiDay := WeekdayUI(wd)
	for _, d := range days {
		if d == uiDay {
			return true
		}
	}
	return false
}

// MinuteKey returns a stable key for deduplicating runs within the same clock minute.
func MinuteKey(now time.Time) string {
	return now.Format("2006-01-02T15:04")
}

// DelayToNextBoundary returns wait until the next local clock minute (:00 seconds).
func DelayToNextBoundary(now time.Time) time.Duration {
	delay := time.Duration(60-now.Second())*time.Second - time.Duration(now.Nanosecond())
	if delay <= 0 {
		return time.Minute
	}
	return delay
}
