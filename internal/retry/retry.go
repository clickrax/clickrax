package retry

import (
	"strings"
	"time"
)

func Backoff(attempt int, base time.Duration) time.Duration {
	if attempt <= 0 {
		return base
	}
	if attempt > 30 {
		attempt = 30
	}
	d := base
	for i := 0; i < attempt; i++ {
		d *= 2
	}
	if d > 2*time.Minute {
		return 2 * time.Minute
	}
	return d
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	nonRetryable := []string{
		"authentication error",
		"authentication",
		"unauthorized",
		"forbidden",
		"pbs backup upgrade",
		"fingerprint",
		"secret",
		"passphrase",
		"не указаны",
		"not found",
		"invalid",
		"access denied",
	}
	for _, k := range nonRetryable {
		if strings.Contains(s, k) {
			return false
		}
	}
	keywords := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"network",
		"temporary",
		"eof",
		"broken pipe",
		"tls handshake",
		"i/o timeout",
		"no such host",
	}
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
