//go:build !windows

package backuplock

type Lock struct{}

func ClearStale() bool { return true }

func IsHeld() bool { return false }

func ForceClearOwn() bool { return false }

func ClearStaleFileLock(path string) bool { return true }

func ForceClearFileLock(path string) bool { return true }

func Acquire() (*Lock, error) { return &Lock{}, nil }

func (l *Lock) Release() {}
