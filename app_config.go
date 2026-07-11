package main

import (
	"time"

	"pbs-win-backup/internal/appstore"
	"pbs-win-backup/internal/locale"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/notify"
	"pbs-win-backup/internal/pbsbackup"
)

func (a *App) GetConfig() *models.Config {
	return a.store.Get()
}

func (a *App) SaveDestination(dest models.BackupDestination, secret string) error {
	return a.store.SaveDestination(dest, secret, a.bundle())
}

func (a *App) SaveServer(server models.PBSServer, secret string) error {
	return a.SaveDestination(models.PBSServerToDestination(server), secret)
}

func (a *App) DeleteDestination(destID string) error {
	return a.store.DeleteDestination(destID)
}

func (a *App) DeleteServer(serverID string) error {
	return a.DeleteDestination(serverID)
}

func (a *App) ListDestinations() []models.BackupDestination {
	return a.store.ListDestinations()
}

func (a *App) ListServers() []models.PBSServer {
	return a.store.ListPBSServers()
}

func (a *App) SaveJob(job models.BackupJob, passphrase string) error {
	if err := a.store.SaveJob(job, passphrase, a.bundle()); err != nil {
		return err
	}
	a.applyScheduleAfterJobSave(job)
	return nil
}

func (a *App) applyScheduleAfterJobSave(job models.BackupJob) {
	if appstore.ShouldRunCatchUpAfterJobSave(job) {
		go a.runScheduledJobs(time.Now())
	}
}

func (a *App) DeleteJob(jobID string) error {
	return a.store.DeleteJob(jobID)
}

func (a *App) ListJobs() []models.BackupJob {
	return a.store.ListJobs()
}

func (a *App) SaveSettings(settings models.AppSettings, smtpPassword string) error {
	settings.Language = locale.Normalize(settings.Language)
	settings.NotifyBackup = notify.NormalizeNotifyMode(settings.NotifyBackup)
	settings.NotifyRestore = notify.NormalizeNotifyMode(settings.NotifyRestore)
	if settings.SMTP.Port == 0 {
		settings.SMTP.Port = 587
	}
	if err := notify.ValidateWebhookURL(settings.WebhookURL); err != nil {
		return err
	}
	if err := a.store.SaveSettings(settings, smtpPassword, a.bundle()); err != nil {
		return err
	}
	pbsbackup.SetChunkWorkersSetting(settings.ChunkWorkers)
	a.emitLocalizedProgress(settings.Language)
	return nil
}

func (a *App) GetSettings() models.AppSettings {
	return a.store.Settings()
}

func (a *App) sendRestoreEmail(job models.BackupJob, result models.JobRunResult) {
	settings := a.store.Settings()
	go func() {
		_ = notify.DispatchRestoreEmail(settings, job, result)
	}()
}
