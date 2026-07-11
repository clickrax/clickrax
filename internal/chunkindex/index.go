package chunkindex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/paths"

	"github.com/cornelk/hashmap"
)

type Store struct {
	JobID        string    `json:"job_id"`
	ChunkHashes  []string  `json:"chunk_hashes"`
	LastSnapshot string    `json:"last_snapshot,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

const (
	primaryFile = "chunks.json"
	overlayFile = "chunks.overlay.json"
)

var mu sync.Mutex

func indexPaths(jobID string) (primary, overlay string, err error) {
	dir, err := paths.IndexDir(jobID)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, primaryFile), filepath.Join(dir, overlayFile), nil
}

func readStore(path, jobID string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	st := &Store{JobID: jobID, ChunkHashes: []string{}}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, err
	}
	return st, nil
}

func mergeStores(jobID string, parts ...*Store) *Store {
	out := &Store{JobID: jobID, ChunkHashes: []string{}}
	seen := map[string]bool{}
	var latest time.Time
	for _, part := range parts {
		if part == nil {
			continue
		}
		for _, h := range part.ChunkHashes {
			if !seen[h] {
				seen[h] = true
				out.ChunkHashes = append(out.ChunkHashes, h)
			}
		}
		if part.UpdatedAt.After(latest) {
			latest = part.UpdatedAt
			out.LastSnapshot = part.LastSnapshot
		}
	}
	out.UpdatedAt = latest
	return out
}

func Load(jobID string) (*Store, error) {
	mu.Lock()
	defer mu.Unlock()
	primary, overlay, err := indexPaths(jobID)
	if err != nil {
		return nil, err
	}
	p, err := readStore(primary, jobID)
	if err != nil {
		return nil, err
	}
	o, err := readStore(overlay, jobID)
	if err != nil {
		return nil, err
	}
	return mergeStores(jobID, p, o), nil
}

func Save(jobID string, known *hashmap.Map[string, bool], snapshot string) error {
	if known == nil {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	primary, overlay, err := indexPaths(jobID)
	if err != nil {
		return err
	}
	hashes := make([]string, 0, known.Len())
	known.Range(func(k string, _ bool) bool {
		hashes = append(hashes, k)
		return true
	})
	st := Store{
		JobID:        jobID,
		ChunkHashes:  hashes,
		LastSnapshot: snapshot,
		UpdatedAt:    time.Now().UTC(),
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := paths.AtomicWrite(primary, data, 0o600); err == nil {
		_ = os.Remove(overlay)
		return nil
	} else if overlayErr := paths.AtomicWrite(overlay, data, 0o600); overlayErr != nil {
		return i18nconfig.FromConfig().Ef("pbs.chunk_index_save_dual", map[string]string{
			"overlay": overlayErr.Error(),
			"primary": err.Error(),
		})
	}
	return nil
}

func Clear(jobID string) error {
	mu.Lock()
	defer mu.Unlock()
	primary, overlay, err := indexPaths(jobID)
	if err != nil {
		return err
	}
	_ = os.Remove(primary)
	_ = os.Remove(overlay)
	return nil
}

// ApplyLocal is deprecated: known chunks must come only from PBS previous-index.
func ApplyLocal(jobID string, known *hashmap.Map[string, bool], forceFull bool) error {
	_ = known
	if forceFull {
		return Clear(jobID)
	}
	return nil
}

// CollectFrom merges all hashes from known map for persistence.
func CollectFrom(known *hashmap.Map[string, bool]) []string {
	out := make([]string, 0, known.Len())
	if known == nil {
		return out
	}
	known.Range(func(k string, _ bool) bool {
		out = append(out, k)
		return true
	})
	return out
}
