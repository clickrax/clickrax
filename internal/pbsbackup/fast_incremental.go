package pbsbackup

import (
	"os"
	"path/filepath"

	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/winattr"

	pbscommon "pbscommon"
)

type fastIncremental struct {
	backupRoot   string
	prev         *PBSFileIndex
	live         *livePBSFileIndex
	reuseEnabled bool
	cacheEnabled bool
}

func (fi *fastIncremental) ReuseActive() bool {
	return fi != nil && fi.reuseEnabled
}

func newFastIncremental(jobID, backupRoot string, forceFull bool) (*fastIncremental, error) {
	fi := &fastIncremental{
		backupRoot:   backupRoot,
		live:         newLivePBSFileIndex(jobID),
		cacheEnabled: true,
	}
	// Drop legacy blob caches — metadata-only index is enough for fast reuse.
	_ = ClearPBSEntryCache(jobID)

	if forceFull {
		if err := ClearPBSFileIndex(jobID); err != nil {
			return nil, err
		}
		return fi, nil
	}

	prev, err := LoadPBSFileIndex(jobID)
	if err != nil {
		return nil, err
	}
	fi.prev = prev
	if len(prev.Files) > 0 && isPBSCacheReady(jobID) {
		fi.reuseEnabled = true
	}
	return fi, nil
}

func (fi *fastIncremental) close() {}

func (fi *fastIncremental) catalogKey(absPath string) (string, error) {
	rel, err := filepath.Rel(fi.backupRoot, absPath)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || filepath.IsAbs(rel) {
		return "", i18n.Ef("pbs.path_outside_root", map[string]string{"path": absPath})
	}
	return normalizeIndexKey(rel), nil
}

func (fi *fastIncremental) lookupReuse(path string, info os.FileInfo) (PBSFileRecord, bool) {
	if fi == nil || !fi.reuseEnabled || fi.prev == nil {
		return PBSFileRecord{}, false
	}
	key, err := fi.catalogKey(path)
	if err != nil {
		return PBSFileRecord{}, false
	}
	aclHash := ""
	if e, err := filemeta.CaptureFile(path); err == nil {
		aclHash = winattr.ACLHash(e)
	}
	rec, ok := fi.prev.canReuse(key, info.Size(), info.ModTime().UnixNano(), aclHash)
	if !ok || !spansReusableForFastReuse(rec.ChunkSpans) {
		return PBSFileRecord{}, false
	}
	fi.live.put(key, rec)
	return rec, true
}

func (fi *fastIncremental) reuseChunkSpans(path, basename string, info os.FileInfo) ([]pbscommon.PXARFastChunk, bool) {
	rec, ok := fi.lookupReuse(path, info)
	if !ok {
		return nil, false
	}
	out := make([]pbscommon.PXARFastChunk, len(rec.ChunkSpans))
	for i, sp := range rec.ChunkSpans {
		out[i] = pbscommon.PXARFastChunk{DigestHex: sp.Digest, Len: sp.Len}
	}
	return out, true
}

func (fi *fastIncremental) cacheFileCount() int {
	if fi == nil || fi.prev == nil {
		return 0
	}
	return len(fi.prev.Files)
}

func (fi *fastIncremental) recordFile(path string, info os.FileInfo, spans []fileChunkSpan, pxarLen int64) {
	if fi == nil || !fi.cacheEnabled || len(spans) == 0 {
		return
	}
	key, err := fi.catalogKey(path)
	if err != nil {
		return
	}
	if _, exists := fi.live.files[key]; exists {
		return
	}
	aclHash := ""
	if e, err := filemeta.CaptureFile(path); err == nil {
		aclHash = winattr.ACLHash(e)
	}
	fi.live.put(key, PBSFileRecord{
		Size:       info.Size(),
		PxarLen:    pxarLen,
		Mtime:      info.ModTime().UnixNano(),
		ACLHash:    aclHash,
		ChunkSpans: spans,
	})
}

func (fi *fastIncremental) wire(archive *pbscommon.PXARArchive) {
	if fi == nil || !fi.cacheEnabled {
		return
	}
	if fi.reuseEnabled {
		archive.ReuseFileChunks = fi.reuseChunkSpans
	}
}

func (fi *fastIncremental) save(snapshotTime string) error {
	if fi == nil || fi.live == nil {
		return nil
	}
	idx := fi.live.snapshot(snapshotTime)
	if idx == nil {
		return nil
	}
	if len(idx.Files) == 0 {
		return nil
	}
	if err := SavePBSFileIndex(idx); err != nil {
		return err
	}
	return markPBSCacheReady(fi.live.jobID)
}
