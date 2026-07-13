package backuprunner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"pbs-win-backup/internal/backup"
	"pbs-win-backup/internal/backupqueue"
	"pbs-win-backup/internal/backuplock"
	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/credential"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/pbsbackup"
	"pbs-win-backup/internal/retry"
)

// LaunchParams holds resolved inputs for starting a backup worker.
type LaunchParams struct {
	Job        models.BackupJob
	Dest       *models.BackupDestination
	Secret     string
	Exclusions []string
	Bandwidth  int
	Settings   models.AppSettings
}

// ResolveLaunch loads job, destination, and secret for a queue item.
func ResolveLaunch(cfg *models.Config, item backupqueue.Item, b *i18n.Bundle) (LaunchParams, error) {
	if cfg == nil {
		return LaunchParams{}, backupqueue.PermanentStartError(fmt.Errorf("config is nil"))
	}
	var job *models.BackupJob
	for i := range cfg.Jobs {
		if cfg.Jobs[i].ID == item.JobID {
			j := cfg.Jobs[i]
			job = &j
			break
		}
	}
	if job == nil {
		return LaunchParams{}, backupqueue.PermanentStartError(b.Ef("job.not_found", nil))
	}
	destID := job.EffectiveDestinationID()
	dest, ok := models.FindDestination(cfg, destID)
	if !ok || dest == nil {
		return LaunchParams{}, backupqueue.PermanentStartError(b.Ef("job.dest_secret_unavailable", nil))
	}
	if len(job.Sources) == 0 {
		return LaunchParams{}, backupqueue.PermanentStartError(b.Ef("job.no_sources", nil))
	}
	secret, err := credential.GetSecret(dest.ID)
	if err != nil {
		if dest.IsPBS() {
			return LaunchParams{}, backupqueue.PermanentStartError(b.Ef("job.secret_not_saved", nil))
		}
		return LaunchParams{}, backupqueue.PermanentStartError(b.Ef("job.password_not_saved", nil))
	}
	pbsbackup.SetChunkWorkersSetting(cfg.Settings.ChunkWorkers)
	exclusions := append([]string(nil), cfg.Settings.DefaultExclusions...)
	return LaunchParams{
		Job:        *job,
		Dest:       dest,
		Secret:     secret,
		Exclusions: exclusions,
		Bandwidth:  cfg.Settings.BandwidthMbps,
		Settings:   cfg.Settings,
	}, nil
}

// ExecuteInput configures a backup run with retries.
type ExecuteInput struct {
	Ctx            context.Context
	Engine         *backup.Engine
	Item           backupqueue.Item
	Job            models.BackupJob
	Dest           *models.BackupDestination
	Secret         string
	Exclusions     []string
	Bandwidth      int
	Settings       models.AppSettings
	OnRetryMessage func(attempt, maxRetries int, err error, delay time.Duration) models.ProgressEvent
	EmitProgress   func(models.ProgressEvent)
}

// ExecuteOutput is the result of a backup execution attempt.
type ExecuteOutput struct {
	Result   models.JobRunResult
	Err      error
	Requeued bool
}

// Execute runs a backup with network retries and queue contention handling.
func Execute(in ExecuteInput) ExecuteOutput {
	trigger := in.Item.Trigger
	if trigger == "" {
		trigger = "manual"
	}
	lang := in.Settings.Language
	b := i18n.New(lang)
	maxRetries := config.EffectiveNetworkRetries(in.Settings.NetworkRetries)

	var result models.JobRunResult
	var runErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if runErr == nil || !retry.IsRetryable(runErr) {
				break
			}
			delay := retry.Backoff(attempt-1, 2*time.Second)
			if in.EmitProgress != nil && in.OnRetryMessage != nil {
				in.EmitProgress(in.OnRetryMessage(attempt, maxRetries, runErr, delay))
			}
			if in.Ctx != nil {
				select {
				case <-time.After(delay):
				case <-in.Ctx.Done():
					return ExecuteOutput{Result: result, Err: in.Ctx.Err()}
				}
			} else {
				time.Sleep(delay)
			}
		}
		result, runErr = in.Engine.Run(in.Ctx, backup.RunParams{
			Job:               in.Job,
			Destination:       in.Dest,
			Secret:            in.Secret,
			GlobalExclusions:  in.Exclusions,
			ForceFull:         in.Item.ForceFull,
			BandwidthMbps:     in.Bandwidth,
			NetworkTimeoutSec: in.Settings.NetworkTimeoutSec,
			Trigger:           trigger,
			Lang:              lang,
		})
		if in.Item.FromQueue {
			willRetry := runErr != nil && retry.IsRetryable(runErr) && !errors.Is(runErr, context.Canceled) && attempt < maxRetries
			if !willRetry {
				_ = backupqueue.ClearInflight()
			}
		}
		if runErr != nil && in.Item.FromQueue && (backuplock.IsContention(runErr) || errors.Is(runErr, backup.ErrJobAlreadyRunning)) {
			if err := backupqueue.EnqueueFront(in.Item); err != nil {
				backupqueue.RecordDeadLetter(in.Item, err)
			}
			return ExecuteOutput{Result: result, Err: runErr, Requeued: true}
		}
		if runErr != nil && result.Status == "" {
			result.Status = "error"
			result.Error = runErr.Error()
		}
		if runErr != nil && !retry.IsRetryable(runErr) && !errors.Is(runErr, context.Canceled) {
			if in.EmitProgress != nil {
				in.EmitProgress(models.ProgressEvent{
					JobID: in.Job.ID, JobName: in.Job.Name,
					Phase:   models.PhaseError,
					Message: FriendlyError(b, runErr),
				})
			}
			break
		}
		if runErr == nil || errors.Is(runErr, context.Canceled) || attempt == maxRetries {
			break
		}
	}
	if runErr != nil && result.Status == "" {
		if errors.Is(runErr, context.Canceled) {
			result.Status = "cancelled"
			result.Error = b.T("backup.cancelled")
		} else {
			result.Status = "error"
			result.Error = runErr.Error()
		}
	}
	if result.Error != "" && result.Message == "" {
		result.Message = result.Error
	}
	return ExecuteOutput{Result: result, Err: runErr}
}
