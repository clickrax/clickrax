package pbsbackup

import (
	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/models"

	pbscommon "pbscommon"
)

// uploadWinMeta does not upload to PBS. WinMeta/ACL sidecars are local-only:
// PBS blobs are capped at ~16 MiB and large trees break Finish manifest update.
// NTFS metadata is already stored in the PXAR stream.
func uploadWinMeta(_ *pbscommon.PBSClient, _ string, _ *Stats) error {
	return nil
}

func loadSnapshotMeta(server models.PBSServer, secret string, ref SnapshotRef) (filemeta.Archive, error) {
	client, cleanup, err := openReader(server, secret, ref)
	if err != nil {
		return filemeta.Archive{}, err
	}
	defer cleanup()

	raw, err := downloadPBSBlob(client, filemeta.PBSBlobName, filemeta.PBSBlobNameLegacy)
	if err != nil || len(raw) == 0 {
		return filemeta.NewArchive(), nil
	}
	return filemeta.Unmarshal(raw)
}

func applyRestoredMeta(meta filemeta.Archive, catalogPath, destPath, modifiedRFC3339 string) error {
	return filemeta.ApplyFile(destPath, filemeta.PrepareEntry(meta, catalogPath, modifiedRFC3339))
}
