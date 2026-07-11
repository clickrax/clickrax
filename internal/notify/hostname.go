package notify

import (
	"os"
	"strings"
)

func machineHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "windows-host"
	}
	h = strings.TrimSpace(h)
	if h == "" {
		return "windows-host"
	}
	return h
}
