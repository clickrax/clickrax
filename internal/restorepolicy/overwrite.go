package restorepolicy

import (
	"fmt"
	"os"
	"time"

	"pbs-win-backup/internal/i18nconfig"
)

// PrepareExistingDest handles an existing destination file before restore write.
func PrepareExistingDest(dest, mode string, forceOverwrite bool) error {
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return nil
	}
	if forceOverwrite || mode == "overwrite" {
		return nil
	}
	switch mode {
	case "backup":
		bak := dest + ".bak-" + time.Now().Format("20060102-150405")
		if err := os.Rename(dest, bak); err != nil {
			return i18nconfig.FromConfig().Ewrap("restore.overwrite_backup_failed", map[string]string{"path": dest}, err)
		}
		return nil
	case "ask":
		return fmt.Errorf("file_exists:%s", dest)
	default:
		return i18nconfig.FromConfig().Ef("restore.overwrite_unknown_mode", map[string]string{"mode": mode})
	}
}
