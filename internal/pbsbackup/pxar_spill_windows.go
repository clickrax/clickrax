//go:build windows

package pbsbackup

import (
	"fmt"
	"os"
	"unsafe"

	"pbs-win-backup/internal/i18nconfig"

	"golang.org/x/sys/windows"
)

func withMmapView(f *os.File, size int64, fn func([]byte) error) error {
	if size == 0 {
		return fn(nil)
	}
	if size < 0 {
		return i18nconfig.FromConfig().Ef("pbs.mmap.negative_size", map[string]string{"n": fmt.Sprintf("%d", size)})
	}
	n := int(size)
	if int64(n) != size {
		return i18nconfig.FromConfig().E("pbs.mmap.archive_too_large")
	}

	handle := windows.Handle(f.Fd())
	low := uint32(n)
	high := uint32(uint64(n) >> 32)
	mapping, err := windows.CreateFileMapping(handle, nil, windows.PAGE_READONLY, high, low, nil)
	if err != nil {
		return fmt.Errorf("mmap CreateFileMapping: %w", err)
	}
	defer windows.CloseHandle(mapping)

	addr, err := windows.MapViewOfFile(mapping, windows.FILE_MAP_READ, 0, 0, uintptr(n))
	if err != nil {
		return fmt.Errorf("mmap MapViewOfFile: %w", err)
	}
	defer windows.UnmapViewOfFile(addr)

	view := unsafe.Slice((*byte)(unsafe.Pointer(addr)), n)
	return fn(view)
}
