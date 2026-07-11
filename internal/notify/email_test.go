package notify

import (
	"strings"
	"testing"
	"time"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

func TestBuildEmailContent_includesHostname(t *testing.T) {
	b := i18n.New("ru")
	result := models.JobRunResult{
		JobName:    "Everyday",
		Status:     "ok",
		BackupType: "incremental",
		StartedAt:  time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2026, 7, 10, 12, 5, 0, 0, time.UTC),
	}
	subject, body := buildEmailContent(b, result, "бэкап", "DESKTOP-TEST")
	if !strings.Contains(subject, "[DESKTOP-TEST]") {
		t.Fatalf("subject %q missing hostname", subject)
	}
	if !strings.Contains(body, "Компьютер: DESKTOP-TEST") {
		t.Fatalf("body %q missing host line", body)
	}
}
