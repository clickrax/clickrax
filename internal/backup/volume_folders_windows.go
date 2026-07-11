//go:build windows

package backup

import (
	"os"
	"path/filepath"

	"pbs-win-backup/internal/backup/exclude"
	"pbs-win-backup/internal/models"
)

// ListVolumeFolders lists top-level directories on a volume root for exclusion UI.
func ListVolumeFolders(volume string) ([]models.VolumeFolder, error) {
	root := NormalizeSourcePath(volume)
	if root == "" || !IsVolumeRoot(root) {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := make([]models.VolumeFolder, 0, len(entries))
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		if ent.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := ent.Name()
		full := filepath.Join(root, name)
		out = append(out, models.VolumeFolder{
			Name:   name,
			Path:   full,
			System: exclude.IsSystemName(name, true),
		})
	}
	return out, nil
}
