package config

import (
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/models"
)

const (
	MaxNetworkRetries = 10
)

// NormalizeSettings clamps user-controlled settings and applies runtime side effects.
func NormalizeSettings(s *models.AppSettings) {
	if s == nil {
		return
	}
	if s.NetworkRetries <= 0 {
		s.NetworkRetries = 5
	}
	if s.NetworkRetries > MaxNetworkRetries {
		s.NetworkRetries = MaxNetworkRetries
	}
	eventlog.SetLevel(s.LogLevel)
}

// EffectiveNetworkRetries returns a bounded retry count for backup workers.
func EffectiveNetworkRetries(n int) int {
	if n <= 0 {
		return 3
	}
	if n > MaxNetworkRetries {
		return MaxNetworkRetries
	}
	return n
}
