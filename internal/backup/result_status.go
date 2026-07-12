package backup

import (
	"fmt"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

func finalizeRunResult(result *models.JobRunResult, statsWarning string, b *i18n.Bundle) {
	if result == nil || b == nil {
		return
	}
	result.Status = "ok"
	if statsWarning != "" {
		result.Status = "warning"
		result.Message = statsWarning
	}
	if result.FilesSkipped > 0 && result.Message == "" {
		result.Message = b.Tf("backup.files_skipped_info", map[string]string{
			"n": fmt.Sprintf("%d", result.FilesSkipped),
		})
	}
}
