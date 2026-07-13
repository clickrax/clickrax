package pbsbackup

import (
	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"

	pbscommon "pbscommon"
)

func uploadWinMeta(client *pbscommon.PBSClient, backupdir string, stats *Stats) error {
	meta, err := filemeta.CollectTree(backupdir, true)
	if err != nil {
		return i18n.Ewrap("pbs.acl_collect", nil, err)
	}
	if len(meta.Files) == 0 {
		return nil
	}
	data, err := filemeta.Marshal(meta)
	if err != nil {
		return err
	}
	return uploadBlobToPBS(client, stats, filemeta.PBSBlobName, data)
}

func loadSnapshotMeta(server models.PBSServer, secret string, ref SnapshotRef) (filemeta.Archive, error) {
	client, cleanup, err := openReader(server, secret, ref)
	if err != nil {
		return filemeta.Archive{}, err
	}
	defer cleanup()

	raw, err := client.DownloadToBytes(filemeta.PBSBlobName)
	if err != nil {
		return filemeta.NewArchive(), nil
	}
	return filemeta.Unmarshal(raw)
}

func applyRestoredMeta(meta filemeta.Archive, catalogPath, destPath, modifiedRFC3339 string) error {
	return filemeta.ApplyFile(destPath, filemeta.PrepareEntry(meta, catalogPath, modifiedRFC3339))
}
