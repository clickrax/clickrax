package service

import (
	"context"
	"fmt"
	"time"

	"pbs-win-backup/internal/backuprunner"
	"pbs-win-backup/internal/backupqueue"
	"pbs-win-backup/internal/config"
	appeventlog "pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/history"
	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/models"
	schedpkg "pbs-win-backup/internal/schedule"
)

func (h *handler) canStartBackup() bool {
	return backuprunner.CanStartBackup(h.engine)
}

func (h *handler) processBackupQueue() {
	backuprunner.DrainQueue(h.engine, h.launchQueuedBackup)
}

func (h *handler) submitScheduledBackup(job models.BackupJob, now time.Time) error {
	item := backupqueue.ItemFromJob(job, schedpkg.ShouldForceFull(job.Schedule, now), now)
	b := i18nconfig.FromConfig()
	err := backuprunner.Submit(backuprunner.SubmitInput{
		Item: item,
		CanStart: func() bool {
			return h.canStartBackup()
		},
		Launch: h.startQueuedBackup,
		FindJob: func(jobID string) (*models.BackupJob, error) {
			cfg, err := config.Load()
			if err != nil {
				return nil, err
			}
			for i := range cfg.Jobs {
				if cfg.Jobs[i].ID == jobID {
					j := cfg.Jobs[i]
					return &j, nil
				}
			}
			return nil, b.Ef("job.not_found", nil)
		},
		SkipIfRunning: true,
		QueuedMessage: func(name string, pos int) string {
			return b.Tf("schedule.queued", map[string]string{
				"name": name,
				"n":    fmt.Sprintf("%d", pos),
			})
		},
		SkipMessage: func(name string) string {
			return b.Tf("schedule.skip_running", map[string]string{"name": name})
		},
		Schedule: backuprunner.ScheduleHooks{
			EndRun:            EndScheduledRun,
			ReleaseClaim:      ReleaseSlotClaim,
			ReleaseClaimByKey: ReleaseSlotClaimByKey,
		},
	})
	if err != nil {
		h.logScheduledStartError(job, now, err)
	}
	return err
}

func (h *handler) logScheduledStartError(job models.BackupJob, now time.Time, err error) {
	msg := "расписание " + job.Name + ": " + err.Error()
	appeventlog.Error(msg)
	_ = history.Append(models.JobRunResult{
		JobID:      job.ID,
		JobName:    job.Name,
		Status:     "error",
		Trigger:    "scheduled",
		Error:      msg,
		StartedAt:  now,
		FinishedAt: now,
	})
}

func (h *handler) launchQueuedBackup(item backupqueue.Item) error {
	if err := h.startQueuedBackup(item); err != nil {
		appeventlog.Error("очередь бэкапов: " + err.Error())
		return err
	}
	return nil
}

func (h *handler) startQueuedBackup(item backupqueue.Item) error {
	cfg, err := config.Load()
	if err != nil {
		ReleaseLaunchFailure(item, models.BackupJob{ID: item.JobID}, item.ScheduledAt)
		return backupqueue.PermanentStartError(err)
	}
	params, err := backuprunner.ResolveLaunch(cfg, item, i18nconfig.FromConfig())
	if err != nil {
		var job models.BackupJob
		for i := range cfg.Jobs {
			if cfg.Jobs[i].ID == item.JobID {
				job = cfg.Jobs[i]
				break
			}
		}
		ReleaseLaunchFailure(item, job, item.ScheduledAt)
		return err
	}

	go func(j models.BackupJob, d models.BackupDestination, sec string, settings models.AppSettings) {
		defer func() {
			if r := recover(); r != nil {
				panicErr := fmt.Errorf("%w: %v", backupqueue.ErrQueuePanic, r)
				backupqueue.RecordDeadLetter(item, panicErr)
				if !item.ScheduledAt.IsZero() {
					ReleaseSlotClaimByKey(item.JobID, item.SlotKey)
				}
			}
			_ = backupqueue.ClearInflight()
			EndScheduledRun(j.ID, item.SlotKey)
			h.processBackupQueue()
		}()
		ctx := h.shutdownCtx
		if ctx == nil {
			ctx = context.Background()
		}
		h.runScheduledJob(ctx, j, &d, sec, params.Exclusions, params.Bandwidth, settings, item)
	}(params.Job, *params.Dest, params.Secret, params.Settings)
	return nil
}

func (h *handler) runScheduledJob(
	ctx context.Context,
	job models.BackupJob,
	dest *models.BackupDestination,
	secret string,
	exclusions []string,
	bandwidth int,
	settings models.AppSettings,
	item backupqueue.Item,
) {
	out := backuprunner.Execute(backuprunner.ExecuteInput{
		Ctx:        ctx,
		Engine:     h.engine,
		Item:       item,
		Job:        job,
		Dest:       dest,
		Secret:     secret,
		Exclusions: exclusions,
		Bandwidth:  bandwidth,
		Settings:   settings,
	})
	if out.Requeued {
		appeventlog.Info("очередь: повторная постановка после блокировки (" + job.Name + ")")
		return
	}
	RecordScheduleOutcome(job, item.ScheduledAt, out.Result.Status)
	if out.Err != nil {
		appeventlog.Error("расписание " + job.Name + ": " + out.Err.Error())
	} else {
		appeventlog.Info("расписание " + job.Name + ": OK")
	}
	backuprunner.FinishCommon(backuprunner.FinishInput{
		Result:         out.Result,
		Job:            job,
		Settings:       settings,
		ReloadFromDisk: true,
	})
}

func resolveDestination(cfg *models.Config, job models.BackupJob) (*models.BackupDestination, string, bool) {
	params, err := backuprunner.ResolveLaunch(cfg, backupqueue.Item{JobID: job.ID}, i18nconfig.FromConfig())
	if err != nil {
		return nil, "", false
	}
	return params.Dest, params.Secret, true
}

func scheduledTrigger(item backupqueue.Item) string {
	if item.Trigger != "" {
		return item.Trigger
	}
	return "scheduled"
}
