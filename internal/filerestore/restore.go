package filerestore

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/netio"
	"pbs-win-backup/internal/pbsbackup"
	"pbs-win-backup/internal/restorepolicy"
)

type FolderProgress func(done, total int, currentPath string)

func restoreCtxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func RestoreFile(ctx context.Context, dest models.BackupDestination, password string, job models.BackupJob, snapshotTime, filePath, destPath, overwriteMode string, forceOverwrite bool) error {
	if err := restoreCtxErr(ctx); err != nil {
		return err
	}
	view, err := buildSnapshotView(ctx, dest, password, job, snapshotTime)
	if err != nil {
		return err
	}
	key := strings.ToLower(filePath)
	archiveName, ok := view.fileSource[key]
	if !ok {
		return i18n.E("restore.file_not_in_snapshot", map[string]string{"path": filePath})
	}
	z, err := openRemoteZip(ctx, dest, password, job.BackupID, archiveName)
	if err != nil {
		return err
	}
	defer z.Close()

	zr, err := zipReader(z)
	if err != nil {
		return err
	}
	entry, err := findZipEntry(zr, filePath)
	if err != nil {
		return err
	}
	if err := restorepolicy.PrepareExistingDest(destPath, overwriteMode, forceOverwrite); err != nil {
		return err
	}
	rc, err := entry.Open()
	if err != nil {
		return i18n.Ef("restore.archive_open", map[string]string{"path": filePath, "err": err.Error()})
	}
	defer rc.Close()
	if err := writeRestoredFromReader(ctx, destPath, rc); err != nil {
		return err
	}
	modified := ""
	if f, ok := view.files[strings.ToLower(filePath)]; ok {
		modified = f.Modified
	}
	return filemeta.ApplyFile(destPath, filemeta.PrepareEntry(view.meta, filePath, modified))
}

func RestoreFolder(
	ctx context.Context,
	dest models.BackupDestination,
	password string,
	job models.BackupJob,
	snapshotTime, folderPath, destRoot, overwriteMode string,
	forceOverwrite bool,
	onProgress FolderProgress,
) (int, error) {
	if err := restoreCtxErr(ctx); err != nil {
		return 0, err
	}
	view, err := buildSnapshotView(ctx, dest, password, job, snapshotTime)
	if err != nil {
		return 0, err
	}
	files := view.catalog()
	targets := filesUnderPrefix(files, folderPath)
	if len(targets) == 0 {
		return 0, i18n.E("restore.folder_empty", map[string]string{"path": folderPath})
	}
	return restoreTargets(ctx, view, dest, password, job.BackupID, targets, folderPath, destRoot, overwriteMode, forceOverwrite, onProgress)
}

func RestoreBatch(
	ctx context.Context,
	dest models.BackupDestination,
	password string,
	job models.BackupJob,
	snapshotTime string,
	paths []string,
	sources []string,
	destRoot string,
	toOriginal bool,
	overwriteMode string,
	forceOverwrite bool,
	onProgress FolderProgress,
) (int, error) {
	if err := restoreCtxErr(ctx); err != nil {
		return 0, err
	}
	if len(paths) == 0 {
		return 0, i18n.E("restore.no_selection", nil)
	}
	view, err := buildSnapshotView(ctx, dest, password, job, snapshotTime)
	if err != nil {
		return 0, err
	}
	files := view.catalog()
	targets, err := collectBatchTargets(files, paths, sources, destRoot, toOriginal)
	if err != nil {
		return 0, err
	}
	if len(targets) == 0 {
		return 0, i18n.E("restore.no_files", nil)
	}
	return restoreTargetsWithPreset(ctx, view, dest, password, job.BackupID, targets, overwriteMode, forceOverwrite, onProgress)
}

type batchTarget struct {
	filePath  string
	dest      string
	folderSel string
}

func restoreTargets(ctx context.Context, view *snapshotView, dest models.BackupDestination, password, backupID string, files []models.SnapshotFile, folderPrefix, destRoot, overwriteMode string, forceOverwrite bool, onProgress FolderProgress) (int, error) {
	prefix := normalizeFolderPrefix(folderPrefix)
	targets := make([]batchTarget, 0, len(files))
	for _, f := range files {
		destPath, err := resolveFolderDest(f.Path, prefix, destRoot, folderPrefix)
		if err != nil {
			return 0, i18n.Ewrap("restore.path_resolve", map[string]string{"path": f.Path}, err)
		}
		targets = append(targets, batchTarget{filePath: f.Path, dest: destPath})
	}
	return restoreTargetsWithPreset(ctx, view, dest, password, backupID, targets, overwriteMode, forceOverwrite, onProgress)
}

func restoreTargetsWithPreset(ctx context.Context, view *snapshotView, dest models.BackupDestination, password, backupID string, targets []batchTarget, overwriteMode string, forceOverwrite bool, onProgress FolderProgress) (count int, err error) {
	cache := map[string]*remoteZip{}
	closeCache := func() {
		for _, z := range cache {
			z.Close()
		}
	}
	defer closeCache()

	zipReaders := map[string]*zip.Reader{}
	restored := []string{}
	defer func() {
		if err == nil || len(restored) == 0 {
			return
		}
		for i := len(restored) - 1; i >= 0; i-- {
			_ = os.Remove(restored[i])
		}
	}()

	total := len(targets)
	count = 0
	for i, t := range targets {
		if err := restoreCtxErr(ctx); err != nil {
			return count, err
		}
		if onProgress != nil {
			onProgress(i, total, t.filePath)
		}
		key := strings.ToLower(t.filePath)
		archiveName, ok := view.fileSource[key]
		if !ok {
			return count, i18n.E("restore.file_not_in_snapshot", map[string]string{"path": t.filePath})
		}
		z, ok := cache[archiveName]
		if !ok {
			var err error
			z, err = openRemoteZip(ctx, dest, password, backupID, archiveName)
			if err != nil {
				return count, err
			}
			cache[archiveName] = z
		}
		zr, ok := zipReaders[archiveName]
		if !ok {
			var err error
			zr, err = zipReader(z)
			if err != nil {
				return count, err
			}
			zipReaders[archiveName] = zr
		}
		entry, err := findZipEntry(zr, t.filePath)
		if err != nil {
			return count, err
		}
		_, statErr := os.Stat(t.dest)
		if err := restorepolicy.PrepareExistingDest(t.dest, overwriteMode, forceOverwrite); err != nil {
			return count, fmt.Errorf("%s: %w", t.filePath, err)
		}
		rc, err := entry.Open()
		if err != nil {
			return count, fmt.Errorf("%s: %w", t.filePath, err)
		}
		err = writeRestoredFromReader(ctx, t.dest, rc)
		_ = rc.Close()
		if err != nil {
			return count, fmt.Errorf("%s: %w", t.filePath, err)
		}
		if os.IsNotExist(statErr) {
			restored = append(restored, t.dest)
		}
		modified := ""
		if f, ok := view.files[key]; ok {
			modified = f.Modified
		}
		if err := filemeta.ApplyFile(t.dest, filemeta.PrepareEntry(view.meta, t.filePath, modified)); err != nil {
			return count, i18n.Ef("restore.metadata_failed", map[string]string{"path": t.filePath, "err": err.Error()})
		}
		count++
		if onProgress != nil {
			onProgress(i+1, total, t.filePath)
		}
	}
	return count, nil
}

func writeRestoredFromReader(ctx context.Context, destPath string, r io.Reader) error {
	return pbsbackup.WriteRestoredFileFromReader(destPath, netio.Reader(ctx, r))
}

func filesUnderPrefix(files []models.SnapshotFile, folderPath string) []models.SnapshotFile {
	prefix := normalizeFolderPrefix(folderPath)
	if prefix == "" {
		return nil
	}
	out := make([]models.SnapshotFile, 0)
	for _, f := range files {
		if f.IsDir {
			continue
		}
		if pathUnderPrefix(f.Path, prefix) {
			out = append(out, f)
		}
	}
	return out
}

func collectBatchTargets(catalog []models.SnapshotFile, paths []string, sources []string, destRoot string, toOriginal bool) ([]batchTarget, error) {
	normPaths := make([]string, 0, len(paths))
	seenPath := make(map[string]bool)
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" || seenPath[strings.ToLower(p)] {
			continue
		}
		seenPath[strings.ToLower(p)] = true
		normPaths = append(normPaths, p)
	}
	if len(normPaths) == 0 {
		return nil, i18n.E("restore.no_selection", nil)
	}

	sort.Slice(normPaths, func(i, j int) bool {
		return len(normalizeFolderPrefix(normPaths[i])) < len(normalizeFolderPrefix(normPaths[j]))
	})

	filesByPath := make(map[string]models.SnapshotFile)
	for _, f := range catalog {
		if !f.IsDir {
			filesByPath[strings.ToLower(f.Path)] = f
		}
	}

	var targets []batchTarget
	seenTarget := make(map[string]bool)
	for _, sel := range normPaths {
		if isFolderSelection(sel, catalog) {
			prefix := normalizeFolderPrefix(sel)
			for _, f := range catalog {
				if f.IsDir || !pathUnderPrefix(f.Path, prefix) {
					continue
				}
				if coveredByShorterPrefix(f.Path, sel, normPaths) {
					continue
				}
				destPath, err := resolveBatchDest(f.Path, sel, sources, destRoot, toOriginal)
				if err != nil {
					return nil, i18n.Ewrap("restore.path_resolve", map[string]string{"path": f.Path}, err)
				}
				key := strings.ToLower(f.Path)
				if seenTarget[key] {
					continue
				}
				seenTarget[key] = true
				targets = append(targets, batchTarget{filePath: f.Path, dest: destPath, folderSel: sel})
			}
			continue
		}
		f, ok := filesByPath[strings.ToLower(sel)]
		if !ok {
			return nil, i18n.E("restore.file_not_in_archive", map[string]string{"path": sel})
		}
		if coveredByShorterPrefix(f.Path, sel, normPaths) {
			continue
		}
		destPath, err := resolveBatchDest(f.Path, "", sources, destRoot, toOriginal)
		if err != nil {
			return nil, i18n.Ewrap("restore.path_resolve", map[string]string{"path": f.Path}, err)
		}
		key := strings.ToLower(f.Path)
		if seenTarget[key] {
			continue
		}
		seenTarget[key] = true
		targets = append(targets, batchTarget{filePath: f.Path, dest: destPath})
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].filePath < targets[j].filePath })
	return targets, nil
}

func isFolderSelection(path string, catalog []models.SnapshotFile) bool {
	prefix := normalizeFolderPrefix(path)
	for _, f := range catalog {
		if !f.IsDir && pathUnderPrefix(f.Path, prefix) && !strings.EqualFold(f.Path, path) {
			return true
		}
	}
	return false
}

func coveredByShorterPrefix(filePath, currentSel string, all []string) bool {
	curPref := normalizeFolderPrefix(currentSel)
	for _, other := range all {
		if strings.EqualFold(other, currentSel) {
			continue
		}
		otherPref := normalizeFolderPrefix(other)
		if otherPref == curPref || len(otherPref) >= len(curPref) {
			continue
		}
		if pathUnderPrefix(filePath, otherPref) {
			return true
		}
	}
	return false
}

func resolveBatchDest(filePath, folderSel string, sources []string, destRoot string, toOriginal bool) (string, error) {
	if toOriginal {
		if folderSel != "" {
			folderDest, err := pbsbackup.ResolveOriginalDest(folderSel, sources)
			if err != nil {
				return "", err
			}
			return resolveFolderDest(filePath, normalizeFolderPrefix(folderSel), folderDest, folderSel)
		}
		return pbsbackup.ResolveOriginalDest(filePath, sources)
	}
	if destRoot == "" {
		return "", i18n.E("restore.dest_not_set", nil)
	}
	if folderSel != "" {
		return resolveFolderDest(filePath, normalizeFolderPrefix(folderSel), destRoot, folderSel)
	}
	return pbsbackup.ResolveUnderDest(filePath, destRoot)
}

func resolveFileDest(filePath, destRoot string) (string, error) {
	return pbsbackup.ResolveUnderDest(filePath, destRoot)
}

func normalizeFolderPrefix(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimSuffix(p, `\`)
	p = strings.TrimSuffix(p, `/`)
	return strings.ToLower(strings.ReplaceAll(p, `/`, `\`))
}

func pathUnderPrefix(filePath, prefix string) bool {
	norm := strings.ToLower(strings.ReplaceAll(filePath, `/`, `\`))
	if norm == prefix {
		return true
	}
	return strings.HasPrefix(norm, prefix+`\`)
}

func resolveFolderDest(filePath, _ string, destRoot, originalFolder string) (string, error) {
	if destRoot == "" {
		return "", i18n.E("restore.dest_empty", nil)
	}
	normFile := strings.ReplaceAll(filePath, `/`, `\`)
	normPref := strings.TrimSuffix(strings.ReplaceAll(originalFolder, `/`, `\`), `\`)

	destBase := strings.ToLower(filepath.Base(destRoot))
	folderBase := strings.ToLower(filepath.Base(normPref))
	if folderBase != "" && destBase == folderBase {
		rel := normFile
		lowerFile := strings.ToLower(normFile)
		lowerPref := strings.ToLower(normPref)
		if strings.HasPrefix(lowerFile, lowerPref) {
			rel = normFile[len(normPref):]
			rel = strings.TrimPrefix(rel, `\`)
		}
		dest, err := pbsbackup.ResolveUnderDest(rel, destRoot)
		if err != nil {
			return "", err
		}
		return dest, nil
	}

	return pbsbackup.ResolveUnderDest(filePath, destRoot)
}
