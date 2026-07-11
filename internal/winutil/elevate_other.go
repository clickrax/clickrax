//go:build !windows

package winutil

import "errors"

func RunElevated(_, _ string) (uint32, error) {
	return 1, errors.New("повышение прав доступно только в Windows")
}
