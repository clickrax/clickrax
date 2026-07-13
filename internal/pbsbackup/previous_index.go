package pbsbackup

import "strings"

// previousIndexUnavailable reports whether PBS has no usable previous index
// (first backup, or orphaned snapshot after cancel/crash).
func previousIndexUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, k := range []string{
		"no valid previous",
		"no previous",
		"previous http 404",
		"http 404",
	} {
		if strings.Contains(msg, k) {
			return true
		}
	}
	return false
}
