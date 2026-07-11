package paths

import (
	"path/filepath"

	"pbs-win-backup/internal/branding"
)

func ServiceBinaryPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bin", branding.ExeName+".exe"), nil
}
