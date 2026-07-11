//go:build !windows

package winutil

import "os/exec"

func HiddenCommand(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
