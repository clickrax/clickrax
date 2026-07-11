//go:build windows

package winutil

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// MachineGUID reads the Windows MachineGuid used for config integrity signing.
func MachineGUID() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("open MachineGuid key: %w", err)
	}
	defer k.Close()
	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return "", fmt.Errorf("read MachineGuid: %w", err)
	}
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return "", fmt.Errorf("MachineGuid is empty")
	}
	return guid, nil
}

// Hostname returns the local computer name.
func Hostname() string {
	if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
		return strings.TrimSpace(h)
	}
	return "windows-host"
}
