package filebackup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"pbs-win-backup/internal/backup/exclude"
	"pbs-win-backup/internal/backuproot"
	"pbs-win-backup/internal/fileindex"
	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/ftpclient"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/smbclient"
	"pbs-win-backup/internal/winattr"
)

type Stats struct {
	BytesTransferred atomic.Int64
	BytesSkipped     atomic.Int64
	FilesTotal       atomic.Int64
	FilesChanged     atomic.Int64
	FilesSkipped     atomic.Int64
	BackupKind       string
	RemotePath       string
}

// significantSkipRatio is the maximum allowed fraction of skipped files (10%).
const significantSkipRatioNum = 10

var (
	runUploadArchive       = uploadArchive
	runRemoteArchiveExists = remoteArchiveExists
	runWriteZip            = writeZip
	verifyArchiveUpload    = defaultVerifyArchiveUpload
)

type Options struct {
	Destination      models.BackupDestination
	Password         string
	Job              models.BackupJob
	GlobalExclusions []string
	Hostname         string
	ForceFull        bool
	OnProgress       func(models.ProgressEvent)
}

type localFile struct {
	absPath string
	rel     string
	catalog string
	size    int64
	mtime   time.Time
	aclHash string
	meta    winattr.Entry
}

func Run(ctx context.Context, opts Options) (*Stats, error) {
	var stats Stats
	if len(opts.Job.Sources) == 0 {
		return nil, i18n.E("filebackup.no_sources", nil)
	}

	backupRoot, cleanup, err := backuproot.Resolve(opts.Job.Sources)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	hostname := strings.TrimSpace(opts.Hostname)
	if hostname == "" {
		hostname = "windows-host"
	}
	backupID := strings.TrimSpace(opts.Job.BackupID)
	if backupID == "" {
		backupID = hostname
	}

	store, err := fileindex.Load(opts.Job.ID)
	if err != nil {
		return nil, err
	}
	if opts.ForceFull {
		_ = fileindex.Clear(opts.Job.ID)
		store = &fileindex.Store{
			JobID: opts.Job.ID,
			Files: map[string]fileindex.FileRecord{},
			Meta:  map[string]winattr.Entry{},
		}
	}
	if store.Meta == nil {
		store.Meta = map[string]winattr.Entry{}
	}

	exc := exclude.NewForRoot(backupRoot, exclude.Merge(opts.GlobalExclusions, opts.Job.Exclusions))
	startedAt := time.Now().UTC().Format(time.RFC3339)
	pe := newProgressEmitter(opts, startedAt)
	scanProgress := func(msg string) {
		pe.emit(models.PhasePreparing, 2, msg, stats.BytesTransferred.Load(), stats.BytesSkipped.Load(), 0, int(stats.FilesTotal.Load()))
	}
	scanProgress(i18n.L("filebackup.scan_acl", nil))
	files, dirMeta, walkSkipped, unverifiedDirs, err := scanFiles(ctx, backupRoot, exc, opts.Job.SkipAccessErrors, &stats, func(scanned int, path string) {
		if scanned%200 == 0 {
			scanProgress(i18n.L("filebackup.scanning", map[string]string{"n": fmt.Sprintf("%d", scanned)}))
		}
		_ = path
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, i18n.E("filebackup.no_files_archive", nil)
	}

	for _, f := range files {
		stats.FilesTotal.Add(1)
		prev, has := store.Files[f.catalog]
		if fileindex.NeedsContentBackup(prev, f.size, f.mtime, has) {
			continue
		}
		stats.BytesSkipped.Add(f.size)
	}

	fullRequired := opts.ForceFull || store.BaseFull == "" || !runRemoteArchiveExists(opts.Destination, opts.Password, backupID, store.BaseFull)
	kind := fileindex.KindIncremental
	if fullRequired {
		kind = fileindex.KindFull
	}

	var toArchive []localFile
	var deleted []string
	metaChanged := false
	if kind == fileindex.KindFull {
		toArchive = files
		metaChanged = true
	} else {
		current := make(map[string]bool, len(files))
		for _, f := range files {
			current[f.catalog] = true
			prev, has := store.Files[f.catalog]
			content := fileindex.NeedsContentBackup(prev, f.size, f.mtime, has)
			acl := fileindex.NeedsACLBackup(prev, f.aclHash, has)
			if acl {
				metaChanged = true
			}
			if content {
				toArchive = append(toArchive, f)
			} else if acl {
				// ACL-only change: meta updated, content reused from chain.
			}
		}
		deleted = computeDeleted(store, current, unverifiedDirs)
		if len(deleted) > 0 {
			metaChanged = true
		}
		if len(toArchive) == 0 && len(deleted) == 0 && !metaChanged {
			stats.BackupKind = fileindex.KindIncremental
			last := lastArchiveName(store)
			if last == "" || !remoteArchiveExists(opts.Destination, opts.Password, backupID, last) {
				return nil, i18n.E("filebackup.remote_archive_missing", map[string]string{"name": last})
			}
			stats.RemotePath = last
			return &stats, nil
		}
	}

	stamp := time.Now().UTC()
	zipName := fmt.Sprintf("%s_%s.zip", hostname, stamp.Format("20060102-150405"))
	if kind == fileindex.KindIncremental {
		zipName = fmt.Sprintf("%s_%s_incr.zip", hostname, stamp.Format("20060102-150405"))
	}
	stats.BackupKind = kind
	pe.setKind(kind)
	stats.RemotePath = zipName
	stats.FilesChanged.Store(int64(len(toArchive)))

	archiveTotal := len(toArchive)
	if archiveTotal == 0 {
		archiveTotal = 1
	}

	pe.emit(models.PhasePreparing, 4, i18n.L("filebackup.preparing_archive", nil), stats.BytesTransferred.Load(), stats.BytesSkipped.Load(), 0, int(stats.FilesTotal.Load()))

	tmpFile, err := os.CreateTemp("", "clickrax-backup-*.zip")
	if err != nil {
		return nil, err
	}
	tmpZip := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpZip)

	intentionallySkipped := make(map[string]struct{})
	if err := runWriteZip(ctx, toArchive, tmpZip, opts.Job.SkipAccessErrors, &stats, intentionallySkipped, throttleEmit(func(rel string, done int) {
		pct := archivePercent(done, archiveTotal)
		pe.emit(models.PhaseAnalyzing, pct, i18n.L("filebackup.archiving", map[string]string{"path": rel}),
			stats.BytesTransferred.Load(), stats.BytesSkipped.Load(), done, archiveTotal)
	})); err != nil {
		return nil, err
	}

	archivedInZip, err := listZipEntries(tmpZip)
	if err != nil {
		return nil, err
	}
	metaOnly := len(toArchive) == 0 && kind == fileindex.KindIncremental
	if len(archivedInZip) == 0 {
		if kind == fileindex.KindFull {
			if stats.FilesSkipped.Load() > 0 {
				return nil, i18n.E("filebackup.no_files_skipped", map[string]string{"count": fmt.Sprintf("%d", stats.FilesSkipped.Load())})
			}
			return nil, i18n.E("filebackup.no_files", nil)
		}
		if !metaOnly {
			return nil, i18n.E("filebackup.no_files", nil)
		}
		pe.emit(models.PhaseAnalyzing, progArchiveEnd, i18n.L("filebackup.no_changes", nil),
			stats.BytesTransferred.Load(), stats.BytesSkipped.Load(), 0, archiveTotal)
	}
	if err := checkSignificantSkips(&stats); err != nil {
		return nil, err
	}
	if err := checkPlannedFilesArchived(toArchive, archivedInZip, intentionallySkipped); err != nil {
		return nil, err
	}
	archived := make([]localFile, 0, len(archivedInZip))
	if len(archivedInZip) > 0 {
		byCatalog := make(map[string]localFile, len(toArchive))
		for _, f := range toArchive {
			byCatalog[f.catalog] = f
		}
		for _, z := range archivedInZip {
			if orig, ok := byCatalog[z.catalog]; ok {
				archived = append(archived, orig)
			} else {
				archived = append(archived, z)
			}
		}
	}
	if kind == fileindex.KindFull {
		toArchive = archived
	}

	zi, err := os.Stat(tmpZip)
	if err != nil {
		return nil, err
	}
	total := zi.Size()
	pe.setBytesTotal(total)

	pe.emit(models.PhaseTransfer, progArchiveEnd, i18n.L("filebackup.upload_zip", map[string]string{
		"path": zipName,
		"vol":  fmt.Sprintf("%.1f", float64(total)/(1024*1024)),
	}), 0, stats.BytesSkipped.Load(), archiveTotal, archiveTotal)

	onXfer := func(written, tot int64) {
		stats.BytesTransferred.Store(written)
		pct := transferPercent(written, tot)
		pe.emit(models.PhaseTransfer, pct, i18n.L("filebackup.transfer_progress", map[string]string{
			"n":   fmt.Sprintf("%.1f", float64(written)/(1024*1024)),
			"max": fmt.Sprintf("%.1f", float64(tot)/(1024*1024)),
		}), written, stats.BytesSkipped.Load(), archiveTotal, archiveTotal)
	}

	if err := runUploadArchive(ctx, opts.Destination, opts.Password, backupID, tmpZip, zipName, total, onXfer); err != nil {
		return nil, err
	}
	stats.BytesTransferred.Store(total)

	manifest := buildManifest(kind, zipName, stamp, store, deleted)
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = removeRemoteArchive(opts.Destination, opts.Password, backupID, zipName)
		return nil, err
	}
	manifestName := fileindex.ManifestName(zipName)
	if err := uploadManifest(ctx, opts.Destination, opts.Password, backupID, manifestName, manifestBytes); err != nil {
		_ = removeRemoteArchive(opts.Destination, opts.Password, backupID, zipName)
		_ = removeRemoteArchive(opts.Destination, opts.Password, backupID, manifestName)
		return nil, err
	}

	snapshotMeta := buildSnapshotMeta(store, kind, files, archived, dirMeta, deleted)
	metaBytes, err := filemeta.Marshal(snapshotMeta)
	if err != nil {
		rollbackRemoteBackup(opts.Destination, opts.Password, backupID, zipName, manifestName)
		return nil, err
	}
	metaName := filemeta.MetaFileName(zipName)
	pe.emit(models.PhaseTransfer, progTransferEnd+2, i18n.L("filebackup.upload_meta", nil),
		stats.BytesTransferred.Load(), stats.BytesSkipped.Load(), archiveTotal, archiveTotal)
	if err := uploadManifest(ctx, opts.Destination, opts.Password, backupID, metaName, metaBytes); err != nil {
		rollbackRemoteBackup(opts.Destination, opts.Password, backupID, zipName, manifestName)
		_ = removeRemoteArchive(opts.Destination, opts.Password, backupID, metaName)
		return nil, err
	}

	if err := checkSignificantSkips(&stats); err != nil {
		rollbackRemoteBackup(opts.Destination, opts.Password, backupID, zipName, manifestName)
		_ = removeRemoteArchive(opts.Destination, opts.Password, backupID, metaName)
		return nil, err
	}
	if err := updateStore(store, kind, zipName, stamp, files, archived, dirMeta, deleted); err != nil {
		rollbackRemoteBackup(opts.Destination, opts.Password, backupID, zipName, manifestName)
		_ = removeRemoteArchive(opts.Destination, opts.Password, backupID, metaName)
		return nil, i18n.E("filebackup.index_not_updated", map[string]string{"err": err.Error()})
	}

	pe.emit(models.PhaseFinalizing, progManifestEnd, i18n.L("filebackup.finalizing", nil),
		stats.BytesTransferred.Load(), stats.BytesSkipped.Load(), archiveTotal, archiveTotal)
	_ = walkSkipped
	return &stats, nil
}

func buildSnapshotMeta(store *fileindex.Store, kind string, allFiles, archived []localFile, dirMeta map[string]winattr.Entry, deleted []string) filemeta.Archive {
	out := filemeta.NewArchive()
	if kind == fileindex.KindFull {
		for _, f := range archived {
			out.Set(f.catalog, f.meta)
		}
		for path, e := range dirMeta {
			out.Set(path, e)
		}
		return out
	}
	for path, e := range store.Meta {
		out.Files[path] = e
	}
	for _, p := range deleted {
		out.Delete(p)
	}
	for _, f := range allFiles {
		prev, has := store.Files[f.catalog]
		if fileindex.NeedsContentBackup(prev, f.size, f.mtime, has) || fileindex.NeedsACLBackup(prev, f.aclHash, has) {
			out.Set(f.catalog, f.meta)
		}
	}
	for path, e := range dirMeta {
		out.Set(path, e)
	}
	return out
}

func lastArchiveName(store *fileindex.Store) string {
	if store == nil {
		return ""
	}
	if len(store.Archives) > 0 {
		return store.Archives[len(store.Archives)-1].Name
	}
	return store.BaseFull
}

func buildManifest(kind, zipName string, stamp time.Time, store *fileindex.Store, deleted []string) fileindex.Manifest {
	if kind == fileindex.KindFull {
		return fileindex.NewFullManifest(zipName, stamp)
	}
	chain := []string{store.BaseFull}
	for _, a := range store.Archives {
		if a.Kind == fileindex.KindIncremental {
			chain = append(chain, a.Name)
		}
	}
	chain = append(chain, zipName)
	return fileindex.NewIncrementalManifest(zipName, stamp, store.BaseFull, chain, deleted)
}

func updateStore(store *fileindex.Store, kind, zipName string, stamp time.Time, allFiles, archived []localFile, dirMeta map[string]winattr.Entry, deleted []string) error {
	if store.Meta == nil {
		store.Meta = map[string]winattr.Entry{}
	}
	if kind == fileindex.KindFull {
		store.BaseFull = zipName
		store.Archives = []fileindex.ArchiveRecord{{
			Name: zipName,
			Kind: fileindex.KindFull,
			Time: stamp.UTC(),
		}}
		store.Files = map[string]fileindex.FileRecord{}
		store.Meta = map[string]winattr.Entry{}
		for _, f := range archived {
			store.Files[f.catalog] = fileindex.FileRecord{
				Size:    f.size,
				Mtime:   f.mtime.UnixNano(),
				ACLHash: f.aclHash,
				Archive: zipName,
			}
			store.Meta[f.catalog] = f.meta
		}
		for path, e := range dirMeta {
			store.Meta[path] = e
		}
		return fileindex.Save(store)
	}

	for _, p := range deleted {
		delete(store.Files, p)
		delete(store.Meta, p)
	}
	archivedSet := make(map[string]bool, len(archived))
	for _, f := range archived {
		archivedSet[f.catalog] = true
		store.Files[f.catalog] = fileindex.FileRecord{
			Size:    f.size,
			Mtime:   f.mtime.UnixNano(),
			ACLHash: f.aclHash,
			Archive: zipName,
		}
		store.Meta[f.catalog] = f.meta
	}
	for _, f := range allFiles {
		if archivedSet[f.catalog] {
			continue
		}
		prev, has := store.Files[f.catalog]
		if has && fileindex.NeedsACLBackup(prev, f.aclHash, true) {
			store.Meta[f.catalog] = f.meta
			rec := store.Files[f.catalog]
			rec.ACLHash = f.aclHash
			store.Files[f.catalog] = rec
		}
	}
	for path, e := range dirMeta {
		store.Meta[path] = e
	}
	store.Archives = append(store.Archives, fileindex.ArchiveRecord{
		Name:    zipName,
		Kind:    fileindex.KindIncremental,
		Time:    stamp.UTC(),
		Deleted: append([]string(nil), deleted...),
	})
	return fileindex.Save(store)
}

func significantSkips(skipped, total int64) bool {
	if skipped <= 0 || total <= 0 {
		return false
	}
	return skipped*significantSkipRatioNum > total
}

func checkPlannedFilesArchived(planned []localFile, archivedInZip []localFile, intentionallySkipped map[string]struct{}) error {
	if len(planned) == 0 {
		return nil
	}
	inZip := make(map[string]struct{}, len(archivedInZip))
	for _, f := range archivedInZip {
		inZip[f.catalog] = struct{}{}
	}
	var missing []string
	for _, f := range planned {
		if intentionallySkipped != nil {
			if _, ok := intentionallySkipped[f.catalog]; ok {
				continue
			}
		}
		if _, ok := inZip[f.catalog]; !ok {
			missing = append(missing, f.catalog)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	total := len(missing)
	if len(missing) > 5 {
		missing = missing[:5]
	}
	return i18n.Ef("filebackup.archive_incomplete", map[string]string{
		"missing": strings.Join(missing, "; "),
		"count":   fmt.Sprintf("%d", total),
	})
}

func checkSignificantSkips(stats *Stats) error {
	skipped := stats.FilesSkipped.Load()
	total := stats.FilesTotal.Load()
	if !significantSkips(skipped, total) {
		return nil
	}
	return i18n.E("filebackup.too_many_skipped", map[string]string{
		"skipped": fmt.Sprintf("%d", skipped),
		"total":   fmt.Sprintf("%d", total),
	})
}

func underUnverifiedDir(catalog string, unverified []string) bool {
	if len(unverified) == 0 {
		return false
	}
	cat := strings.ToLower(catalog)
	for _, prefix := range unverified {
		p := strings.ToLower(prefix)
		if cat == p || strings.HasPrefix(cat, p+`\`) {
			return true
		}
	}
	return false
}

func computeDeleted(store *fileindex.Store, current map[string]bool, unverified []string) []string {
	if store == nil || len(store.Files) == 0 {
		return nil
	}
	var deleted []string
	for path := range store.Files {
		if !current[path] && !underUnverifiedDir(path, unverified) {
			deleted = append(deleted, path)
		}
	}
	return deleted
}

func scanFiles(ctx context.Context, root string, exc *exclude.Engine, skipAccess bool, stats *Stats, onProgress func(scanned int, path string)) ([]localFile, map[string]winattr.Entry, int, []string, error) {
	out := make([]localFile, 0, 256)
	dirMeta := map[string]winattr.Entry{}
	unverifiedDirs := make([]string, 0)
	skipped := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if skipAccess {
				skipped++
				if stats != nil {
					stats.FilesSkipped.Add(1)
				}
				if d != nil && d.IsDir() && path != root {
					if rel, relErr := filepath.Rel(root, path); relErr == nil {
						unverifiedDirs = append(unverifiedDirs, catalogPath(filepath.ToSlash(rel)))
					}
				}
				return filepath.SkipDir
			}
			return err
		}
		name := d.Name()
		if exc != nil && exc.MatchPath(path, name, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = name
		}
		rel = filepath.ToSlash(rel)
		catalog := catalogPath(rel)

		if d.IsDir() {
			if path == root {
				return nil
			}
			meta, capErr := filemeta.CaptureFile(path)
			if capErr != nil && !skipAccess {
				return capErr
			}
			if meta.HasMeta() {
				dirMeta[catalog] = meta
			}
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			if skipAccess {
				skipped++
				if stats != nil {
					stats.FilesSkipped.Add(1)
				}
				if rel, relErr := filepath.Rel(root, path); relErr == nil {
					unverifiedDirs = append(unverifiedDirs, catalogPath(filepath.ToSlash(rel)))
				}
				return nil
			}
			return infoErr
		}
		meta, capErr := filemeta.CaptureFile(path)
		if capErr != nil && !skipAccess {
			return capErr
		}
		aclHash := winattr.ACLHash(meta)
		out = append(out, localFile{
			absPath: path,
			rel:     rel,
			catalog: catalog,
			size:    info.Size(),
			mtime:   info.ModTime(),
			aclHash: aclHash,
			meta:    meta,
		})
		if onProgress != nil {
			onProgress(len(out), rel)
		}
		return nil
	})
	return out, dirMeta, skipped, unverifiedDirs, err
}

func throttleEmit(fn func(rel string, done int)) func(rel string) {
	var last time.Time
	var count int
	return func(rel string) {
		count++
		now := time.Now()
		if count == 1 || count%100 == 0 || now.Sub(last) >= 400*time.Millisecond {
			last = now
			fn(rel, count)
		}
	}
}

func writeZip(ctx context.Context, files []localFile, destZip string, skipAccess bool, stats *Stats, intentionallySkipped map[string]struct{}, onFile func(rel string)) error {
	f, err := os.Create(destZip)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)

	for _, item := range files {
		if ctx.Err() != nil {
			_ = zw.Close()
			_ = f.Close()
			_ = os.Remove(destZip)
			return ctx.Err()
		}
		info, err := os.Stat(item.absPath)
		if err != nil {
			if skipAccess {
				if stats != nil {
					stats.FilesSkipped.Add(1)
				}
				if intentionallySkipped != nil {
					intentionallySkipped[item.catalog] = struct{}{}
				}
				continue
			}
			_ = zw.Close()
			_ = f.Close()
			_ = os.Remove(destZip)
			return err
		}
		src, err := os.Open(item.absPath)
		if err != nil {
			if skipAccess {
				if stats != nil {
					stats.FilesSkipped.Add(1)
				}
				if intentionallySkipped != nil {
					intentionallySkipped[item.catalog] = struct{}{}
				}
				continue
			}
			_ = zw.Close()
			_ = f.Close()
			_ = os.Remove(destZip)
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			_ = src.Close()
			_ = zw.Close()
			_ = f.Close()
			_ = os.Remove(destZip)
			return err
		}
		hdr.Name = item.rel
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			_ = src.Close()
			_ = zw.Close()
			_ = f.Close()
			_ = os.Remove(destZip)
			return err
		}
		_, copyErr := io.Copy(w, src)
		_ = src.Close()
		if copyErr != nil {
			_ = zw.Close()
			_ = f.Close()
			_ = os.Remove(destZip)
			if skipAccess {
				return i18n.Ewrap("filebackup.read", map[string]string{"path": item.absPath}, copyErr)
			}
			return copyErr
		}
		if onFile != nil {
			onFile(item.rel)
		}
	}

	if err := zw.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(destZip)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(destZip)
		return err
	}
	return f.Close()
}

func listZipEntries(zipPath string) ([]localFile, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, i18n.Ewrap("filebackup.zip_read", nil, err)
	}
	defer zr.Close()
	out := make([]localFile, 0, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		out = append(out, localFile{
			rel:     f.Name,
			catalog: catalogPath(f.Name),
			size:    int64(f.UncompressedSize64),
			mtime:   f.Modified,
		})
	}
	return out, nil
}

func rollbackRemoteBackup(dest models.BackupDestination, password, backupID, zipName, manifestName string) {
	_ = removeRemoteArchive(dest, password, backupID, zipName)
	_ = removeRemoteArchive(dest, password, backupID, manifestName)
}

func removeRemoteArchive(dest models.BackupDestination, password, backupID, fileName string) error {
	switch dest.NormalizedType() {
	case models.DestSMB:
		return smbclient.RemoveRemote(dest, password, backupID, fileName)
	case models.DestFTP:
		return ftpclient.RemoveRemote(dest, password, backupID, fileName)
	default:
		return nil
	}
}

func catalogPath(rel string) string {
	return strings.ReplaceAll(rel, "/", `\`)
}

func remoteArchiveExists(dest models.BackupDestination, password, backupID, fileName string) bool {
	switch dest.NormalizedType() {
	case models.DestSMB:
		return smbclient.RemoteFileExists(dest, password, backupID, fileName)
	case models.DestFTP:
		return ftpclient.RemoteFileExists(dest, password, backupID, fileName)
	default:
		return false
	}
}

func localFileSHA256(path string) ([32]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return [32]byte{}, err
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

func defaultVerifyArchiveUpload(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string, total int64, sha256Sum [32]byte) error {
	switch dest.NormalizedType() {
	case models.DestSMB:
		return smbclient.VerifyUploaded(ctx, dest, password, backupID, fileName, total, sha256Sum)
	case models.DestFTP:
		return ftpclient.VerifyUploaded(ctx, dest, password, backupID, fileName, total, sha256Sum)
	default:
		return i18n.E("filebackup.dest_unsupported", map[string]string{"type": dest.Type})
	}
}

func uploadArchive(ctx context.Context, dest models.BackupDestination, password, backupID, localPath, fileName string, total int64, onProgress func(written, total int64)) error {
	localHash, err := localFileSHA256(localPath)
	if err != nil {
		return err
	}
	switch dest.NormalizedType() {
	case models.DestSMB:
		if err := smbclient.Upload(ctx, dest, password, localPath, backupID, fileName, onProgress); err != nil {
			return err
		}
	case models.DestFTP:
		if err := ftpclient.Upload(ctx, dest, password, localPath, backupID, fileName, onProgress); err != nil {
			return err
		}
	default:
		return i18n.E("filebackup.dest_unsupported", map[string]string{"type": dest.Type})
	}
	if err := verifyArchiveUpload(ctx, dest, password, backupID, fileName, total, localHash); err != nil {
		_ = removeRemoteArchive(dest, password, backupID, fileName)
		return err
	}
	return nil
}

func uploadManifest(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string, data []byte) error {
	tmp := filepath.Join(os.TempDir(), filepath.Base(fileName))
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	defer os.Remove(tmp)
	info, err := os.Stat(tmp)
	if err != nil {
		return err
	}
	return runUploadArchive(ctx, dest, password, backupID, tmp, fileName, info.Size(), nil)
}
