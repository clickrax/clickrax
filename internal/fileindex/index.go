package fileindex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pbs-win-backup/internal/paths"
	"pbs-win-backup/internal/winattr"
)

const KindFull = "full"
const KindIncremental = "incremental"

// IsIncrementalArchiveName reports whether a remote archive filename is an incremental ZIP.
func IsIncrementalArchiveName(name string) bool {
	return strings.Contains(strings.ToLower(name), "_incr.zip")
}

type ArchiveRecord struct {
	Name    string    `json:"name"`
	Kind    string    `json:"kind"`
	Time    time.Time `json:"time"`
	Deleted []string  `json:"deleted,omitempty"`
}

type FileRecord struct {
	Size    int64  `json:"size"`
	Mtime   int64  `json:"mtime"`
	ACLHash string `json:"acl_hash,omitempty"`
	Archive string `json:"archive"`
}

type Store struct {
	JobID     string                `json:"job_id"`
	BaseFull  string                `json:"base_full"`
	Archives  []ArchiveRecord       `json:"archives"`
	Files     map[string]FileRecord `json:"files"`
	Meta      map[string]winattr.Entry `json:"meta,omitempty"`
	UpdatedAt time.Time             `json:"updated_at"`
}

var mu sync.Mutex

func storePath(jobID string) (string, error) {
	dir, err := paths.IndexDir(jobID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "fileindex.json"), nil
}

func Load(jobID string) (*Store, error) {
	mu.Lock()
	defer mu.Unlock()
	path, err := storePath(jobID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{JobID: jobID, Files: map[string]FileRecord{}, Meta: map[string]winattr.Entry{}}, nil
		}
		return nil, err
	}
	st := &Store{JobID: jobID, Files: map[string]FileRecord{}}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, err
	}
	if st.Files == nil {
		st.Files = map[string]FileRecord{}
	}
	if st.Meta == nil {
		st.Meta = map[string]winattr.Entry{}
	}
	return st, nil
}

func Save(st *Store) error {
	if st == nil {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	path, err := storePath(st.JobID)
	if err != nil {
		return err
	}
	st.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return paths.AtomicWrite(path, data, 0o644)
}

func Clear(jobID string) error {
	mu.Lock()
	defer mu.Unlock()
	path, err := storePath(jobID)
	if err != nil {
		return err
	}
	_ = os.Remove(path)
	return nil
}

// NeedsBackup reports whether a file should be included in an incremental archive (content or ACL).
func NeedsBackup(prev FileRecord, size int64, mtime time.Time, hasPrev bool, aclHash string) bool {
	return NeedsContentBackup(prev, size, mtime, hasPrev) || NeedsACLBackup(prev, aclHash, hasPrev)
}

// NeedsContentBackup reports whether file bytes changed.
func NeedsContentBackup(prev FileRecord, size int64, mtime time.Time, hasPrev bool) bool {
	if !hasPrev {
		return true
	}
	if prev.Size != size {
		return true
	}
	return !MtimeMatches(prev.Mtime, mtime.UnixNano())
}

// MtimeMatches compares mtimes, tolerating second-only precision from PXAR/catalog.
func MtimeMatches(storedNs, statNs int64) bool {
	if storedNs == statNs {
		return true
	}
	return storedNs/1e9 == statNs/1e9
}

// NeedsACLBackup reports whether security metadata changed.
func NeedsACLBackup(prev FileRecord, aclHash string, hasPrev bool) bool {
	if !hasPrev {
		return aclHash != ""
	}
	return aclHash != "" && prev.ACLHash != aclHash
}

func ManifestName(archiveName string) string {
	return archiveName + ".manifest.json"
}
