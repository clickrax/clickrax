package backup

import (
	"strings"
	"unicode"
)

// IsVolumeRoot reports Windows drive roots like C:\ or D:\.
func IsVolumeRoot(path string) bool {
	path = strings.TrimSpace(path)
	if len(path) == 2 && path[1] == ':' && unicode.IsLetter(rune(path[0])) {
		return true
	}
	if len(path) == 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') && unicode.IsLetter(rune(path[0])) {
		return true
	}
	return false
}
