package pbsbackup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/pbs"
)

const (
	verifyMinTimeout = 15 * time.Minute
	verifyMaxTimeout = 48 * time.Hour
)

// verifyJobTimeout estimates how long PBS verify may take for a snapshot size.
func verifyJobTimeout(bytesProcessed int64) time.Duration {
	timeout := verifyMinTimeout
	if bytesProcessed > 0 {
		// ~1 minute per 10 GiB processed, on top of the base timeout.
		timeout += time.Duration(bytesProcessed/(10*1024*1024*1024)) * time.Minute
	}
	if timeout > verifyMaxTimeout {
		return verifyMaxTimeout
	}
	return timeout
}

// VerifyAfterBackup runs PBS native snapshot verify (updates verification status in PBS UI).
func VerifyAfterBackup(ctx context.Context, server models.PBSServer, secret, backupID string, backupTime int64, bytesProcessed int64) error {
	if backupTime <= 0 {
		return i18n.E("pbs.verify.invalid_time", nil)
	}

	checkCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := verifySnapshotExists(checkCtx, server, secret, backupID, backupTime); err != nil {
		return err
	}

	client := pbs.NewClient(server, secret)
	upid, err := client.StartSnapshotVerify("host", backupID, backupTime)
	if err != nil {
		if strings.Contains(err.Error(), "403") {
			return i18n.Ef("pbs.verify.pbs_denied", map[string]string{"err": err.Error()})
		}
		return i18n.Ef("pbs.verify.pbs", map[string]string{"err": err.Error()})
	}

	jobCtx, jobCancel := context.WithTimeout(ctx, verifyJobTimeout(bytesProcessed))
	defer jobCancel()
	if err := client.WaitTask(jobCtx, upid); err != nil {
		return i18n.Ef("pbs.verify.pbs", map[string]string{"err": err.Error()})
	}
	return nil
}

// VerifyTimeout reports whether verify failed only because the client stopped waiting.
func VerifyTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded")
}

// VerifySnapshotAPI kept for callers.
func VerifySnapshotAPI(ctx context.Context, server models.PBSServer, secret, backupID, snapshotRFC3339 string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	t, err := time.Parse(time.RFC3339, snapshotRFC3339)
	if err != nil {
		return i18n.Ewrap("pbs.verify.snapshot_time", nil, err)
	}
	return VerifyAfterBackup(ctx, server, secret, backupID, t.Unix(), 0)
}

func verifySnapshotExists(ctx context.Context, server models.PBSServer, secret, backupID string, backupTime int64) error {
	snaps, err := listSnapshotsWithContext(ctx, server, secret)
	if err != nil {
		return i18n.Ewrap("pbs.verify.snapshot_list", nil, err)
	}
	if !snapshotListed(snaps, backupID, backupTime) {
		return i18n.Ef("pbs.verify.snapshot_not_found", map[string]string{
			"id": backupID, "time": fmt.Sprintf("%d", backupTime),
		})
	}

	client := pbs.NewClient(server, secret)
	files, err := client.SnapshotManifestFiles("host", backupID, backupTime)
	if err != nil {
		return i18n.Ewrap("pbs.verify.manifest", nil, err)
	}
	if len(files) == 0 {
		return i18n.E("pbs.verify.manifest_empty", nil)
	}
	if !manifestHasBackupArchives(files) {
		return i18n.E("pbs.verify.no_pxar_didx", nil)
	}
	return nil
}

func listSnapshotsWithContext(ctx context.Context, server models.PBSServer, secret string) ([]models.SnapshotInfo, error) {
	if err := abortIfCancelled(ctx); err != nil {
		return nil, err
	}
	return pbs.NewClient(server, secret).ListSnapshotsWithContext(ctx)
}

func snapshotListed(snaps []models.SnapshotInfo, backupID string, backupTime int64) bool {
	wantID := strings.TrimSpace(backupID)
	for _, s := range snaps {
		id := strings.TrimSpace(s.Backup)
		if id == "" {
			continue
		}
		if !strings.EqualFold(id, wantID) {
			continue
		}
		if s.BackupTime == backupTime {
			return true
		}
		if s.BackupTime == 0 && s.Time != "" {
			if t, err := time.Parse(time.RFC3339, s.Time); err == nil && t.Unix() == backupTime {
				return true
			}
		}
	}
	return false
}

func manifestHasBackupArchives(files []string) bool {
	for _, f := range files {
		name := strings.ToLower(strings.TrimSpace(f))
		if name == "backup.pxar.didx" || strings.HasSuffix(name, "/backup.pxar.didx") {
			return true
		}
	}
	return false
}
