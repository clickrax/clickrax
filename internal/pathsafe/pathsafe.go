package pathsafe

import (
	"path/filepath"
	"strings"

	"pbs-win-backup/internal/i18n"
)

// SafeRelativePath rejects absolute paths, drive letters, and .. components.
func SafeRelativePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, `/`, `\`)
	p = strings.TrimLeft(p, `\`)
	if p == "" {
		return "", i18n.E("path.empty", nil)
	}
	if strings.Contains(p, ":") {
		return "", i18n.E("path.absolute_forbidden", nil)
	}
	for _, part := range strings.Split(p, `\`) {
		if part == ".." {
			return "", i18n.E("path.unsafe", nil)
		}
	}
	return p, nil
}

// JoinUnderRoot joins rel under root and verifies the result stays inside root.
func JoinUnderRoot(root, rel string) (string, error) {
	if root == "" {
		return "", i18n.E("path.root_empty", nil)
	}
	safe, err := SafeRelativePath(rel)
	if err != nil {
		return "", err
	}
	dest := filepath.Clean(filepath.Join(root, safe))
	cleanRoot := filepath.Clean(root)
	prefix := cleanRoot
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	lowerDest := strings.ToLower(dest) + string(filepath.Separator)
	lowerPrefix := strings.ToLower(prefix)
	if dest != cleanRoot && !strings.HasPrefix(lowerDest, lowerPrefix) {
		return "", i18n.E("path.outside_dest", nil)
	}
	return dest, nil
}

// IsUnderRoot reports whether path is equal to or under root.
func IsUnderRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(strings.ToLower(path)+string(filepath.Separator), strings.ToLower(prefix))
}
