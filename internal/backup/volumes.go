package backup

import (
	"os"
	"strings"
)

// ListVolumes returns available Windows drive roots (e.g. C:\, D:\).
func ListVolumes() []string {
	vols := make([]string, 0, 8)
	for i := 'A'; i <= 'Z'; i++ {
		root := string(i) + `:\`
		if _, err := os.Stat(root); err == nil {
			vols = append(vols, root)
		}
	}
	return vols
}

// NormalizeSourcePath ensures trailing backslash for volume roots.
func NormalizeSourcePath(p string) string {
	p = strings.TrimSpace(p)
	if len(p) == 2 && p[1] == ':' {
		return strings.ToUpper(p[:1]) + `:\`
	}
	return p
}
