package pbsbackup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/paths"
)

const pbsEntryBlobName = "pbs_entries.blob"

var entryCacheMu sync.Mutex

func pbsEntryBlobPath(jobID string) (string, error) {
	dir, err := paths.IndexDir(jobID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, pbsEntryBlobName), nil
}

// PBSEntryCache stores concatenated PXAR file entry blobs for fast reuse.
type PBSEntryCache struct {
	jobID  string
	path   string
	file   *os.File
	offset uint64
}

func OpenPBSEntryCache(jobID string, truncate bool) (*PBSEntryCache, error) {
	entryCacheMu.Lock()
	defer entryCacheMu.Unlock()
	path, err := pbsEntryBlobPath(jobID)
	if err != nil {
		return nil, err
	}
	flags := os.O_RDWR | os.O_CREATE
	if truncate {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return nil, err
	}
	off, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &PBSEntryCache{jobID: jobID, path: path, file: f, offset: uint64(off)}, nil
}

func OpenPBSEntryCacheRead(jobID string) (*PBSEntryCache, error) {
	entryCacheMu.Lock()
	defer entryCacheMu.Unlock()
	path, err := pbsEntryBlobPath(jobID)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &PBSEntryCache{jobID: jobID, path: path, file: f}, nil
}

func (c *PBSEntryCache) Close() error {
	if c == nil || c.file == nil {
		return nil
	}
	err := c.file.Close()
	c.file = nil
	return err
}

func (c *PBSEntryCache) ReadEntry(offset, length uint64) ([]byte, error) {
	if c == nil || c.file == nil {
		return nil, i18nconfig.FromConfig().E("pbs.entry_cache.not_open")
	}
	if length == 0 {
		return nil, i18nconfig.FromConfig().E("pbs.entry_cache.empty_entry")
	}
	if length > 512*1024*1024 {
		return nil, i18nconfig.FromConfig().Ef("pbs.entry_cache.too_large", map[string]string{"n": fmt.Sprintf("%d", length)})
	}
	buf := make([]byte, length)
	n, err := c.file.ReadAt(buf, int64(offset))
	if err != nil {
		return nil, err
	}
	if uint64(n) != length {
		return nil, i18nconfig.FromConfig().Ef("pbs.entry_cache.partial_read", map[string]string{
			"n":   fmt.Sprintf("%d", n),
			"max": fmt.Sprintf("%d", length),
		})
	}
	return buf, nil
}

func (c *PBSEntryCache) Append(data []byte) (uint64, error) {
	if c == nil || c.file == nil {
		return 0, i18nconfig.FromConfig().E("pbs.entry_cache.not_open")
	}
	if len(data) == 0 {
		return 0, i18nconfig.FromConfig().E("pbs.entry_cache.empty_data")
	}
	start := c.offset
	n, err := c.file.Write(data)
	if err != nil {
		return 0, err
	}
	c.offset += uint64(n)
	return start, nil
}

func ClearPBSEntryCache(jobID string) error {
	entryCacheMu.Lock()
	defer entryCacheMu.Unlock()
	path, err := pbsEntryBlobPath(jobID)
	if err != nil {
		return err
	}
	_ = os.Remove(path)
	return nil
}
