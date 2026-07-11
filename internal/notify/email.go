package notify

import (
	"fmt"
	"strings"

	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

func maybeSendEmail(settings models.AppSettings, password string, mode string, result models.JobRunResult, kind string) error {
	if !ShouldNotify(mode, result.Status) {
		return nil
	}
	b := i18n.New(settings.Language)
	smtpCfg := settings.SMTP
	if !SMTPConfigured(smtpCfg) {
		return b.E("notify.smtp_not_configured")
	}
	subject, body := buildEmailContent(b, result, kind, machineHostname())
	return sendSMTPMessage(
		smtpCfg.Host,
		smtpCfg.Port,
		smtpCfg.Username,
		password,
		smtpCfg.From,
		smtpCfg.To,
		subject,
		body,
		smtpCfg.InsecureTLS,
	)
}

func SendTestEmail(settings models.AppSettings, password string) error {
	b := i18n.New(settings.Language)
	smtpCfg := settings.SMTP
	if !SMTPConfigured(smtpCfg) {
		return b.E("notify.smtp_fields_required")
	}
	host := machineHostname()
	subject := b.Tf("notify.test_subject", map[string]string{"app": branding.Name, "host": host})
	body := b.Tf("notify.test_body", map[string]string{"app": branding.Name, "host": host})
	return sendSMTPMessage(
		smtpCfg.Host,
		smtpCfg.Port,
		smtpCfg.Username,
		password,
		smtpCfg.From,
		smtpCfg.To,
		subject,
		body,
		smtpCfg.InsecureTLS,
	)
}

func buildEmailContent(b *i18n.Bundle, result models.JobRunResult, kind, host string) (string, string) {
	if host == "" {
		host = machineHostname()
	}
	statusLabel := statusLabel(b, result.Status)
	subject := fmt.Sprintf("%s [%s]: %s — %s", branding.Name, host, statusLabel, result.JobName)

	lines := []string{
		b.Tf("notify.email_host", map[string]string{"host": host}),
		b.Tf("notify.email_operation", map[string]string{"type": kind}),
		b.Tf("notify.email_job", map[string]string{"name": result.JobName}),
		b.Tf("notify.email_status", map[string]string{"status": statusLabel}),
	}
	if result.BackupType != "" && result.BackupType != "restore" {
		lines = append(lines, b.Tf("notify.email_backup_type", map[string]string{"type": result.BackupType}))
	}
	if result.Snapshot != "" {
		lines = append(lines, b.Tf("notify.email_snapshot", map[string]string{"path": result.Snapshot}))
	}
	if !result.StartedAt.IsZero() {
		lines = append(lines, b.Tf("notify.email_started", map[string]string{"time": result.StartedAt.Format("02.01.2006 15:04:05")}))
	}
	if !result.FinishedAt.IsZero() {
		lines = append(lines, b.Tf("notify.email_finished", map[string]string{"time": result.FinishedAt.Format("02.01.2006 15:04:05")}))
	}
	if result.DurationSec > 0 {
		lines = append(lines, b.Tf("notify.email_duration", map[string]string{"duration": formatDuration(b, result.DurationSec)}))
	}
	if result.BytesTransferred > 0 || result.BytesReused > 0 {
		lines = append(lines, b.Tf("notify.email_transferred", map[string]string{"vol": formatBytes(b, result.BytesTransferred)}))
		lines = append(lines, b.Tf("notify.email_reused", map[string]string{"vol": formatBytes(b, result.BytesReused)}))
	}
	if result.FilesTotal > 0 {
		lines = append(lines, b.Tf("notify.email_files", map[string]string{"n": fmt.Sprintf("%d", result.FilesTotal)}))
	}
	if result.FilesSkipped > 0 {
		lines = append(lines, b.Tf("notify.email_files_skipped", map[string]string{"n": fmt.Sprintf("%d", result.FilesSkipped)}))
	}
	if result.Message != "" {
		lines = append(lines, b.Tf("notify.email_message", map[string]string{"message": result.Message}))
	}
	if result.Error != "" && result.Error != result.Message {
		lines = append(lines, b.Tf("notify.email_error", map[string]string{"err": result.Error}))
	}
	return subject, strings.Join(lines, "\r\n")
}

func statusLabel(b *i18n.Bundle, status string) string {
	switch status {
	case "ok":
		return b.T("notify.status_ok")
	case "warning":
		return b.T("notify.status_warning")
	case "error":
		return b.T("notify.status_error")
	case "cancelled":
		return b.T("notify.status_cancelled")
	default:
		if status == "" {
			return b.T("notify.status_unknown")
		}
		return status
	}
}

func formatDuration(b *i18n.Bundle, sec int64) string {
	if sec < 60 {
		return b.Tf("notify.duration_sec", map[string]string{"n": fmt.Sprintf("%d", sec)})
	}
	m := sec / 60
	s := sec % 60
	if m < 60 {
		return b.Tf("notify.duration_min_sec", map[string]string{"m": fmt.Sprintf("%d", m), "s": fmt.Sprintf("%d", s)})
	}
	h := m / 60
	m = m % 60
	return b.Tf("notify.duration_hour_min", map[string]string{"h": fmt.Sprintf("%d", h), "m": fmt.Sprintf("%d", m)})
}

func formatBytes(b *i18n.Bundle, n int64) string {
	if n <= 0 {
		return b.T("notify.bytes_zero")
	}
	const unit = 1024
	if n < unit {
		return b.Tf("notify.bytes_b", map[string]string{"n": fmt.Sprintf("%d", n)})
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	val := float64(n) / float64(div)
	suffixKeys := []string{"notify.bytes_kb", "notify.bytes_mb", "notify.bytes_gb", "notify.bytes_tb"}
	if exp >= len(suffixKeys) {
		exp = len(suffixKeys) - 1
	}
	return b.Tf(suffixKeys[exp], map[string]string{"n": fmt.Sprintf("%.1f", val)})
}
