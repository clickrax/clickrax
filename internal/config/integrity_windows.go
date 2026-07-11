//go:build windows

package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pbs-win-backup/internal/paths"
	"pbs-win-backup/internal/winutil"
)

const integritySalt = "ClickRAX-config-integrity-v1"

func configSigPath(configPath string) string {
	return configPath + ".hmac"
}

func configHMACKey() ([]byte, error) {
	guid, err := winutil.MachineGUID()
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(integritySalt + ":" + strings.TrimSpace(guid)))
	return sum[:], nil
}

func signConfigBytes(data []byte) (string, error) {
	key, err := configHMACKey()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyConfigBytes(data []byte, sigPath string) error {
	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	expected, err := signConfigBytes(data)
	if err != nil {
		return err
	}
	got := strings.TrimSpace(string(sigData))
	if got != expected {
		return fmt.Errorf("config integrity check failed (file may have been modified)")
	}
	return nil
}

func writeConfigSignature(configPath string, data []byte) error {
	sig, err := signConfigBytes(data)
	if err != nil {
		return err
	}
	sigPath := configSigPath(configPath)
	tmp := sigPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(sig), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, sigPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = paths.RestrictSensitiveACL(sigPath)
	return paths.SyncDir(filepath.Dir(sigPath))
}

func ensureConfigSignature(configPath string, data []byte) error {
	sigPath := configSigPath(configPath)
	if _, err := os.Stat(sigPath); err == nil {
		return verifyConfigBytes(data, sigPath)
	}
	return writeConfigSignature(configPath, data)
}

// IntegrityFingerprint returns a short diagnostic hash of the signing key source (not secret).
func IntegrityFingerprint() string {
	key, err := configHMACKey()
	if err != nil {
		return ""
	}
	return hex.EncodeToString(key[:8])
}
