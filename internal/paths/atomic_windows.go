//go:build windows

package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite replaces path atomically. Sensitive files should call RestrictSensitiveACL after write.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := durableWriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := renameFile(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s %s: %w", tmp, path, err)
	}
	return syncDirAfterRename(filepath.Dir(path))
}
