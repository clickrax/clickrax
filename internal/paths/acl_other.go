//go:build !windows

package paths

import "os"

func GrantUsersModify(path string) error {
	return nil
}

func RestrictSensitiveACL(path string) error {
	return nil
}

func AtomicWriteSensitive(path string, data []byte, perm os.FileMode) error {
	return AtomicWrite(path, data, perm)
}

func EnsureSharedDataAccess() {}
