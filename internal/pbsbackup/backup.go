package pbsbackup

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"maps"
	"os"
	"pbscommon"
	"slices"
	"snapshot"
	"strings"
	"sync/atomic"
	"time"

	"pbs-win-backup/internal/chunkindex"
	"pbs-win-backup/internal/backup/exclude"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

var didxMagic = []byte{28, 145, 78, 165, 25, 186, 179, 205}

const pbsBackupTimeout = 30 * time.Minute

type chunkState struct {
	ctx                context.Context
	assignments        []string
	assignmentsOffset  []uint64
	pos                uint64
	wrid               uint64
	chunkcount         uint64
	chunkdigests       hash.Hash
	currentChunk       []byte
	chunker            pbscommon.Chunker
	newchunk           *atomic.Uint64
	reusechunk         *atomic.Uint64
	bytesNew           *atomic.Int64
	bytesReused        *atomic.Int64
	knownChunks        *knownChunks
	limiter            *bandwidthLimiter
	uploads            *chunkUploadPipeline
}

func (c *chunkState) init(ctx context.Context, stats *Stats, known *knownChunks, limiter *bandwidthLimiter) {
	c.ctx = ctx
	c.assignments = make([]string, 0)
	c.assignmentsOffset = make([]uint64, 0)
	c.chunkdigests = sha256.New()
	c.currentChunk = make([]byte, 0)
	c.chunker = pbscommon.Chunker{}
	c.chunker.New(1024 * 1024 * 4)
	c.newchunk = &stats.NewChunks
	c.reusechunk = &stats.ReusedChunks
	c.bytesNew = &stats.BytesNew
	c.bytesReused = &stats.BytesReused
	c.knownChunks = known
	c.limiter = limiter
	c.uploads = nil
}

func (c *chunkState) bindUploads(client *pbscommon.PBSClient) {
	if c.uploads == nil {
		c.uploads = newChunkUploadPipeline(c.ctx, client, c.limiter, ChunkWorkers())
	}
}

func (c *chunkState) cancelled() bool {
	if c.ctx == nil {
		return false
	}
	select {
	case <-c.ctx.Done():
		return true
	default:
		return false
	}
}

func chunkMissingOnServer(client *pbscommon.PBSClient, digestHex string, inKnown bool) bool {
	if client == nil || !inKnown || client.BaseURL == "" {
		return false
	}
	return !client.ChunkExists(digestHex)
}

func (c *chunkState) commitChunk(shahash string, chunkLen int, inKnown, missingOnServer bool, client *pbscommon.PBSClient, digest [32]byte) error {
	if inKnown && !missingOnServer {
		missingOnServer = chunkMissingOnServer(client, shahash, inKnown)
	}
	if shouldUploadChunk(inKnown, missingOnServer) {
		c.bindUploads(client)
		c.newchunk.Add(1)
		if err := c.uploads.upload(c.wrid, shahash, c.currentChunk); err != nil {
			return err
		}
		c.knownChunks.Add(digest)
		c.bytesNew.Add(int64(chunkLen))
	} else {
		c.knownChunks.Add(digest)
		c.reusechunk.Add(1)
		c.bytesReused.Add(int64(chunkLen))
	}

	_ = binary.Write(c.chunkdigests, binary.LittleEndian, c.pos+uint64(chunkLen))
	_, _ = c.chunkdigests.Write(digest[:])
	c.assignmentsOffset = append(c.assignmentsOffset, c.pos)
	c.assignments = append(c.assignments, shahash)
	c.pos += uint64(chunkLen)
	c.chunkcount++
	c.currentChunk = c.currentChunk[:0]
	return nil
}

func (c *chunkState) flushPendingChunk(client *pbscommon.PBSClient) error {
	if len(c.currentChunk) == 0 {
		return nil
	}
	bindigest := sha256.Sum256(c.currentChunk)
	shahash := hex.EncodeToString(bindigest[:])
	chunkLen := len(c.currentChunk)
	inKnown := c.knownChunks.Has(bindigest)
	return c.commitChunk(shahash, chunkLen, inKnown, false, client, bindigest)
}

func (c *chunkState) reuseChunks(chunks []pbscommon.PXARFastChunk, client *pbscommon.PBSClient) error {
	if err := c.flushPendingChunk(client); err != nil {
		return err
	}
	for _, ch := range chunks {
		if c.cancelled() {
			return c.ctx.Err()
		}
		if ch.Len <= 0 || ch.DigestHex == "" {
			continue
		}
		raw, err := hex.DecodeString(ch.DigestHex)
		if err != nil || len(raw) != 32 {
			return i18n.Ef("pbs.chunk_digest_invalid", map[string]string{
				"digest": ch.DigestHex[:min(12, len(ch.DigestHex))],
			})
		}
		var digest [32]byte
		copy(digest[:], raw)
		inKnown := c.knownChunks.Has(digest)
		missingOnServer := chunkMissingOnServer(client, ch.DigestHex, inKnown)
		if shouldUploadChunk(inKnown, missingOnServer) {
			return i18n.Ef("pbs.chunk_reuse_not_known", map[string]string{
				"digest": ch.DigestHex[:min(12, len(ch.DigestHex))],
			})
		}
		if err := c.commitChunk(ch.DigestHex, ch.Len, true, missingOnServer, client, digest); err != nil {
			return err
		}
	}
	return nil
}

func (c *chunkState) handleData(b []byte, client *pbscommon.PBSClient) error {
	if c.cancelled() {
		return c.ctx.Err()
	}
	chunkpos := c.chunker.Scan(b)
	if chunkpos == 0 {
		c.currentChunk = append(c.currentChunk, b...)
		return nil
	}
	for chunkpos > 0 {
		if c.cancelled() {
			return c.ctx.Err()
		}
		c.currentChunk = append(c.currentChunk, b[:chunkpos]...)
		bindigest := sha256.Sum256(c.currentChunk)
		shahash := hex.EncodeToString(bindigest[:])

		chunkLen := len(c.currentChunk)
		inKnown := c.knownChunks.Has(bindigest)
		if err := c.commitChunk(shahash, chunkLen, inKnown, false, client, bindigest); err != nil {
			return err
		}
		b = b[chunkpos:]
		chunkpos = c.chunker.Scan(b)
	}
	c.currentChunk = append(c.currentChunk, b...)
	return nil
}

func (c *chunkState) drainUploads() error {
	if c.uploads == nil {
		return nil
	}
	return c.uploads.wait()
}

func (c *chunkState) eof(client *pbscommon.PBSClient) error {
	if len(c.currentChunk) > 0 {
		bindigest := sha256.Sum256(c.currentChunk)
		shahash := hex.EncodeToString(bindigest[:])
		chunkLen := len(c.currentChunk)
		inKnown := c.knownChunks.Has(bindigest)
		if err := c.commitChunk(shahash, chunkLen, inKnown, false, client, bindigest); err != nil {
			return err
		}
	}
	if c.uploads != nil {
		if err := c.uploads.wait(); err != nil {
			return err
		}
	}
	for k := 0; k < len(c.assignments); k += 128 {
		k2 := k + 128
		if k2 > len(c.assignments) {
			k2 = len(c.assignments)
		}
		if err := client.AssignDynamicChunks(c.wrid, c.assignments[k:k2], c.assignmentsOffset[k:k2]); err != nil {
			return err
		}
	}
	return client.CloseDynamicIndex(c.wrid, hex.EncodeToString(c.chunkdigests.Sum(nil)), c.pos, c.chunkcount)
}

func loadKnownChunks(client *pbscommon.PBSClient, archiveName string, stats *Stats) (*knownChunks, int, error) {
	if stats != nil {
		stats.SetStage(i18n.L("pbs.index_load_prev", nil))
	}
	previous, err := client.DownloadPreviousToBytes(archiveName)
	if err != nil {
		if previousIndexUnavailable(err) {
			return newKnownChunks(0), 0, nil
		}
		return nil, 0, err
	}
	if len(previous) == 0 {
		return newKnownChunks(0), 0, nil
	}
	if stats != nil {
		stats.SetStage(i18n.L("pbs.index_parse", map[string]string{"vol": formatByteSize(int64(len(previous)))}))
	}
	known, count, err := parseKnownFromPrevious(previous)
	if err != nil {
		return nil, 0, err
	}
	if stats != nil && count > 0 {
		stats.SetStage(i18n.L("pbs.index_chunks", map[string]string{"n": fmt.Sprintf("%d", count)}))
	}
	return known, count, nil
}

func formatByteSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f ГБ", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f МБ", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f КБ", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d Б", n)
	}
}

func backupReal(ctx context.Context, client *pbscommon.PBSClient, server models.PBSServer, secret, backupdir string, stats *Stats, jobID string, forceFull bool, bandwidthMbps int, globalExclusions, jobExclusions []string, skipAccessErrors bool) (*knownChunks, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	client.Connect(false, "host")
	client.Client.Timeout = pbsBackupTimeout
	committed := false

	var known *knownChunks
	var err error
	var hasPreviousIndex bool
	if forceFull {
		_ = chunkindex.Clear(jobID)
		known = newKnownChunks(0)
	} else {
		var serverChunks int
		known, serverChunks, err = loadKnownChunks(client, "backup.pxar.didx", stats)
		if err != nil {
			return nil, i18n.Ewrap("pbs.index_load_prev_err", nil, err)
		}
		hasPreviousIndex = serverChunks > 0
		// Known chunks come only from PBS previous-index; local chunks.json is write-only cache.
		if serverChunks == 0 {
			_ = chunkindex.Clear(jobID)
		}
	}

	ensureFastCache(ctx, server, secret, jobID, client.Manifest.BackupID, backupdir, forceFull, hasPreviousIndex, stats)

	if stats != nil {
		stats.SetStage(i18n.L("pbs.scan_files", nil))
	}

	fi, err := newFastIncremental(jobID, backupdir, forceFull)
	if err != nil {
		return nil, i18n.Ewrap("pbs.fast_inc_err", nil, err)
	}
	defer func() {
		fi.close()
		if !committed {
			client.AbortBackupSession()
		}
	}()
	if stats != nil {
		stats.SetFastReuseActive(fi.ReuseActive())
		if fi.reuseEnabled {
			stats.EstimatedFilesTotal.Store(int64(fi.cacheFileCount()))
		}
	}

	exc := exclude.NewForRoot(backupdir, exclude.Merge(globalExclusions, jobExclusions))
	if fi.reuseEnabled && fi.prev != nil {
		enrichIndexFromDisk(backupdir, fi.prev, exc, skipAccessErrors)
		if stats != nil {
			stats.SetStage(i18n.L("pbs.index_fast_cache", map[string]string{"count": formatCount(fi.cacheFileCount())}))
		}
	} else if stats != nil && fi.cacheEnabled && !forceFull {
		stats.SetStage(i18n.L("pbs.index_incr_first_pass", nil))
		warnCacheDiskSpace(stats, jobID)
	}

	var limiter *bandwidthLimiter
	if bandwidthMbps > 0 {
		limiter = newBandwidthLimiter(int64(bandwidthMbps) * 1024 * 1024 / 8)
	}

	archive := &pbscommon.PXARArchive{
		ArchiveName:  "backup.pxar.didx",
		FilesTotal:   &stats.FilesTotal,
		FilesSkipped: &stats.FilesSkipped,
		SkipUnreadableDirs: skipAccessErrors,
		Abort: func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		},
	}
	archive.ShouldSkip = func(fullPath, name string, isDir bool) bool {
		return exc.MatchPath(fullPath, name, isDir)
	}
	fi.wire(archive)

	pxarChunk := chunkState{}
	pxarChunk.init(ctx, stats, known, limiter)
	pcatKnown := newKnownChunks(0)
	pcatChunk := chunkState{}
	pcatChunk.init(ctx, stats, pcatKnown, limiter)

	pxarChunk.wrid, err = client.CreateDynamicIndex(archive.ArchiveName)
	if err != nil {
		return nil, err
	}
	pcatChunk.wrid, err = client.CreateDynamicIndex("catalog.pcat1.didx")
	if err != nil {
		return nil, err
	}

	abortScan := func(primary error) (*knownChunks, error) {
		var waitErr error
		if err := pxarChunk.drainUploads(); err != nil {
			waitErr = err
		}
		if err := pcatChunk.drainUploads(); err != nil && waitErr == nil {
			waitErr = err
		}
		if primary != nil {
			return nil, primary
		}
		return nil, waitErr
	}

	if st, err := os.Stat(backupdir); err != nil {
		return nil, i18n.Ewrap("pbs.source_inaccessible", map[string]string{"path": backupdir}, err)
	} else if !st.IsDir() {
		return nil, i18n.Ef("pbs.source_not_dir", map[string]string{"path": backupdir})
	}

	var streamErr error
	var pxarStreamBytes uint64
	var filePxarStart uint64
	indexRecorder := newPxarIndexRecorder()
	archive.OnPxarChunksReuse = func(header []byte, chunks []pbscommon.PXARFastChunk, _ uint64) error {
		if streamErr != nil {
			return streamErr
		}
		var virtual int64
		virtual += int64(len(header))
		pxarStreamBytes += uint64(len(header))
		for _, ch := range chunks {
			if ch.Len > 0 {
				virtual += int64(ch.Len)
				pxarStreamBytes += uint64(ch.Len)
			}
		}
		if stats != nil {
			stats.VirtualBytesProcessed.Add(virtual)
		}
		if err := indexRecorder.feedHeadersThenSkipPayload(header); err != nil {
			streamErr = err
			return err
		}
		if err := pxarChunk.handleData(header, client); err != nil {
			streamErr = err
			return err
		}
		if err := pxarChunk.reuseChunks(chunks, client); err != nil {
			streamErr = err
			return err
		}
		return nil
	}
	archive.OnPxarStreamReuse = func(blob []byte, chunks []pbscommon.PXARFastChunk) error {
		if streamErr != nil {
			return streamErr
		}
		if stats != nil && len(blob) > 0 {
			stats.VirtualBytesProcessed.Add(int64(len(blob)))
		}
		pxarStreamBytes += uint64(len(blob))
		if _, err := indexRecorder.feed(blob); err != nil {
			streamErr = err
			return err
		}
		if err := pxarChunk.reuseChunks(chunks, client); err != nil {
			streamErr = err
			return err
		}
		return nil
	}
	archive.OnFilePxarBegin = func() {
		filePxarStart = pxarStreamBytes
	}
	archive.OnFilePxarEnd = func(path, basename string, info os.FileInfo) {
		spans := chunkSpansFromAssignments(
			pxarChunk.assignments,
			pxarChunk.assignmentsOffset,
			pxarChunk.pos,
			filePxarStart,
			pxarStreamBytes,
		)
		fi.recordFile(path, info, spans, int64(pxarStreamBytes-filePxarStart))
	}
	archive.WriteCB = func(b []byte) {
		if streamErr != nil {
			return
		}
		pxarStreamBytes += uint64(len(b))
		if _, err := indexRecorder.feed(b); err != nil {
			streamErr = err
			return
		}
		if err := pxarChunk.handleData(b, client); err != nil {
			streamErr = err
		}
	}
	archive.CatalogWriteCB = func(b []byte) {
		if streamErr != nil {
			return
		}
		if err := pcatChunk.handleData(b, client); err != nil {
			streamErr = err
		}
	}

	select {
	case <-ctx.Done():
		return abortScan(ctx.Err())
	default:
	}
	if _, err := archive.WriteDir(backupdir, "", true); err != nil {
		if errors.Is(err, context.Canceled) {
			return abortScan(err)
		}
		return abortScan(i18n.Ewrap("pbs.walk_dir", nil, err))
	}

	if streamErr != nil {
		return abortScan(streamErr)
	}
	if err := ctx.Err(); err != nil {
		return abortScan(err)
	}
	if err := pxarChunk.eof(client); err != nil {
		return nil, err
	}
	if pxarChunk.chunkcount == 0 {
		return nil, i18n.Ef("pbs.pxar_empty", map[string]string{"path": backupdir})
	}
	if err := pcatChunk.eof(client); err != nil {
		return nil, i18n.Ewrap("pbs.catalog_close", nil, err)
	}
	if pcatChunk.chunkcount == 0 {
		return nil, i18n.Ef("pbs.catalog_empty", map[string]string{"path": backupdir})
	}
	if err := uploadWinMeta(client, backupdir); err != nil {
		return nil, err
	}
	if err := uploadPxarIndex(client, indexRecorder.index); err != nil {
		return nil, i18n.Ewrap("pbs.pxar_index_upload", nil, err)
	}
	backupTime := client.Manifest.BackupTime
	if backupTime <= 0 {
		backupTime = time.Now().Unix()
	}
	snapshotTime := time.Unix(backupTime, 0).UTC().Format(time.RFC3339)
	if err := client.UploadManifest(); err != nil {
		return nil, err
	}
	if err := client.Finish(); err != nil {
		return nil, err
	}
	committed = true
	if err := fi.save(snapshotTime); err != nil {
		_ = ClearPBSFileIndex(jobID)
		return nil, i18n.Ewrap("pbs.fast_index_save", nil, err)
	}
	_ = saveLocalPxarIndex(SnapshotRef{
		BackupID:   client.Manifest.BackupID,
		Time:       snapshotTime,
		BackupTime: backupTime,
	}, indexRecorder.index)
	return known, nil
}

func runDirectoryBackup(ctx context.Context, client *pbscommon.PBSClient, server models.PBSServer, secret, backupdir string, useVSS bool, stats *Stats, jobID string, forceFull bool, bandwidthMbps int, globalExclusions, jobExclusions []string, skipAccessErrors bool) (*knownChunks, error) {
	if useVSS {
		var known *knownChunks
		err := snapshot.CreateVSSSnapshot([]string{backupdir}, func(snaps map[string]snapshot.SnapShot) error {
			k := maps.Keys(snaps)
			k2 := slices.Collect(k)
			if len(k2) == 0 {
				return i18n.E("pbs.vss_snapshot_failed", nil)
			}
			snapPath := snaps[k2[0]].FullPath
			var innerErr error
			known, innerErr = backupReal(ctx, client, server, secret, snapPath, stats, jobID, forceFull, bandwidthMbps, globalExclusions, jobExclusions, skipAccessErrors)
			return innerErr
		})
		return known, err
	}
	return backupReal(ctx, client, server, secret, backupdir, stats, jobID, forceFull, bandwidthMbps, globalExclusions, jobExclusions, skipAccessErrors)
}

func newPBSClient(server models.PBSServer, secret, backupID string) *pbscommon.PBSClient {
	return &pbscommon.PBSClient{
		BaseURL:         strings.TrimRight(server.URL, "/"),
		CertFingerPrint: server.Fingerprint,
		AuthID:          server.TokenID,
		Secret:          secret,
		Datastore:       server.Datastore,
		Namespace:       server.Namespace,
		Manifest: pbscommon.BackupManifest{
			BackupID: backupID,
		},
	}
}

// backupHostname avoids import cycle with backup package.
func backupHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "windows-host"
	}
	return h
}
