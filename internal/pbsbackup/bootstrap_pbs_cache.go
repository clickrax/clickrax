package pbsbackup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

// maxBootstrapChunks — выше этого числа блоков PXAR полная подготовка с PBS слишком долга;
// инкремент идёт сразу, локальный кэш накапливается по ходу бэкапа.
const maxBootstrapChunks = 50_000

type catalogFileMeta struct {
	Size    int64
	MtimeNs int64
}

func loadCatalogMetaMap(server models.PBSServer, secret string, ref SnapshotRef) (map[string]catalogFileMeta, error) {
	meta := make(map[string]catalogFileMeta)
	err := ForEachCatalogEntry(server, secret, ref, func(f models.SnapshotFile) error {
		if f.IsDir {
			return nil
		}
		key := strings.ToLower(catalogRelPath(f.Path))
		var mtimeNs int64
		if f.Modified != "" {
			if t, err := time.Parse(time.RFC3339, f.Modified); err == nil {
				mtimeNs = t.UnixNano()
			}
		}
		meta[key] = catalogFileMeta{Size: f.Size, MtimeNs: mtimeNs}
		return nil
	})
	return meta, err
}

func lookupCatalogMeta(meta map[string]catalogFileMeta, pxarPath string) (catalogFileMeta, bool) {
	if len(meta) == 0 {
		return catalogFileMeta{}, false
	}
	key := strings.ToLower(indexKeyFromPxarPath(pxarPath))
	v, ok := meta[key]
	return v, ok
}

// bootstrapFastCacheFromPBS builds the local fast-incremental cache from the latest PBS snapshot.
// This is a one-time download from PBS — not a new full backup to the source disk.
func bootstrapFastCacheFromPBS(
	ctx context.Context,
	server models.PBSServer,
	secret, jobID, backupID, backupRoot string,
	stats *Stats,
) error {
	ref, err := ResolveSnapshot(server, secret, backupID, "latest")
	if err != nil {
		return err
	}
	pxarName, err := resolvePxarArchive(server, secret, ref)
	if err != nil {
		return err
	}

	catalogMeta, err := loadCatalogMetaMap(server, secret, ref)
	if err != nil {
		return i18n.Ewrap("pbs.snapshot_catalog", nil, err)
	}

	client, err := connectReader(server, secret, ref)
	if err != nil {
		return err
	}
	defer closeReader(client)

	if stats != nil {
		stats.SetStage(i18n.L("pbs.index_fast_prep", nil))
	}
	raw, err := client.DownloadToBytes(pxarName)
	if err != nil {
		return i18n.Ewrap("pbs.restore.load", map[string]string{"name": pxarName}, err)
	}
	if len(raw) == 0 {
		return i18n.Ef("pbs.restore.empty_pbs", map[string]string{"name": pxarName})
	}
	if bytesHasCatalogMagic(raw) {
		return i18n.Ef("pbs.restore.not_pxar", map[string]string{"name": pxarName})
	}
	records, err := parseDidxRecords(raw)
	if err != nil {
		return err
	}

	files := make(map[string]PBSFileRecord)
	parser := newPxarIndexRecorder()
	view := newChunkView(records)
	var safePruneOffset uint64
	parser.onIndexEntry = func(filePath string, entryStart, entryEnd, payloadSize uint64) error {
		if err := abortIfCancelled(ctx); err != nil {
			return err
		}
		if entryEnd <= entryStart {
			return nil
		}
		key := indexKeyFromPxarPath(filePath)
		rec := PBSFileRecord{
			ChunkSpans: chunkSpansForRange(records, entryStart, entryEnd),
		}
		if cm, ok := lookupCatalogMeta(catalogMeta, filePath); ok {
			rec.Size = cm.Size
			rec.Mtime = cm.MtimeNs
		}
		if rec.Size == 0 && payloadSize > 0 {
			rec.Size = int64(payloadSize)
		}
		files[key] = rec
		if entryEnd > safePruneOffset {
			safePruneOffset = entryEnd
			view.pruneBefore(safePruneOffset)
		}
		return nil
	}

	getChunk := func(digest string) ([]byte, error) {
		return getChunkVerified(ctx, client, digest, chunkDownloadTimeout)
	}

	totalChunks := len(records)
	var downloaded uint64
	batchSize := ChunkWorkers() * 8
	if batchSize < 32 {
		batchSize = 32
	}
	for start := 0; start < totalChunks; start += batchSize {
		if err := abortIfCancelled(ctx); err != nil {
			return err
		}
		if parser.indexErr != nil {
			return parser.indexErr
		}
		end := start + batchSize
		if end > totalChunks {
			end = totalChunks
		}
		indices := make([]int, 0, end-start)
		for i := start; i < end; i++ {
			indices = append(indices, i)
		}
		fetched, err := downloadChunksParallel(ctx, getChunk, records, indices, ChunkWorkers(), func(done, total int) {
			if stats == nil {
				return
			}
			i := start + done
			mb := float64(downloaded) / (1024 * 1024)
			stats.SetStage(i18n.L("pbs.index_fast_block", map[string]string{
				"n": fmt.Sprintf("%d", i), "max": fmt.Sprintf("%d", totalChunks), "vol": fmt.Sprintf("%.0f MB", mb),
			}))
		})
		if err != nil {
			return err
		}
		for i := start; i < end; i++ {
			if err := abortIfCancelled(ctx); err != nil {
				return err
			}
			if parser.indexErr != nil {
				return parser.indexErr
			}
			chunk, ok := fetched[i]
			if !ok {
				return i18n.Ef("pbs.block_not_loaded", map[string]string{"n": fmt.Sprintf("%d", i+1)})
			}
			r := records[i]
			endOff := downloaded + uint64(len(chunk))
			if endOff != r.offset {
				return i18n.Ef("pbs.didx.offset_mismatch", map[string]string{
					"end": fmt.Sprintf("%d", endOff), "offset": fmt.Sprintf("%d", r.offset),
				})
			}
			view.add(i, chunk)
			if _, err := parser.feed(chunk); err != nil {
				return err
			}
			if parser.indexErr != nil {
				return parser.indexErr
			}
			downloaded = endOff
		}
	}

	view.chunks = nil

	if len(files) == 0 {
		return i18n.E("pbs.cache_no_files", nil)
	}
	idx := &PBSFileIndex{
		Version:      pbsFileIndexVersion,
		JobID:        jobID,
		SnapshotTime: ref.Time,
		Files:        files,
	}
	if backupRoot != "" {
		enrichIndexFromDisk(backupRoot, idx, nil, true)
	}
	normalizePBSFileIndexKeys(idx)
	if err := SavePBSFileIndex(idx); err != nil {
		return err
	}
	_ = markPBSCacheReady(idx.JobID)
	if stats != nil {
		stats.SetStage(i18n.L("pbs.index_fast_ready", map[string]string{"count": fmt.Sprintf("%d", len(files))}))
	}
	return nil
}

// ensureFastCache bootstraps from PBS when local cache is missing. On failure backup continues normally.
func ensureFastCache(
	ctx context.Context,
	server models.PBSServer,
	secret, jobID, backupID, backupRoot string,
	forceFull bool,
	hasPreviousIndex bool,
	stats *Stats,
) {
	if forceFull || !hasPreviousIndex || !needsPBSBootstrap(jobID) {
		return
	}
	ref, err := ResolveSnapshot(server, secret, backupID, "latest")
	if err != nil {
		return
	}
	pxarName, err := resolvePxarArchive(server, secret, ref)
	if err != nil {
		return
	}
	raw, err := readerDownloadIndex(server, secret, ref, pxarName)
	if err != nil || len(raw) == 0 {
		return
	}
	if bytesHasCatalogMagic(raw) {
		return
	}
	records, err := parseDidxRecords(raw)
	if err != nil {
		return
	}
	if len(records) > maxBootstrapChunks {
		if stats != nil {
			if isPBSCacheReady(jobID) {
				stats.SetStage(i18n.L("pbs.index_fast_local", map[string]string{
					"count": formatCount(len(loadPBSFileIndexOrEmpty(jobID).Files)),
				}))
			} else {
				stats.SetStage(i18n.L("pbs.index_fast_skip_blocks", map[string]string{
					"n": formatCount(len(records)),
				}))
			}
		}
		return
	}
	if err := bootstrapFastCacheFromPBS(ctx, server, secret, jobID, backupID, backupRoot, stats); err != nil {
		if stats != nil {
			if ctx.Err() != nil {
				stats.SetStage(i18n.L("pbs.index_fast_aborted", nil))
			} else {
				stats.SetStage(i18n.L("pbs.index_fast_failed", map[string]string{"err": err.Error()}))
			}
		}
	}
}

func formatCount(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1f млн", float64(n)/1_000_000)
	}
	if n >= 10_000 {
		return fmt.Sprintf("%.0f тыс", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func loadPBSFileIndexOrEmpty(jobID string) *PBSFileIndex {
	idx, err := LoadPBSFileIndex(jobID)
	if err != nil || idx == nil {
		return &PBSFileIndex{Files: map[string]PBSFileRecord{}}
	}
	return idx
}
