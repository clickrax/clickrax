//go:build !windows

package backup

import "pbs-win-backup/internal/models"

func ListVolumeFolders(volume string) ([]models.VolumeFolder, error) {
	return nil, nil
}
