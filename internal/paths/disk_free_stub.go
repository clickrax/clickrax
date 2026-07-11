//go:build !windows

package paths

import "fmt"

func FreeBytes(path string) (uint64, error) {
	return 0, fmt.Errorf("FreeBytes: unsupported on this platform")
}
