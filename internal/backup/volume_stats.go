//go:build !windows

package backup

import (
	"pbs-win-backup/internal/i18n"
)

func volumeStats(root string) (totalBytes, usedBytes uint64, err error) {
	return 0, 0, i18n.E("backup.volume_windows_only", nil)
}
