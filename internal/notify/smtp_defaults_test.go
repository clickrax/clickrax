package notify

import (
	"testing"

	"pbs-win-backup/internal/models"
)

func TestEnableNotifyWhenSMTP(t *testing.T) {
	s := models.AppSettings{
		NotifyBackup: NotifyOff,
		SMTP: models.SMTPSettings{
			Host: "smtp.example.com",
			From: "a@example.com",
			To:   "b@example.com",
		},
	}
	if !EnableNotifyWhenSMTP(&s) {
		t.Fatal("expected notify to be enabled")
	}
	if s.NotifyBackup != NotifyAlways {
		t.Fatalf("notify = %q, want always", s.NotifyBackup)
	}
	if EnableNotifyWhenSMTP(&s) {
		t.Fatal("second call should not change already-enabled notify")
	}
}
