package ftpclient

import (
	"fmt"
	"strings"
	"time"

	"context"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/remotepath"
)

// TestWriteAccess creates and deletes a temporary file in the backup directory.
func TestWriteAccess(dest models.BackupDestination, password, backupID string) error {
	c, err := connect(context.Background(), dest, password)
	if err != nil {
		return err
	}
	defer c.Quit()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return err
	}
	if err := mkdirAll(c, dir); err != nil {
		return err
	}
	if err := c.ChangeDir(dir); err != nil {
		return i18n.Ewrap("ftp.dir", map[string]string{"path": dir}, err)
	}
	name := fmt.Sprintf(".clickrax-write-test-%d", time.Now().UnixNano())
	if _, err := remotepath.SafeComponent(name); err != nil {
		return err
	}
	if err := c.Stor(name, strings.NewReader("ok")); err != nil {
		return i18n.Ewrap("ftp.write_test", map[string]string{"path": name}, err)
	}
	_ = c.Delete(name)
	return nil
}
