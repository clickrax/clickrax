//go:build !windows

package winattr

func lookupAccountName(sid string) string {
	return ""
}
