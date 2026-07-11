package pbsbackup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

// StreamProgress reports chunk download progress (done/total chunks, human message).
type StreamProgress func(done, total int, message string)

// streamStopFn is called after each downloaded chunk. Return stop=true to end the PBS stream early.
type streamStopFn func(view []byte) (stop bool, err error)

func isTruncatedPXARError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "ожидался") ||
		strings.Contains(s, "header") ||
		strings.Contains(s, "filename") ||
		strings.Contains(s, "entry data") ||
		strings.Contains(s, "block data") ||
		strings.Contains(s, "skip dir EOF")
}

func tryExtractFromBuffer(buf []byte, filePath string) ([]byte, bool) {
	if len(buf) < 56 {
		return nil, false
	}
	for _, rel := range []string{catalogRelPath(filePath), normalizeRestorePath(filePath)} {
		payload, err := extractFileFromPXAR(buf, rel)
		if err == nil {
			return payload, true
		}
		if isTruncatedPXARError(err) {
			return nil, false
		}
	}
	return nil, false
}

// ExtractFileStreaming downloads pxar chunks from PBS and extracts one file, stopping early when possible.
func ExtractFileStreaming(ctx context.Context, server models.PBSServer, secret string, ref SnapshotRef, filePath string, onProgress StreamProgress) ([]byte, error) {
	pxarName, err := resolvePxarArchive(server, secret, ref)
	if err != nil {
		return nil, err
	}
	var extracted []byte
	pxar, earlyStop, err := readerReassembleStream(ctx, server, secret, ref, pxarName, onProgress, func(view []byte) (bool, error) {
		payload, ok := tryExtractFromBuffer(view, filePath)
		if ok {
			extracted = payload
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	if extracted != nil {
		return extracted, nil
	}
	if earlyStop {
		return nil, i18n.Ef("pbs.didx.file_not_in_loaded", map[string]string{"path": filePath})
	}
	return extractPayload(pxar, filePath)
}

func reassembleFromRecordsProgress(
	ctx context.Context,
	records []didxRecord,
	getChunk func(digest string) ([]byte, error),
	onProgress StreamProgress,
	stopFn streamStopFn,
	outSpill *pxarSpillBuffer,
) (pxar []byte, earlyStop bool, err error) {
	if len(records) == 0 {
		return nil, false, i18n.E("pbs.didx.no_chunks", nil)
	}
	totalChunks := len(records)
	expected := records[len(records)-1].offset

	spill := outSpill
	ownSpill := false
	if spill == nil {
		spill = newPxarSpillBuffer()
		ownSpill = true
	}
	if ownSpill {
		defer spill.Close()
	}

	if stopFn == nil && totalChunks > 1 {
		indices := make([]int, len(records))
		for i := range records {
			indices[i] = i
		}
		if onProgress != nil {
			onProgress(0, totalChunks, "Параллельная загрузка блоков с PBS…")
		}
		fetched, err := downloadChunksParallel(ctx, getChunk, records, indices, ChunkWorkers(), func(done, total int) {
			if onProgress != nil {
				onProgress(done, total, fmt.Sprintf("Загрузка с PBS: блок %d/%d…", done, total))
			}
		})
		if err != nil {
			return nil, false, err
		}
		var start uint64
		for i, r := range records {
			chunk := fetched[i]
			if chunk == nil {
				return nil, false, i18n.Ef("pbs.didx.chunk_not_loaded", map[string]string{
					"digest": r.digest[:min(12, len(r.digest))],
				})
			}
			end := start + uint64(len(chunk))
			if end != r.offset {
				return nil, false, i18n.Ef("pbs.didx.offset_mismatch", map[string]string{
					"end":    fmt.Sprintf("%d", end),
					"offset": fmt.Sprintf("%d", r.offset),
				})
			}
			if err := spill.append(chunk); err != nil {
				return nil, false, err
			}
			start = end
			if onProgress != nil {
				onProgress(i+1, totalChunks, fmt.Sprintf("Блок %d/%d загружен (%.1f МБ)", i+1, totalChunks, float64(start)/(1024*1024)))
			}
		}
		if start != expected {
			return nil, false, i18n.Ef("pbs.didx.stream_size_mismatch", map[string]string{
				"got":  fmt.Sprintf("%d", start),
				"want": fmt.Sprintf("%d", expected),
			})
		}
		if outSpill != nil {
			return nil, false, nil
		}
		full, err := spill.bytes()
		return full, false, err
	}

	var start uint64
	for i, r := range records {
		if onProgress != nil {
			mb := float64(start) / (1024 * 1024)
			onProgress(i, totalChunks, fmt.Sprintf("Загрузка с PBS: блок %d/%d (%.1f МБ)…", i+1, totalChunks, mb))
		}
		chunk, err := getChunk(r.digest)
		if err != nil {
			return nil, false, fmt.Errorf("chunk %s: %w", r.digest[:min(12, len(r.digest))], err)
		}
		end := start + uint64(len(chunk))
		if end != r.offset {
			return nil, false, i18n.Ef("pbs.didx.offset_mismatch", map[string]string{
				"end":    fmt.Sprintf("%d", end),
				"offset": fmt.Sprintf("%d", r.offset),
			})
		}
		if err := spill.append(chunk); err != nil {
			return nil, false, err
		}
		start = end
		if onProgress != nil {
			onProgress(i+1, totalChunks, fmt.Sprintf("Блок %d/%d загружен (%.1f МБ)", i+1, totalChunks, float64(start)/(1024*1024)))
		}
		if stopFn != nil {
			var stop bool
			err := spill.withView(func(view []byte) error {
				var fnErr error
				stop, fnErr = stopFn(view)
				return fnErr
			})
			if err != nil {
				return nil, false, err
			}
			if stop {
				return nil, true, nil
			}
		}
	}
	if start != expected {
		return nil, false, i18n.Ef("pbs.didx.stream_size_mismatch", map[string]string{
			"got":  fmt.Sprintf("%d", start),
			"want": fmt.Sprintf("%d", expected),
		})
	}
	if outSpill != nil {
		return nil, false, nil
	}
	full, err := spill.bytes()
	return full, false, err
}

// LoadPXARWithProgress downloads the full pxar stream with chunk progress.
func LoadPXARWithProgress(ctx context.Context, server models.PBSServer, secret string, ref SnapshotRef, onProgress StreamProgress) ([]byte, error) {
	pxarName, err := resolvePxarArchive(server, secret, ref)
	if err != nil {
		return nil, err
	}
	pxar, err := readerReassembleFull(ctx, server, secret, ref, pxarName, onProgress)
	return pxar, err
}

// readerReassembleStream downloads pxar chunks; stopFn can end the stream once targets are satisfied.
func readerReassembleStream(
	ctx context.Context,
	server models.PBSServer,
	secret string,
	ref SnapshotRef,
	archiveName string,
	onProgress StreamProgress,
	stopFn streamStopFn,
) ([]byte, bool, error) {
	client, err := connectReader(server, secret, ref)
	if err != nil {
		return nil, false, err
	}
	defer closeReader(client)

	if onProgress != nil {
		onProgress(0, 0, "Загрузка индекса архива…")
	}
	raw, err := client.DownloadToBytes(archiveName)
	if err != nil {
		return nil, false, i18n.Ef("pbs.restore.load", map[string]string{"name": archiveName, "err": err.Error()})
	}
	if len(raw) == 0 {
		return nil, false, i18n.Ef("pbs.restore.empty_pbs", map[string]string{"name": archiveName})
	}
	if bytesHasCatalogMagic(raw) {
		return raw, false, nil
	}

	records, err := parseDidxRecords(raw)
	if err != nil {
		return nil, false, err
	}
	getChunk := func(digest string) ([]byte, error) {
		return getChunkVerified(ctx, client, digest, chunkDownloadTimeout)
	}
	return reassembleFromRecordsProgress(ctx, records, getChunk, onProgress, stopFn, nil)
}

// readerReassembleFull downloads and returns the complete archive bytes.
func readerReassembleFull(
	ctx context.Context,
	server models.PBSServer,
	secret string,
	ref SnapshotRef,
	archiveName string,
	onProgress StreamProgress,
) ([]byte, error) {
	client, err := connectReader(server, secret, ref)
	if err != nil {
		return nil, err
	}
	defer closeReader(client)

	if onProgress != nil {
		onProgress(0, 0, "Загрузка индекса архива…")
	}
	raw, err := client.DownloadToBytes(archiveName)
	if err != nil {
		return nil, i18n.Ef("pbs.restore.load", map[string]string{"name": archiveName, "err": err.Error()})
	}
	if len(raw) == 0 {
		return nil, i18n.Ef("pbs.restore.empty_pbs", map[string]string{"name": archiveName})
	}
	if bytesHasCatalogMagic(raw) {
		return raw, nil
	}

	records, err := parseDidxRecords(raw)
	if err != nil {
		return nil, err
	}
	getChunk := func(digest string) ([]byte, error) {
		return getChunkVerified(ctx, client, digest, chunkDownloadTimeout)
	}
	pxar, _, err := reassembleFromRecordsProgress(ctx, records, getChunk, onProgress, nil, nil)
	return pxar, err
}

func bytesHasCatalogMagic(raw []byte) bool {
	return len(raw) >= len(catalogMagicBytes) && string(raw[:len(catalogMagicBytes)]) == string(catalogMagicBytes)
}

const pbsReaderTimeout = 15 * time.Minute
