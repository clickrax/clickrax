//go:build !windows

package config

import "pbs-win-backup/internal/models"

func RecoverFromQuarantine(string) (bool, error) { return false, nil }

func ConfigNeedsRecovery(*models.Config, string) bool { return false }

func LoadResilient() (*models.Config, error) { return Load() }
