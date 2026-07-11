package destination

import (
	"errors"
	"testing"

	"pbs-win-backup/internal/models"
)

func TestTestFull_SMB_ReadOnlyShare_Fails(t *testing.T) {
	oldConn := smbConnectTest
	oldWrite := smbWriteTest
	defer func() {
		smbConnectTest = oldConn
		smbWriteTest = oldWrite
	}()
	smbConnectTest = func(models.BackupDestination, string) error { return nil }
	smbWriteTest = func(models.BackupDestination, string, string) error {
		return errors.New("read-only share")
	}

	dest := models.BackupDestination{
		Type:  "smb",
		Host:  "nas.local",
		Share: "backups",
	}
	result := TestFullLang(dest, "secret", "backup-1", "en")
	if result.OK {
		t.Fatal("read-only SMB share should fail full destination test")
	}
}
