package pbsbackup

import (
	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/models"
)

// SnapshotRef identifies a snapshot for PBS reader API (exact backup-time from PBS).
type SnapshotRef struct {
	Time       string
	BackupID   string
	BackupTime int64
}

func snapshotRefFrom(s models.SnapshotInfo, jobBackupID string) SnapshotRef {
	bid := s.Backup
	if bid == "" {
		bid = jobBackupID
	}
	unix := s.BackupTime
	if unix == 0 {
		unix, _ = snapshotToUnix(s.Time)
	}
	return SnapshotRef{
		Time:       s.Time,
		BackupID:   bid,
		BackupTime: unix,
	}
}

// ResolveSnapshot finds snapshot metadata from PBS list (preferred) or parses time string.
func ResolveSnapshot(server models.PBSServer, secret, jobBackupID, snapshotTime string) (SnapshotRef, error) {
	snaps, err := ListSnapshots(server, secret)
	if err != nil {
		return SnapshotRef{}, err
	}

	if snapshotTime == "" || snapshotTime == "latest" {
		for _, s := range snaps {
			if jobBackupID == "" || s.Backup == jobBackupID {
				return snapshotRefFrom(s, jobBackupID), nil
			}
		}
		if len(snaps) > 0 {
			return snapshotRefFrom(snaps[0], jobBackupID), nil
		}
		return SnapshotRef{}, i18nconfig.FromConfig().E("pbs.snapshots_not_found")
	}

	for _, s := range snaps {
		if s.Time == snapshotTime {
			return snapshotRefFrom(s, jobBackupID), nil
		}
	}

	unix, err := snapshotToUnix(snapshotTime)
	if err != nil {
		return SnapshotRef{}, err
	}
	bid := jobBackupID
	if bid == "" {
		return SnapshotRef{}, i18nconfig.FromConfig().E("pbs.backup_id_missing")
	}
	return SnapshotRef{
		Time:       snapshotTime,
		BackupID:   bid,
		BackupTime: unix,
	}, nil
}
