package pbsbackup

import (
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestSnapshotListedExactTime(t *testing.T) {
	snaps := []models.SnapshotInfo{
		{Backup: "Dan", BackupTime: 1783398000, Time: time.Unix(1783398000, 0).UTC().Format(time.RFC3339)},
		{Backup: "other", BackupTime: 1},
	}
	if !snapshotListed(snaps, "Dan", 1783398000) {
		t.Fatal("expected match")
	}
	if snapshotListed(snaps, "Dan", 1783398001) {
		t.Fatal("unexpected match")
	}
}

func TestManifestHasBackupArchives(t *testing.T) {
	if !manifestHasBackupArchives([]string{"backup.pxar.didx", "catalog.pcat1.didx"}) {
		t.Fatal("expected pxar")
	}
	if manifestHasBackupArchives([]string{"catalog.pcat1.didx"}) {
		t.Fatal("unexpected pxar")
	}
}
