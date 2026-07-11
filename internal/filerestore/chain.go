package filerestore

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"strings"

	"pbs-win-backup/internal/fileindex"
	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/ftpclient"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/remotepath"
	"pbs-win-backup/internal/smbclient"
)

var (
	snapshotListArchives    = listArchives
	snapshotLoadManifest    = loadManifest
	snapshotLoadArchiveMeta = loadArchiveMeta
	snapshotOpenRemoteZip   = openRemoteZip
)

type snapshotView struct {
	targetArchive string
	chain         []string
	manifests     map[string]fileindex.Manifest
	files         map[string]models.SnapshotFile
	fileSource    map[string]string
	meta          filemeta.Archive
}

func buildSnapshotView(ctx context.Context, dest models.BackupDestination, password string, job models.BackupJob, snapshotTime string) (*snapshotView, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	archives, err := snapshotListArchives(dest, password, job.BackupID)
	if err != nil {
		return nil, err
	}
	ref, err := resolveArchive(archives, snapshotTime)
	if err != nil {
		return nil, err
	}

	manifest, err := snapshotLoadManifest(ctx, dest, password, job.BackupID, ref.FileName)
	if err != nil {
		return nil, err
	}
	if manifest == nil && fileindex.IsIncrementalArchiveName(ref.FileName) {
		return nil, i18n.Ef("filerestore.manifest_missing", map[string]string{"name": ref.FileName})
	}

	chain := []string{ref.FileName}
	if manifest != nil && len(manifest.Chain) > 0 {
		chain = append([]string(nil), manifest.Chain...)
	}
	for _, name := range chain {
		if _, err := remotepath.SafeComponent(name); err != nil {
			return nil, err
		}
	}

	view := &snapshotView{
		targetArchive: ref.FileName,
		chain:         chain,
		manifests:     map[string]fileindex.Manifest{},
		files:         map[string]models.SnapshotFile{},
		fileSource:    map[string]string{},
	}
	if manifest != nil {
		view.manifests[ref.FileName] = *manifest
	}

	meta, err := snapshotLoadArchiveMeta(ctx, dest, password, job.BackupID, ref.FileName)
	if err != nil {
		return nil, err
	}
	view.meta = meta

	for _, name := range chain {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		m, err := snapshotLoadManifest(ctx, dest, password, job.BackupID, name)
		if err != nil {
			return nil, err
		}
		if m == nil {
			if name != ref.FileName {
				return nil, i18n.Ef("filerestore.manifest_missing", map[string]string{"name": name})
			}
		}
		if m != nil {
			view.manifests[name] = *m
			if m.Kind == fileindex.KindIncremental {
				for _, del := range m.Deleted {
					key := strings.ToLower(del)
					delete(view.files, key)
					delete(view.fileSource, key)
				}
			}
		}

		z, err := snapshotOpenRemoteZip(ctx, dest, password, job.BackupID, name)
		if err != nil {
			return nil, i18n.Ewrap("filerestore.archive_open", map[string]string{"name": name}, err)
		}
		entries, err := listZipFiles(z)
		z.Close()
		if err != nil {
			return nil, err
		}
		for _, f := range entries {
			key := strings.ToLower(f.Path)
			view.files[key] = f
			view.fileSource[key] = name
		}
	}

	return view, nil
}

func (v *snapshotView) catalog() []models.SnapshotFile {
	out := make([]models.SnapshotFile, 0, len(v.files))
	for _, f := range v.files {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return filemeta.EnrichSnapshotFiles(v.meta, out)
}

func loadManifest(ctx context.Context, dest models.BackupDestination, password, backupID, archiveName string) (*fileindex.Manifest, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	name := fileindex.ManifestName(archiveName)
	if !remoteSidecarExists(dest, password, backupID, name) {
		return nil, nil
	}
	z, err := openRemoteZip(ctx, dest, password, backupID, name)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	if z.size <= 0 || z.size > 4*1024*1024 {
		return nil, i18n.Ef("filerestore.manifest_size", map[string]string{"name": name})
	}
	data := make([]byte, z.size)
	if _, err := z.reader.ReadAt(data, 0); err != nil && err != io.EOF {
		return nil, i18n.Ef("filerestore.manifest_read", map[string]string{"name": name, "err": err.Error()})
	}
	var m fileindex.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, i18n.Ef("filerestore.manifest_parse", map[string]string{"name": name, "err": err.Error()})
	}
	return &m, nil
}

func remoteSidecarExists(dest models.BackupDestination, password, backupID, fileName string) bool {
	switch dest.NormalizedType() {
	case models.DestSMB:
		return smbclient.RemoteFileExists(dest, password, backupID, fileName)
	case models.DestFTP:
		return ftpclient.RemoteFileExists(dest, password, backupID, fileName)
	default:
		return false
	}
}

func loadArchiveMeta(ctx context.Context, dest models.BackupDestination, password, backupID, archiveName string) (filemeta.Archive, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	name := filemeta.MetaFileName(archiveName)
	if !remoteSidecarExists(dest, password, backupID, name) {
		return filemeta.NewArchive(), nil
	}
	z, err := openRemoteZip(ctx, dest, password, backupID, name)
	if err != nil {
		return filemeta.Archive{}, err
	}
	defer z.Close()
	if z.size <= 0 || z.size > 64*1024*1024 {
		return filemeta.Archive{}, i18n.Ef("filerestore.meta_size", map[string]string{"name": name})
	}
	data := make([]byte, z.size)
	if _, err := z.reader.ReadAt(data, 0); err != nil && err != io.EOF {
		return filemeta.Archive{}, i18n.Ef("filerestore.meta_read", map[string]string{"name": name, "err": err.Error()})
	}
	return filemeta.Unmarshal(data)
}

func mergeCatalog(files []models.SnapshotFile, search string) []models.SnapshotFile {
	if search != "" {
		return filterFiles(files, search)
	}
	return files
}
