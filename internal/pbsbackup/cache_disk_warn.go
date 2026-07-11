package pbsbackup

import (
	"path/filepath"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/paths"
)

const minCacheBuildFreeBytes = 5 << 30 // warn below 5 GiB free on cache volume

func warnCacheDiskSpace(stats *Stats, jobID string) {
	if stats == nil {
		return
	}
	idxDir, err := paths.IndexDir(jobID)
	if err != nil {
		return
	}
	free, err := paths.FreeBytes(idxDir)
	if err != nil {
		return
	}
	if free >= minCacheBuildFreeBytes {
		return
	}
	vol := filepath.VolumeName(idxDir)
	if vol == "" {
		vol = "C:"
	}
	stats.SetStage(i18n.L("pbs.disk_warn", map[string]string{
		"path": vol,
		"vol":  formatByteSize(int64(free)),
	}))
}
