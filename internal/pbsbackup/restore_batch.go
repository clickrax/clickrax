package pbsbackup



import (
	"context"
	"sort"
	"strings"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)



type batchTarget struct {

	filePath string

	dest     string

	modified string

}



// RestoreBatch restores multiple files and/or folders from one snapshot (single pxar stream).

func RestoreBatch(

	ctx context.Context,

	server models.PBSServer,

	secret string,

	ref SnapshotRef,

	paths []string,

	sources []string,

	destRoot string,

	toOriginal bool,

	overwriteMode string,

	forceOverwrite bool,

	onProgress RestoreFolderProgress,

) (int, error) {

	if len(paths) == 0 {

		return 0, i18n.E("restore.no_selection", nil)

	}



	targets, err := collectBatchTargetsSnapshot(server, secret, ref, paths, sources, destRoot, toOriginal)

	if err != nil {

		return 0, err

	}

	if len(targets) == 0 {

		return 0, i18n.E("restore.no_files", nil)

	}



	pxarTargets := make([]pxarRestoreTarget, len(targets))

	for i, t := range targets {

		pxarTargets[i] = pxarRestoreTarget{

			FilePath: t.filePath,

			Dest:     t.dest,

			Modified: t.modified,

		}

	}



	meta, err := loadSnapshotMeta(server, secret, ref)

	if err != nil {

		return 0, err

	}



	return restorePXARTargetsStreaming(

		ctx, server, secret, ref, pxarTargets, meta, overwriteMode, forceOverwrite,

		func(_, _ int, msg string) {

			if onProgress != nil {

				onProgress(0, len(pxarTargets), msg)

			}

		},

		onProgress,

	)

}



func collectBatchTargetsSnapshot(server models.PBSServer, secret string, ref SnapshotRef, paths []string, sources []string, destRoot string, toOriginal bool) ([]batchTarget, error) {
	dirs := map[string]bool{}
	if err := ForEachCatalogEntry(server, secret, ref, func(f models.SnapshotFile) error {
		if f.IsDir {
			dirs[strings.ToLower(f.Path)] = true
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return collectBatchTargetsStreaming(server, secret, ref, paths, sources, destRoot, toOriginal, dirs)
}

func collectBatchTargetsStreaming(server models.PBSServer, secret string, ref SnapshotRef, paths []string, sources []string, destRoot string, toOriginal bool, dirs map[string]bool) ([]batchTarget, error) {
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

	fileSelections := make([]string, 0)
	folderSelections := make([]string, 0)
	for _, sel := range normPaths {
		if isFolderSelection(sel, dirs) {
			folderSelections = append(folderSelections, sel)
		} else {
			fileSelections = append(fileSelections, sel)
		}
	}

	var targets []batchTarget
	seenFile := make(map[string]bool)
	pendingFiles := make(map[string]string, len(fileSelections))
	for _, sel := range fileSelections {
		pendingFiles[strings.ToLower(sel)] = sel
	}

	err := ForEachCatalogEntry(server, secret, ref, func(f models.SnapshotFile) error {
		if f.IsDir {
			return nil
		}
		lower := strings.ToLower(f.Path)

		if sel, ok := pendingFiles[lower]; ok {
			_ = sel
			if !seenFile[lower] {
				seenFile[lower] = true
				delete(pendingFiles, lower)
				dest, err := resolveBatchDest(f.Path, "", sources, destRoot, toOriginal)
				if err != nil {
					return i18n.Ewrap("restore.path_resolve", map[string]string{"path": f.Path}, err)
				}
				targets = append(targets, batchTarget{filePath: f.Path, dest: dest, modified: f.Modified})
			}
		}

		for _, sel := range folderSelections {
			prefix := normalizeFolderPrefix(sel)
			if !pathUnderPrefix(f.Path, prefix) {
				continue
			}
			if coveredByShorterPrefix(f.Path, sel, normPaths) {
				continue
			}
			if seenFile[lower] {
				continue
			}
			seenFile[lower] = true
			dest, err := resolveBatchDest(f.Path, sel, sources, destRoot, toOriginal)
			if err != nil {
				return i18n.Ewrap("restore.path_resolve", map[string]string{"path": f.Path}, err)
			}
			targets = append(targets, batchTarget{filePath: f.Path, dest: dest, modified: f.Modified})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, sel := range pendingFiles {
		return nil, i18n.E("restore.file_not_in_catalog", map[string]string{"path": sel})
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].filePath < targets[j].filePath
	})
	return targets, nil
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

	seenFile := make(map[string]bool)

	for _, sel := range normPaths {

		prefix := normalizeFolderPrefix(sel)

		isFolder := isFolderSelectionCatalog(sel, catalog)



		if isFolder {

			for _, f := range catalog {

				if f.IsDir || !pathUnderPrefix(f.Path, prefix) {

					continue

				}

				if coveredByShorterPrefix(f.Path, sel, normPaths) {

					continue

				}

				key := strings.ToLower(f.Path)

				if seenFile[key] {

					continue

				}

				seenFile[key] = true

				dest, err := resolveBatchDest(f.Path, sel, sources, destRoot, toOriginal)
			if err != nil {
				return nil, i18n.Ewrap("restore.path_resolve", map[string]string{"path": f.Path}, err)
			}

				targets = append(targets, batchTarget{filePath: f.Path, dest: dest, modified: f.Modified})

			}

			continue

		}



		f, ok := filesByPath[strings.ToLower(sel)]

		if !ok {

			return nil, i18n.E("restore.file_not_in_catalog", map[string]string{"path": sel})

		}

		key := strings.ToLower(f.Path)

		if seenFile[key] {

			continue

		}

		seenFile[key] = true

		dest, err := resolveBatchDest(f.Path, "", sources, destRoot, toOriginal)
		if err != nil {
			return nil, i18n.Ewrap("restore.path_resolve", map[string]string{"path": f.Path}, err)
		}

		targets = append(targets, batchTarget{filePath: f.Path, dest: dest, modified: f.Modified})

	}



	sort.Slice(targets, func(i, j int) bool {

		return targets[i].filePath < targets[j].filePath

	})

	return targets, nil

}



func isFolderSelection(path string, dirs map[string]bool) bool {
	lower := strings.ToLower(strings.ReplaceAll(path, `/`, `\`))
	return dirs[lower]
}

func isFolderSelectionCatalog(path string, catalog []models.SnapshotFile) bool {

	lower := strings.ToLower(strings.ReplaceAll(path, `/`, `\`))

	for _, f := range catalog {

		if f.IsDir && strings.ToLower(f.Path) == lower {

			return true

		}

	}

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
			folderDest, err := ResolveOriginalDest(folderSel, sources)
			if err != nil {
				return "", err
			}
			return resolveFolderDest(filePath, normalizeFolderPrefix(folderSel), folderDest, folderSel)
		}
		return ResolveOriginalDest(filePath, sources)
	}
	if destRoot == "" {
		return "", i18n.E("restore.dest_not_set", nil)
	}
	if folderSel != "" {
		return resolveFolderDest(filePath, normalizeFolderPrefix(folderSel), destRoot, folderSel)
	}
	return ResolveUnderDest(filePath, destRoot)
}

func resolveFileDest(filePath, destRoot string) (string, error) {
	return ResolveUnderDest(filePath, destRoot)
}

