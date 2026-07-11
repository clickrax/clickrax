package backuproot

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/winutil"
)

func NormalizeSourcePath(p string) string {
	p = strings.TrimSpace(p)
	if len(p) == 2 && p[1] == ':' {
		return strings.ToUpper(p[:1]) + `:\`
	}
	return p
}

func Resolve(sources []string) (string, func(), error) {
	cleaned := make([]string, 0, len(sources))
	for _, s := range sources {
		if p := NormalizeSourcePath(s); p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) == 0 {
		return "", func() {}, i18nconfig.FromConfig().E("backup.no_sources")
	}
	if len(cleaned) == 1 {
		return cleaned[0], func() {}, nil
	}

	root := filepath.Join(os.TempDir(), "pbs-win-backup-"+randomSuffix())
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }

	used := map[string]int{}
	for _, src := range cleaned {
		name := sourceLabel(src)
		if n := used[name]; n > 0 {
			name = fmt.Sprintf("%s_%d", name, n+1)
		}
		used[name]++
		link := filepath.Join(root, name)
		if err := createJunction(link, src); err != nil {
			cleanup()
			return "", func() {}, i18nconfig.FromConfig().Ewrap("backup.junction_failed", map[string]string{"path": src}, err)
		}
	}
	return root, cleanup, nil
}

func sourceLabel(path string) string {
	path = strings.TrimRight(path, `\`)
	if len(path) == 2 && path[1] == ':' {
		return strings.ToUpper(string(path[0])) + "_drive"
	}
	base := filepath.Base(path)
	if base == "" || base == "." {
		base = strings.NewReplacer(":", "", `\`, "_", "/", "_").Replace(path)
	}
	return base
}

func createJunction(link, target string) error {
	if _, err := os.Stat(link); err == nil {
		return nil
	}
	cmd := winutil.HiddenCommand("cmd", "/c", "mklink", "/J", link, target)
	return cmd.Run()
}

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
