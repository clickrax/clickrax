//go:build !windows

package credential

func preferServiceSecrets() bool { return false }

func MigratePassphrases([]string) {}
