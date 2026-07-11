package pbsbackup

import (
	"testing"

	"pbs-win-backup/internal/models"
)

func TestVerifyAfterBackupEnabled(t *testing.T) {
	if verifyAfterBackupEnabled(models.BackupJob{VerifyAfterBackup: true}) != true {
		t.Fatal("expected true when enabled")
	}
	if verifyAfterBackupEnabled(models.BackupJob{VerifyAfterBackup: false}) != false {
		t.Fatal("expected false when disabled")
	}
}
