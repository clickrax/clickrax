package pbsbackup

import (
	"path/filepath"
	"strings"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/pathsafe"
)

// ResolveOriginalDest maps a catalog-relative path to an on-disk path using backup job sources.
func ResolveOriginalDest(catalogPath string, sources []string) (string, error) {
	norm := strings.ReplaceAll(catalogPath, `/`, `\`)
	if isAbsoluteWindowsPath(norm) {
		for _, src := range sources {
			root := normalizeSourceRoot(src)
			if root != "" && pathsafe.IsUnderRoot(norm, root) {
				return norm, nil
			}
		}
		return "", i18n.E("restore.path_outside", map[string]string{"path": catalogPath})
	}
	root := normalizeSourceRoot(pickSourceRoot(norm, sources))
	if root == "" {
		return "", i18n.E("restore.path_root", map[string]string{"path": catalogPath})
	}
	rel := strings.TrimLeft(norm, `\`)
	return pathsafe.JoinUnderRoot(root, rel)
}

// ResolveUnderDest joins a catalog path under destRoot with traversal checks.
func ResolveUnderDest(filePath, destRoot string) (string, error) {
	if destRoot == "" {
		return "", i18n.E("restore.dest_empty", nil)
	}
	return pathsafe.JoinUnderRoot(destRoot, filePath)
}

func normalizeSourceRoot(src string) string {
	s := strings.TrimRight(strings.ReplaceAll(src, `/`, `\`), `\`)
	if len(s) == 2 && s[1] == ':' {
		return s + `\`
	}
	return s
}

func isAbsoluteWindowsPath(p string) bool {
	if len(p) < 3 {
		return false
	}
	return p[1] == ':' && (p[2] == '\\' || p[2] == '/')
}

func pickSourceRoot(catalogPath string, sources []string) string {
	if len(sources) == 0 {
		return ""
	}
	if len(sources) == 1 {
		return normalizeSourceRoot(sources[0])
	}
	lower := strings.ToLower(catalogPath)
	var best string
	for _, src := range sources {
		s := strings.TrimRight(strings.ReplaceAll(src, `/`, `\`), `\`)
		if s == "" {
			continue
		}
		base := filepath.Base(s)
		if base != "" && base != "." && strings.HasPrefix(lower, strings.ToLower(base)+`\`) {
			if len(s) > len(best) {
				best = s
			}
		}
	}
	if best != "" {
		return normalizeSourceRoot(best)
	}
	return normalizeSourceRoot(sources[0])
}
