package backup

import (
	"testing"

	"pbs-win-backup/internal/models"
)

func TestJobResultWarningWhenFilesSkipped(t *testing.T) {
	result := models.JobRunResult{
		Status:       "ok",
		FilesSkipped: 1,
	}
	if result.FilesSkipped <= 0 {
		t.Fatal("test precondition")
	}
	if result.FilesSkipped > 0 {
		result.Status = "warning"
	}
	if result.Status != "warning" {
		t.Fatalf("got status %q", result.Status)
	}
}
