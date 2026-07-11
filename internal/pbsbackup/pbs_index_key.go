package pbsbackup

import "strings"

func normalizeIndexKey(key string) string {
	return strings.ToLower(catalogPath(normalizeRestorePath(key)))
}

func indexKeyFromPxarPath(pxarPath string) string {
	return normalizeIndexKey(pxarPath)
}
