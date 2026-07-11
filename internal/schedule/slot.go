package schedule

import (
	"time"

	"pbs-win-backup/internal/models"
)

// SlotKey returns a stable key for the scheduled time slot on the day of now.
func SlotKey(job models.BackupJob, now time.Time) string {
	if slot, ok := activeSlot(job.Schedule, now); ok {
		return slot.Format("2006-01-02T15:04")
	}
	return MinuteKey(now)
}

func activeSlot(s models.Schedule, now time.Time) (time.Time, bool) {
	var best time.Time
	found := false
	for _, t := range Times(s) {
		h, m, err := ParseClock(t)
		if err != nil {
			continue
		}
		slot := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		if ClockMatches(now, t) || (now.After(slot) && now.Sub(slot) <= CatchUpGrace) {
			if !found || slot.After(best) {
				best = slot
				found = true
			}
		}
		prev := slot.Add(-24 * time.Hour)
		if now.After(prev) && now.Sub(prev) <= CatchUpGrace {
			if !found || prev.After(best) {
				best = prev
				found = true
			}
		}
	}
	return best, found
}

// SlotCompleted reports whether this scheduled slot was already claimed (running or done).
func SlotCompleted(job models.BackupJob, now time.Time, claimed func(jobID, slotKey string) bool) bool {
	if claimed == nil {
		return false
	}
	return claimed(job.ID, SlotKey(job, now))
}
