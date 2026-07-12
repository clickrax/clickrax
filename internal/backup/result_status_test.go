package backup

import (
	"testing"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

func TestFinalizeRunResult_skippedFilesStayOk(t *testing.T) {
	b := i18n.New("en")
	result := models.JobRunResult{FilesSkipped: 26}
	finalizeRunResult(&result, "", b)
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if result.Message == "" {
		t.Fatal("expected informational message for skipped files")
	}
}

func TestFinalizeRunResult_statsWarning(t *testing.T) {
	b := i18n.New("en")
	result := models.JobRunResult{FilesSkipped: 3}
	finalizeRunResult(&result, "verify timeout", b)
	if result.Status != "warning" {
		t.Fatalf("status = %q, want warning", result.Status)
	}
	if result.Message != "verify timeout" {
		t.Fatalf("message = %q", result.Message)
	}
}
