package schedule

import (
	"time"

	"pbs-win-backup/internal/models"
)

const CatchUpGrace = 30 * time.Minute

// Due reports whether the job should run at now (exact minute or catch-up within grace).
func Due(job models.BackupJob, now time.Time) bool {
	if !job.Schedule.Enabled {
		return false
	}
	if len(Times(job.Schedule)) == 0 {
		return false
	}
	sch := job.Schedule
	switch sch.Type {
	case "weekly":
		if len(sch.Weekdays) > 0 && !weekdayAllowed(sch.Weekdays, now.Weekday()) {
			return false
		}
	case "startup":
		return false
	}
	for _, t := range Times(job.Schedule) {
		if clockDue(now, t) {
			return true
		}
	}
	return false
}

func clockDue(now time.Time, scheduleTime string) bool {
	if ClockMatches(now, scheduleTime) {
		return true
	}
	h, m, err := ParseClock(scheduleTime)
	if err != nil {
		return false
	}
	loc := now.Location()
	scheduled := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, loc)
	if now.After(scheduled) && now.Sub(scheduled) <= CatchUpGrace {
		return true
	}
	prev := scheduled.Add(-24 * time.Hour)
	if now.After(prev) && now.Sub(prev) <= CatchUpGrace {
		return true
	}
	return false
}
