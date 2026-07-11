package restore

import (
	"os"
	"path/filepath"
	"strings"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/pathsafe"
	"pbs-win-backup/internal/pbsbackup"
	"pbs-win-backup/internal/restorepolicy"
)

func resolveFileDest(filePath, destRoot string) (string, error) {
	if destRoot == "" {
		return "", i18n.E("restore.dest_empty", nil)
	}
	return pathsafe.JoinUnderRoot(destRoot, filePath)
}

func prepareDestPath(dest, mode string, forceOverwrite bool) (string, error) {
	if err := restorepolicy.PrepareExistingDest(dest, mode, forceOverwrite); err != nil {
		return "", err
	}
	return dest, nil
}

func ensureParentDir(dest string) error {
	dir := filepath.Dir(dest)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// resolveOriginalDest wraps pbsbackup path resolution for restore service.
func resolveOriginalDest(catalogPath string, sources []string) (string, error) {
	return pbsbackup.ResolveOriginalDest(catalogPath, sources)
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
		return pathsafe.JoinUnderRoot(destRoot, rel)
	}
	return resolveFileDest(filePath, destRoot)
}
