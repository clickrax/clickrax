package main

import (
	"fmt"

	"pbs-win-backup/internal/backuprunner"
	"pbs-win-backup/internal/backupqueue"
	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) findJob(jobID string) (*models.BackupJob, error) {
	return a.store.FindJob(jobID, a.bundle())
}

func (a *App) submitBackup(item backupqueue.Item) error {
	b := a.bundle()
	return backuprunner.Submit(backuprunner.SubmitInput{
		Item: item,
		CanStart: func() bool {
			return backuprunner.CanStartBackup(a.engine)
		},
		Launch: a.launchBackupWorker,
		FindJob: func(jobID string) (*models.BackupJob, error) {
			return a.store.FindJob(jobID, b)
		},
		SkipIfRunning: true,
		QueuedMessage: func(name string, pos int) string {
			return b.Tf("job.queued", map[string]string{
				"name": name,
				"n":    fmt.Sprintf("%d", pos),
			})
		},
		SkipMessage: func(name string) string {
			return "расписание " + name + ": пропуск — уже выполняется другой бэкап"
		},
		OnQueued: func(name string, pos int) {
			if a.ctx == nil {
				return
			}
			runtime.EventsEmit(a.ctx, "backup-queued", map[string]interface{}{
				"job_id":   item.JobID,
				"job_name": name,
				"position": pos,
			})
		},
		Schedule: backuprunner.ScheduleHooks{
			EndRun:              service.EndScheduledRun,
			ReleaseClaim:        service.ReleaseSlotClaim,
			ReleaseClaimByKey:   service.ReleaseSlotClaimByKey,
		},
	})
}

func (a *App) launchBackupWorker(item backupqueue.Item) error {
	if cfg, err := config.LoadResilient(); err == nil {
		a.mu.Lock()
		a.store.Replace(cfg)
		a.mu.Unlock()
	}
	params, err := backuprunner.ResolveLaunch(a.store.ConfigSnapshot(), item, a.bundle())
	if err != nil {
		if job, jerr := a.store.FindJob(item.JobID, a.bundle()); jerr == nil {
			service.ReleaseLaunchFailure(item, *job, item.ScheduledAt)
		} else {
			service.ReleaseLaunchFailure(item, models.BackupJob{ID: item.JobID}, item.ScheduledAt)
		}
		return err
	}
	go a.runBackupWorker(item, params.Job, params.Dest, params.Secret, params.Exclusions, params.Bandwidth, params.Settings)
	return nil
}

func (a *App) processBackupQueue() {
	backuprunner.DrainQueue(a.engine, a.launchBackupWorker)
}
