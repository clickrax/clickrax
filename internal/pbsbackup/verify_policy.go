package pbsbackup

import "pbs-win-backup/internal/models"

func verifyAfterBackupEnabled(job models.BackupJob) bool {
	return job.VerifyAfterBackup
}
