//go:build !windows

package winutil

import (
	"fmt"
	"os"
	"strings"
)

func MachineGUID() (string, error) {
	return "", fmt.Errorf("MachineGuid unavailable on this platform")
}

func Hostname() string {
	if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
		return strings.TrimSpace(h)
	}
	return "host"
}
