package schedule

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"pbs-win-backup/internal/models"
)

// NextRun returns human-readable next scheduled run across all jobs.
func NextRun(jobs []models.BackupJob, now time.Time) string {
	if item, ok := NextScheduled(jobs, now); ok {
		return item.RunAt + " (" + item.BackupType + ")"
	}
	return ""
}

// NextScheduled returns the nearest upcoming scheduled run.
func NextScheduled(jobs []models.BackupJob, now time.Time) (models.ScheduledRunInfo, bool) {
	all := ListUpcoming(jobs, now, 1)
	if len(all) == 0 {
		return models.ScheduledRunInfo{}, false
	}
	return all[0], true
}

// ListUpcoming returns upcoming scheduled runs sorted by time.
func ListUpcoming(jobs []models.BackupJob, now time.Time, limit int) []models.ScheduledRunInfo {
	var out []models.ScheduledRunInfo
	for i := range jobs {
		job := &jobs[i]
		if !job.Schedule.Enabled || len(Times(job.Schedule)) == 0 {
			continue
		}
		label := strings.Join(Times(job.Schedule), ", ")
		for _, c := range nextOccurrences(job.Schedule, now, 14) {
			out = append(out, models.ScheduledRunInfo{
				JobID:      job.ID,
				JobName:    job.Name,
				RunAt:      c.Format("2006-01-02 15:04"),
				BackupType: DescribeRun(job.Schedule, c),
				TimesLabel: label,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RunAt < out[j].RunAt })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func nextOccurrences(s models.Schedule, now time.Time, days int) []time.Time {
	var out []time.Time
	for _, tstr := range Times(s) {
		h, m, err := ParseClock(tstr)
		if err != nil {
			continue
		}
		for d := 0; d < days; d++ {
			day := now.AddDate(0, 0, d)
			if s.Type == "weekly" && len(s.Weekdays) > 0 {
				wd := int(day.Weekday())
				if wd == 0 {
					wd = 7
				}
				ok := false
				for _, w := range s.Weekdays {
					if w == wd {
						ok = true
						break
					}
				}
				if !ok {
					continue
				}
			}
			candidate := time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, now.Location())
			if candidate.After(now) {
				out = append(out, candidate)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func FormatNext(job models.BackupJob) string {
	if !job.Schedule.Enabled {
		return "—"
	}
	times := Times(job.Schedule)
	if len(times) == 0 {
		return "—"
	}
	return fmt.Sprintf("%s %s", job.Schedule.Type, strings.Join(times, ", "))
}
