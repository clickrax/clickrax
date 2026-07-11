package pbsbackup

import (
	"strings"

	"pbs-win-backup/internal/models"
)

// ProbeBackupAccess checks PBS backup protocol auth (not just REST API).
func ProbeBackupAccess(server models.PBSServer, secret, backupID string) error {
	secret = strings.TrimSpace(secret)
	client := newPBSClient(server, secret, backupID)
	client.Connect(false, "host")
	defer client.AbortBackupSession()
	_, err := client.DownloadPreviousToBytes("backup.pxar.didx")
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") || strings.Contains(msg, "404") ||
		strings.Contains(msg, "no valid previous") || strings.Contains(msg, "no previous") {
		return nil
	}
	return err
}
