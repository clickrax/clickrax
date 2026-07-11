//go:build !windows

package winutil

func IsElevated() bool { return true }
