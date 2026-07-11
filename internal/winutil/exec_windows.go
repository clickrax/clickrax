//go:build windows

package winutil

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func HiddenCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return cmd
}
