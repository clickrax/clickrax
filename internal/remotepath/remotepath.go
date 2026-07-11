package remotepath

import (
	"path"
	"strings"

	"pbs-win-backup/internal/i18nconfig"
)

// SafeComponent validates a single remote path segment (file name or directory part).
func SafeComponent(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", i18nconfig.FromConfig().E("remotepath.empty")
	}
	norm := strings.ReplaceAll(name, `\`, `/`)
	base := path.Base(norm)
	if base == "." || base == ".." || base != strings.Trim(norm, `/`) {
		return "", i18nconfig.FromConfig().Ef("remotepath.invalid", map[string]string{"name": name})
	}
	if strings.ContainsAny(base, `/\:`) {
		return "", i18nconfig.FromConfig().Ef("remotepath.invalid", map[string]string{"name": name})
	}
	return base, nil
}

// SafeRemotePath validates and normalizes a slash-separated remote relative path.
func SafeRemotePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, `/\`)
	if p == "" {
		return "", nil
	}
	p = strings.ReplaceAll(p, `\`, `/`)
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		c, err := SafeComponent(part)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	return strings.Join(out, "/"), nil
}

// JoinDir joins validated remote path components with "/".
func JoinDir(dir, fileName string) (string, error) {
	fileName, err := SafeComponent(fileName)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(dir) == "" {
		return fileName, nil
	}
	safeDir, err := SafeRemotePath(dir)
	if err != nil {
		return "", err
	}
	if safeDir == "" {
		return fileName, nil
	}
	return safeDir + "/" + fileName, nil
}

// RemoteDir builds the backup destination directory from base path and backup ID.
func RemoteDir(basePath, backupID string) (string, error) {
	base, err := SafeRemotePath(basePath)
	if err != nil {
		return "", err
	}
	bid := strings.TrimSpace(backupID)
	if bid == "" {
		return base, nil
	}
	safeID, err := SafeComponent(bid)
	if err != nil {
		return "", err
	}
	if base == "" {
		return safeID, nil
	}
	return base + "/" + safeID, nil
}
