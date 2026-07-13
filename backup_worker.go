package main

import (
	"fmt"
	"time"

	"pbs-win-backup/internal/backuprunner"
	"pbs-win-backup/internal/backupqueue"
	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/notify"
	"pbs-win-backup/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) runBackupWorker(
	item backupqueue.Item,
	job models.BackupJob,
	dest *models.BackupDestination,
	secret string,
	exclusions []string,
	bandwidth int,
	settings models.AppSettings,
) {
	b := a.bundle()
	scheduledAt := item.ScheduledAt
	trigger := item.Trigger
	if trigger == "" {
		trigger = "manual"
	}

	defer func() {
		_ = backupqueue.ClearInflight()
		service.EndScheduledRun(item.JobID, item.SlotKey)
		a.processBackupQueue()
	}()
	var panicked bool
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			eventlog.Error(fmt.Sprintf("panic during backup %s: %v", job.Name, r))
			errMsg := b.Tf("backup.critical_panic", map[string]string{"err": fmt.Sprintf("%v", r)})
			result := models.JobRunResult{
				JobID: job.ID, JobName: job.Name,
				Status: "error", Error: errMsg, Message: errMsg,
			}
			if !scheduledAt.IsZero() {
				service.ReleaseSlotClaim(job, scheduledAt)
			}
			a.finishBackupRun(item, job, scheduledAt, trigger, result, b)
		}
	}()

	out := backuprunner.Execute(backuprunner.ExecuteInput{
		Ctx:        a.ctx,
		Engine:     a.engine,
		Item:       item,
		Job:        job,
		Dest:       dest,
		Secret:     secret,
		Exclusions: exclusions,
		Bandwidth:  bandwidth,
		Settings:   settings,
		EmitProgress: func(ev models.ProgressEvent) {
			a.emitProgress(ev)
		},
		OnRetryMessage: func(attempt, maxRetries int, err error, delay time.Duration) models.ProgressEvent {
			return backuprunner.RetryProgress(job, i18n.New(settings.Language), attempt, maxRetries, err, delay)
		},
	})
	if out.Requeued {
		eventlog.Info("очередь: повторная постановка после блокировки (" + job.Name + ")")
		return
	}
	if panicked {
		return
	}
	service.RecordScheduleOutcome(job, scheduledAt, out.Result.Status)
	a.finishBackupRun(item, job, scheduledAt, trigger, out.Result, b)
}

func (a *App) finishBackupRun(
	item backupqueue.Item,
	job models.BackupJob,
	scheduledAt time.Time,
	trigger string,
	result models.JobRunResult,
	b *i18n.Bundle,
) {
	a.mu.Lock()
	a.history = append([]models.JobRunResult{result}, a.history...)
	if len(a.history) > 200 {
		a.history = a.history[:200]
	}
	a.mu.Unlock()

	title := branding.Name
	out := backuprunner.FinishCommon(backuprunner.FinishInput{
		Result:   result,
		Job:      job,
		Settings: a.store.Settings(),
	})
	if out.EmailErr != nil {
		notify.ShowToast(title, b.Tf("notify.email_failed", map[string]string{"err": out.EmailErr.Error()}))
	}

	msg := fmt.Sprintf("%s: %s (%s)", result.JobName, result.Status, result.BackupType)
	if result.Error != "" {
		msg = result.JobName + ": " + result.Error
	}
	notify.ShowToast(title, msg)

	finalPhase := models.PhaseDone
	finalMsg := b.Tf("backup.toast_done", map[string]string{"type": result.BackupType})
	finalPct := 100.0
	if result.Status == "cancelled" {
		finalPhase = models.PhaseCancelled
		finalMsg = b.T("backup.toast_cancelled")
		finalPct = 0
	} else if result.Status == "error" {
		finalPhase = models.PhaseError
		finalMsg = result.Error
	} else if result.Status == "warning" {
		finalPhase = models.PhaseDone
		finalMsg = result.Message
		if finalMsg == "" {
			finalMsg = b.T("backup.toast_warning")
		}
	} else if result.Status != "ok" {
		finalPhase = models.PhaseError
		finalMsg = result.Status
	}

	a.mu.RLock()
	lp := a.lastProgress
	a.mu.RUnlock()

	doneEv := models.ProgressEvent{
		JobID:            job.ID,
		JobName:          job.Name,
		Trigger:          trigger,
		Phase:            finalPhase,
		Percent:          finalPct,
		BytesTransferred: result.BytesTransferred,
		BytesReused:      result.BytesReused,
		BackupType:       result.BackupType,
		Message:          finalMsg,
	}
	if lp.JobID == job.ID {
		if lp.FilesDone > 0 {
			doneEv.FilesDone = lp.FilesDone
		}
		if lp.FilesTotal > 0 {
			doneEv.FilesTotal = lp.FilesTotal
		}
		if lp.FilesSkipped > 0 {
			doneEv.FilesSkipped = lp.FilesSkipped
		}
		if lp.FilesChanged > 0 {
			doneEv.FilesChanged = lp.FilesChanged
		}
		if lp.ChunksNew > 0 {
			doneEv.ChunksNew = lp.ChunksNew
		}
		if lp.ChunksReused > 0 {
			doneEv.ChunksReused = lp.ChunksReused
		}
		if lp.BytesTotal > 0 {
			doneEv.BytesTotal = lp.BytesTotal
		}
		if doneEv.BytesTransferred == 0 && lp.BytesTransferred > 0 {
			doneEv.BytesTransferred = lp.BytesTransferred
		}
		if doneEv.BytesReused == 0 && lp.BytesReused > 0 {
			doneEv.BytesReused = lp.BytesReused
		}
	}
	a.emitProgress(doneEv)
	runtime.EventsEmit(a.ctx, "backup-finished", result.ToRecord())
}
