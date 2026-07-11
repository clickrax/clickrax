//go:build !windows

package winattr

import "fmt"

func Capture(path string) (Entry, error) {
	return Entry{}, fmt.Errorf("winattr: only supported on Windows")
}

func Apply(path string, e Entry) error {
	return fmt.Errorf("winattr: only supported on Windows")
}

func ACLHash(e Entry) string {
	return ""
}
