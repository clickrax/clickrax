package pbsbackup

import (
	"context"
	"path/filepath"
	"strings"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/pathsafe"
)

type RestoreFolderProgress func(done, total int, currentPath string)

// RestoreFolder restores all files under folderPath from snapshot.
func RestoreFolder(
	ctx context.Context,
	server models.PBSServer,
	secret string,
	ref SnapshotRef,
	folderPath, destRoot string,
	overwriteMode string,
	forceOverwrite bool,
	onProgress RestoreFolderProgress,
) (int, error) {
	prefix := normalizeFolderPrefix(folderPath)
	if prefix == "" {
		return 0, i18n.E("restore.no_folder", nil)
	}

	var targets []pxarRestoreTarget
	err := ForEachCatalogEntry(server, secret, ref, func(f models.SnapshotFile) error {
		if f.IsDir {
			return nil
		}
		if pathUnderPrefix(f.Path, prefix) {
			dest, err := resolveFolderDest(f.Path, prefix, destRoot, folderPath)
			if err != nil {
				return i18n.Ewrap("restore.path_resolve", map[string]string{"path": f.Path}, err)
			}
			targets = append(targets, pxarRestoreTarget{
				FilePath: f.Path,
				Dest:     dest,
				Modified: f.Modified,
			})
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if len(targets) == 0 {
		return 0, i18n.E("restore.folder_empty", map[string]string{"path": folderPath})
	}

	meta, err := loadSnapshotMeta(server, secret, ref)
	if err != nil {
		return 0, err
	}

	return restorePXARTargetsStreaming(
		ctx, server, secret, ref, targets, meta, overwriteMode, forceOverwrite,
		func(_, _ int, msg string) {
			if onProgress != nil {
				onProgress(0, len(targets), msg)
			}
		},
		onProgress,
	)
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

	// When destRoot is the resolved original folder (bases match), place files inside it.
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
		return pathsafe.JoinUnderRoot(destRoot, rel)
	}

	// Custom destination: preserve catalog path (including selected folder name).
	return ResolveUnderDest(filePath, destRoot)
}
