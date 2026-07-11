package smbclient

import (
	"context"
	"fmt"
	"io"
	"time"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/remotepath"
)

// TestWriteAccess creates and deletes a temporary file in the backup directory.
func TestWriteAccess(dest models.BackupDestination, password, backupID string) error {
	sh, sess, conn, err := dial(context.Background(), dest, password)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer sess.Logoff()
	defer sh.Umount()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return err
	}
	if err := mkdirAll(sh, dir); err != nil {
		return err
	}
	name := fmt.Sprintf(".clickrax-write-test-%d", time.Now().UnixNano())
	remotePath, err := remotepath.JoinDir(dir, name)
	if err != nil {
		return err
	}
	f, err := sh.Create(remotePath)
	if err != nil {
		return i18n.Ewrap("smb.write_test", map[string]string{"path": remotePath}, err)
	}
	if _, err := io.WriteString(f, "ok"); err != nil {
		_ = f.Close()
		_ = sh.Remove(remotePath)
		return i18n.Ewrap("smb.write_test", map[string]string{"path": remotePath}, err)
	}
	if err := f.Close(); err != nil {
		_ = sh.Remove(remotePath)
		return i18n.Ewrap("smb.write_test", map[string]string{"path": remotePath}, err)
	}
	_ = sh.Remove(remotePath)
	return nil
}
