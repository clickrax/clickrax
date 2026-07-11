package paths

import (
	"os"
	"path/filepath"
)

const AppFolderName = "ClickRAX"
const legacyAppFolderName = "PbsWinBackup"

// ConfigFileHasUserData reports whether a config.json file has destinations, legacy servers, or jobs.
func ConfigFileHasUserData(path string) bool {
	return configHasUserData(path)
}

func DataDir() (string, error) {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	ensureLegacyMigrated(programData)

	newDir := filepath.Join(programData, AppFolderName)
	legacyDir := filepath.Join(programData, legacyAppFolderName)
	newCfg := filepath.Join(newDir, "config.json")
	legacyCfg := filepath.Join(legacyDir, "config.json")

	if configHasUserData(newCfg) {
		return newDir, nil
	}
	if configHasUserData(legacyCfg) {
		return legacyDir, nil
	}
	if st, err := os.Stat(newDir); err == nil && st.IsDir() {
		return newDir, nil
	}
	if st, err := os.Stat(legacyDir); err == nil && st.IsDir() {
		return legacyDir, nil
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return "", err
	}
	return newDir, nil
}

func dataDirNoACL() (string, error) {
	return DataDir()
}

func ConfigPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LogsDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		return "", err
	}
	return logs, nil
}

func IndexDir(jobID string) (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	idx := filepath.Join(dir, "index", jobID)
	if err := os.MkdirAll(idx, 0o755); err != nil {
		return "", err
	}
	return idx, nil
}

func LastStatusPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "last_status.json"), nil
}

func CheckpointsDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	cp := filepath.Join(dir, "checkpoints")
	if err := os.MkdirAll(cp, 0o755); err != nil {
		return "", err
	}
	return cp, nil
}

func CancelRequestsDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	cp := filepath.Join(dir, "cancel")
	if err := os.MkdirAll(cp, 0o755); err != nil {
		return "", err
	}
	return cp, nil
}

// InstallHasData reports whether the data directory has markers of an existing
// installation aside from a missing config.json.
func InstallHasData(dir string) bool {
	markers := []string{"history.json", "backup_queue.json", "last_status.json", "schedule_state.json"}
	for _, name := range markers {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && st.Size() > 2 {
			return true
		}
	}
	for _, sub := range []string{"secrets", "index", "checkpoints"} {
		if dirHasEntries(filepath.Join(dir, sub)) {
			return true
		}
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "logs")); err == nil && len(entries) > 0 {
		return true
	}
	return false
}

func dirHasEntries(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}
