package config

import "pbs-win-backup/internal/models"

// IsEmpty reports whether config has no user-defined destinations or jobs.
func IsEmpty(cfg *models.Config) bool {
	if cfg == nil {
		return true
	}
	return len(cfg.Destinations) == 0 && len(cfg.Servers) == 0 && len(cfg.Jobs) == 0
}
