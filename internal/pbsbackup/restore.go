package pbsbackup

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
)

var catalogDrivePrefix = regexp.MustCompile(`(?i)^[a-z]:[/\\]`)

// ListCatalogFiles returns user files from snapshot catalog.
func ListCatalogFiles(server models.PBSServer, secret string, ref SnapshotRef) ([]models.SnapshotFile, error) {
	return ListCatalogDir(server, secret, ref, "")
}

// ListCatalogDir returns direct children of a directory in the snapshot catalog.
func ListCatalogDir(server models.PBSServer, secret string, ref SnapshotRef, dirPath string) ([]models.SnapshotFile, error) {
	view, err := OpenCatalogCache(server, secret, ref, nil)
	if err != nil {
		return nil, err
	}

	var files []models.SnapshotFile
	err = view.withView(func(catalog []byte) error {
		var innerErr error
		files, innerErr = listCatalogDirEntries(catalog, dirPath)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	meta, err := loadSnapshotMeta(server, secret, ref)
	if err != nil {
		return files, nil
	}
	return filemeta.EnrichSnapshotFiles(meta, files), nil
}

// SearchCatalogFiles filters catalog by query (substring, case-insensitive).
func SearchCatalogFiles(server models.PBSServer, secret string, ref SnapshotRef, query string) ([]models.SnapshotFile, error) {
	view, err := OpenCatalogCache(server, secret, ref, nil)
	if err != nil {
		return nil, err
	}

	var files []models.SnapshotFile
	err = view.withView(func(catalog []byte) error {
		var innerErr error
		files, innerErr = searchCatalogFiles(catalog, query, 500)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	meta, err := loadSnapshotMeta(server, secret, ref)
	if err != nil {
		return files, nil
	}
	return filemeta.EnrichSnapshotFiles(meta, files), nil
}

// ForEachCatalogEntry walks all catalog entries without loading the full list into memory.
func ForEachCatalogEntry(server models.PBSServer, secret string, ref SnapshotRef, fn func(models.SnapshotFile) error) error {
	view, err := OpenCatalogCache(server, secret, ref, nil)
	if err != nil {
		return err
	}
	return view.withView(func(catalog []byte) error {
		return forEachCatalogFile(catalog, fn)
	})
}

// LoadPXAR downloads and reassembles the pxar archive for a snapshot.
func LoadPXAR(server models.PBSServer, secret string, ref SnapshotRef) ([]byte, error) {
	return LoadPXARWithProgress(context.Background(), server, secret, ref, nil)
}

func catalogRelPath(filePath string) string {
	rel := filePath
	if m := catalogDrivePrefix.FindStringIndex(filePath); m != nil {
		rel = filePath[m[1]:]
	}
	return normalizeRestorePath(rel)
}

func extractPayload(pxar []byte, filePath string) ([]byte, error) {
	rel := catalogRelPath(filePath)
	payload, err := extractFileFromPXAR(pxar, rel)
	if err != nil {
		payload, err = extractFileFromPXAR(pxar, normalizeRestorePath(filePath))
	}
	return payload, err
}

// WriteRestoredFile writes extracted payload to destPath atomically.
func WriteRestoredFile(destPath string, payload []byte) error {
	return WriteRestoredFileFromReader(destPath, bytes.NewReader(payload))
}

// WriteRestoredFileFromReader streams payload to destPath atomically.
func WriteRestoredFileFromReader(destPath string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	tmp := destPath + ".restoring"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, restoreWriteBufferSize)
	if _, err := io.Copy(bw, r); err != nil {
		_ = bw.Flush()
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := bw.Flush(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return paths.SyncDir(filepath.Dir(destPath))
}

// RestoreFile downloads and writes a single file from snapshot.
func RestoreFile(server models.PBSServer, secret string, ref SnapshotRef, filePath, destPath string) error {
	return RestoreFileWithProgress(context.Background(), server, secret, ref, filePath, destPath, nil)
}

// RestoreFileWithProgress restores one file with chunk download progress.
func RestoreFileWithProgress(ctx context.Context, server models.PBSServer, secret string, ref SnapshotRef, filePath, destPath string, onProgress StreamProgress) error {
	meta, err := loadSnapshotMeta(server, secret, ref)
	if err != nil {
		return err
	}
	modified := lookupCatalogModified(server, secret, ref, filePath)
	target := pxarRestoreTarget{FilePath: filePath, Dest: destPath, Modified: modified}
	count, err := streamRestorePXARTargets(ctx, server, secret, ref, []pxarRestoreTarget{target}, meta, "overwrite", true, onProgress, nil)
	if err != nil {
		return err
	}
	if count != 1 {
		return i18n.Ef("restore.file_not_in_archive", map[string]string{"path": filePath})
	}
	return nil
}

func lookupCatalogModified(server models.PBSServer, secret string, ref SnapshotRef, filePath string) string {
	all, err := ListCatalogFiles(server, secret, ref)
	if err != nil {
		return ""
	}
	want := strings.ToLower(strings.ReplaceAll(filePath, `/`, `\`))
	for _, f := range all {
		if strings.ToLower(strings.ReplaceAll(f.Path, `/`, `\`)) == want {
			return f.Modified
		}
	}
	return ""
}

// ResolveSnapshotTime returns RFC3339 for "latest" or validates time string.
func ResolveSnapshotTime(server models.PBSServer, secret, backupID, snapshotTime string) (string, error) {
	ref, err := ResolveSnapshot(server, secret, backupID, snapshotTime)
	if err != nil {
		return "", err
	}
	return ref.Time, nil
}

// OpenPBSWebURL builds deep link for datastore browser.
func OpenPBSWebURL(server models.PBSServer) string {
	return strings.TrimRight(server.URL, "/") + "/#Datastore/" + server.Datastore
}
