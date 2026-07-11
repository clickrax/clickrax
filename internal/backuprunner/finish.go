package backuprunner

import (
	"pbs-win-backup/internal/backup"
	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/history"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/notify"
	"pbs-win-backup/internal/status"
)

// FinishInput configures post-backup persistence and notifications.
type FinishInput struct {
	Result         models.JobRunResult
	Job            models.BackupJob
	Settings       models.AppSettings
	ReloadFromDisk bool
}

// FinishOutput contains settings/job possibly refreshed from config.
type FinishOutput struct {
	Settings models.AppSettings
	Job      models.BackupJob
}

// FinishCommon persists history, status, webhook, and email notifications.
func FinishCommon(in FinishInput) FinishOutput {
	job := in.Job
	settings := in.Settings

	_ = history.Append(in.Result)
	if err := status.WriteLastStatus(status.FromJobResult(in.Result, backup.Hostname())); err != nil {
		eventlog.Error("не удалось записать last_status: " + err.Error())
	}
	if in.ReloadFromDisk {
		if cfg, err := config.Load(); err == nil {
			settings = cfg.Settings
			for i := range cfg.Jobs {
				if cfg.Jobs[i].ID == job.ID {
					job = cfg.Jobs[i]
					break
				}
			}
		}
	}
	if settings.WebhookURL != "" {
		if err := notify.SendWebhook(settings.WebhookURL, in.Result); err != nil {
			eventlog.Error("webhook: " + err.Error())
		}
	}
	if err := notify.DispatchBackupEmail(settings, job, in.Result); err != nil {
		eventlog.Error("SMTP: " + err.Error())
	}
	return FinishOutput{Settings: settings, Job: job}
}
