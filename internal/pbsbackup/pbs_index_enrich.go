package pbsbackup

import (
	"os"
	"path/filepath"

	"pbs-win-backup/internal/backup/exclude"
	"pbs-win-backup/internal/fileindex"
)

// enrichIndexFromDisk updates cache metadata from a metadata-only source walk.
func enrichIndexFromDisk(backupRoot string, idx *PBSFileIndex, exc *exclude.Engine, skipAccessErrors bool) int {
	if idx == nil || len(idx.Files) == 0 || backupRoot == "" {
		return 0
	}
	updated := 0
	_ = filepath.WalkDir(backupRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if skipAccessErrors {
				return nil
			}
			return err
		}
		if d.IsDir() {
			if exc != nil && exc.MatchPath(path, d.Name(), true) {
				return filepath.SkipDir
			}
			return nil
		}
		if exc != nil && exc.MatchPath(path, d.Name(), false) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if skipAccessErrors {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(backupRoot, path)
		if err != nil {
			return nil
		}
		key := normalizeIndexKey(rel)
		rec, ok := idx.lookup(key)
		if !ok {
			return nil
		}
		size := info.Size()
		mtime := info.ModTime().UnixNano()
		if rec.Size != size || !fileindex.MtimeMatches(rec.Mtime, mtime) {
			rec.ChunkSpans = nil
			rec.BlobOffset = 0
			rec.BlobLength = 0
		}
		rec.Size = size
		rec.Mtime = mtime
		idx.Files[key] = rec
		updated++
		return nil
	})
	return updated
}
