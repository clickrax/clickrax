package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"pbs-win-backup/internal/datalock"
	"pbs-win-backup/internal/locale"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
)

func trimJSONBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

func parseConfigFile(path string, data []byte) (*models.Config, bool, error) {
	data = trimJSONBOM(data)
	if err := ensureConfigSignature(path, data); err != nil {
		return nil, false, err
	}
	var loaded models.Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, false, err
	}
	if loaded.Settings.DefaultExclusions == nil {
		loaded.Settings.DefaultExclusions = DefaultExclusions()
	}
	normLang := locale.Normalize(loaded.Settings.Language)
	if loaded.Settings.Language != normLang {
		loaded.Settings.Language = normLang
	}
	migrated := normalizeConfig(&loaded)
	normalizeJobs(&loaded)
	NormalizeSettings(&loaded.Settings)
	return &loaded, migrated, nil
}

func createDefaultConfigFile() (*models.Config, error) {
	var cfg *models.Config
	err := datalock.With("config", func() error {
		path, err := paths.ConfigPath()
		if err != nil {
			return err
		}
		_, statErr := os.Stat(path)
		if statErr == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			loaded, migrated, err := parseConfigFile(path, data)
			if err != nil {
				return err
			}
			cfg = loaded
			if migrated {
				return saveUnlocked(loaded)
			}
			return nil
		}
		if !os.IsNotExist(statErr) {
			return statErr
		}
		dir, derr := paths.DataDir()
		if derr != nil {
			return derr
		}
		if paths.InstallHasData(dir) {
			return fmt.Errorf("config.json not found in %s but other installation data exists; restore config from backup", dir)
		}
		c := DefaultConfig()
		if saveErr := saveUnlocked(c); saveErr != nil {
			return saveErr
		}
		cfg = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func persistMigration(cfg *models.Config) {
	if cfg == nil {
		return
	}
	_ = datalock.With("config", func() error {
		return saveUnlocked(cfg)
	})
}
