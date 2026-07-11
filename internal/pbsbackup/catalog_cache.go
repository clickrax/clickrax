package pbsbackup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
)

const catalogCacheDirName = "catalog-cache"
const catalogCacheMaxEntries = 4

type catalogView struct {
	path string
	file *os.File
	size int64
}

func (v *catalogView) withView(fn func([]byte) error) error {
	if v == nil || v.file == nil {
		return i18n.E("restore.catalog.not_open", nil)
	}
	return withMmapView(v.file, v.size, fn)
}

func (v *catalogView) Close() error {
	if v == nil || v.file == nil {
		return nil
	}
	err := v.file.Close()
	v.file = nil
	return err
}

var (
	catalogDownloadMu sync.Mutex
	catalogOpenMu     sync.Mutex
	catalogOpenCache  = map[string]*catalogView{}
	catalogOpenOrder  []string
)

func catalogCacheTouch(path string) {
	for i, p := range catalogOpenOrder {
		if p == path {
			catalogOpenOrder = append(append(catalogOpenOrder[:i], catalogOpenOrder[i+1:]...), path)
			return
		}
	}
	catalogOpenOrder = append(catalogOpenOrder, path)
}

func catalogCacheEvict() {
	for len(catalogOpenOrder) > catalogCacheMaxEntries {
		oldest := catalogOpenOrder[0]
		catalogOpenOrder = catalogOpenOrder[1:]
		if v := catalogOpenCache[oldest]; v != nil {
			_ = v.Close()
			delete(catalogOpenCache, oldest)
		}
	}
}

func catalogCachePath(ref SnapshotRef) (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s_%d.pcat", sanitizeCacheName(ref.BackupID), ref.BackupTime)
	if ref.BackupTime == 0 {
		name = fmt.Sprintf("%s_%s.pcat", sanitizeCacheName(ref.BackupID), sanitizeCacheName(ref.Time))
	}
	return filepath.Join(dir, catalogCacheDirName, name), nil
}

func sanitizeCacheName(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "snapshot"
	}
	return string(out)
}

// OpenCatalogCache downloads catalog.pcat1.didx once, caches it on disk, and opens it read-only.
// The mmap handle is kept open for the process lifetime to make folder navigation instant.
func OpenCatalogCache(server models.PBSServer, secret string, ref SnapshotRef, onProgress StreamProgress) (*catalogView, error) {
	cachePath, err := catalogCachePath(ref)
	if err != nil {
		return nil, err
	}
	if err := ensureCatalogCached(server, secret, ref, cachePath, onProgress); err != nil {
		return nil, err
	}

	catalogOpenMu.Lock()
	defer catalogOpenMu.Unlock()
	if v := catalogOpenCache[cachePath]; v != nil && v.file != nil {
		catalogCacheTouch(cachePath)
		return v, nil
	}
	f, err := os.Open(cachePath)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if info.Size() < 16 {
		_ = f.Close()
		return nil, i18n.Ef("restore.catalog.cache_corrupt", map[string]string{"n": fmt.Sprintf("%d", info.Size())})
	}
	v := &catalogView{path: cachePath, file: f, size: info.Size()}
	catalogOpenCache[cachePath] = v
	catalogCacheTouch(cachePath)
	catalogCacheEvict()
	return v, nil
}

func ensureCatalogCached(server models.PBSServer, secret string, ref SnapshotRef, cachePath string, onProgress StreamProgress) error {
	if info, err := os.Stat(cachePath); err == nil && info.Size() >= 16 {
		return nil
	}
	catalogDownloadMu.Lock()
	defer catalogDownloadMu.Unlock()
	if info, err := os.Stat(cachePath); err == nil && info.Size() >= 16 {
		return nil
	}
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	_ = paths.GrantUsersModify(cacheDir)
	tmpPath := cachePath + ".part"
	_ = os.Remove(tmpPath)
	if err := downloadCatalogArchive(server, secret, ref, tmpPath, onProgress); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	_ = os.Remove(cachePath)
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return err
	}
	_ = paths.GrantUsersModify(cachePath)
	return nil
}

func downloadCatalogArchive(server models.PBSServer, secret string, ref SnapshotRef, destPath string, onProgress StreamProgress) error {
	client, err := connectReader(server, secret, ref)
	if err != nil {
		return err
	}
	defer closeReader(client)

	if onProgress != nil {
		onProgress(0, 0, i18n.L("restore.catalog.loading", nil))
	}
	raw, err := client.DownloadToBytes("catalog.pcat1.didx")
	if err != nil {
		return i18n.Ewrap("restore.catalog.load_failed", map[string]string{"err": err.Error()}, err)
	}
	if len(raw) == 0 {
		return i18n.E("restore.catalog.empty_pbs", nil)
	}
	if bytesHasCatalogMagic(raw) {
		return os.WriteFile(destPath, raw, 0o644)
	}

	records, err := parseDidxRecords(raw)
	if err != nil {
		return err
	}
	getChunk := func(digest string) ([]byte, error) {
		return getChunkVerified(nil, client, digest, chunkDownloadTimeout)
	}
	spill := newPxarSpillBuffer()
	defer spill.Close()
	if _, _, err := reassembleFromRecordsProgress(context.Background(), records, getChunk, onProgress, nil, spill); err != nil {
		return err
	}
	return spill.persistTo(destPath)
}
