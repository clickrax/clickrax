package filerestore

import (
	"archive/zip"
	"context"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"pbs-win-backup/internal/ftpclient"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/smbclient"
)

var zipStampRE = regexp.MustCompile(`_(\d{8}-\d{6})(?:_incr)?\.zip$`)

type archiveRef struct {
	FileName string
	Time     time.Time
}

type remoteZip struct {
	reader io.ReaderAt
	size   int64
	close  func() error
}

func (z *remoteZip) Close() {
	if z.close != nil {
		_ = z.close()
	}
}

func ListSnapshots(dest models.BackupDestination, password string, job models.BackupJob) ([]models.SnapshotInfo, error) {
	archives, err := listArchives(dest, password, job.BackupID)
	if err != nil {
		return nil, err
	}
	backupID := strings.TrimSpace(job.BackupID)
	if backupID == "" {
		backupID = "windows-host"
	}
	out := make([]models.SnapshotInfo, 0, len(archives))
	for _, a := range archives {
		t := a.Time
		if t.IsZero() {
			continue
		}
		out = append(out, models.SnapshotInfo{
			Time:       t.UTC().Format(time.RFC3339),
			Backup:     backupID,
			BackupTime: t.Unix(),
			Comment:    a.FileName,
			HasCatalog: true,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out, nil
}

func ListFiles(dest models.BackupDestination, password string, job models.BackupJob, snapshotTime, search string) ([]models.SnapshotFile, error) {
	view, err := buildSnapshotView(context.Background(), dest, password, job, snapshotTime)
	if err != nil {
		return nil, err
	}
	files := view.catalog()
	return mergeCatalog(files, search), nil
}

func listArchives(dest models.BackupDestination, password, backupID string) ([]archiveRef, error) {
	switch dest.NormalizedType() {
	case models.DestSMB:
		entries, err := smbclient.ListArchives(dest, password, backupID)
		if err != nil {
			return nil, err
		}
		return archivesFromSMB(entries), nil
	case models.DestFTP:
		entries, err := ftpclient.ListArchives(dest, password, backupID)
		if err != nil {
			return nil, err
		}
		return archivesFromFTP(entries), nil
	default:
		return nil, i18n.Ef("filerestore.dest_unsupported", map[string]string{"type": dest.Type})
	}
}

func archivesFromSMB(entries []smbclient.ArchiveEntry) []archiveRef {
	out := make([]archiveRef, 0, len(entries))
	for _, e := range entries {
		t := parseArchiveTime(e.Name)
		if t.IsZero() && e.ModTime > 0 {
			t = time.Unix(e.ModTime, 0).UTC()
		}
		out = append(out, archiveRef{FileName: e.Name, Time: t})
	}
	return out
}

func archivesFromFTP(entries []ftpclient.ArchiveEntry) []archiveRef {
	out := make([]archiveRef, 0, len(entries))
	for _, e := range entries {
		t := parseArchiveTime(e.Name)
		if t.IsZero() && e.ModTime > 0 {
			t = time.Unix(e.ModTime, 0).UTC()
		}
		out = append(out, archiveRef{FileName: e.Name, Time: t})
	}
	return out
}

func parseArchiveTime(name string) time.Time {
	m := zipStampRE.FindStringSubmatch(name)
	if len(m) < 2 {
		return time.Time{}
	}
	t, err := time.ParseInLocation("20060102-150405", m[1], time.UTC)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func resolveArchive(archives []archiveRef, snapshotTime string) (archiveRef, error) {
	snapshotTime = strings.TrimSpace(snapshotTime)
	if snapshotTime == "" || strings.EqualFold(snapshotTime, "latest") {
		if len(archives) == 0 {
			return archiveRef{}, i18n.E("filerestore.archives_not_found", nil)
		}
		sort.Slice(archives, func(i, j int) bool { return archives[i].Time.After(archives[j].Time) })
		return archives[0], nil
	}
	for _, a := range archives {
		if a.Time.UTC().Format(time.RFC3339) == snapshotTime {
			return a, nil
		}
	}
	for _, a := range archives {
		if a.FileName == snapshotTime {
			return a, nil
		}
	}
	return archiveRef{}, i18n.Ef("filerestore.archive_not_found", map[string]string{"path": snapshotTime})
}

func openArchive(dest models.BackupDestination, password string, job models.BackupJob, snapshotTime string) (*remoteZip, error) {
	archives, err := listArchives(dest, password, job.BackupID)
	if err != nil {
		return nil, err
	}
	ref, err := resolveArchive(archives, snapshotTime)
	if err != nil {
		return nil, err
	}
	return openRemoteZip(context.Background(), dest, password, job.BackupID, ref.FileName)
}

func openRemoteZip(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string) (*remoteZip, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	switch dest.NormalizedType() {
	case models.DestSMB:
		ra, size, closeFn, err := smbclient.OpenRemoteFile(ctx, dest, password, backupID, fileName)
		if err != nil {
			return nil, err
		}
		return &remoteZip{reader: ra, size: size, close: closeFn}, nil
	case models.DestFTP:
		ra, size, closeFn, err := ftpclient.OpenRemoteFile(ctx, dest, password, backupID, fileName)
		if err != nil {
			return nil, err
		}
		return &remoteZip{reader: ra, size: size, close: closeFn}, nil
	default:
		return nil, i18n.Ef("filerestore.dest_unsupported", map[string]string{"type": dest.Type})
	}
}

func zipReader(z *remoteZip) (*zip.Reader, error) {
	if z.size <= 0 {
		return nil, i18n.E("filerestore.archive_empty", nil)
	}
	return zip.NewReader(z.reader, z.size)
}

func listZipFiles(z *remoteZip) ([]models.SnapshotFile, error) {
	zr, err := zipReader(z)
	if err != nil {
		return nil, i18n.Ef("filerestore.zip_read", map[string]string{"err": err.Error()})
	}
	out := make([]models.SnapshotFile, 0, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !safeZipEntryName(f.Name) {
			continue
		}
		out = append(out, models.SnapshotFile{
			Path:     zipPathToCatalog(f.Name),
			Size:     int64(f.UncompressedSize64),
			IsDir:    false,
			Modified: f.Modified.UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func filterFiles(files []models.SnapshotFile, query string) []models.SnapshotFile {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return files
	}
	out := make([]models.SnapshotFile, 0)
	for _, f := range files {
		if strings.Contains(strings.ToLower(f.Path), q) {
			out = append(out, f)
		}
	}
	return out
}

func zipPathToCatalog(name string) string {
	return strings.ReplaceAll(name, "/", `\`)
}

func catalogPathToZip(path string) string {
	return strings.ReplaceAll(path, `\`, "/")
}

func pathsMatch(a, b string) bool {
	na := strings.ToLower(strings.ReplaceAll(a, `/`, `\`))
	nb := strings.ToLower(strings.ReplaceAll(b, `/`, `\`))
	return na == nb
}

// safeZipEntryName rejects absolute paths and zip-slip (..) entries.
func safeZipEntryName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return false
	}
	if strings.Contains(name, ":") {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

func findZipEntry(zr *zip.Reader, filePath string) (*zip.File, error) {
	if !safeZipEntryName(catalogPathToZip(filePath)) {
		return nil, i18n.Ef("filerestore.unsafe_path", map[string]string{"path": filePath})
	}
	wantZip := catalogPathToZip(filePath)
	for _, f := range zr.File {
		if pathsMatch(f.Name, wantZip) || pathsMatch(zipPathToCatalog(f.Name), filePath) {
			return f, nil
		}
	}
	return nil, i18n.Ef("restore.file_not_in_archive", map[string]string{"path": filePath})
}
