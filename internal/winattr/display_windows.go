//go:build windows

package winattr

import (
	"strings"

	"golang.org/x/sys/windows"
)

func lookupAccountName(sid string) string {
	sid = strings.TrimSpace(sid)
	if sid == "" || !strings.HasPrefix(strings.ToUpper(sid), "S-") {
		return ""
	}
	sidObj, err := windows.StringToSid(sid)
	if err != nil {
		return ""
	}
	account, domain, _, err := sidObj.LookupAccount("")
	if err != nil || account == "" {
		return ""
	}
	if domain != "" {
		return domain + `\` + account
	}
	return account
}
