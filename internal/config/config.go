package config

import (
	"encoding/json"
	"os"
	"sync"

	"pbs-win-backup/internal/datalock"
	"pbs-win-backup/internal/locale"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
	"pbs-win-backup/internal/schedule"

	"github.com/google/uuid"
)

var mu sync.RWMutex

func DefaultExclusions() []string {
	return []string{
		"System Volume Information",
		"$RECYCLE.BIN",
		"$Recycle.Bin",
		"Recovery",
		"Config.Msi",
		"pagefile.sys",
		"hiberfil.sys",
		"swapfile.sys",
		"DumpStack.log.tmp",
	}
}

func DefaultSettings() models.AppSettings {
	return models.AppSettings{
		Language:           locale.SystemPreferred(),
		StartWithWindows:   false,
		MinimizeToTray:     true,
		DefaultExclusions:  DefaultExclusions(),
		BandwidthMbps:      0,
		ChunkWorkers:       0,
		NetworkTimeoutSec:  120,
		NetworkRetries:     5,
		SkipBehavior:       "warning",
		CriticalErrorLimit: 100,
		RestoreOverwrite:   "ask",
		LogLevel:           "info",
		CheckUpdates:       true,
		WebhookURL:         "",
		NotifyBackup:       "off",
		NotifyRestore:      "off",
	}
}

func DefaultConfig() *models.Config {
	cfg := &models.Config{
		Version:      1,
		Destinations: []models.BackupDestination{},
		Jobs:         []models.BackupJob{},
		Settings:     DefaultSettings(),
	}
	NormalizeSettings(&cfg.Settings)
	return cfg
}

func Load() (*models.Config, error) {
	mu.Lock()
	defer mu.Unlock()

	path, err := paths.ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return createDefaultConfigFile()
		}
		return nil, err
	}

	cfg, migrated, err := parseConfigFile(path, data)
	if err != nil {
		return nil, err
	}
	if migrated {
		persistMigration(cfg)
	}
	return cfg, nil
}

func normalizeConfig(cfg *models.Config) bool {
	migrated := false
	if len(cfg.Destinations) == 0 && len(cfg.Servers) > 0 {
		for _, s := range cfg.Servers {
			cfg.Destinations = append(cfg.Destinations, models.PBSServerToDestination(s))
		}
		migrated = true
	}
	for _, s := range cfg.Servers {
		found := false
		for _, d := range cfg.Destinations {
			if d.ID == s.ID {
				found = true
				break
			}
		}
		if !found {
			cfg.Destinations = append(cfg.Destinations, models.PBSServerToDestination(s))
			migrated = true
		}
	}
	if len(cfg.Servers) > 0 {
		cfg.Servers = nil
		migrated = true
	}
	for i := range cfg.Jobs {
		j := &cfg.Jobs[i]
		if j.DestinationID == "" && j.ServerID != "" {
			j.DestinationID = j.ServerID
			migrated = true
		}
		if j.ServerID == "" && j.DestinationID != "" {
			j.ServerID = j.DestinationID
		}
	}
	if enableNotifyWhenSMTP(&cfg.Settings) {
		migrated = true
	}
	return migrated
}

func enableNotifyWhenSMTP(s *models.AppSettings) bool {
	if s == nil || s.SMTP.Host == "" || s.SMTP.From == "" || s.SMTP.To == "" {
		return false
	}
	if s.NotifyBackup != "" && s.NotifyBackup != "off" {
		return false
	}
	s.NotifyBackup = "always"
	return true
}

func normalizeJobs(cfg *models.Config) {
	for i := range cfg.Jobs {
		schedule.ReconcileSchedule(&cfg.Jobs[i].Schedule)
	}
}

func Save(cfg *models.Config) error {
	mu.Lock()
	defer mu.Unlock()
	return datalock.With("config", func() error {
		if cfg != nil {
			NormalizeSettings(&cfg.Settings)
		}
		return saveUnlocked(cfg)
	})
}

func saveUnlocked(cfg *models.Config) error {
	normalizeConfig(cfg)
	normalizeJobs(cfg)
	path, err := paths.ConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := paths.AtomicWriteSensitive(path, data, 0o600); err != nil {
		return err
	}
	return writeConfigSignature(path, data)
}

func Clone(cfg *models.Config) *models.Config {
	if cfg == nil {
		return nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil
	}
	var out models.Config
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return &out
}

func NewServerID() string {
	return uuid.NewString()
}

func NewDestinationID() string {
	return uuid.NewString()
}

func NewJobID() string {
	return uuid.NewString()
}
