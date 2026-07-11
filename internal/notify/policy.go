package notify

import "pbs-win-backup/internal/models"

const (
	NotifyInherit = "inherit"
	NotifyOff     = "off"
	NotifyAlways  = "always"
	NotifyFailure = "failure"
)

func NormalizeNotifyMode(mode string) string {
	switch mode {
	case NotifyAlways, NotifyFailure:
		return mode
	default:
		return NotifyOff
	}
}

func NormalizeJobNotifyMode(mode string) string {
	switch mode {
	case NotifyInherit, "":
		return NotifyInherit
	case NotifyAlways, NotifyFailure, NotifyOff:
		return mode
	default:
		return NotifyInherit
	}
}

func EffectiveNotifyMode(jobMode, globalMode string) string {
	switch NormalizeJobNotifyMode(jobMode) {
	case NotifyInherit:
		return NormalizeNotifyMode(globalMode)
	default:
		return NormalizeJobNotifyMode(jobMode)
	}
}

func ShouldNotify(mode string, status string) bool {
	mode = NormalizeNotifyMode(mode)
	if mode == NotifyOff {
		return false
	}
	if mode == NotifyAlways {
		return true
	}
	switch status {
	case "error", "warning", "cancelled":
		return true
	default:
		return false
	}
}

func SMTPConfigured(s models.SMTPSettings) bool {
	return s.Host != "" && s.From != "" && s.To != ""
}
