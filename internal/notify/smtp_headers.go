package notify

import (
	"fmt"
	"mime"
	"net/mail"
	"strings"

	"pbs-win-backup/internal/i18nconfig"
)

func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func encodeSubject(subject string) string {
	subject = sanitizeHeader(subject)
	if subject == "" {
		return ""
	}
	return mime.QEncoding.Encode("utf-8", subject)
}

func envelopeAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", i18nconfig.FromConfig().E("smtp.empty_address")
	}
	parsed, err := mail.ParseAddress(addr)
	if err != nil {
		if strings.Contains(addr, "@") && !strings.ContainsAny(addr, " \t\r\n<>") {
			return addr, nil
		}
		return "", i18nconfig.FromConfig().Ewrap("smtp.invalid_address", map[string]string{"path": addr}, err)
	}
	return parsed.Address, nil
}

func headerAddr(addr string) string {
	return sanitizeHeader(addr)
}

func buildMessage(from string, recipients []string, subject, body string) (string, error) {
	fromHdr := headerAddr(from)
	toHdr := sanitizeHeader(strings.Join(recipients, ", "))
	subjHdr := encodeSubject(subject)
	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		fromHdr, toHdr, subjHdr,
	)
	return headers + body, nil
}
