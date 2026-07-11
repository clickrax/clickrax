//go:build !windows

package locale

func SystemPreferred() string {
	return English
}
