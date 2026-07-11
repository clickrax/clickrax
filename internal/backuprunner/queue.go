package backuprunner

import (
	"errors"
	"fmt"
	"time"

	"pbs-win-backup/internal/backup"
	"pbs-win-backup/internal/backupqueue"
	"pbs-win-backup/internal/backuplock"
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

// ScheduleHooks release or finalize scheduled slot state during queue submit.
type ScheduleHooks struct {
	EndRun            func(jobID, slotKey string)
	ReleaseClaim      func(job models.BackupJob, at time.Time)
	ReleaseClaimByKey func(jobID, slotKey string)
}

// SubmitInput configures queue submission for a backup item.
type SubmitInput struct {
	Item          backupqueue.Item
	CanStart      func() bool
	Launch        func(item backupqueue.Item) error
	FindJob       func(jobID string) (*models.BackupJob, error)
	SkipIfRunning bool
	QueuedMessage func(name string, position int) string
	SkipMessage   func(name string) string
	OnQueued      func(name string, position int)
	Schedule      ScheduleHooks
}

// Submit enqueues or immediately launches a backup item.
func Submit(in SubmitInput) error {
	if in.Item.JobName == "" && in.FindJob != nil {
		if job, err := in.FindJob(in.Item.JobID); err == nil {
			in.Item.JobName = job.Name
		}
	}
	_ = backupqueue.ClearStaleLock()
	if in.CanStart != nil && in.CanStart() {
		return in.Launch(in.Item)
	}

	job, err := in.FindJob(in.Item.JobID)
	if err != nil {
		if in.Schedule.EndRun != nil {
			in.Schedule.EndRun(in.Item.JobID, in.Item.SlotKey)
		}
		if in.Item.Trigger == "scheduled" && !in.Item.ScheduledAt.IsZero() && in.Schedule.ReleaseClaimByKey != nil {
			in.Schedule.ReleaseClaimByKey(in.Item.JobID, in.Item.SlotKey)
		}
		return err
	}
	if in.Item.Trigger == "scheduled" && in.SkipIfRunning && job.Schedule.SkipIfRunning {
		if in.Schedule.EndRun != nil {
			in.Schedule.EndRun(in.Item.JobID, in.Item.SlotKey)
		}
		if in.Schedule.ReleaseClaim != nil {
			in.Schedule.ReleaseClaim(*job, in.Item.ScheduledAt)
		}
		if in.SkipMessage != nil {
			eventlog.Info(in.SkipMessage(job.Name))
		}
		return nil
	}

	pos, err := backupqueue.Enqueue(in.Item)
	if err != nil {
		if errors.Is(err, backupqueue.ErrAlreadyQueued) {
			if in.Schedule.EndRun != nil {
				in.Schedule.EndRun(in.Item.JobID, in.Item.SlotKey)
			}
			if in.Schedule.ReleaseClaim != nil {
				in.Schedule.ReleaseClaim(*job, in.Item.ScheduledAt)
			}
			return nil
		}
		if in.Schedule.EndRun != nil {
			in.Schedule.EndRun(in.Item.JobID, in.Item.SlotKey)
		}
		if in.Schedule.ReleaseClaim != nil {
			in.Schedule.ReleaseClaim(*job, in.Item.ScheduledAt)
		}
		return err
	}
	if in.QueuedMessage != nil {
		eventlog.Info(in.QueuedMessage(in.Item.JobName, pos))
	}
	if in.OnQueued != nil {
		in.OnQueued(in.Item.JobName, pos)
	}
	return nil
}

// CanStartBackup reports whether a new backup worker may start now.
func CanStartBackup(engine *backup.Engine) bool {
	if engine == nil || engine.IsRunning() {
		return false
	}
	return backuplock.ClearStale()
}

// DrainQueue processes pending backup queue items.
func DrainQueue(engine *backup.Engine, launch func(item backupqueue.Item) error) {
	backupqueue.Drain(
		func() bool { return engine != nil && engine.IsRunning() },
		func() bool { return backuplock.ClearStale() },
		launch,
	)
}

// RetryProgress builds a standard retry progress event for the GUI.
func RetryProgress(job models.BackupJob, b *i18n.Bundle, attempt, maxRetries int, err error, delay fmt.Stringer) models.ProgressEvent {
	return models.ProgressEvent{
		JobID: job.ID, JobName: job.Name,
		Phase: models.PhasePreparing,
		Message: b.Tf("backup.retry", map[string]string{
			"err":     ShortenErr(err),
			"attempt": fmt.Sprintf("%d", attempt),
			"max":     fmt.Sprintf("%d", maxRetries),
			"delay":   delay.String(),
		}),
	}
}
