package pbsbackup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"pbs-win-backup/internal/fileindex"
	"pbs-win-backup/internal/paths"
)

const pbsFileIndexVersion = 1
const pbsFileIndexName = "pbs_files.json"

// PBSFileRecord holds per-file metadata and a blob offset for fast incremental reuse.
type PBSFileRecord struct {
	Size       int64           `json:"size"`
	Mtime      int64           `json:"mtime_ns"`
	ACLHash    string          `json:"acl_hash,omitempty"`
	BlobOffset uint64          `json:"blob_offset"`
	BlobLength uint64          `json:"blob_length"`
	ChunkSpans []fileChunkSpan `json:"chunk_spans,omitempty"`
}

// PBSFileIndex is a local metadata index for PBS fast incremental backups.
type PBSFileIndex struct {
	Version      int              `json:"version"`
	JobID        string           `json:"job_id"`
	SnapshotTime string           `json:"snapshot_time,omitempty"`
	Files        map[string]PBSFileRecord `json:"files"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

var pbsFileIndexMu sync.Mutex

func pbsFileIndexPath(jobID string) (string, error) {
	dir, err := paths.IndexDir(jobID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, pbsFileIndexName), nil
}

func LoadPBSFileIndex(jobID string) (*PBSFileIndex, error) {
	pbsFileIndexMu.Lock()
	defer pbsFileIndexMu.Unlock()
	path, err := pbsFileIndexPath(jobID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &PBSFileIndex{
				Version: pbsFileIndexVersion,
				JobID:   jobID,
				Files:   map[string]PBSFileRecord{},
			}, nil
		}
		return nil, err
	}
	idx := &PBSFileIndex{JobID: jobID, Files: map[string]PBSFileRecord{}}
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, err
	}
	if idx.Files == nil {
		idx.Files = map[string]PBSFileRecord{}
	}
	normalizePBSFileIndexKeys(idx)
	return idx, nil
}

func SavePBSFileIndex(idx *PBSFileIndex) error {
	if idx == nil {
		return nil
	}
	pbsFileIndexMu.Lock()
	defer pbsFileIndexMu.Unlock()
	path, err := pbsFileIndexPath(idx.JobID)
	if err != nil {
		return err
	}
	idx.Version = pbsFileIndexVersion
	idx.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return paths.AtomicWrite(path, data, 0o600)
}

func ClearPBSFileIndex(jobID string) error {
	pbsFileIndexMu.Lock()
	defer pbsFileIndexMu.Unlock()
	path, err := pbsFileIndexPath(jobID)
	if err != nil {
		return err
	}
	_ = os.Remove(path)
	clearPBSCacheReady(jobID)
	return ClearPBSEntryCache(jobID)
}

func (idx *PBSFileIndex) lookup(key string) (PBSFileRecord, bool) {
	if idx == nil || len(idx.Files) == 0 {
		return PBSFileRecord{}, false
	}
	key = normalizeIndexKey(key)
	rec, ok := idx.Files[key]
	return rec, ok
}

func (idx *PBSFileIndex) canReuse(key string, size int64, mtimeNs int64, aclHash string) (PBSFileRecord, bool) {
	if idx == nil || len(idx.Files) == 0 {
		return PBSFileRecord{}, false
	}
	prev, ok := idx.lookup(key)
	if !ok {
		return PBSFileRecord{}, false
	}
	if prev.Size != size {
		return PBSFileRecord{}, false
	}
	if prev.Mtime != 0 && !fileindex.MtimeMatches(prev.Mtime, mtimeNs) {
		return PBSFileRecord{}, false
	}
	var contentLen int64
	for _, sp := range prev.ChunkSpans {
		contentLen += int64(sp.Len)
	}
	if contentLen != size || len(prev.ChunkSpans) == 0 {
		return PBSFileRecord{}, false
	}
	if aclHash != "" && (prev.ACLHash == "" || prev.ACLHash != aclHash) {
		return PBSFileRecord{}, false
	}
	if prev.BlobLength == 0 && len(prev.ChunkSpans) == 0 {
		return PBSFileRecord{}, false
	}
	return prev, true
}

func needsPBSBootstrap(jobID string) bool {
	idx, err := LoadPBSFileIndex(jobID)
	if err != nil || idx == nil || len(idx.Files) == 0 {
		return true
	}
	for _, rec := range idx.Files {
		if len(rec.ChunkSpans) > 0 {
			return false
		}
	}
	return true
}

func normalizePBSFileIndexKeys(idx *PBSFileIndex) {
	if idx == nil || len(idx.Files) == 0 {
		return
	}
	out := make(map[string]PBSFileRecord, len(idx.Files))
	for k, v := range idx.Files {
		out[normalizeIndexKey(k)] = v
	}
	idx.Files = out
}

type livePBSFileIndex struct {
	jobID string
	files map[string]PBSFileRecord
}

func newLivePBSFileIndex(jobID string) *livePBSFileIndex {
	return &livePBSFileIndex{
		jobID: jobID,
		files: make(map[string]PBSFileRecord),
	}
}

func (l *livePBSFileIndex) put(key string, rec PBSFileRecord) {
	if l == nil {
		return
	}
	l.files[normalizeIndexKey(key)] = rec
}

func (l *livePBSFileIndex) snapshot(snapshotTime string) *PBSFileIndex {
	if l == nil {
		return nil
	}
	return &PBSFileIndex{
		Version:      pbsFileIndexVersion,
		JobID:        l.jobID,
		SnapshotTime: snapshotTime,
		Files:        l.files,
	}
}
