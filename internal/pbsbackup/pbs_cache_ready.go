package pbsbackup

import (
	"os"
	"path/filepath"

	"pbs-win-backup/internal/paths"
)

const pbsCacheReadyName = "pbs_cache.ready"

const minCacheFiles = 1

func pbsCacheReadyPath(jobID string) (string, error) {
	dir, err := paths.IndexDir(jobID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, pbsCacheReadyName), nil
}

func markPBSCacheReady(jobID string) error {
	path, err := pbsCacheReadyPath(jobID)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte("1"), 0o644)
}

func isPBSCacheReady(jobID string) bool {
	return isPBSCacheSubstantial(jobID)
}

func isPBSCacheSubstantial(jobID string) bool {
	idx, err := LoadPBSFileIndex(jobID)
	if err != nil || idx == nil {
		return false
	}
	withSpans := 0
	for _, rec := range idx.Files {
		if len(rec.ChunkSpans) > 0 {
			withSpans++
		}
	}
	return withSpans >= minCacheFiles
}

func clearPBSCacheReady(jobID string) {
	path, _ := pbsCacheReadyPath(jobID)
	_ = os.Remove(path)
}
