//go:build !windows

package datalock

func With(name string, fn func() error) error {
	return fn()
}
