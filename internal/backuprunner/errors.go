package backuprunner

import (
	"strings"

	"pbs-win-backup/internal/i18n"
)

// FriendlyError maps common backup failures to localized messages.
func FriendlyError(b *i18n.Bundle, err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "authentication error") || strings.Contains(lower, "authentication") || strings.Contains(lower, "авторизац") {
		return b.T("err.auth")
	}
	if strings.Contains(lower, "pbs backup upgrade") || strings.Contains(lower, "no valid previous") {
		return b.T("err.pbs_incomplete_snapshot")
	}
	return msg
}

// ShortenErr trims verbose HTTP errors for UI display.
func ShortenErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if idx := strings.Index(msg, "Get \""); idx >= 0 {
		if end := strings.Index(msg[idx:], "\": "); end > 0 {
			return msg[idx+end+3:]
		}
	}
	if len(msg) > 120 {
		return msg[:120] + "…"
	}
	return msg
}
