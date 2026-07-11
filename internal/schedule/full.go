package schedule

import (
	"strings"
	"time"

	"pbs-win-backup/internal/models"
)

// WeekdayUI converts Go weekday to UI convention: 1=Mon … 7=Sun.
func WeekdayUI(wd time.Weekday) int {
	d := int(wd)
	if d == 0 {
		return 7
	}
	return d
}

func fullBackupWeekday(s models.Schedule) int {
	fullDay := s.FullBackupWeekday
	if fullDay == 0 {
		return 7
	}
	return fullDay
}

func matchesFullWeekday(s models.Schedule, now time.Time) bool {
	fullDay := fullBackupWeekday(s)
	if WeekdayUI(now.Weekday()) != fullDay {
		return false
	}
	if s.Type == "weekly" && len(s.Weekdays) > 0 {
		for _, d := range s.Weekdays {
			if d == fullDay {
				return true
			}
		}
		return false
	}
	return true
}

func parseFullBackupAnchor(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local), true
}

func isBiweeklyFullWeek(s models.Schedule, now time.Time) bool {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	anchor, ok := parseFullBackupAnchor(s.FullBackupAnchor)
	if !ok {
		_, week := now.ISOWeek()
		return week%2 == 0
	}
	if day.Before(anchor) {
		return false
	}
	days := int(day.Sub(anchor).Hours() / 24)
	return (days/7)%2 == 0
}

// ShouldForceFull reports whether a scheduled run at now should be a full backup.
func ShouldForceFull(s models.Schedule, now time.Time) bool {
	mode := s.FullBackupMode
	if mode == "" {
		mode = "weekly"
	}
	if mode == "never" {
		return false
	}
	if !matchesFullWeekday(s, now) {
		return false
	}
	switch mode {
	case "biweekly":
		return isBiweeklyFullWeek(s, now)
	case "monthly":
		return now.Day() <= 7
	default:
		return true
	}
}

// NormalizeSchedule fills defaults for schedule fields.
func NormalizeSchedule(s *models.Schedule) {
	if s == nil {
		return
	}
	SyncTimes(s)
	if s.Type == "" && len(Times(*s)) > 0 {
		s.Type = "daily"
	}
	if s.FullBackupMode == "" {
		s.FullBackupMode = "weekly"
	}
	if s.FullBackupWeekday == 0 && s.FullBackupMode != "never" {
		s.FullBackupWeekday = 7
	}
	if s.Type == "weekly" && len(s.Weekdays) == 0 {
		s.Weekdays = []int{1, 2, 3, 4, 5}
	}
}

// ReconcileSchedule applies defaults for schedule fields without changing enabled state.
func ReconcileSchedule(s *models.Schedule) {
	if s == nil {
		return
	}
	NormalizeSchedule(s)
}

// DescribeRun returns canonical backup type code for a scheduled run.
func DescribeRun(s models.Schedule, now time.Time) string {
	if ShouldForceFull(s, now) {
		return "full"
	}
	return "incremental"
}
