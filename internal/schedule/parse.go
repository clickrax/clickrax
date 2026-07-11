package schedule

import (
	"fmt"
	"strings"
	"time"

	"pbs-win-backup/internal/i18n"
)

func ParseClock(s string) (hour, minute int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, i18n.New("").E("schedule.empty_time")
	}
	for _, layout := range []string{"15:04:05", "15:04"} {
		t, e := time.Parse(layout, s)
		if e == nil {
			return t.Hour(), t.Minute(), nil
		}
	}
	return 0, 0, i18n.New("").Ef("schedule.invalid_time", map[string]string{"time": s})
}

func NormalizeClock(s string) string {
	h, m, err := ParseClock(s)
	if err != nil {
		return strings.TrimSpace(s)
	}
	return fmt.Sprintf("%02d:%02d", h, m)
}

func ClockMatches(now time.Time, scheduleTime string) bool {
	h, m, err := ParseClock(scheduleTime)
	if err != nil {
		return false
	}
	return now.Hour() == h && now.Minute() == m
}
