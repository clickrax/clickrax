package paths

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var legacyMigrateOnce sync.Once

type configProbe struct {
	Destinations []json.RawMessage `json:"destinations"`
	Servers      []json.RawMessage `json:"servers"`
	Jobs         []json.RawMessage `json:"jobs"`
}

func configHasUserData(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return false
	}
	var probe configProbe
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return len(probe.Destinations) > 0 || len(probe.Servers) > 0 || len(probe.Jobs) > 0
}

func ensureLegacyMigrated(programData string) {
	legacyMigrateOnce.Do(func() {
		_ = migrateLegacyIfNeeded(programData)
	})
}

func migrateLegacyIfNeeded(programData string) error {
	newDir := filepath.Join(programData, AppFolderName)
	legacyDir := filepath.Join(programData, legacyAppFolderName)

	newCfg := filepath.Join(newDir, "config.json")
	legacyCfg := filepath.Join(legacyDir, "config.json")

	if !configHasUserData(legacyCfg) {
		return nil
	}
	if configHasUserData(newCfg) {
		return nil
	}

	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return err
	}
	_ = os.Remove(newCfg)
	return copyTree(legacyDir, newDir)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		srcInfo, err := d.Info()
		if err != nil {
			return err
		}
		if dstInfo, err := os.Stat(target); err == nil {
			if dstInfo.Size() >= srcInfo.Size() {
				return nil
			}
			_ = os.Remove(target)
		}
		return copyFile(path, target, d)
	})
}

func copyFile(src, dst string, d os.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return AtomicWrite(dst, data, info.Mode().Perm())
}
