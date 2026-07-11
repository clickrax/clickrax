//go:build !windows

package pbsbackup

import (
	"io"
	"os"

	"pbs-win-backup/internal/i18nconfig"
)

func withMmapView(f *os.File, size int64, fn func([]byte) error) error {
	if size == 0 {
		return fn(nil)
	}
	if size > int64(int(^uint(0)>>1)) {
		return i18nconfig.FromConfig().E("pbs.mmap.archive_memory")
	}
	data := make([]byte, int(size))
	if _, err := io.ReadFull(f, data); err != nil {
		return err
	}
	return fn(data)
}
