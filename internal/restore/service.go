package restore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/credential"
	"pbs-win-backup/internal/filerestore"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/pbsbackup"
)

type Service struct {
	getServer func(id string) (*models.PBSServer, error)
	getDest   func(job models.BackupJob) (*models.BackupDestination, string, error)
}

type FolderProgressFunc func(done, total int, path string)

func New(getServer func(id string) (*models.PBSServer, error)) *Service {
	return &Service{getServer: getServer}
}

func NewWithDestinations(
	getServer func(id string) (*models.PBSServer, error),
	getDest func(job models.BackupJob) (*models.BackupDestination, string, error),
) *Service {
	return &Service{getServer: getServer, getDest: getDest}
}

func (s *Service) bundle() *i18n.Bundle {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return i18n.New("")
	}
	return i18n.New(cfg.Settings.Language)
}

func restoreErrMsg(err error, b *i18n.Bundle) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return b.T("restore.cancelled")
	}
	return err.Error()
}

func (s *Service) overwriteMode() string {
	cfg, err := config.Load()
	if err != nil {
		return "ask"
	}
	if cfg.Settings.RestoreOverwrite == "" {
		return "ask"
	}
	return cfg.Settings.RestoreOverwrite
}

func (s *Service) resolveJob(job models.BackupJob) (*models.BackupDestination, string, error) {
	if s.getDest != nil {
		return s.getDest(job)
	}
	destID := job.EffectiveDestinationID()
	server, err := s.getServer(destID)
	if err != nil {
		return nil, "", err
	}
	secret, err := credential.GetSecret(server.ID)
	if err != nil {
		return nil, "", s.bundle().E("restore.secret_not_found")
	}
	d := models.PBSServerToDestination(*server)
	return &d, secret, nil
}

func (s *Service) ListSnapshots(job models.BackupJob) ([]models.SnapshotInfo, error) {
	dest, secret, err := s.resolveJob(job)
	if err != nil {
		return nil, err
	}
	if !dest.IsPBS() {
		return filerestore.ListSnapshots(*dest, secret, job)
	}

	server := dest.ToPBSServer()
	all, err := pbsbackup.ListSnapshots(server, secret)
	if err != nil {
		return nil, err
	}
	out := make([]models.SnapshotInfo, 0)
	for _, snap := range all {
		if job.BackupID != "" && snap.Backup != job.BackupID {
			continue
		}
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	enrichSnapshotCatalog(server, secret, out)
	return out, nil
}

func enrichSnapshotCatalog(server models.PBSServer, secret string, snaps []models.SnapshotInfo) {
	for i := range snaps {
		ref := pbsbackup.SnapshotRef{
			Time:       snaps[i].Time,
			BackupID:   snaps[i].Backup,
			BackupTime: snaps[i].BackupTime,
		}
		hasCat, err := pbsbackup.CatalogAvailable(server, secret, ref)
		if err != nil || !hasCat {
			snaps[i].HasCatalog = false
			continue
		}
		hasPxar, err := pbsbackup.PxarAvailable(server, secret, ref)
		if err == nil {
			snaps[i].HasCatalog = hasPxar
		}
	}
}

func (s *Service) ListFiles(job models.BackupJob, snapshotTime, dirPath, search string) ([]models.SnapshotFile, error) {
	dest, secret, err := s.resolveJob(job)
	if err != nil {
		return nil, err
	}
	if !dest.IsPBS() {
		return filerestore.ListFiles(*dest, secret, job, snapshotTime, search)
	}

	server := dest.ToPBSServer()
	ref, err := pbsbackup.ResolveSnapshot(server, secret, job.BackupID, snapshotTime)
	if err != nil {
		return nil, err
	}
	if search != "" {
		return pbsbackup.SearchCatalogFiles(server, secret, ref, search)
	}
	return pbsbackup.ListCatalogDir(server, secret, ref, dirPath)
}

func (s *Service) Restore(ctx context.Context, req models.RestoreRequest, job models.BackupJob, onStream pbsbackup.StreamProgress) models.RestoreResult {
	b := s.bundle()
	i18n.SetActive(b)
	defer i18n.SetActive(nil)

	if req.FilePath == "" {
		return models.RestoreResult{OK: false, Message: b.T("restore.no_file")}
	}

	dest, secret, err := s.resolveJob(job)
	if err != nil {
		return models.RestoreResult{OK: false, Message: err.Error()}
	}

	var destPath string
	if req.ToOriginal {
		destPath, err = resolveOriginalDest(req.FilePath, job.Sources)
		if err != nil {
			return models.RestoreResult{OK: false, Message: err.Error()}
		}
	} else if strings.TrimSpace(req.DestPath) == "" {
		return models.RestoreResult{OK: false, Message: b.T("restore.dest_empty")}
	} else {
		destPath, err = resolveFileDest(req.FilePath, req.DestPath)
		if err != nil {
			return models.RestoreResult{OK: false, Message: err.Error()}
		}
	}

	mode := s.overwriteMode()
	prepared, err := prepareDestPath(destPath, mode, req.Overwrite)
	if err != nil {
		if isFileExistsErr(err) && mode == "ask" && !req.Overwrite {
			return models.RestoreResult{
				OK:           false,
				NeedsConfirm: true,
				ExistingPath: destPath,
				Message:      b.Tf("restore.file_exists_label", map[string]string{"path": destPath}),
			}
		}
		return models.RestoreResult{OK: false, Message: err.Error()}
	}
	destPath = prepared

	if err := ensureParentDir(destPath); err != nil {
		return models.RestoreResult{OK: false, Message: err.Error()}
	}

	if !dest.IsPBS() {
		if err := filerestore.RestoreFile(ctx, *dest, secret, job, req.Snapshot, req.FilePath, destPath, mode, req.Overwrite); err != nil {
			return models.RestoreResult{OK: false, Message: err.Error()}
		}
		return models.RestoreResult{OK: true, Message: b.T("restore.file_restored"), Path: destPath}
	}

	server := dest.ToPBSServer()
	ref, err := pbsbackup.ResolveSnapshot(server, secret, job.BackupID, req.Snapshot)
	if err != nil {
		return models.RestoreResult{OK: false, Message: err.Error()}
	}
	if err := pbsbackup.RestoreFileWithProgress(ctx, server, secret, ref, req.FilePath, destPath, onStream); err != nil {
		return models.RestoreResult{OK: false, Message: restoreErrMsg(err, b)}
	}
	return models.RestoreResult{OK: true, Message: b.T("restore.file_restored"), Path: destPath}
}

func (s *Service) RestoreFolder(ctx context.Context, req models.RestoreFolderRequest, job models.BackupJob, onProgress FolderProgressFunc) models.RestoreFolderResult {
	b := s.bundle()
	i18n.SetActive(b)
	defer i18n.SetActive(nil)

	if req.FolderPath == "" {
		return models.RestoreFolderResult{OK: false, Message: b.T("restore.no_folder")}
	}

	dest, secret, err := s.resolveJob(job)
	if err != nil {
		return models.RestoreFolderResult{OK: false, Message: err.Error()}
	}

	destRoot := req.DestPath
	if req.ToOriginal {
		destRoot, err = resolveOriginalDest(req.FolderPath, job.Sources)
		if err != nil {
			return models.RestoreFolderResult{OK: false, Message: err.Error()}
		}
	} else if strings.TrimSpace(destRoot) == "" {
		return models.RestoreFolderResult{OK: false, Message: b.T("restore.dest_empty")}
	}
	if err := EnsureDestDir(destRoot); err != nil {
		return models.RestoreFolderResult{OK: false, Message: err.Error()}
	}

	mode := s.overwriteMode()
	progress := func(done, total int, path string) {
		if onProgress != nil {
			onProgress(done, total, path)
		}
	}

	if !dest.IsPBS() {
		count, err := filerestore.RestoreFolder(ctx, *dest, secret, job, req.Snapshot, req.FolderPath, destRoot, mode, req.Overwrite, progress)
		if err != nil {
			return models.RestoreFolderResult{OK: false, Message: err.Error(), Count: count}
		}
		return models.RestoreFolderResult{
			OK:      true,
			Message: b.Tf("restore.files_count", map[string]string{"count": fmt.Sprintf("%d", count)}),
			Count:   count,
		}
	}

	server := dest.ToPBSServer()
	ref, err := pbsbackup.ResolveSnapshot(server, secret, job.BackupID, req.Snapshot)
	if err != nil {
		return models.RestoreFolderResult{OK: false, Message: err.Error()}
	}
	count, err := pbsbackup.RestoreFolder(ctx, server, secret, ref, req.FolderPath, destRoot, mode, req.Overwrite, progress)
	if err != nil {
		return models.RestoreFolderResult{OK: false, Message: restoreErrMsg(err, b), Count: count}
	}
	return models.RestoreFolderResult{
		OK:      true,
		Message: b.Tf("restore.files_count", map[string]string{"count": fmt.Sprintf("%d", count)}),
		Count:   count,
	}
}

func (s *Service) OpenPBSWeb(job models.BackupJob) (string, error) {
	dest, _, err := s.resolveJob(job)
	if err != nil {
		return "", err
	}
	if !dest.IsPBS() {
		return "", s.bundle().E("restore.web_pbs_only")
	}
	return pbsbackup.OpenPBSWebURL(dest.ToPBSServer()), nil
}

func EnsureDestDir(dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return nil
}

func isFileExistsErr(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "file_exists:")
}

func (s *Service) RestoreBatch(ctx context.Context, req models.RestoreBatchRequest, job models.BackupJob, onProgress FolderProgressFunc) models.RestoreBatchResult {
	b := s.bundle()
	i18n.SetActive(b)
	defer i18n.SetActive(nil)

	if len(req.Paths) == 0 {
		return models.RestoreBatchResult{OK: false, Message: b.T("restore.no_selection")}
	}

	dest, secret, err := s.resolveJob(job)
	if err != nil {
		return models.RestoreBatchResult{OK: false, Message: err.Error()}
	}

	destRoot := req.DestPath
	if req.ToOriginal {
		destRoot = ""
	} else {
		if strings.TrimSpace(destRoot) == "" {
			return models.RestoreBatchResult{OK: false, Message: b.T("restore.dest_empty")}
		}
		if err := EnsureDestDir(destRoot); err != nil {
			return models.RestoreBatchResult{OK: false, Message: err.Error()}
		}
	}

	mode := s.overwriteMode()
	progress := func(done, total int, path string) {
		if onProgress != nil {
			onProgress(done, total, path)
		}
	}

	if !dest.IsPBS() {
		count, err := filerestore.RestoreBatch(ctx, *dest, secret, job, req.Snapshot, req.Paths, job.Sources, destRoot, req.ToOriginal, mode, req.Overwrite, progress)
		if err != nil {
			return models.RestoreBatchResult{OK: false, Message: err.Error(), Count: count}
		}
		return models.RestoreBatchResult{
			OK:      true,
			Message: b.Tf("restore.files_count", map[string]string{"count": fmt.Sprintf("%d", count)}),
			Count:   count,
		}
	}

	server := dest.ToPBSServer()
	ref, err := pbsbackup.ResolveSnapshot(server, secret, job.BackupID, req.Snapshot)
	if err != nil {
		return models.RestoreBatchResult{OK: false, Message: err.Error()}
	}
	count, err := pbsbackup.RestoreBatch(ctx, server, secret, ref, req.Paths, job.Sources, destRoot, req.ToOriginal, mode, req.Overwrite, progress)
	if err != nil {
		return models.RestoreBatchResult{OK: false, Message: restoreErrMsg(err, b), Count: count}
	}
	return models.RestoreBatchResult{
		OK:      true,
		Message: b.Tf("restore.files_count", map[string]string{"count": fmt.Sprintf("%d", count)}),
		Count:   count,
	}
}
