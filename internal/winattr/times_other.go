//go:build !windows

package winattr

func captureFileTimes(path string, entry *Entry) {}

func applyFileTimes(path string, e Entry) error {
	return nil
}
