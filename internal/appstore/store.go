package appstore

import (
	"sync"
	"time"

	"pbs-win-backup/internal/backup"
	"pbs-win-backup/internal/backupqueue"
	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/credential"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/notify"
	"pbs-win-backup/internal/schedule"
	"pbs-win-backup/internal/service"
)

// Store holds the in-memory config guarded by a mutex.
type Store struct {
	mu  sync.RWMutex
	cfg *models.Config
}

// New returns a store with the given config snapshot.
func New(cfg *models.Config) *Store {
	return &Store{cfg: cfg}
}

// Get returns a clone of the current config.
func (s *Store) Get() *models.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return config.Clone(s.cfg)
}

// Replace swaps the in-memory config (caller must hold no locks elsewhere).
func (s *Store) Replace(cfg *models.Config) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}

// Update runs fn against the latest on-disk config under write lock and saves on success.
func (s *Store) Update(fn func(*models.Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	diskCfg, err := config.LoadResilient()
	if err != nil {
		return err
	}
	s.cfg = diskCfg
	if err := fn(s.cfg); err != nil {
		return err
	}
	return config.Save(s.cfg)
}

// SaveDestination upserts a destination and optional secret.
func (s *Store) SaveDestination(dest models.BackupDestination, secret string, b *i18n.Bundle) error {
	return s.Update(func(cfg *models.Config) error {
		if dest.ID == "" {
			dest.ID = config.NewDestinationID()
		}
		dest.Type = dest.NormalizedType()
		if dest.IsPBS() {
			if err := models.ValidatePBSURL(b, dest.URL); err != nil {
				return err
			}
		}
		found := false
		for i, d := range cfg.Destinations {
			if d.ID == dest.ID {
				cfg.Destinations[i] = dest
				found = true
				break
			}
		}
		if !found {
			cfg.Destinations = append(cfg.Destinations, dest)
		}
		if secret != "" {
			if err := credential.SetSecret(dest.ID, secret); err != nil {
				return b.Ewrap("cred.save_failed", map[string]string{"err": err.Error()}, err)
			}
		} else if !credential.HasSecret(dest.ID) {
			if _, err := credential.GetSecret(dest.ID); err != nil {
				if dest.IsPBS() {
					return b.E("cred.need_secret_schedule")
				}
				if dest.Username != "" && dest.Username != "anonymous" {
					return b.E("cred.need_password_schedule")
				}
			}
		}
		return nil
	})
}

// DeleteDestination removes a destination and its secret.
func (s *Store) DeleteDestination(destID string) error {
	return s.Update(func(cfg *models.Config) error {
		dests := make([]models.BackupDestination, 0, len(cfg.Destinations))
		for _, d := range cfg.Destinations {
			if d.ID != destID {
				dests = append(dests, d)
			}
		}
		cfg.Destinations = dests
		_ = credential.DeleteSecret(destID)
		return nil
	})
}

// ListDestinations returns a copy of configured destinations.
func (s *Store) ListDestinations() []models.BackupDestination {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.BackupDestination, len(s.cfg.Destinations))
	copy(out, s.cfg.Destinations)
	return out
}

// ListPBSServers returns PBS destinations as legacy server records.
func (s *Store) ListPBSServers() []models.PBSServer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.PBSServer
	for _, d := range s.cfg.Destinations {
		if d.IsPBS() {
			out = append(out, d.ToPBSServer())
		}
	}
	return out
}

// SaveJob upserts a backup job and optional encryption passphrase.
func (s *Store) SaveJob(job models.BackupJob, passphrase string, b *i18n.Bundle) error {
	if err := s.Update(func(cfg *models.Config) error {
		if job.ID == "" {
			job.ID = config.NewJobID()
		}
		isNewJob := true
		for _, j := range cfg.Jobs {
			if j.ID == job.ID {
				isNewJob = false
				break
			}
		}
		if isNewJob {
			job.VerifyAfterBackup = true
		}
		if job.BackupID == "" {
			job.BackupID = backup.Hostname()
		}
		if job.Exclusions == nil {
			job.Exclusions = []string{}
		}
		job.NotifyBackup = notify.NormalizeJobNotifyMode(job.NotifyBackup)
		job.NotifyRestore = notify.NormalizeJobNotifyMode(job.NotifyRestore)
		if job.Schedule.Time != "" || len(job.Schedule.Times) > 0 {
			schedule.ReconcileSchedule(&job.Schedule)
		} else {
			schedule.NormalizeSchedule(&job.Schedule)
		}
		if job.Schedule.Enabled {
			destID := job.EffectiveDestinationID()
			if destID == "" {
				return b.E("job.need_destination")
			}
			if _, err := credential.GetSecret(destID); err != nil {
				return b.E("job.need_cred_schedule")
			}
		}
		destID := job.EffectiveDestinationID()
		if destID != "" {
			job.DestinationID = destID
			job.ServerID = destID
		}
		if job.EncryptionEnabled {
			if passphrase != "" {
				if err := credential.SetPassphrase(job.ID, passphrase); err != nil {
					return b.Ewrap("pass.save_failed", map[string]string{"err": err.Error()}, err)
				}
			} else if _, err := credential.GetPassphrase(job.ID); err != nil {
				return b.E("pass.need_for_encryption")
			}
		} else {
			_ = credential.DeletePassphrase(job.ID)
		}
		found := false
		for i, j := range cfg.Jobs {
			if j.ID == job.ID {
				cfg.Jobs[i] = job
				found = true
				break
			}
		}
		if !found {
			cfg.Jobs = append(cfg.Jobs, job)
		}
		return nil
	}); err != nil {
		return err
	}
	service.ClearScheduleClaims(job.ID)
	_ = service.NudgeScheduler()
	return nil
}

// DeleteJob removes a job and related credentials.
func (s *Store) DeleteJob(jobID string) error {
	if err := s.Update(func(cfg *models.Config) error {
		jobs := make([]models.BackupJob, 0, len(cfg.Jobs))
		for _, j := range cfg.Jobs {
			if j.ID != jobID {
				jobs = append(jobs, j)
			}
		}
		cfg.Jobs = jobs
		_ = credential.DeletePassphrase(jobID)
		return nil
	}); err != nil {
		return err
	}
	service.ClearScheduleClaims(jobID)
	_ = backupqueue.RemoveJob(jobID)
	_ = service.NudgeScheduler()
	return nil
}

// ListJobs returns a copy of configured jobs.
func (s *Store) ListJobs() []models.BackupJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.BackupJob, len(s.cfg.Jobs))
	copy(out, s.cfg.Jobs)
	return out
}

// FindJob returns a job by ID.
func (s *Store) FindJob(jobID string, b *i18n.Bundle) (*models.BackupJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.cfg.Jobs {
		if s.cfg.Jobs[i].ID == jobID {
			j := s.cfg.Jobs[i]
			return &j, nil
		}
	}
	return nil, b.Ef("job.not_found", nil)
}

// SaveSettings updates app settings and optional SMTP password.
func (s *Store) SaveSettings(settings models.AppSettings, smtpPassword string, b *i18n.Bundle) error {
	return s.Update(func(cfg *models.Config) error {
		if smtpPassword != "" {
			if err := credential.SetSMTPPassword(smtpPassword); err != nil {
				return b.Ewrap("smtp.save_failed", map[string]string{"err": err.Error()}, err)
			}
		}
		notify.EnableNotifyWhenSMTP(&settings)
		cfg.Settings = settings
		config.NormalizeSettings(&cfg.Settings)
		return nil
	})
}

// Settings returns current app settings.
func (s *Store) Settings() models.AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Settings
}

// ConfigSnapshot returns a clone of the current config for read-only hot paths.
func (s *Store) ConfigSnapshot() *models.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return config.Clone(s.cfg)
}

// ShouldRunCatchUpAfterJobSave reports whether GUI schedule loop should fire immediately.
func ShouldRunCatchUpAfterJobSave(job models.BackupJob) bool {
	if !job.Schedule.Enabled {
		return false
	}
	st := service.QueryStatus()
	if st.Installed && st.Running && !st.PendingDelete {
		return false
	}
	return schedule.Due(job, time.Now())
}
