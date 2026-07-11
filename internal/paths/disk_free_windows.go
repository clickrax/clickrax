//go:build windows

package paths

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// FreeBytes reports available bytes on the volume containing path.
func FreeBytes(path string) (uint64, error) {
	root := filepath.VolumeName(path)
	if root == "" {
		root = `C:\`
	} else if len(root) == 2 && root[1] == ':' {
		root += `\`
	}
	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return 0, err
	}
	var free, total, avail uint64
	if err := windows.GetDiskFreeSpaceEx(rootPtr, &free, &total, &avail); err != nil {
		return 0, err
	}
	return avail, nil
}
