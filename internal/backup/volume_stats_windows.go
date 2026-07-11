//go:build windows

package backup

import (
	"pbs-win-backup/internal/i18n"

	"golang.org/x/sys/windows"
)

func volumeStats(root string) (totalBytes, usedBytes uint64, err error) {
	if !IsVolumeRoot(root) {
		return 0, 0, i18n.Ef("backup.volume_not_root", map[string]string{"path": root})
	}
	path := root
	if len(path) == 2 {
		path += `\`
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var freeAvail, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &freeAvail, &total, &totalFree); err != nil {
		return 0, 0, i18n.Ewrap("backup.volume_size", map[string]string{"path": root}, err)
	}
	if total < totalFree {
		return total, 0, nil
	}
	return total, total - totalFree, nil
}
