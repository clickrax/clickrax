//go:build !windows

package pbscommon

import "os"

func openBackupFile(path string) (*os.File, error) {
	return os.Open(path)
}
