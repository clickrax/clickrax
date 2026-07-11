package pbsbackup

import (
	"context"

	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/models"
)

type pxarRestoreTarget struct {
	FilePath string
	Dest     string
	Modified string
	restored bool
}

// restorePXARTargetsStreaming restores files by streaming pxar from PBS directly to disk.
func restorePXARTargetsStreaming(
	ctx context.Context,
	server models.PBSServer,
	secret string,
	ref SnapshotRef,
	targets []pxarRestoreTarget,
	meta filemeta.Archive,
	overwriteMode string,
	forceOverwrite bool,
	onChunkProgress StreamProgress,
	onFileProgress RestoreFolderProgress,
) (int, error) {
	return streamRestorePXARTargets(
		ctx, server, secret, ref, targets, meta, overwriteMode, forceOverwrite,
		onChunkProgress, onFileProgress,
	)
}
