package schedule

import (
	"sort"

	"pbs-win-backup/internal/models"
)

// Times returns normalized schedule times (migrates legacy single Time field).
func Times(s models.Schedule) []string {
	if len(s.Times) > 0 {
		out := make([]string, 0, len(s.Times))
		for _, t := range s.Times {
			t = NormalizeClock(t)
			if t != "" {
				out = append(out, t)
			}
		}
		sort.Strings(out)
		return out
	}
	if s.Time != "" {
		return []string{NormalizeClock(s.Time)}
	}
	return nil
}

// SyncTimes copies Times to legacy Time and normalizes both fields.
func SyncTimes(s *models.Schedule) {
	if s == nil {
		return
	}
	if len(s.Times) == 0 && s.Time != "" {
		s.Times = []string{NormalizeClock(s.Time)}
	}
	out := make([]string, 0, len(s.Times))
	seen := map[string]struct{}{}
	for _, t := range s.Times {
		t = NormalizeClock(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	s.Times = out
	if len(s.Times) > 0 {
		s.Time = s.Times[0]
	} else {
		s.Time = ""
	}
}
