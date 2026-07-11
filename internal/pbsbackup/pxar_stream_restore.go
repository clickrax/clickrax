package pbsbackup

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
	"pbs-win-backup/internal/restorepolicy"
)

var (
	payloadFileSync = func(f *os.File) error { return f.Sync() }
	payloadSyncDir  = func(dir string) error { return paths.SyncDir(dir) }
)

type pxarWalkState int

const (
	pxarWalkRootEntry pxarWalkState = iota
	pxarWalkInDir
	pxarWalkReadPayload
	pxarWalkSkipPayload
)

type pxarPathFrame struct {
	name string
	skip bool
}

type pxarStreamParser struct {
	targets   map[string]*pxarRestoreTarget
	pending   int
	meta      filemeta.Archive
	overwrite string
	force     bool
	onFile    RestoreFolderProgress
	total     int
	done      int

	indexMode   bool
	recordIndex bool
	index       *pxarFileIndex
	streamOff   uint64
	ctx         context.Context

	pendingEntryStart uint64
	pendingEntryPath  string

	onIndexEntry func(filePath string, entryStart, entryEnd, payloadSize uint64) error
	indexErr     error

	carry []byte

	state         pxarWalkState
	pathStack     []pxarPathFrame
	payloadRemain int
	payloadTarget *pxarRestoreTarget
	payloadPath   string
	payloadOut    *os.File
	payloadBuf    *bufio.Writer
	payloadDest   string
	payloadTmp    string
}

func newPxarStreamParser(
	ctx context.Context,
	targets []pxarRestoreTarget,
	meta filemeta.Archive,
	overwriteMode string,
	forceOverwrite bool,
	onFile RestoreFolderProgress,
) *pxarStreamParser {
	m := make(map[string]*pxarRestoreTarget, len(targets))
	for i := range targets {
		for _, key := range targetPathKeys(targets[i].FilePath) {
			m[key] = &targets[i]
		}
	}
	return &pxarStreamParser{
		targets:   m,
		pending:   len(targets),
		meta:      meta,
		overwrite: overwriteMode,
		force:     forceOverwrite,
		onFile:    onFile,
		total:     len(targets),
		ctx:       ctx,
	}
}

func targetPathKeys(filePath string) []string {
	keys := []string{
		strings.ToLower(normalizeRestorePath(filePath)),
	}
	rel := strings.ToLower(catalogRelPath(filePath))
	if rel != keys[0] {
		keys = append(keys, rel)
	}
	return keys
}

func (p *pxarStreamParser) currentPath() string {
	parts := make([]string, len(p.pathStack))
	for i, f := range p.pathStack {
		parts[i] = f.name
	}
	return strings.Join(parts, `\`)
}

func (p *pxarStreamParser) inSkip() bool {
	for _, f := range p.pathStack {
		if f.skip {
			return true
		}
	}
	return false
}

func (p *pxarStreamParser) dirRelevant(dirPath string) bool {
	if p.indexMode {
		return true
	}
	norm := strings.ToLower(normalizeRestorePath(dirPath))
	for key := range p.targets {
		if strings.HasPrefix(key, norm+`\`) {
			return true
		}
	}
	return norm == ""
}

func (p *pxarStreamParser) lookupTarget(filePath string) *pxarRestoreTarget {
	for _, key := range targetPathKeys(filePath) {
		if t, ok := p.targets[key]; ok {
			if !t.restored {
				return t
			}
		}
	}
	return nil
}

func (p *pxarStreamParser) feed(chunk []byte) (allDone bool, err error) {
	if err := abortIfCancelled(p.ctx); err != nil {
		return false, err
	}
	if len(chunk) > 0 {
		p.carry = append(p.carry, chunk...)
	}
	for {
		progressed, err := p.step()
		if err != nil {
			_ = p.abortPayload()
			return false, err
		}
		if p.indexErr != nil {
			_ = p.abortPayload()
			return false, p.indexErr
		}
		if !progressed {
			return p.pending == 0, nil
		}
		if p.pending == 0 && !p.indexMode {
			_ = p.abortPayload()
			return true, nil
		}
	}
}

func (p *pxarStreamParser) finish() (int, error) {
	if p.pending > 0 {
		return p.done, i18n.Ef("restore.partial_files", map[string]string{
			"done":  fmt.Sprintf("%d", p.done),
			"total": fmt.Sprintf("%d", p.total),
		})
	}
	return p.done, nil
}

func (p *pxarStreamParser) step() (bool, error) {
	switch p.state {
	case pxarWalkRootEntry:
		return p.stepRootEntry()
	case pxarWalkInDir:
		return p.stepInDir()
	case pxarWalkReadPayload:
		return p.stepPayload(false)
	case pxarWalkSkipPayload:
		return p.stepPayload(true)
	default:
		return false, i18n.Ef("pxar.unknown_state", map[string]string{"state": fmt.Sprintf("%d", p.state)})
	}
}

func (p *pxarStreamParser) stepRootEntry() (bool, error) {
	if !p.need(16) {
		return false, nil
	}
	if p.u64(0) != pxarEntry {
		return false, i18n.E("pxar.expected_root_entry", nil)
	}
	entryLen := int(p.u64(8))
	if !p.need(entryLen) {
		return false, nil
	}
	mode := p.u64(16)
	if mode&0o170000 != pxarIFDIR {
		return false, i18n.E("pxar.root_not_dir", nil)
	}
	p.consume(entryLen)
	p.state = pxarWalkInDir
	return true, nil
}

func (p *pxarStreamParser) stepInDir() (bool, error) {
	if !p.need(8) {
		return false, nil
	}
	hdr := p.u64(0)
	switch hdr {
	case pxarGoodbye:
		return p.stepGoodbye()
	case pxarFilename:
		return p.stepFilename()
	default:
		return false, i18n.Ef("pxar.unexpected_dir_block", map[string]string{
			"hdr": fmt.Sprintf("%x", hdr), "path": p.currentPath(),
		})
	}
}

func (p *pxarStreamParser) stepGoodbye() (bool, error) {
	if !p.need(16) {
		return false, nil
	}
	blockLen := int(p.u64(8))
	if !p.need(blockLen) {
		return false, nil
	}
	p.consume(blockLen)
	if len(p.pathStack) > 0 {
		p.pathStack = p.pathStack[:len(p.pathStack)-1]
	}
	p.state = pxarWalkInDir
	return true, nil
}

func (p *pxarStreamParser) stepFilename() (bool, error) {
	if !p.need(16) {
		return false, nil
	}
	nameBlockLen := int(p.u64(8))
	if nameBlockLen < 17 || !p.need(nameBlockLen+16) {
		return false, nil
	}
	entryLen := int(p.u64(nameBlockLen + 8))
	if !p.need(nameBlockLen + entryLen) {
		return false, nil
	}
	if p.carry[nameBlockLen-1] != 0 {
		return false, fmt.Errorf("pxar: filename terminator")
	}
	name := string(p.carry[16 : nameBlockLen-1])
	if p.u64(nameBlockLen) != pxarEntry {
		return false, i18n.Ef("pxar.expected_entry_for", map[string]string{"name": name})
	}
	mode := p.u64(nameBlockLen + 16)
	p.consume(nameBlockLen + entryLen)

	childPath := name
	if cp := p.currentPath(); cp != "" {
		childPath = cp + `\` + name
	}
	isDir := mode&0o170000 == pxarIFDIR
	if p.indexMode && !isDir {
		p.pendingEntryStart = p.streamOff
		p.pendingEntryPath = childPath
	}
	skipping := p.inSkip()

	if isDir {
		skipSubtree := skipping || !p.dirRelevant(childPath)
		p.pathStack = append(p.pathStack, pxarPathFrame{name: name, skip: skipSubtree})
		p.state = pxarWalkInDir
		return true, nil
	}

	if skipping {
		p.payloadPath = childPath
		return p.beginSkipPayload()
	}
	if t := p.lookupTarget(childPath); t != nil {
		p.payloadPath = childPath
		return p.beginReadPayload(t)
	}
	p.payloadPath = childPath
	return p.beginSkipPayload()
}

func (p *pxarStreamParser) beginSkipPayload() (bool, error) {
	if !p.need(16) {
		return false, nil
	}
	if p.u64(0) != pxarPayload {
		return false, i18n.E("pxar.expected_payload", nil)
	}
	plen := int(p.u64(8))
	if plen < 16 {
		return false, i18n.E("pxar.invalid_payload_header", nil)
	}
	p.recordIndexPayload(p.payloadPath, plen-16)
	p.payloadRemain = plen - 16
	p.consume(16)
	p.state = pxarWalkSkipPayload
	return true, nil
}

func (p *pxarStreamParser) beginReadPayload(t *pxarRestoreTarget) (bool, error) {
	if err := restorepolicy.PrepareExistingDest(t.Dest, p.overwrite, p.force); err != nil {
		return false, err
	}
	if !p.need(16) {
		return false, nil
	}
	if p.u64(0) != pxarPayload {
		return false, i18n.E("pxar.expected_payload", nil)
	}
	plen := int(p.u64(8))
	if plen < 16 {
		return false, i18n.E("pxar.invalid_payload_header", nil)
	}
	p.recordIndexPayload(t.FilePath, plen-16)
	p.payloadRemain = plen - 16
	p.consume(16)
	p.payloadTarget = t
	p.payloadDest = t.Dest
	p.payloadTmp = t.Dest + ".restoring"
	if err := os.MkdirAll(filepath.Dir(t.Dest), 0o755); err != nil {
		return false, err
	}
	out, err := os.OpenFile(p.payloadTmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return false, err
	}
	p.payloadOut = out
	p.payloadBuf = bufio.NewWriterSize(out, restoreWriteBufferSize)
	p.state = pxarWalkReadPayload
	return true, nil
}

func (p *pxarStreamParser) finishIndexFileEntry() {
	if p.index == nil || p.pendingEntryPath == "" {
		return
	}
	pos, ok := p.index.lookup(p.pendingEntryPath)
	if !ok {
		pos = pxarFilePos{}
	}
	pos.EntryStart = p.pendingEntryStart
	pos.EntryEnd = p.streamOff
	p.index.set(p.pendingEntryPath, pos)
	if p.onIndexEntry != nil {
		if err := p.onIndexEntry(p.pendingEntryPath, p.pendingEntryStart, p.streamOff, pos.Size); err != nil {
			p.indexErr = err
		}
	}
	p.pendingEntryPath = ""
}

// advanceSkipPayload completes a skipped payload in index mode without feeding payload bytes.
func (p *pxarStreamParser) advanceSkipPayload() error {
	if p.state != pxarWalkSkipPayload {
		return nil
	}
	p.streamOff += uint64(p.payloadRemain)
	p.payloadRemain = 0
	if p.indexMode {
		p.finishIndexFileEntry()
	}
	p.state = pxarWalkInDir
	return nil
}

func (p *pxarStreamParser) recordIndexPayload(filePath string, dataSize int) {
	if p.index == nil || filePath == "" || dataSize < 0 {
		return
	}
	if !p.recordIndex && !p.indexMode {
		return
	}
	p.index.set(filePath, pxarFilePos{
		Offset: p.streamOff,
		Size:   uint64(dataSize),
	})
}

func (p *pxarStreamParser) stepPayload(skip bool) (bool, error) {
	if p.payloadRemain == 0 {
		if skip {
			if p.indexMode {
				p.finishIndexFileEntry()
			}
			p.state = pxarWalkInDir
			return true, nil
		}
		return p.finishPayload()
	}
	n := p.payloadRemain
	if n > len(p.carry) {
		n = len(p.carry)
	}
	if n == 0 {
		return false, nil
	}
	if !skip {
		if _, err := p.payloadBuf.Write(p.carry[:n]); err != nil {
			return false, err
		}
	}
	p.consume(n)
	p.payloadRemain -= n
	if p.payloadRemain > 0 {
		return true, nil
	}
	if skip {
		if p.indexMode {
			p.finishIndexFileEntry()
		}
		p.state = pxarWalkInDir
		return true, nil
	}
	return p.finishPayload()
}

func (p *pxarStreamParser) finishPayload() (bool, error) {
	t := p.payloadTarget
	out := p.payloadOut
	bw := p.payloadBuf
	tmp := p.payloadTmp
	dest := p.payloadDest

	p.payloadTarget = nil
	p.payloadOut = nil
	p.payloadBuf = nil
	p.payloadTmp = ""
	p.payloadDest = ""
	p.state = pxarWalkInDir

	if err := bw.Flush(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return false, err
	}
	if err := payloadFileSync(out); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return false, err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	if err := payloadSyncDir(filepath.Dir(dest)); err != nil {
		return false, err
	}
	if err := applyRestoredMeta(p.meta, t.FilePath, dest, t.Modified); err != nil {
		return false, err
	}
	p.markDone(t)
	return true, nil
}

func (p *pxarStreamParser) markDone(t *pxarRestoreTarget) {
	if t.restored {
		return
	}
	t.restored = true
	for key, cur := range p.targets {
		if cur == t {
			delete(p.targets, key)
		}
	}
	p.pending--
	p.done++
	if p.onFile != nil {
		p.onFile(p.done, p.total, t.FilePath)
	}
}

func (p *pxarStreamParser) abortPayload() error {
	if p.payloadOut == nil {
		return nil
	}
	tmp := p.payloadTmp
	if p.payloadBuf != nil {
		_ = p.payloadBuf.Flush()
	}
	_ = p.payloadOut.Close()
	p.payloadOut = nil
	p.payloadBuf = nil
	p.payloadTmp = ""
	p.payloadTarget = nil
	if tmp != "" {
		_ = os.Remove(tmp)
	}
	return nil
}

func (p *pxarStreamParser) need(n int) bool {
	return len(p.carry) >= n
}

func (p *pxarStreamParser) u64(off int) uint64 {
	return binary.LittleEndian.Uint64(p.carry[off:])
}

func (p *pxarStreamParser) consume(n int) {
	if n > len(p.carry) {
		n = len(p.carry)
	}
	p.carry = p.carry[n:]
	p.streamOff += uint64(n)
}

// streamRestorePXARTargets downloads pxar chunks and writes matching files directly to disk.
func streamRestorePXARTargets(
	ctx context.Context,
	server models.PBSServer,
	secret string,
	ref SnapshotRef,
	targets []pxarRestoreTarget,
	meta filemeta.Archive,
	overwriteMode string,
	forceOverwrite bool,
	onChunkProgress StreamProgress,
	onFileProgress RestoreFolderProgress,
) (int, error) {
	if len(targets) == 0 {
		return 0, i18n.E("restore.no_files_pxar", nil)
	}

	if idx, ok, err := resolvePxarIndex(server, secret, ref, targets); err != nil {
		return 0, err
	} else if ok {
		indexed, _ := partitionTargetsByIndex(idx, targets)
		if onFileProgress != nil {
			onFileProgress(0, len(targets), i18n.L("pbs.selective_index", map[string]string{"count": fmt.Sprintf("%d", len(indexed))}))
		}
		return streamRestorePXARTargetsIndexed(
			ctx, server, secret, ref, targets, idx, meta, overwriteMode, forceOverwrite,
			onChunkProgress, onFileProgress,
		)
	}

	return streamRestorePXARTargetsSequential(
		ctx, server, secret, ref, targets, meta, overwriteMode, forceOverwrite,
		onChunkProgress, onFileProgress,
	)
}

func streamRestorePXARTargetsSequential(
	ctx context.Context,
	server models.PBSServer,
	secret string,
	ref SnapshotRef,
	targets []pxarRestoreTarget,
	meta filemeta.Archive,
	overwriteMode string,
	forceOverwrite bool,
	onChunkProgress StreamProgress,
	onFileProgress RestoreFolderProgress,
) (int, error) {
	parser := newPxarStreamParser(ctx, targets, meta, overwriteMode, forceOverwrite, onFileProgress)
	parser.recordIndex = true
	parser.index = newPxarFileIndex()
	if onFileProgress != nil {
		onFileProgress(0, len(targets), i18n.L("pbs.full_pass", nil))
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

	if onChunkProgress != nil {
		onChunkProgress(0, 0, i18n.L("pbs.archive_index_load", nil))
	}
	raw, err := client.DownloadToBytes(pxarName)
	if err != nil {
		return 0, i18n.Ef("pbs.restore.load", map[string]string{"name": pxarName, "err": err.Error()})
	}
	if len(raw) == 0 {
		return 0, i18n.Ef("pbs.restore.empty_pbs", map[string]string{"name": pxarName})
	}
	if bytesHasCatalogMagic(raw) {
		return 0, i18n.Ef("pbs.restore.not_pxar", map[string]string{"name": pxarName})
	}

	records, err := parseDidxRecords(raw)
	if err != nil {
		return 0, err
	}
	getChunk := func(digest string) ([]byte, error) {
		return getChunkVerified(ctx, client, digest, chunkDownloadTimeout)
	}

	totalChunks := len(records)
	var downloaded uint64
	for i, r := range records {
		if err := abortIfCancelled(ctx); err != nil {
			return parser.done, err
		}
		if onChunkProgress != nil {
			mb := float64(downloaded) / (1024 * 1024)
			onChunkProgress(i, totalChunks, i18n.L("pbs.chunk_load", map[string]string{
				"path": pxarName,
				"n":    fmt.Sprintf("%d", i+1),
				"max":  fmt.Sprintf("%d", totalChunks),
				"vol":  fmt.Sprintf("%.1f MB", mb),
			}))
		}
		chunk, err := getChunk(r.digest)
		if err != nil {
			return parser.done, fmt.Errorf("chunk %s: %w", r.digest[:min(12, len(r.digest))], err)
		}
		end := downloaded + uint64(len(chunk))
		if end != r.offset {
			return parser.done, i18n.Ef("pbs.didx.offset_mismatch", map[string]string{
				"end":    fmt.Sprintf("%d", end),
				"offset": fmt.Sprintf("%d", r.offset),
			})
		}
		allDone, err := parser.feed(chunk)
		if err != nil {
			return parser.done, err
		}
		downloaded = end
		if onChunkProgress != nil {
			onChunkProgress(i+1, totalChunks, i18n.L("pbs.chunk_loaded", map[string]string{
				"n":   fmt.Sprintf("%d", i+1),
				"max": fmt.Sprintf("%d", totalChunks),
				"vol": fmt.Sprintf("%.1f MB", float64(downloaded)/(1024*1024)),
			}))
		}
		if allDone {
			_ = saveLocalPxarIndex(ref, parser.index)
			return parser.done, nil
		}
	}
	count, err := parser.finish()
	if err == nil {
		_ = saveLocalPxarIndex(ref, parser.index)
	}
	return count, err
}
