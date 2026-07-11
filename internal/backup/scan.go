package backup

import (
	"fmt"
	"os"
	"path/filepath"

	"pbs-win-backup/internal/backup/exclude"
	"pbs-win-backup/internal/backuproot"
	"pbs-win-backup/internal/i18n"
)

type ScanResult struct {
	Files        int64
	Bytes        int64
	Inaccessible int64
	Approx       bool // fast volume estimate (used space), not a file walk
}

func ScanPath(root string, globalExclusions, jobExclusions []string) (ScanResult, error) {
	root = backuproot.NormalizeSourcePath(root)
	if root == "" {
		return ScanResult{}, nil
	}
	if IsVolumeRoot(root) {
		total, used, err := volumeStats(root)
		if err != nil {
			return ScanResult{}, err
		}
		_ = total
		return ScanResult{
			Bytes:  int64(used),
			Approx: true,
		}, nil
	}
	exc := exclude.NewForRoot(root, exclude.Merge(globalExclusions, jobExclusions))
	var res ScanResult
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			res.Inaccessible++
			return nil
		}
		name := d.Name()
		if exc.MatchPath(path, name, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			res.Inaccessible++
			return nil
		}
		res.Files++
		res.Bytes += info.Size()
		return nil
	})
	if res.Inaccessible > 0 {
		if err != nil {
			return res, i18n.Ef("backup.scan_inaccessible", map[string]string{
				"path": root, "n": fmt.Sprintf("%d", res.Inaccessible), "err": err.Error(),
			})
		}
		// Partial inaccessible paths are normal on large trees; still return counts.
		return res, nil
	}
	return res, err
}
