package models

import (
	"net/url"
	"strings"
)

// Localizer provides localized user-facing strings (e.g. *i18n.Bundle).
type Localizer interface {
	T(key string) string
	E(key string) error
	Ef(key string, vars map[string]string) error
}

// ValidatePBSURL requires HTTPS for PBS API endpoints.
func ValidatePBSURL(l Localizer, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return l.E("pbs_url.empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return l.Ef("pbs_url.invalid", map[string]string{"err": err.Error()})
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return l.E("pbs_url.need_https")
	}
	if u.Host == "" {
		return l.E("pbs_url.no_host")
	}
	return nil
}
