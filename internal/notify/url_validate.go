package notify

import (
	"net"
	"net/url"
	"strings"

	"pbs-win-backup/internal/i18nconfig"
)

func ValidateWebhookURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	b := i18nconfig.FromConfig()
	u, err := url.Parse(raw)
	if err != nil {
		return b.Ef("webhook.invalid_url", map[string]string{"err": err.Error()})
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
	default:
		return b.E("webhook.need_scheme")
	}
	if u.Host == "" {
		return b.E("webhook.no_host")
	}
	host := u.Hostname()
	if host == "" {
		return b.E("webhook.no_host")
	}
	if isBlockedWebhookHost(host) {
		return b.E("webhook.blocked_host")
	}
	return validateWebhookResolvedHost(host)
}

func validateWebhookResolvedHost(host string) error {
	_, err := resolveWebhookIPs(host)
	return err
}

func isBlockedWebhookHost(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return isBlockedWebhookIP(ip)
}

func isBlockedWebhookIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
