package pbsbackup

import (
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/pbs"
)

// ListSnapshots returns snapshots via PBS JSON API.
func ListSnapshots(server models.PBSServer, secret string) ([]models.SnapshotInfo, error) {
	return pbs.NewClient(server, secret).ListSnapshots()
}
