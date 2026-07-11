//go:build !windows

package config

func ensureConfigSignature(string, []byte) error { return nil }
func writeConfigSignature(string, []byte) error { return nil }
func verifyConfigBytes([]byte, string) error    { return nil }

// IntegrityFingerprint is unavailable off Windows.
func IntegrityFingerprint() string { return "" }
