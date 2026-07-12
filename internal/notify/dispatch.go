package notify

import (
	"pbs-win-backup/internal/credential"
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

func DispatchBackupEmail(settings models.AppSettings, job models.BackupJob, result models.JobRunResult) error {
	mode := EffectiveNotifyMode(job.NotifyBackup, settings.NotifyBackup)
	if !SMTPConfigured(settings.SMTP) {
		return nil
	}
	if !ShouldNotify(mode, result.Status) {
		b := i18n.New(settings.Language)
		eventlog.Info(b.Tf("notify.email_skipped", map[string]string{
			"mode":   mode,
			"status": result.Status,
		}))
		return nil
	}
	pw, err := credential.GetSMTPPassword()
	if err != nil {
		return err
	}
	b := i18n.New(settings.Language)
	return maybeSendEmail(settings, pw, mode, result, b.T("notify.kind_backup"))
}

func DispatchRestoreEmail(settings models.AppSettings, job models.BackupJob, result models.JobRunResult) error {
	mode := EffectiveNotifyMode(job.NotifyRestore, settings.NotifyRestore)
	if !ShouldNotify(mode, result.Status) || !SMTPConfigured(settings.SMTP) {
		return nil
	}
	pw, err := credential.GetSMTPPassword()
	if err != nil {
		return err
	}
	b := i18n.New(settings.Language)
	return maybeSendEmail(settings, pw, mode, result, b.T("notify.kind_restore"))
}
