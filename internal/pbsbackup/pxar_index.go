package pbsbackup

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
	"pbs-win-backup/internal/restorepolicy"

	pbscommon "pbscommon"
)

const pxarIndexBlobName = "backup.pxar.index.json"
const pxarIndexVersion = 1

type pxarFilePos struct {
	Offset     uint64 `json:"offset"` // absolute stream offset of PXAR_PAYLOAD header
	Size       uint64 `json:"size"`   // payload data bytes (excluding 16-byte header)
	EntryStart uint64 `json:"entry_start,omitempty"`
	EntryEnd   uint64 `json:"entry_end,omitempty"`
}

type pxarFileIndex struct {
	Version int                    `json:"version"`
	Files   map[string]pxarFilePos `json:"files"`
}

func newPxarFileIndex() *pxarFileIndex {
	return &pxarFileIndex{
		Version: pxarIndexVersion,
		Files:   make(map[string]pxarFilePos),
	}
}

func (idx *pxarFileIndex) set(filePath string, pos pxarFilePos) {
	for _, key := range targetPathKeys(filePath) {
		idx.Files[key] = pos
	}
}

func (idx *pxarFileIndex) lookup(filePath string) (pxarFilePos, bool) {
	for _, key := range targetPathKeys(filePath) {
		if pos, ok := idx.Files[key]; ok {
			return pos, true
		}
	}
	return pxarFilePos{}, false
}

func (idx *pxarFileIndex) covers(targets []pxarRestoreTarget) bool {
	for _, t := range targets {
		if _, ok := idx.lookup(t.FilePath); !ok {
			return false
		}
	}
	return len(targets) > 0
}

func marshalPxarIndex(idx *pxarFileIndex) ([]byte, error) {
	if idx == nil {
		idx = newPxarFileIndex()
	}
	if idx.Files == nil {
		idx.Files = map[string]pxarFilePos{}
	}
	idx.Version = pxarIndexVersion
	return json.Marshal(idx)
}

func unmarshalPxarIndex(raw []byte) (pxarFileIndex, error) {
	var idx pxarFileIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		return idx, err
	}
	if idx.Files == nil {
		idx.Files = map[string]pxarFilePos{}
	}
	return idx, nil
}

func uploadPxarIndex(client *pbscommon.PBSClient, idx *pxarFileIndex, stats *Stats) error {
	if idx == nil || len(idx.Files) == 0 {
		return nil
	}
	data, err := marshalPxarIndex(idx)
	if err != nil {
		return err
	}
	return uploadBlobToPBS(client, stats, pxarIndexBlobName, data)
}

func loadPxarIndex(server models.PBSServer, secret string, ref SnapshotRef) (pxarFileIndex, bool, error) {
	client, cleanup, err := openReader(server, secret, ref)
	if err != nil {
		return pxarFileIndex{}, false, err
	}
	defer cleanup()

	raw, err := client.DownloadToBytes(pxarIndexBlobName)
	if err != nil {
		return pxarFileIndex{}, false, nil
	}
	idx, err := unmarshalPxarIndex(raw)
	if err != nil {
		return pxarFileIndex{}, false, fmt.Errorf("pxar index: %w", err)
	}
	return idx, len(idx.Files) > 0, nil
}

func localPxarIndexPath(ref SnapshotRef) (string, error) {
	base, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s_%s.json", ref.BackupID, strings.ReplaceAll(ref.Time, ":", "-"))
	return filepath.Join(base, "pxar-index", name), nil
}

func loadLocalPxarIndex(ref SnapshotRef) (pxarFileIndex, bool, error) {
	path, err := localPxarIndexPath(ref)
	if err != nil {
		return pxarFileIndex{}, false, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return pxarFileIndex{}, false, nil
	}
	idx, err := unmarshalPxarIndex(raw)
	if err != nil {
		return pxarFileIndex{}, false, err
	}
	return idx, len(idx.Files) > 0, nil
}

func saveLocalPxarIndex(ref SnapshotRef, idx *pxarFileIndex) error {
	if idx == nil || len(idx.Files) == 0 {
		return nil
	}
	path, err := localPxarIndexPath(ref)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := marshalPxarIndex(idx)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func newPxarIndexRecorder() *pxarStreamParser {
	p := newPxarStreamParser(context.Background(), nil, filemeta.Archive{}, "", false, nil)
	p.indexMode = true
	p.index = newPxarFileIndex()
	return p
}

func (p *pxarStreamParser) feedHeadersThenSkipPayload(header []byte) error {
	if p == nil {
		return nil
	}
	if _, err := p.feed(header); err != nil {
		return err
	}
	if p.indexErr != nil {
		return p.indexErr
	}
	return p.advanceSkipPayload()
}

func chunkIndicesForRange(records []didxRecord, start, end uint64) []int {
	if end <= start || len(records) == 0 {
		return nil
	}
	var prev uint64
	out := make([]int, 0)
	for i, r := range records {
		chunkEnd := r.offset
		chunkStart := prev
		if start < chunkEnd && end > chunkStart {
			out = append(out, i)
		}
		prev = chunkEnd
	}
	return out
}

func unionChunkIndices(records []didxRecord, ranges [][2]uint64) []int {
	seen := make(map[int]bool)
	out := make([]int, 0)
	for _, rg := range ranges {
		for _, i := range chunkIndicesForRange(records, rg[0], rg[1]) {
			if !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	sort.Ints(out)
	return out
}

type chunkView struct {
	records []didxRecord
	chunks  map[int][]byte
}

func newChunkView(records []didxRecord) *chunkView {
	return &chunkView{records: records, chunks: make(map[int][]byte)}
}

func (v *chunkView) add(index int, data []byte) {
	v.chunks[index] = data
}

func (v *chunkView) readAt(off uint64, n int) ([]byte, error) {
	if n <= 0 {
		return nil, nil
	}
	out := make([]byte, n)
	written := 0
	remaining := n
	pos := off
	for remaining > 0 {
		idx, chunkStart, chunkEnd, ok := v.locate(pos)
		if !ok {
			return nil, i18n.Ef("pxar.read_beyond_blocks", map[string]string{"offset": fmt.Sprintf("%d", pos)})
		}
		chunk, ok := v.chunks[idx]
		if !ok {
			return nil, i18n.Ef("pxar.block_not_loaded", map[string]string{"n": fmt.Sprintf("%d", idx+1)})
		}
		inChunk := int(pos - chunkStart)
		avail := int(chunkEnd-chunkStart) - inChunk
		if avail <= 0 {
			return nil, i18n.Ef("pxar.empty_block", map[string]string{"offset": fmt.Sprintf("%d", pos)})
		}
		nCopy := remaining
		if nCopy > avail {
			nCopy = avail
		}
		copy(out[written:], chunk[inChunk:inChunk+nCopy])
		written += nCopy
		remaining -= nCopy
		pos += uint64(nCopy)
	}
	return out, nil
}

func (v *chunkView) locate(off uint64) (index int, chunkStart, chunkEnd uint64, ok bool) {
	var prev uint64
	for i, r := range v.records {
		if off < r.offset {
			return i, prev, r.offset, true
		}
		prev = r.offset
	}
	return 0, 0, 0, false
}

func (v *chunkView) hasRange(start, end uint64) bool {
	if end <= start {
		return true
	}
	_, err := v.readAt(start, 1)
	if err != nil {
		return false
	}
	_, err = v.readAt(end-1, 1)
	return err == nil
}

func (v *chunkView) pruneBefore(minOffset uint64) {
	for idx := range v.chunks {
		if idx+1 >= len(v.records) {
			delete(v.chunks, idx)
			continue
		}
		chunkEnd := v.records[idx].offset
		if chunkEnd <= minOffset {
			delete(v.chunks, idx)
		}
	}
}

func partitionTargetsByIndex(idx pxarFileIndex, targets []pxarRestoreTarget) (indexed, missing []pxarRestoreTarget) {
	for _, t := range targets {
		if _, ok := idx.lookup(t.FilePath); ok {
			indexed = append(indexed, t)
		} else {
			missing = append(missing, t)
		}
	}
	return indexed, missing
}

func extractPayloadFromView(view *chunkView, pos pxarFilePos) ([]byte, error) {
	hdr, err := view.readAt(pos.Offset, 16)
	if err != nil {
		return nil, err
	}
	if len(hdr) < 16 || binary.LittleEndian.Uint64(hdr) != pxarPayload {
		return nil, i18n.Ef("pxar.invalid_payload", map[string]string{"offset": fmt.Sprintf("%d", pos.Offset)})
	}
	plen := binary.LittleEndian.Uint64(hdr[8:])
	want := int(plen) - 16
	if want < 0 || uint64(want) != pos.Size {
		return nil, i18n.Ef("pxar.payload_size_mismatch", map[string]string{"offset": fmt.Sprintf("%d", pos.Offset)})
	}
	return view.readAt(pos.Offset+16, want)
}

func indexRangesForTargets(idx pxarFileIndex, targets []pxarRestoreTarget) ([][2]uint64, error) {
	ranges := make([][2]uint64, 0, len(targets))
	for _, t := range targets {
		pos, ok := idx.lookup(t.FilePath)
		if !ok {
			return nil, i18n.Ef("pxar.file_not_in_index", map[string]string{"path": t.FilePath})
		}
		end := pos.Offset + 16 + pos.Size
		ranges = append(ranges, [2]uint64{pos.Offset, end})
	}
	return ranges, nil
}

func resolvePxarIndex(server models.PBSServer, secret string, ref SnapshotRef, targets []pxarRestoreTarget) (pxarFileIndex, bool, error) {
	idx, ok, err := loadPxarIndex(server, secret, ref)
	if err != nil {
		return pxarFileIndex{}, false, err
	}
	if !ok {
		local, lok, err := loadLocalPxarIndex(ref)
		if err != nil {
			return pxarFileIndex{}, false, err
		}
		if lok {
			idx = local
			ok = true
		}
	}
	if !ok || len(idx.Files) == 0 {
		return pxarFileIndex{}, false, nil
	}
	indexed, _ := partitionTargetsByIndex(idx, targets)
	if len(indexed) == 0 {
		return pxarFileIndex{}, false, nil
	}
	return idx, true, nil
}

func streamRestorePXARTargetsIndexed(
	ctx context.Context,
	server models.PBSServer,
	secret string,
	ref SnapshotRef,
	targets []pxarRestoreTarget,
	idx pxarFileIndex,
	meta filemeta.Archive,
	overwriteMode string,
	forceOverwrite bool,
	onChunkProgress StreamProgress,
	onFileProgress RestoreFolderProgress,
) (int, error) {
	indexed, missing := partitionTargetsByIndex(idx, targets)
	if len(indexed) == 0 {
		return 0, i18n.E("pxar.no_selective_files", nil)
	}

	pxarName, err := resolvePxarArchive(server, secret, ref)
	if err != nil {
		return 0, err
	}
	client, err := connectReader(server, secret, ref)
	if err != nil {
		return 0, err
	}
	defer closeReader(client)

	raw, err := client.DownloadToBytes(pxarName)
	if err != nil {
		return 0, i18n.Ewrap("pbs.restore.load", map[string]string{"name": pxarName}, err)
	}
	records, err := parseDidxRecords(raw)
	if err != nil {
		return 0, err
	}

	type orderedTarget struct {
		target pxarRestoreTarget
		pos    pxarFilePos
	}
	ordered := make([]orderedTarget, 0, len(indexed))
	for _, t := range indexed {
		pos, ok := idx.lookup(t.FilePath)
		if !ok {
			continue
		}
		ordered = append(ordered, orderedTarget{target: t, pos: pos})
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].pos.Offset < ordered[j].pos.Offset
	})

	view := newChunkView(records)

	done := 0
	total := len(targets)
	var downloaded uint64
	for i, item := range ordered {
		if err := abortIfCancelled(ctx); err != nil {
			return done, err
		}
		t := item.target
		pos := item.pos
		end := pos.Offset + 16 + pos.Size
		needed := chunkIndicesForRange(records, pos.Offset, end)
		missing := make([]int, 0, len(needed))
		for _, recIdx := range needed {
			if _, ok := view.chunks[recIdx]; !ok {
				missing = append(missing, recIdx)
			}
		}
		if len(missing) > 0 {
			getChunk := func(digest string) ([]byte, error) {
				return getChunkVerified(ctx, client, digest, chunkDownloadTimeout)
			}
			fetched, err := downloadChunksParallel(ctx, getChunk, records, missing, ChunkWorkers(), func(fetched, totalChunks int) {
				if onChunkProgress != nil {
					onChunkProgress(fetched, totalChunks, fmt.Sprintf("Загрузка %s: блок %d/%d (%.1f МБ)…",
						filepath.Base(t.FilePath), fetched, totalChunks, float64(downloaded)/(1024*1024)))
				}
			})
			if err != nil {
				return done, fmt.Errorf("chunk %w", err)
			}
			for recIdx, chunk := range fetched {
				view.add(recIdx, chunk)
				downloaded += uint64(len(chunk))
			}
		}

		if onFileProgress != nil {
			onFileProgress(done, total, t.FilePath)
		}
		if err := restorepolicy.PrepareExistingDest(t.Dest, overwriteMode, forceOverwrite); err != nil {
			return done, err
		}
		payload, err := extractPayloadFromView(view, pos)
		if err != nil {
			return done, fmt.Errorf("%s: %w", t.FilePath, err)
		}
		if err := WriteRestoredFile(t.Dest, payload); err != nil {
			return done, err
		}
		if err := applyRestoredMeta(meta, t.FilePath, t.Dest, t.Modified); err != nil {
			return done, err
		}
		done++
		if onFileProgress != nil {
			onFileProgress(done, total, t.FilePath)
		}

		if i+1 < len(ordered) {
			view.pruneBefore(ordered[i+1].pos.Offset)
		}
	}

	if len(missing) > 0 {
		if onFileProgress != nil {
			onFileProgress(done, total, fmt.Sprintf("Дозагрузка %d файлов без индекса…", len(missing)))
		}
		baseDone := done
		fileCB := onFileProgress
		if fileCB != nil {
			fileCB = func(d, tot int, path string) {
				onFileProgress(baseDone+d, total, path)
			}
		}
		extra, err := streamRestorePXARTargetsSequential(
			ctx, server, secret, ref, missing, meta, overwriteMode, forceOverwrite,
			onChunkProgress, fileCB,
		)
		done += extra
		if err != nil {
			return done, err
		}
	}
	return done, nil
}
