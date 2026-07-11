//go:build windows

package credential

import (
	"os"
	"strings"
)

func runningAsService() bool {
	for _, a := range os.Args[1:] {
		if a == "--service" || a == "--service-debug" {
			return true
		}
	}
	return false
}

func runningAsSystem() bool {
	user := strings.TrimSpace(os.Getenv("USERNAME"))
	return strings.EqualFold(user, "SYSTEM")
}

func preferServiceSecrets() bool {
	return runningAsService() || runningAsSystem()
}
