//go:build windows

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
)

type configProbeScore struct {
	path   string
	score  int
	dests  int
	jobs   int
	legacy int
}

func scoreConfig(cfg *models.Config) int {
	if cfg == nil {
		return 0
	}
	return len(cfg.Destinations)*10 + len(cfg.Servers)*10 + len(cfg.Jobs)
}

func bestQuarantineScore(configPath string) int {
	dir := filepath.Dir(configPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	best := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "config.json.corrupt-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || !json.Valid(data) {
			continue
		}
		var probe struct {
			Destinations []json.RawMessage `json:"destinations"`
			Servers      []json.RawMessage `json:"servers"`
			Jobs         []json.RawMessage `json:"jobs"`
		}
		if err := json.Unmarshal(data, &probe); err != nil {
			continue
		}
		score := len(probe.Destinations)*10 + len(probe.Servers)*10 + len(probe.Jobs)
		if score > best {
			best = score
		}
	}
	return best
}

// RecoverFromQuarantine restores the richest valid config.json.corrupt-* backup.
// Returns true when a file was restored (caller should Load again).
func RecoverFromQuarantine(configPath string) (bool, error) {
	dir := filepath.Dir(configPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	var candidates []configProbeScore
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "config.json.corrupt-") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil || !json.Valid(data) {
			continue
		}
		var probe struct {
			Destinations []json.RawMessage `json:"destinations"`
			Servers      []json.RawMessage `json:"servers"`
			Jobs         []json.RawMessage `json:"jobs"`
		}
		if err := json.Unmarshal(data, &probe); err != nil {
			continue
		}
		dests := len(probe.Destinations)
		jobs := len(probe.Jobs)
		legacy := len(probe.Servers)
		score := dests*10 + legacy*10 + jobs
		if score == 0 {
			continue
		}
		candidates = append(candidates, configProbeScore{
			path: path, score: score, dests: dests, jobs: jobs, legacy: legacy,
		})
	}
	if len(candidates) == 0 {
		return false, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].path > candidates[j].path
	})
	best := candidates[0]
	data, err := os.ReadFile(best.path)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return false, err
	}
	if err := writeConfigSignature(configPath, data); err != nil {
		return false, err
	}
	_ = paths.RestrictSensitiveACL(configPath)
	return true, nil
}

// ConfigNeedsRecovery reports whether a richer quarantine backup should replace active config.
func ConfigNeedsRecovery(cfg *models.Config, configPath string) bool {
	qScore := bestQuarantineScore(configPath)
	if qScore == 0 {
		return false
	}
	if scoreConfig(cfg) == 0 {
		return true
	}
	return qScore > scoreConfig(cfg)
}

// LoadResilient loads config, auto-heals HMAC, and restores from quarantine backups if needed.
func LoadResilient() (*models.Config, error) {
	cfg, err := Load()
	if err == nil && !ConfigNeedsRecovery(cfg, mustConfigPath()) {
		return cfg, nil
	}
	path, perr := mustConfigPathErr()
	if perr != nil {
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	restored, rerr := RecoverFromQuarantine(path)
	if rerr != nil {
		return nil, fmt.Errorf("config recovery: %w", rerr)
	}
	if restored || err != nil || ConfigNeedsRecovery(cfg, path) {
		return Load()
	}
	return cfg, nil
}

func mustConfigPath() string {
	p, err := mustConfigPathErr()
	if err != nil {
		return ""
	}
	return p
}

func mustConfigPathErr() (string, error) {
	return paths.ConfigPath()
}
