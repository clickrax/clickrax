package notify

import "pbs-win-backup/internal/models"

// EnableNotifyWhenSMTP turns on backup e-mail notifications when SMTP is
// configured but notifications were left at the default "off".
func EnableNotifyWhenSMTP(s *models.AppSettings) bool {
	if s == nil || !SMTPConfigured(s.SMTP) {
		return false
	}
	if NormalizeNotifyMode(s.NotifyBackup) != NotifyOff {
		return false
	}
	s.NotifyBackup = NotifyAlways
	return true
}
