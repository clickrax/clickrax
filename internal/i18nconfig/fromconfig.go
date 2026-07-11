package i18nconfig

import (
	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/i18n"
)

func FromConfig() *i18n.Bundle {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return i18n.New("")
	}
	return i18n.New(cfg.Settings.Language)
}
